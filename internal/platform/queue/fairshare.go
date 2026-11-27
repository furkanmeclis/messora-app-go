package queue

import (
	"context"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
)

// TenantLimiter keeps one tenant from holding every handler slot of a worker
// process (docs/ARCHITECTURE.md §10.1). A task
// whose tenant already runs its share of the process's concurrency is not
// run: it is deferred (RetryAfterError, which costs no retry budget) for
// about a second and goes back to the queue, freeing the slot for the next
// task — usually another tenant's. Tenant A's ten-thousand-message campaign
// therefore delays tenant B's message by the time it takes to skip A's
// waiting tasks, not by the time it takes to send them.
//
// The limit is per process: each worker replica keeps at least
// (1 − share) of its slots for the other tenants. Counting in memory needs
// no Redis round trip per task and cannot leak a slot when a process dies.
// Platform tasks (no tenant) are not limited.
type TenantLimiter struct {
	concurrency int
	share       func(context.Context) float64
	delay       func() time.Duration

	mu      sync.Mutex
	running map[uuid.UUID]int

	deferred prometheus.Counter
}

// MinTenantShare and MaxTenantShare bound the configured share.
const (
	MinTenantShare = 0.05
	MaxTenantShare = 1.0
)

// NewTenantLimiter limits each tenant to share(ctx) of concurrency handler
// slots (at least one). share is read on every task, so a changed platform
// setting applies at once; nil means the whole process (no limit).
func NewTenantLimiter(concurrency int, share func(context.Context) float64) *TenantLimiter {
	return &TenantLimiter{
		concurrency: max(concurrency, 1), share: share, running: map[uuid.UUID]int{},
		// Spread the returns, so deferred tasks do not come back in one wave.
		delay: func() time.Duration { return 500*time.Millisecond + rand.N(time.Second) }, //nolint:gosec // jitter
		deferred: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "messora_tasks_tenant_deferred_total",
			Help: "Tasks deferred because their tenant already ran its share of a worker's concurrency.",
		}),
	}
}

// Register adds the limiter's metrics to reg.
func (l *TenantLimiter) Register(reg prometheus.Registerer) { reg.MustRegister(l.deferred) }

// Limit is the number of handler slots one tenant may hold.
func (l *TenantLimiter) Limit(ctx context.Context) int {
	s := MaxTenantShare
	if l.share != nil {
		s = l.share(ctx)
	}
	if math.IsNaN(s) || s <= 0 {
		s = MaxTenantShare
	}
	s = math.Min(math.Max(s, MinTenantShare), MaxTenantShare)
	return max(1, int(math.Ceil(float64(l.concurrency)*s)))
}

// Acquire takes a slot for tenant id unless it already holds limit of them.
// force takes it regardless (a task on its last attempt: deferring it would
// archive it, since asynq archives a task whose retries are spent whatever
// the error).
func (l *TenantLimiter) Acquire(id uuid.UUID, limit int, force bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !force && l.running[id] >= limit {
		return false
	}
	l.running[id]++
	return true
}

// Release returns a slot of tenant id.
func (l *TenantLimiter) Release(id uuid.UUID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running[id] <= 1 {
		delete(l.running, id)
		return
	}
	l.running[id]--
}

// Running returns how many tasks of tenant id run now.
func (l *TenantLimiter) Running(id uuid.UUID) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running[id]
}

// Middleware applies the limit to every task with a tenant.
func (l *TenantLimiter) Middleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		id, ok := TaskTenant(t)
		if !ok {
			return next.ProcessTask(ctx, t)
		}
		retried, _ := asynq.GetRetryCount(ctx)
		maxRetry, okMax := asynq.GetMaxRetry(ctx)
		last := okMax && retried >= maxRetry
		if !l.Acquire(id, l.Limit(ctx), last) {
			l.deferred.Inc()
			return &RetryAfterError{After: l.delay(), Reason: "tenant at its share of the worker"}
		}
		defer l.Release(id)
		return next.ProcessTask(ctx, t)
	})
}
