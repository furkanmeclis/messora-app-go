package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/system"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/metrics"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
)

// RunAPI serves the public REST API.
func RunAPI(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleAPI)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openPostgres(ctx, cfg, config.RoleAPI, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	rdb, err := openRedis(ctx, cfg, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer rdb.Close()
	cf, err := centrifugo.New(cfg.Centrifugo)
	if err != nil {
		return reportFatal(log, err)
	}
	insp := queue.NewInspector(rdb)
	defer insp.Close()

	reg := metrics.NewRegistry(Version, string(config.RoleAPI))
	reg.MustRegister(metrics.NewPoolCollector(pool), metrics.NewRedisUpGauge(rdb))
	httpMetrics := httpx.NewHTTPMetrics(reg)

	ready := health.NewChecker(2*time.Second).
		Add("postgres", health.Postgres(pool)).
		Add("schema", health.Schema(pool)).
		Add("redis", health.Redis(rdb))

	r := chi.NewRouter()
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)
	// AccessLog must wrap Recover so panicking requests are logged and counted as 500s.
	r.Use(httpx.RequestID, httpx.AccessLog(log, httpMetrics), httpx.Recover(log), httpx.IdentifyBFF(cfg.BFFSharedSecret.Reveal()))
	r.Get("/healthz", health.LiveHandler)
	r.Get("/readyz", ready.ReadyHandler)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(httpx.MaxBody(1 << 20))
		r.With(httpx.RequireBFF).Method(http.MethodGet, "/system/status", system.NewHandler(Version, pool, rdb, insp, cf))
	})

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serve(gctx, newHTTPServer(cfg.HTTPAddr, r), cfg.ShutdownTimeout, log, "api") })
	g.Go(func() error {
		return serve(gctx, internalServer(cfg.InternalAddr, reg, ready), cfg.ShutdownTimeout, log, "internal")
	})
	if err := g.Wait(); err != nil {
		return reportFatal(log, err)
	}
	log.Info("stopped")
	return nil
}
