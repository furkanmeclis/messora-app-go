package queue

import (
	"context"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// TenantHeader is the task header naming the tenant a task belongs to. The
// outbox relay sets it from the outbox row (outbox.TaskPublisher).
const TenantHeader = "messora-tenant"

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

// TaskContext returns ctx scoped to the task's tenant. Tasks without one
// are the scheduler's periodic jobs: until they fan out per tenant they run
// as tenant #1.
//
// TODO(WP3): the scheduler enqueues every periodic job per tenant (with the
// header), and a task without a tenant is refused instead.
func TaskContext(ctx context.Context, t *asynq.Task) context.Context {
	if id, ok := TaskTenant(t); ok {
		return tenant.WithID(ctx, id)
	}
	return tenant.WithID(ctx, tenant.DefaultID)
}

// TenantMiddleware runs every handler as its task's tenant (TaskContext).
func TenantMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		return next.ProcessTask(TaskContext(ctx, t), t)
	})
}
