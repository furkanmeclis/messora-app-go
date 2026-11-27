package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/conversations"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/offers"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/metrics"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tracing"
)

// outboxRetention is how long published outbox rows are kept for debugging.
const outboxRetention = 7 * 24 * time.Hour

// RunWorker processes queued tasks and relays the outbox.
func RunWorker(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleWorker)
	if err != nil {
		return reportFatal(log, err)
	}
	pool, err := openAppPool(ctx, cfg, config.RoleWorker, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer pool.Close()
	// The outbox relay and the outbox cleanup cover every tenant.
	sysPool, err := openSystemPool(ctx, cfg, config.RoleWorker, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer sysPool.Close()
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
	relay := outbox.NewRelay(sysPool, router, outbox.RelayOptions{Logger: log.With("component", "outbox"), Registerer: reg})

	svc, err := newServices(cfg, pool, rdb, log)
	if err != nil {
		return reportFatal(log, err)
	}

	mux := asynq.NewServeMux()
	// Every handler runs as its task's tenant (queue.TenantMiddleware).
	mux.Use(taskMetrics.Middleware, tracing.TaskMiddleware, queue.TenantMiddleware)
	mux.Handle(queue.TypeOutboxCleanup, outbox.CleanupHandler(sysPool, outboxRetention, log))
	mux.HandleFunc(queue.TypeIdentityCleanup, func(ctx context.Context, _ *asynq.Task) error {
		sessions, keys, err := svc.identity.Cleanup(ctx)
		if err != nil {
			return err
		}
		// The platform admin panel's sessions belong to no tenant.
		platform, err := svc.identity.CleanupPlatform(ctx)
		if err == nil {
			log.InfoContext(ctx, "identity cleanup finished", "sessions_deleted", sessions, "idempotency_keys_deleted", keys,
				"platform_sessions_deleted", platform)
		}
		return err
	})

	// Each number's ceiling is the throughput Meta reports for it (80 when unknown).
	svc.messaging.SetThrottle(svc.throttle)
	registerMessaging(mux, svc.messaging)
	registerSignals(mux, svc.channels, svc.messaging, svc.campaigns, log)
	registerCampaigns(mux, svc.campaigns, svc.messaging, log)
	mux.HandleFunc(offers.TypeTick, func(ctx context.Context, _ *asynq.Task) error {
		n, err := svc.offers.Tick(ctx)
		if n > 0 {
			log.InfoContext(ctx, "offers changed status", "count", n)
		}
		return err
	})
	registerAutomations(mux, svc.automation, log)
	mux.HandleFunc(channels.TypeTokenCheck, func(ctx context.Context, _ *asynq.Task) error {
		// Problems are recorded on the accounts; a failed run is simply
		// repeated by the next one.
		if err := svc.channels.CheckTokens(ctx); err != nil {
			log.WarnContext(ctx, "whatsapp token check", "err", err)
		}
		return nil
	})
	registerWebhooks(mux, svc.webhooks, log)
	mux.HandleFunc(conversations.TypeAIReply, func(ctx context.Context, t *asynq.Task) error {
		var p conversations.AIReplyPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return svc.inbox.HandleAIReply(ctx, p.ConversationID, p.MessageID)
	})
	mux.HandleFunc(ai.TypeModelSync, func(ctx context.Context, _ *asynq.Task) error {
		// Per-provider failures are logged; the next daily run repeats them.
		return svc.ai.SyncAll(ctx)
	})
	mux.HandleFunc(ai.TypeClassifyReply, func(ctx context.Context, t *asynq.Task) error {
		var p struct {
			ClassificationID uuid.UUID `json:"classification_id"`
		}
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return svc.ai.RunClassification(ctx, p.ClassificationID)
	})
	mux.HandleFunc(ai.TypeAttributionReconcile, func(ctx context.Context, _ *asynq.Task) error {
		n, err := svc.ai.ReconcileAttribution(ctx)
		if n > 0 {
			log.InfoContext(ctx, "ai usage attributed to campaigns", "count", n)
		}
		return err
	})
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
		OnArchived: opsFailedJobHint(pool, log),
	})

	ready := health.NewChecker(2*time.Second).
		Add("postgres", health.Postgres(pool)).
		Add("schema", health.Schema(pool)).
		Add("redis", health.Redis(rdb)).
		Add("storage", svc.stores.Check)

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
	flushTracing()
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

