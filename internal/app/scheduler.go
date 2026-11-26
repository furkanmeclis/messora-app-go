package app

import (
	"context"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/licensing"

	"golang.org/x/sync/errgroup"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/offers"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/platform/health"
	"github.com/furkanmeclis/messora-app-go/internal/platform/metrics"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/scheduler"
)

// PeriodicJobs is the schedule. Later phases append campaign, reaper and
// template-sync jobs here.
func PeriodicJobs() []scheduler.Job {
	return []scheduler.Job{
		{Name: "outbox_cleanup", Every: time.Hour, TaskType: queue.TypeOutboxCleanup},
		{Name: "identity_cleanup", Every: time.Hour, TaskType: queue.TypeIdentityCleanup},
		{Name: "campaigns_due", Every: time.Minute, TaskType: campaigns.TypeStartDue},
		{Name: "campaign_reaper", Every: time.Minute, TaskType: campaigns.TypeReaper},
		{Name: "campaign_analytics_reconcile", Every: 5 * time.Minute, TaskType: campaigns.TypeAnalyticsReconcile},
		{Name: "offers_tick", Every: time.Minute, TaskType: offers.TypeTick},
		{Name: "automation_reaper", Every: time.Minute, TaskType: automation.TypeReaper},
		{Name: "webhook_cleanup", Every: 6 * time.Hour, TaskType: webhooks.TypeCleanup},
		{Name: "license_check_in", Every: 6 * time.Hour, TaskType: licensing.TypeCheckIn},
		{Name: "whatsapp_token_check", Every: 6 * time.Hour, TaskType: channels.TypeTokenCheck},
		{Name: "whatsapp_signal_sync", Every: time.Hour, TaskType: channels.TypeSignalSync},
		{Name: "meta_cost_reconcile", Every: 24 * time.Hour, TaskType: channels.TypeCostReconcile},
		{Name: "ai_model_sync", Every: 24 * time.Hour, TaskType: ai.TypeModelSync},
	}
}

// RunScheduler enqueues periodic tasks while holding the leader lease.
func RunScheduler(ctx context.Context) error {
	cfg, log, err := bootstrap(config.RoleScheduler)
	if err != nil {
		return reportFatal(log, err)
	}
	rdb, err := openRedis(ctx, cfg, log)
	if err != nil {
		return reportFatal(log, err)
	}
	defer rdb.Close()

	reg := metrics.NewRegistry(Version, string(config.RoleScheduler))
	reg.MustRegister(metrics.NewRedisUpGauge(rdb))
	client := queue.NewClient(rdb)
	defer client.Close()

	s, err := scheduler.New(rdb, client, PeriodicJobs(), scheduler.Options{Logger: log, Registerer: reg})
	if err != nil {
		return reportFatal(log, err)
	}
	ready := health.NewChecker(2*time.Second).Add("redis", health.Redis(rdb))

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.Run(gctx) })
	g.Go(func() error {
		return serve(gctx, internalServer(cfg.InternalAddr, reg, ready), cfg.ShutdownTimeout, log, "internal")
	})
	if err := g.Wait(); err != nil {
		return reportFatal(log, err)
	}
	flushTracing()
	log.Info("stopped")
	return nil
}
