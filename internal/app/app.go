// Package app is the composition root: one Run function per process role.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/logger"
	"github.com/furkanmeclis/messora-app-go/internal/platform/metrics"
	platformredis "github.com/furkanmeclis/messora-app-go/internal/platform/redis"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tracing"
)

// Version is set at build time with -ldflags "-X …/internal/app.Version=…".
var Version = "dev"

// dependencyTimeout bounds how long a process waits for PostgreSQL and Redis
// at start-up before giving up (the container is then restarted).
const dependencyTimeout = 60 * time.Second

func bootstrap(role config.Role) (*config.Config, *slog.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("configuration: %w", err)
	}
	log := logger.New(os.Stdout, logger.Options{
		Level: cfg.LogLevel, Format: cfg.LogFormat, Service: cfg.ServiceName, Role: string(role),
	})
	if err := cfg.Validate(role); err != nil {
		return nil, log, fmt.Errorf("configuration: %w", err)
	}
	if tracing.Enabled() {
		shutdown, err := tracing.Setup(context.Background(), cfg.ServiceName, string(role), Version)
		if err != nil {
			return nil, log, fmt.Errorf("tracing: %w", err)
		}
		shutdownTracing = shutdown
		log = slog.New(tracing.LogHandler{Handler: log.Handler()})
		log.Info("tracing enabled")
	}
	log.Info("starting", "version", Version, "env", cfg.Env)
	return cfg, log, nil
}

// shutdownTracing flushes spans; set by bootstrap when tracing is on.
var shutdownTracing = func(context.Context) error { return nil }

// flushTracing exports pending spans before the process exits.
func flushTracing() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = shutdownTracing(ctx)
}

func openPostgres(ctx context.Context, cfg *config.Config, role config.Role, log *slog.Logger) (*pgxpool.Pool, error) {
	return database.Open(ctx, database.PoolOptions{
		DSN:             cfg.Database.DSN(),
		MaxConns:        cfg.Database.MaxConns,
		MinConns:        cfg.Database.MinConns,
		ApplicationName: "messora-" + string(role),
		Traced:          tracing.Enabled(),
	}, dependencyTimeout, log)
}

func openRedis(ctx context.Context, cfg *config.Config, log *slog.Logger) (*goredis.Client, error) {
	return platformredis.Open(ctx, cfg.Redis, dependencyTimeout, log)
}

// internalServer serves /metrics, /healthz and /readyz on INTERNAL_ADDR. It
// is never routed by the edge proxy.
func internalServer(addr string, reg *prometheus.Registry, ready *health.Checker) *http.Server {
	r := chi.NewRouter()
	r.Get("/healthz", health.LiveHandler)
	r.Get("/readyz", ready.ReadyHandler)
	r.Handle("/metrics", metrics.Handler(reg))
	return newHTTPServer(addr, r)
}

func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// serve runs srv until ctx is cancelled, then shuts it down gracefully.
func serve(ctx context.Context, srv *http.Server, shutdown time.Duration, log *slog.Logger, name string) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("http listening", "server", name, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("%s server: %w", name, err)
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), shutdown)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("%s shutdown: %w", name, err)
	}
	log.Info("http stopped", "server", name)
	return nil
}
