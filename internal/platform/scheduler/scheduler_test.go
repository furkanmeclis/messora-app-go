package scheduler

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
)

// A job's platform flag must match its task type: platform tasks run once
// without a tenant, every other job once per tenant.
func TestPlatformJobsMustBePlatformTasks(t *testing.T) {
	for _, j := range []Job{
		{Name: "a", Every: time.Minute, TaskType: queue.TypeOutboxCleanup},              // platform task run per tenant
		{Name: "b", Every: time.Minute, TaskType: "campaign:start_due", Platform: true}, // tenant task run without one
		{Name: "c", Every: time.Minute, TaskType: queue.TypeTenantFanOut},               // the fan-out itself
	} {
		if _, err := New(nil, nil, []Job{j}, Options{}); err == nil {
			t.Errorf("job %+v accepted", j)
		}
	}
	if _, err := New(nil, nil, []Job{{Name: "ok", Every: time.Minute, TaskType: queue.TypeOutboxCleanup, Platform: true},
		{Name: "ok2", Every: time.Minute, TaskType: "campaign:start_due"}}, Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestTenantSlotTaskIDs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	if TenantSlotTaskID("j", 60, a) == TenantSlotTaskID("j", 60, b) || TenantSlotTaskID("j", 60, a) == TenantSlotTaskID("j", 120, a) {
		t.Fatal("task IDs must differ per tenant and slot")
	}
	if TenantSlotTaskID("j", 60, a) == SlotTaskID("j", 60) {
		t.Fatal("a tenant's task must not reuse the fan-out's ID")
	}
}
