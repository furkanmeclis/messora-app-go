package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/metrics"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tracing"
)

// RunAPI serves the public REST API.
func RunAPI(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleAPI)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openAppPool(ctx, cfg, config.RoleAPI, log)
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
	svc, err := newServices(cfg, pool, rdb, log)
	if err != nil {
		return reportFatal(log, err)
	}

	reg := metrics.NewRegistry(Version, string(config.RoleAPI))
	reg.MustRegister(metrics.NewPoolCollector(pool), metrics.NewRedisUpGauge(rdb))
	httpMetrics := httpx.NewHTTPMetrics(reg)

	ready := health.NewChecker(2*time.Second).
		Add("postgres", health.Postgres(pool)).
		Add("schema", health.Schema(pool)).
		Add("redis", health.Redis(rdb)).
		Add("storage", svc.stores.Check)
	sys := newSystemHandler(pool, rdb, insp, cf, log)

	r := chi.NewRouter()
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)
	// AccessLog must wrap Recover so panicking requests are logged and counted as 500s.
	r.Use(httpx.RequestID, httpx.AccessLog(log, httpMetrics), httpx.Recover(log), httpx.IdentifyBFF(cfg.BFFSharedSecret.Reveal()))
	r.Get("/healthz", health.LiveHandler)
	r.Get("/readyz", ready.ReadyHandler)
	if err := mountAPI(r, svc, pool, sys, rdb, log); err != nil {
		return reportFatal(log, err)
	}

	// Start-up work of tenant #1: the WhatsApp account from META_*
	// settings (other companies connect theirs in the panel).
	first := tenant.WithID(ctx, tenant.DefaultID)
	if err := svc.channels.EnsureFromEnv(first, cfg.Meta.WABAID, cfg.Meta.AccessToken.Reveal(), cfg.Meta.AppID, cfg.Meta.AppSecret.Reveal()); err != nil {
		// Not fatal: the account can be connected from the panel.
		log.Error("could not connect the WhatsApp account from META_* settings", "err", err)
	}

	internal := internalServer(cfg.InternalAddr, reg, ready)
	// /status on the internal port checks the whole stack (worker, scheduler,
	// Centrifugo); used by operators and the end-to-end smoke test.
	internal.Handler.(chi.Router).Get("/status", sys.ReadyHandler)

	g, gctx := errgroup.WithContext(ctx)
	var handler http.Handler = r
	if tracing.Enabled() {
		// One server span per request; the API guard names it after the
		// route template (e.g. "GET /api/v1/contacts/{contactId}").
		handler = otelhttp.NewHandler(r, "http.request")
	}
	g.Go(func() error {
		return serve(gctx, newHTTPServer(cfg.HTTPAddr, handler), cfg.ShutdownTimeout, log, "api")
	})
	g.Go(func() error { return serve(gctx, internal, cfg.ShutdownTimeout, log, "internal") })
	if err := g.Wait(); err != nil {
		return reportFatal(log, err)
	}
	flushTracing()
	log.Info("stopped")
	return nil
}
