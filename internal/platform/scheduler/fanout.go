package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
)

// FanOut is the payload of a queue.TypeTenantFanOut task: one slot of a
// per-tenant periodic job.
type FanOut struct {
	Job      string        `json:"job"`
	TaskType string        `json:"task_type"`
	Slot     int64         `json:"slot"`
	Every    time.Duration `json:"every"`
	Payload  []byte        `json:"payload,omitempty"`
}

func (f FanOut) encode() []byte {
	b, _ := json.Marshal(f) // plain fields: cannot fail
	return b
}

// TenantSlotTaskID is the deterministic task ID of one tenant's run of a
// job's slot, so a retried fan-out never enqueues a tenant twice.
func TenantSlotTaskID(job string, slotUnix int64, tenantID uuid.UUID) string {
	return fmt.Sprintf("sched:%s:%d:%s", job, slotUnix, tenantID)
}

// TenantLister returns the tenants periodic jobs run for: the active ones
// (tenant.ListActive on the system pool).
type TenantLister func(ctx context.Context) ([]uuid.UUID, error)

// FanOutHandler handles queue.TypeTenantFanOut: it enqueues the slot's task
// once for every active tenant, with the tenant header. Suspended and
// deleted tenants get nothing. A failed enqueue fails the fan-out, whose
// retry skips the tenants already enqueued (TenantSlotTaskID).
func FanOutHandler(tenants TenantLister, client *asynq.Client, log *slog.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var f FanOut
		if err := json.Unmarshal(t.Payload(), &f); err != nil || f.Job == "" || f.TaskType == "" || f.Every < time.Second {
			return fmt.Errorf("%w: bad fan-out payload", asynq.SkipRetry)
		}
		if queue.IsPlatformTask(f.TaskType) {
			return fmt.Errorf("%w: %s is a platform task", asynq.SkipRetry, f.TaskType)
		}
		ids, err := tenants(ctx)
		if err != nil {
			return fmt.Errorf("list tenants: %w", err)
		}
		var errs []error
		enqueued := 0
		for _, id := range ids {
			err := queue.EnqueueFor(ctx, client, id, f.TaskType, f.Payload,
				asynq.TaskID(TenantSlotTaskID(f.Job, f.Slot, id)),
				asynq.Retention(2*f.Every),
				asynq.Timeout(f.Every),
			)
			switch {
			case err == nil:
				enqueued++
			case errors.Is(err, queue.ErrDuplicate):
			default:
				errs = append(errs, fmt.Errorf("tenant %s: %w", id, err))
			}
		}
		if len(errs) > 0 {
			log.WarnContext(ctx, "periodic job fan-out incomplete; retrying", "job", f.Job, "tenants", len(ids), "failed", len(errs))
			return errors.Join(errs...)
		}
		log.DebugContext(ctx, "periodic job fanned out", "job", f.Job, "tenants", len(ids), "enqueued", enqueued)
		return nil
	}
}
