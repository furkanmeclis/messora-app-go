package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
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
	cf, err := centrifugo.New(cfg.Centrifugo)
	if err != nil {
		return reportFatal(log, err)
	}
	router.Handle(realtime.TopicPrefix, realtime.Publisher(cf))
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

	svc.messaging.SetThrottle(messaging.NewThrottle(rdb, 80))
	registerMessaging(mux, svc.messaging)
	registerCampaigns(mux, svc.campaigns, svc.messaging, log)
	mux.HandleFunc(contacts.TypeImport, func(ctx context.Context, t *asynq.Task) error {
		var p struct {
			ImportID uuid.UUID `json:"import_id"`
		}
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return svc.contacts.Run(ctx, p.ImportID)
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

// registerMessaging wires the WhatsApp task handlers.
func registerMessaging(mux *asynq.ServeMux, m *messaging.Service) {
	idFrom := func(t *asynq.Task, key string) (uuid.UUID, error) {
		var p map[string]string
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return uuid.Nil, fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		id, err := uuid.Parse(p[key])
		if err != nil {
			return uuid.Nil, fmt.Errorf("%w: bad %s", asynq.SkipRetry, key)
		}
		return id, nil
	}
	retry := func(err error) error {
		var r *messaging.ErrRetry
		if errors.As(err, &r) {
			return &queue.RetryAfterError{After: r.After, Reason: r.Reason}
		}
		return err
	}
	mux.HandleFunc(messaging.TypeSend, func(ctx context.Context, t *asynq.Task) error {
		id, err := idFrom(t, "message_id")
		if err != nil {
			return err
		}
		return retry(m.Send(ctx, id))
	})
	mux.HandleFunc(messaging.TypeWebhook, func(ctx context.Context, t *asynq.Task) error {
		id, err := idFrom(t, "event_id")
		if err != nil {
			return err
		}
		n, _ := asynq.GetRetryCount(ctx)
		maxN, _ := asynq.GetMaxRetry(ctx)
		return retry(m.ProcessWebhook(ctx, id, n, maxN+1))
	})
	mux.HandleFunc(messaging.TypeMediaDownload, func(ctx context.Context, t *asynq.Task) error {
		id, err := idFrom(t, "media_id")
		if err != nil {
			return err
		}
		return m.DownloadMedia(ctx, id)
	})
}

// registerCampaigns wires the campaign engine's task handlers.
func registerCampaigns(mux *asynq.ServeMux, c *campaigns.Service, m *messaging.Service, log *slog.Logger) {
	var p struct {
		CampaignID uuid.UUID `json:"campaign_id"`
		Gen        int32     `json:"gen"`
	}
	decode := func(t *asynq.Task) error {
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return nil
	}
	mux.HandleFunc(campaigns.TypeMaterialize, func(ctx context.Context, t *asynq.Task) error {
		if err := decode(t); err != nil {
			return err
		}
		return c.Materialize(ctx, p.CampaignID)
	})
	mux.HandleFunc(campaigns.TypeDispatch, func(ctx context.Context, t *asynq.Task) error {
		if err := decode(t); err != nil {
			return err
		}
		return c.Dispatch(ctx, p.CampaignID, p.Gen)
	})
	mux.HandleFunc(campaigns.TypeStartDue, func(ctx context.Context, _ *asynq.Task) error {
		n, err := c.StartDue(ctx)
		if n > 0 {
			log.InfoContext(ctx, "scheduled campaigns started", "count", n)
		}
		return err
	})
	mux.HandleFunc(campaigns.TypeReaper, func(ctx context.Context, _ *asynq.Task) error {
		n, err := c.Reap(ctx, m.Send)
		if n > 0 {
			log.WarnContext(ctx, "campaign reaper repaired work left by a stopped worker", "items", n)
		}
		return err
	})
}
