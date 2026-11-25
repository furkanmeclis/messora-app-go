// Package queue wraps asynq so that the rest of the code base depends on a
// small, stable surface: queue names, task routing, retry policy and
// instrumentation.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"time"

	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"
)

// Queue names. Priorities are configured with WORKER_QUEUES.
const (
	Critical  = "critical"
	Campaigns = "campaigns"
	AI        = "ai"
	Default   = "default"
	Bulk      = "bulk"
)

// AllQueues lists every queue the application uses.
var AllQueues = []string{Critical, Campaigns, AI, Default, Bulk}

// Task types handled in Phase 1.
const (
	TypeOutboxCleanup   = "maintenance:outbox_cleanup"
	TypeIdentityCleanup = "maintenance:identity_cleanup"
)

// Policy is where a task type runs and how often a failing task is retried
// before asynq archives it (the dead letter, see docs/ARCHITECTURE.md §11).
type Policy struct {
	Queue    string
	MaxRetry int
}

// DefaultMaxRetry applies to task types without an explicit policy.
const DefaultMaxRetry = 10

// periodicMaxRetry: scheduled jobs run again at their next slot anyway, so
// a failing run is retried only briefly and then left visible as failed.
const periodicMaxRetry = 2

// policies maps task types to their queue and retry budget; unknown types go
// to Default with DefaultMaxRetry. Deliberate deferrals (RetryAfterError) do
// not consume the budget. Keep the table in docs/ARCHITECTURE.md §11 in sync.
var policies = map[string]Policy{
	TypeOutboxCleanup:   {Default, periodicMaxRetry},
	TypeIdentityCleanup: {Default, periodicMaxRetry},
	// WhatsApp (constants live in the messaging module). A send's attempts
	// against Meta are capped by the messaging module (8) and retried as
	// deferrals; this budget only covers infrastructure errors around them.
	"whatsapp:send":           {Critical, 10},
	"whatsapp:webhook":        {Critical, 20},
	"whatsapp:media_download": {Default, 8},
	"contacts:import":         {Bulk, 3},
	"campaign:materialize":    {Campaigns, 10},
	"campaign:dispatch":       {Campaigns, 10},
	"campaign:start_due":      {Default, periodicMaxRetry},
	"campaign:reaper":         {Default, periodicMaxRetry},
	"offers:tick":             {Default, periodicMaxRetry},
	"automation:evaluate":     {Critical, 10},
	"automation:run":          {Default, 10},
	"automation:resume":       {Default, 10},
	"automation:reaper":       {Default, periodicMaxRetry},
	"webhook:fanout":          {Default, 10},
	"webhook:deliver":         {Default, 10},
	"webhook:cleanup":         {Default, periodicMaxRetry},
	"channels:token_check":    {Default, periodicMaxRetry},
	"license:check_in":        {Default, periodicMaxRetry},
}

// campaignAware task types move to the Campaigns queue when their payload
// names a campaign, so bulk sends never delay inbox, API and automation
// replies waiting in Critical.
var campaignAware = map[string]bool{"whatsapp:send": true}

// PolicyFor returns the policy of a task type.
func PolicyFor(taskType string) Policy {
	if p, ok := policies[taskType]; ok {
		return p
	}
	return Policy{Queue: Default, MaxRetry: DefaultMaxRetry}
}

// Policies returns a copy of every explicit policy.
func Policies() map[string]Policy { return maps.Clone(policies) }

// Route returns the queue a task type is routed to by default.
func Route(taskType string) string { return PolicyFor(taskType).Queue }

// RouteTask returns the queue for a concrete task: campaign-aware types
// whose JSON payload carries a non-empty "campaign_id" go to Campaigns.
func RouteTask(taskType string, payload []byte) string {
	if campaignAware[taskType] {
		var p struct {
			CampaignID string `json:"campaign_id"`
		}
		if json.Unmarshal(payload, &p) == nil && p.CampaignID != "" {
			return Campaigns
		}
	}
	return Route(taskType)
}

// NewClient returns an asynq client sharing the given Redis connection.
func NewClient(rdb goredis.UniversalClient) *asynq.Client {
	return asynq.NewClientFromRedisClient(rdb)
}

// NewInspector returns an asynq inspector sharing the given Redis connection.
func NewInspector(rdb goredis.UniversalClient) *asynq.Inspector {
	return asynq.NewInspectorFromRedisClient(rdb)
}

// Enqueue submits a task routed to its queue (RouteTask) with its retry
// budget (PolicyFor); later options override both. A duplicate TaskID is
// reported as ErrDuplicate so callers can treat it as success.
func Enqueue(ctx context.Context, c *asynq.Client, taskType string, payload []byte, opts ...asynq.Option) error {
	opts = append([]asynq.Option{asynq.Queue(RouteTask(taskType, payload)), asynq.MaxRetry(PolicyFor(taskType).MaxRetry)}, opts...)
	_, err := c.EnqueueContext(ctx, asynq.NewTask(taskType, payload), opts...)
	if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
		return ErrDuplicate
	}
	return err
}

// ErrDuplicate means an identical task (same TaskID or uniqueness key) is
// already queued or retained; the enqueue is idempotently satisfied.
var ErrDuplicate = errors.New("queue: duplicate task")

// RetryDelay is exponential backoff with ±20% jitter: 2s, 4s, 8s … capped at
// 10 minutes. Errors wrapping a *RetryAfterError use the requested delay.
func RetryDelay(n int, err error, _ *asynq.Task) time.Duration {
	var ra *RetryAfterError
	if errors.As(err, &ra) && ra.After > 0 {
		return ra.After
	}
	return backoff(n)
}

