//go:build integration

package scheduler_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/scheduler"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// A per-tenant periodic job: the scheduler enqueues one fan-out per slot;
// the fan-out (listing tenants on the system pool) enqueues the job once
// for every active tenant, with the tenant header. Suspended and deleted
// tenants get nothing, and a repeated fan-out adds nothing.
func TestPeriodicJobsFanOutPerActiveTenant(t *testing.T) {
	ctx := context.Background()
	tdb := testutil.TenantDB(t)
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()
	insp := queue.NewInspector(rdb)
	defer insp.Close()

	newTenant := func(slug, status string) uuid.UUID {
		var id uuid.UUID
		if err := tdb.Owner.QueryRow(ctx, `INSERT INTO tenants (slug, name, status) VALUES ($1, $1, $2) RETURNING id`, slug, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	beta := newTenant("beta", tenant.StatusActive)
	newTenant("gamma", tenant.StatusSuspended)
	newTenant("delta", tenant.StatusDeleted)
	active, err := tenant.ListActive(ctx, tdb.System)
	if err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{tenant.DefaultID, beta}
	slices.SortFunc(want, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if !slices.Equal(active, want) {
		t.Fatalf("active tenants %v, want %v", active, want)
	}

	// The scheduler enqueues the slot as a fan-out (no tenant), not the job.
	jobs := []scheduler.Job{{Name: "campaigns_due", Every: time.Minute, TaskType: "campaign:start_due"}}
	s, err := scheduler.New(rdb, client, jobs, scheduler.Options{LeaseTTL: time.Second, Tick: 50 * time.Millisecond, Logger: testutil.DiscardLogger()})
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithTimeout(ctx, 700*time.Millisecond)
	_ = s.Run(runCtx)
	cancel()
	fanouts, err := insp.ListPendingTasks(queue.Route(queue.TypeTenantFanOut))
	if err != nil || len(fanouts) != 1 || fanouts[0].Type != queue.TypeTenantFanOut || len(fanouts[0].Headers) != 0 {
		t.Fatalf("fan-outs %v %v", fanouts, err)
	}
	var f scheduler.FanOut
	if err := json.Unmarshal(fanouts[0].Payload, &f); err != nil || f.TaskType != "campaign:start_due" || f.Job != "campaigns_due" {
		t.Fatalf("fan-out payload %s %v", fanouts[0].Payload, err)
	}

	// Run it (twice: a retried fan-out must not enqueue anyone again).
	h := scheduler.FanOutHandler(func(ctx context.Context) ([]uuid.UUID, error) { return tenant.ListActive(ctx, tdb.System) },
		client, testutil.DiscardLogger())
	task := asynq.NewTask(fanouts[0].Type, fanouts[0].Payload)
	for range 2 {
		if err := queue.TenantMiddleware(h).ProcessTask(ctx, task); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := insp.ListPendingTasks(queue.Route("campaign:start_due"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]string{}
	for _, ti := range tasks {
		if ti.Type != "campaign:start_due" {
			continue
		}
		id, err := uuid.Parse(ti.Headers[queue.TenantHeader])
		if err != nil {
			t.Fatalf("task %s without a tenant header: %v", ti.ID, ti.Headers)
		}
		if _, dup := got[id]; dup {
			t.Fatalf("tenant %s enqueued twice", id)
		}
		got[id] = ti.ID
	}
	if len(got) != 2 || got[tenant.DefaultID] == "" || got[beta] == "" {
		t.Fatalf("per-tenant tasks %v, want one for tenant #1 and one for beta", got)
	}
	if got[beta] != scheduler.TenantSlotTaskID("campaigns_due", f.Slot, beta) {
		t.Fatalf("task id %s", got[beta])
	}

	// A fan-out of a platform task type is refused.
	bad, _ := json.Marshal(scheduler.FanOut{Job: "x", TaskType: queue.TypeOutboxCleanup, Slot: 1, Every: time.Minute})
	if err := h.ProcessTask(ctx, asynq.NewTask(queue.TypeTenantFanOut, bad)); err == nil {
		t.Fatal("a platform task was fanned out per tenant")
	}
}
