// Package queue wraps asynq so that the rest of the code base depends on a
// small, stable surface: queue names, task routing, retry policy and
// instrumentation.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	Default   = "default"
	Bulk      = "bulk"
)

// AllQueues lists every queue the application uses.
var AllQueues = []string{Critical, Campaigns, Default, Bulk}

// Task types handled in Phase 1.
const (
	TypeOutboxCleanup   = "maintenance:outbox_cleanup"
	TypeIdentityCleanup = "maintenance:identity_cleanup"
)

// routes maps task types to queues; unknown types go to Default.
var routes = map[string]string{
	TypeOutboxCleanup:   Default,
	TypeIdentityCleanup: Default,
	// WhatsApp (constants live in the messaging module).
	"whatsapp:send":           Critical,
	"whatsapp:webhook":        Critical,
	"whatsapp:media_download": Default,
	"contacts:import":         Bulk,
	"campaign:materialize":    Campaigns,
	"campaign:dispatch":       Campaigns,
	"campaign:start_due":      Default,
	"campaign:reaper":         Default,
}

// Route returns the queue a task type is routed to.
func Route(taskType string) string {
	if q, ok := routes[taskType]; ok {
		return q
	}
	return Default
}

// NewClient returns an asynq client sharing the given Redis connection.
func NewClient(rdb goredis.UniversalClient) *asynq.Client {
	return asynq.NewClientFromRedisClient(rdb)
}

// NewInspector returns an asynq inspector sharing the given Redis connection.
func NewInspector(rdb goredis.UniversalClient) *asynq.Inspector {
	return asynq.NewInspectorFromRedisClient(rdb)
}

// Enqueue submits a task routed to its queue. A duplicate TaskID is reported
// as ErrDuplicate so callers can treat it as success.
func Enqueue(ctx context.Context, c *asynq.Client, taskType string, payload []byte, opts ...asynq.Option) error {
	opts = append([]asynq.Option{asynq.Queue(Route(taskType))}, opts...)
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
		Concurrency:     o.Concurrency,
		Queues:          o.Queues,
		RetryDelayFunc:  RetryDelay,
		IsFailure:       IsFailure,
		ShutdownTimeout: o.ShutdownTimeout,
		Logger:          slogAdapter{log},
		LogLevel:        asynq.WarnLevel,
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