func backoff(n int) time.Duration {
	const base, maxDelay = 2 * time.Second, 10 * time.Minute
	if n < 0 {
		n = 0
	}
	d := base
	for i := 0; i < n && d < maxDelay; i++ {
		d *= 2
	}
	if d > maxDelay {
		d = maxDelay
	}
	jitter := 0.8 + rand.Float64()*0.4 //nolint:gosec // jitter, not security
	return time.Duration(float64(d) * jitter)
}

// RetryAfterError asks for a retry after a specific delay without counting as
// a failure (e.g. rate limited). See IsFailure.
type RetryAfterError struct {
	After  time.Duration
	Reason string
}

func (e *RetryAfterError) Error() string {
	return fmt.Sprintf("retry after %s: %s", e.After, e.Reason)
}

// IsFailure tells asynq whether an error should count against the retry
// budget and failure metrics. Deliberate deferrals do not.
func IsFailure(err error) bool {
	var ra *RetryAfterError
	return !errors.As(err, &ra)
}

// Metrics holds task instrumentation.
type Metrics struct {
	processed *prometheus.CounterVec
	duration  *prometheus.HistogramVec
}

// NewMetrics registers task metrics on reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		processed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messora_tasks_processed_total",
			Help: "Tasks processed by type and outcome (success, failure, deferred).",
		}, []string{"task_type", "outcome"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "messora_task_duration_seconds",
			Help:    "Task handler duration.",
			Buckets: []float64{.005, .01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"task_type"}),
	}
	reg.MustRegister(m.processed, m.duration)
	return m
}

// Middleware records outcome and duration of every task.
func (m *Metrics) Middleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		start := time.Now()
		err := next.ProcessTask(ctx, t)
		m.duration.WithLabelValues(t.Type()).Observe(time.Since(start).Seconds())
		outcome := "success"
		switch {
		case err == nil:
		case !IsFailure(err):
			outcome = "deferred"
		default:
			outcome = "failure"
		}
		m.processed.WithLabelValues(t.Type(), outcome).Inc()
		return err
	})
}

// ServerOptions configures NewServer.
type ServerOptions struct {
	Concurrency     int
	Queues          map[string]int
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// NewServer builds an asynq server with Messora's retry policy and logging.
func NewServer(rdb goredis.UniversalClient, o ServerOptions) *asynq.Server {
	log := o.Logger
	return asynq.NewServerFromRedisClient(rdb, asynq.Config{
		Concurrency:    o.Concurrency,
		Queues:         o.Queues,
		RetryDelayFunc: RetryDelay,
		// Paced sends come back after sub-second delays (send scheduler);
		// asynq's default 5 s forwarding would leave capacity unused.
		DelayedTaskCheckInterval: time.Second,
		IsFailure:                IsFailure,
		ShutdownTimeout:          o.ShutdownTimeout,
		Logger:                   slogAdapter{log},
		LogLevel:                 asynq.WarnLevel,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			id, _ := asynq.GetTaskID(ctx)
			level := slog.LevelWarn
			if retried >= maxRetry {
				level = slog.LevelError // final attempt: task is archived (dead letter)
			}
			log.Log(ctx, level, "task failed", "task_type", t.Type(), "job_id", id, "retry", retried, "max_retry", maxRetry, "err", err)
		}),
	})
}

// slogAdapter satisfies asynq.Logger.
type slogAdapter struct{ l *slog.Logger }

func (a slogAdapter) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...), "component", "asynq") }
func (a slogAdapter) Info(args ...any)  { a.l.Info(fmt.Sprint(args...), "component", "asynq") }
func (a slogAdapter) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...), "component", "asynq") }
func (a slogAdapter) Error(args ...any) { a.l.Error(fmt.Sprint(args...), "component", "asynq") }
func (a slogAdapter) Fatal(args ...any) {
	a.l.Error(fmt.Sprint(args...), "component", "asynq", "fatal", true)
}

// StatsCollector exports queue depth from Redis at scrape time.
type StatsCollector struct {
	insp   *asynq.Inspector
	queues []string
	size   *prometheus.Desc
	lat    *prometheus.Desc
}

// NewStatsCollector creates a collector for the given queues.
func NewStatsCollector(insp *asynq.Inspector, queues []string) *StatsCollector {
	return &StatsCollector{
		insp:   insp,
		queues: queues,
		size:   prometheus.NewDesc("messora_queue_tasks", "Tasks in a queue by state.", []string{"queue", "state"}, nil),
		lat:    prometheus.NewDesc("messora_queue_latency_seconds", "Age of the oldest pending task.", []string{"queue"}, nil),
	}
}

// Describe implements prometheus.Collector.
func (c *StatsCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.size; ch <- c.lat }

// Collect implements prometheus.Collector.
func (c *StatsCollector) Collect(ch chan<- prometheus.Metric) {
	for _, q := range c.queues {
		info, err := c.insp.GetQueueInfo(q)
		if err != nil {
			continue // queue not created yet (no task ever enqueued) or Redis down
		}
		for state, n := range map[string]int{
			"pending": info.Pending, "active": info.Active, "scheduled": info.Scheduled,
			"retry": info.Retry, "archived": info.Archived,
		} {
			ch <- prometheus.MustNewConstMetric(c.size, prometheus.GaugeValue, float64(n), q, state)
		}
		ch <- prometheus.MustNewConstMetric(c.lat, prometheus.GaugeValue, info.Latency.Seconds(), q)
	}
}
