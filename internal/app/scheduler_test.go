package app

import (
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/scheduler"
)

// The schedule is valid, and only platform tasks run without a tenant.
func TestPeriodicJobs(t *testing.T) {
	jobs := PeriodicJobs()
	if _, err := scheduler.New(nil, nil, jobs, scheduler.Options{}); err != nil {
		t.Fatal(err)
	}
	perTenant := 0
	for _, j := range jobs {
		if j.Platform != queue.IsPlatformTask(j.TaskType) {
			t.Errorf("%s: platform %v for task %s", j.Name, j.Platform, j.TaskType)
		}
		if !j.Platform {
			perTenant++
		}
	}
	if perTenant < len(jobs)-1 {
		t.Fatalf("%d of %d jobs run per tenant; only the outbox cleanup is a platform job", perTenant, len(jobs))
	}
}
