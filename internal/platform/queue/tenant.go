package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// TenantHeader is the task header naming the tenant a task belongs to. The
// outbox relay sets it from the outbox row (outbox.TaskPublisher) and the
// scheduler's fan-out sets it on every periodic task (TypeTenantFanOut).
const TenantHeader = "messora-tenant"

// TypeTenantFanOut is the scheduler's platform task that turns one slot of
// a periodic job into one task per active tenant (scheduler.FanOutHandler).
const TypeTenantFanOut = "platform:tenant_fanout"

// platformTasks may run without a tenant header. They run as tenant.System
// and touch only platform tables or, on the system pool, do cross-tenant
// maintenance of infrastructure tables (the outbox). Every other task
// without a tenant is refused (TenantMiddleware). Keep the list short.
var platformTasks = map[string]bool{
	TypeTenantFanOut:  true,
	TypeOutboxCleanup: true,
}

// IsPlatformTask reports whether taskType may run without a tenant.
func IsPlatformTask(taskType string) bool { return platformTasks[taskType] }

// PlatformTasks returns the task types allowed to run without a tenant.
func PlatformTasks() []string {
	out := make([]string, 0, len(platformTasks))
	for t := range platformTasks {
		out = append(out, t)
	}
	return out
}

// EnqueueFor is Enqueue for a task of one tenant: the tenant travels in the
// task header and the worker runs the handler as that tenant.
func EnqueueFor(ctx context.Context, c *asynq.Client, tenantID uuid.UUID, taskType string, payload []byte, opts ...asynq.Option) error {
	var headers map[string]string
	if tenantID != uuid.Nil {
		headers = map[string]string{TenantHeader: tenantID.String()}
	}
	return enqueue(ctx, c, asynq.NewTaskWithHeaders(taskType, payload, headers), opts...)
}

// TaskTenant returns the tenant named by a task's header.
func TaskTenant(t *asynq.Task) (uuid.UUID, bool) {
	id, err := uuid.Parse(t.Headers()[TenantHeader])
	return id, err == nil && id != uuid.Nil
}

// TaskContext returns ctx scoped to the task's tenant, or marked as system
// work (tenant.System) for a task without one. Only TenantMiddleware
// decides whether a tenant-less task may run at all.
func TaskContext(ctx context.Context, t *asynq.Task) context.Context {
	if id, ok := TaskTenant(t); ok {
		return tenant.WithID(ctx, id)
	}
	return tenant.System(ctx)
}

// ErrNoTaskTenant is returned (wrapped in asynq.SkipRetry, so the task goes
// straight to the dead letter) for a tenant task that carries no tenant.
var ErrNoTaskTenant = fmt.Errorf("queue: task without a tenant")

// TenantMiddleware runs every handler as its task's tenant. A task without
// a tenant header runs only when its type is a platform task
// (IsPlatformTask), as tenant.System; any other is refused and archived:
// running it as some default tenant would mix tenants' data.
func TenantMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		if id, ok := TaskTenant(t); ok {
			return next.ProcessTask(tenant.WithID(ctx, id), t)
		}
		if IsPlatformTask(t.Type()) {
			return next.ProcessTask(tenant.System(ctx), t)
		}
		return fmt.Errorf("%w: %w: %s", asynq.SkipRetry, ErrNoTaskTenant, t.Type())
	})
}
