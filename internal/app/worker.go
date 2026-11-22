package app

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"
	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/metrics"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
)

// outboxRetention is how long published outbox rows are kept for debugging.
const outboxRetention = 7 * 24 * time.Hour

// RunWorker processes queued tasks and relays the outbox.
func RunWorker(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleWorker)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openPostgres(ctx, cfg, config.RoleWorker, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	rdb, err := openRedis(ctx, cfg, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer rdb.Close()

	reg := metrics.NewRegistry(Version, string(config.RoleWorker))
	insp := queue.NewInspector(rdb)
	defer insp.Close()
	reg.MustRegister(metrics.NewPoolCollector(pool), metrics.NewRedisUpGauge(rdb), queue.NewStatsCollector(insp, queue.AllQueues))
	taskMetrics := queue.NewMetrics(reg)

	client := queue.NewClient(rdb)
	defer client.Close()

	router := outbox.NewRouter()
	router.Handle(outbox.TaskTopicPrefix, outbox.TaskPublisher(client))
	relay := outbox.NewRelay(pool, router, outbox.RelayOptions{Logger: log.With("component", "outbox"), Registerer: reg})

	svc, err := newServices(cfg, pool, rdb, log)
	if err != nil {
		return reportFatal(log, err)
	}

	mux := asynq.NewServeMux()
	mux.Use(taskMetrics.Middleware)
	mux.Handle(queue.TypeOutboxCleanup, outbox.CleanupHandler(pool, outboxRetention, log))
	mux.HandleFunc(queue.TypeIdentityCleanup, func(ctx context.Context, _ *asynq.Task) error {
		sessions, keys, err := svc.identity.Cleanup(ctx)
		if err == nil {
			log.InfoContext(ctx, "identity cleanup finished", "sessions_deleted", sessions, "idempotency_keys_deleted", keys)
		}
		return err
	})

	srv := queue.NewServer(rdb, queue.ServerOptions{
		Concurrency: cfg.Worker.Concurrency, Queues: cfg.Worker.Queues,
		ShutdownTimeout: cfg.ShutdownTimeout, Logger: log.With("component", "asynq"),
	})

	ready := health.NewChecker(2*time.Second).
		Add("postgres", health.Postgres(pool)).
		Add("schema", health.Schema(pool)).
		Add("redis", health.Redis(rdb))

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := srv.Start(mux); err != nil {
			return err
		}
		log.Info("worker processing", "queues", cfg.Worker.Queues, "concurrency", cfg.Worker.Concurrency)
		<-gctx.Done()
		// Shutdown stops pulling new tasks and waits up to ShutdownTimeout
		// for running handlers; unfinished tasks are returned to the queue.
		srv.Shutdown()
		return nil
	})
	g.Go(func() error { return relay.Run(gctx) })
	g.Go(func() error {
		return serve(gctx, internalServer(cfg.InternalAddr, reg, ready), cfg.ShutdownTimeout, log, "internal")
	})
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return reportFatal(log, err)
	}
	log.Info("stopped")
	return nil
}