// registerSignals wires Meta's account signals, cost reconciliation and the
// user-preference event.
func registerSignals(mux *asynq.ServeMux, ch *channels.Service, m *messaging.Service, camp *campaigns.Service, log *slog.Logger) {
	mux.HandleFunc(channels.TypeSignalSync, func(ctx context.Context, t *asynq.Task) error {
		var p struct {
			AccountID *uuid.UUID `json:"account_id"`
		}
		_ = json.Unmarshal(t.Payload(), &p) // empty payload (scheduler) = every account
		// Problems are logged; the next hourly run repeats the sync.
		if err := ch.SyncSignals(ctx, p.AccountID); err != nil {
			log.WarnContext(ctx, "whatsapp signal sync", "err", err)
		}
		// A number's quality may have dropped: campaign spam protection.
		if err := camp.ProtectActive(ctx); err != nil {
			log.WarnContext(ctx, "campaign protection after signal sync", "err", err)
		}
		return nil
	})
	mux.HandleFunc(channels.TypeCostReconcile, func(ctx context.Context, _ *asynq.Task) error {
		// Every run re-reads the last days, so a failed one is repaired by the next.
		if err := ch.ReconcileCosts(ctx, time.Now()); err != nil {
			log.WarnContext(ctx, "meta cost reconciliation", "err", err)
		}
		return nil
	})
	mux.HandleFunc(messaging.TypeUserPreference, func(ctx context.Context, t *asynq.Task) error {
		// Recorded in meta_user_preferences; here it becomes the contact's
		// "stopped marketing in WhatsApp" flag.
		var e messaging.UserPreferenceEvent
		if err := json.Unmarshal(t.Payload(), &e); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return m.ApplyUserPreference(ctx, e)
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
	mux.HandleFunc(campaigns.TypeAnalyticsReconcile, func(ctx context.Context, _ *asynq.Task) error {
		n, err := c.ReconcileCounters(ctx)
		if n > 0 {
			log.WarnContext(ctx, "campaign counters repaired", "campaigns", n)
		}
		return err
	})
	mux.HandleFunc(campaigns.TypeExport, func(ctx context.Context, t *asynq.Task) error {
		var e struct {
			ExportID uuid.UUID `json:"export_id"`
		}
		if err := json.Unmarshal(t.Payload(), &e); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return c.RunExport(ctx, e.ExportID)
	})
	mux.HandleFunc(campaigns.TypeReaper, func(ctx context.Context, _ *asynq.Task) error {
		n, err := c.Reap(ctx, m.Send)
		if n > 0 {
			log.WarnContext(ctx, "campaign reaper repaired work left by a stopped worker", "items", n)
		}
		return err
	})
}

// registerWebhooks wires outgoing webhook fan-out, delivery and retention.
func registerWebhooks(mux *asynq.ServeMux, w *webhooks.Service, log *slog.Logger) {
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
	mux.HandleFunc(webhooks.TypeFanOut, func(ctx context.Context, t *asynq.Task) error {
		id, err := idFrom(t, "event_id")
		if err != nil {
			return err
		}
		_, err = w.FanOut(ctx, id)
		return err
	})
	mux.HandleFunc(webhooks.TypeDeliver, func(ctx context.Context, t *asynq.Task) error {
		id, err := idFrom(t, "delivery_id")
		if err != nil {
			return err
		}
		return w.Deliver(ctx, id)
	})
	mux.HandleFunc(webhooks.TypeCleanup, func(ctx context.Context, _ *asynq.Task) error {
		n, err := w.Cleanup(ctx, webhooks.Retention)
		if n > 0 {
			log.InfoContext(ctx, "webhook cleanup finished", "events_deleted", n)
		}
		return err
	})
}

// registerAutomations wires the automation engine's task handlers.
func registerAutomations(mux *asynq.ServeMux, a *automation.Service, log *slog.Logger) {
	runID := func(t *asynq.Task) (uuid.UUID, error) {
		var p struct {
			RunID uuid.UUID `json:"run_id"`
		}
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return uuid.Nil, fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return p.RunID, nil
	}
	mux.HandleFunc(trigger.TypeEvaluate, func(ctx context.Context, t *asynq.Task) error {
		var e trigger.Event
		if err := json.Unmarshal(t.Payload(), &e); err != nil {
			return fmt.Errorf("%w: %w", asynq.SkipRetry, err)
		}
		return a.Evaluate(ctx, e)
	})
	run := func(ctx context.Context, t *asynq.Task) error {
		id, err := runID(t)
		if err != nil {
			return err
		}
		err = a.Run(ctx, id)
		if errors.Is(err, automation.ErrRetriesExhausted) {
			// The run is marked failed; archiving the task puts it on the
			// failed jobs page (dead-letter queue).
			return fmt.Errorf("%w: run %s: %w", asynq.SkipRetry, id, err)
		}
		return err
	}
	mux.HandleFunc(automation.TypeRun, run)
	mux.HandleFunc(automation.TypeResume, run)
	mux.HandleFunc(automation.TypeScan, func(ctx context.Context, _ *asynq.Task) error {
		n, err := a.Scan(ctx)
		if n > 0 {
			log.InfoContext(ctx, "time-based automation triggers fired", "runs", n)
		}
		return err
	})
	mux.HandleFunc(automation.TypeReaper, func(ctx context.Context, _ *asynq.Task) error {
		n, err := a.Reap(ctx)
		if n > 0 {
			log.WarnContext(ctx, "automation reaper restarted runs", "runs", n)
		}
		return err
	})
}

// opsFailedJobHint tells the operations center that a job reached the dead
// letter (its alerts count failed jobs), as an ops.updated hint.
func opsFailedJobHint(pool *pgxpool.Pool, log *slog.Logger) func(context.Context, string) {
	return func(ctx context.Context, taskType string) {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := realtime.PublishOps(ctx, pool, realtime.OpsSectionAlerts); err != nil {
			log.WarnContext(ctx, "could not announce a failed job", "task_type", taskType, "err", err)
		}
	}
}
