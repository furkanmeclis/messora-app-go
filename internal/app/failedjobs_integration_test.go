//go:build integration

package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/modules/system"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// failJob makes a real worker run a task of the harness's tenant that
// fails with no retries left, so asynq archives it with its error.
func (h *harness) failJob(taskType, payload, id string) {
	h.t.Helper()
	h.failJobOf(h.tenantID, taskType, payload, id)
}

// failJobOf is failJob for a task of tenant tid (uuid.Nil: no tenant).
func (h *harness) failJobOf(tid uuid.UUID, taskType, payload, id string) {
	h.t.Helper()
	ctx := h.ctx()
	client := queue.NewClient(h.rdb)
	defer client.Close()
	if err := queue.EnqueueFor(ctx, client, tid, taskType, []byte(payload), asynq.TaskID(id), asynq.MaxRetry(0)); err != nil {
		h.t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskType, func(context.Context, *asynq.Task) error {
		return errors.New(`Post "https://graph.facebook.com/v26.0/1/messages?access_token=EAASECRET": connection reset`)
	})
	q := queue.RouteTask(taskType, []byte(payload))
	srv := queue.NewServer(h.rdb, queue.ServerOptions{Concurrency: 1, Queues: map[string]int{q: 1},
		ShutdownTimeout: time.Second, Logger: testutil.DiscardLogger(), OnArchived: opsFailedJobHint(h.pool, testutil.DiscardLogger())})
	if err := srv.Start(mux); err != nil {
		h.t.Fatal(err)
	}
	defer srv.Shutdown()
	insp := queue.NewInspector(h.rdb)
	deadline := time.Now().Add(20 * time.Second)
	for {
		if info, err := insp.GetTaskInfo(q, id); err == nil && info.State == asynq.TaskStateArchived {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("task %s was not archived", id)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestFailedJobs(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	agent, _ := h.invite(owner, "agent@example.com", "agent")

	h.failJob("whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-000000000001","campaign_id":"0193a5a0-0000-7000-8000-000000000002","body":"private text"}`, "job-campaign")
	h.failJob("whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-000000000003"}`, "job-inbox")

	r := h.do(call{method: "GET", path: "/api/v1/system/status", token: owner})
	h.expect(r, 200, "")
	if n := r.data()["failed_jobs"]; n != float64(2) {
		t.Fatalf("status failed_jobs = %v, want 2", n)
	}

	r = h.do(call{method: "GET", path: "/api/v1/system/failed-jobs", token: owner})
	h.expect(r, 200, "")
	if total := r.body["meta"].(map[string]any)["total"]; total != float64(2) || len(r.list()) != 2 {
		t.Fatalf("list: total %v, %d items", total, len(r.list()))
	}
	byID := map[string]map[string]any{}
	for _, it := range r.list() {
		j := it.(map[string]any)
		byID[j["id"].(string)] = j
		if strings.Contains(string(r.raw), "private text") || strings.Contains(j["last_error"].(string), "EAASECRET") {
			t.Fatalf("payload or secret exposed: %s", r.raw)
		}
		if !strings.Contains(j["last_error"].(string), "connection reset") || j["failed_at"] == nil || j["type"] != "whatsapp:send" {
			t.Fatalf("job details: %v", j)
		}
	}
	camp := byID["job-campaign"]
	if camp["queue"] != "campaigns" || byID["job-inbox"]["queue"] != "critical" {
		t.Fatalf("queues: %v / %v", camp["queue"], byID["job-inbox"]["queue"])
	}
	if refs := camp["refs"].(map[string]any); refs["campaign_id"] != "0193a5a0-0000-7000-8000-000000000002" || refs["body"] != nil {
		t.Fatalf("refs = %v", refs)
	}

	r = h.do(call{method: "GET", path: "/api/v1/system/failed-jobs?queue=campaigns&limit=1&page=1", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 1 || r.body["meta"].(map[string]any)["total"] != float64(1) {
		t.Fatalf("filtered list: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/system/failed-jobs?page=2&limit=1", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("second page: %s", r.raw)
	}

	// Agents cannot see or touch the dead letter.
	h.expect(h.do(call{method: "GET", path: "/api/v1/system/failed-jobs", token: agent}), 403, "FORBIDDEN")
	h.expect(h.do(call{method: "POST", path: "/api/v1/system/failed-jobs/campaigns/job-campaign/retry", token: agent}), 403, "FORBIDDEN")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/system/failed-jobs/critical/job-inbox", token: agent}), 403, "FORBIDDEN")

	// Retry puts the job back in its queue.
	h.expect(h.do(call{method: "POST", path: "/api/v1/system/failed-jobs/campaigns/job-campaign/retry", token: owner}), 204, "")
	insp := queue.NewInspector(h.rdb)
	if info, err := insp.GetTaskInfo(queue.Campaigns, "job-campaign"); err != nil || info.State != asynq.TaskStatePending {
		t.Fatalf("after retry: %v %v", info, err)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/system/failed-jobs/campaigns/job-campaign/retry", token: owner}), 404, "NOT_FOUND")
	h.expect(h.do(call{method: "POST", path: "/api/v1/system/failed-jobs/critical/job-campaign/retry", token: owner}), 404, "NOT_FOUND")

	h.expect(h.do(call{method: "DELETE", path: "/api/v1/system/failed-jobs/critical/job-inbox", token: owner}), 204, "")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/system/failed-jobs/critical/job-inbox", token: owner}), 404, "NOT_FOUND")

	r = h.do(call{method: "GET", path: "/api/v1/system/failed-jobs", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 0 {
		t.Fatalf("list after retry/delete: %s", r.raw)
	}
	if n := h.count(`SELECT count(*) FROM audit_logs WHERE action IN ('failed_job.retry', 'failed_job.delete')
		AND entity_type = 'failed_job' AND changes->>'job_id' IN ('job-campaign', 'job-inbox')`); n != 2 {
		t.Fatalf("audit entries = %d, want 2", n)
	}
}

// Every tenant sees only its own failed jobs: the list, the counts, retry
// and delete; the platform scope (admin panel) sees all of them.
// (run by TestTenantBackgroundWork, sharing its harness)
func checkFailedJobsScope(t *testing.T, a, b *harness, ownerA, ownerB string) {

	a.failJob("whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-00000000000a"}`, "job-a")
	b.failJob("whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-00000000000b"}`, "job-b")
	a.failJobOf(uuid.Nil, "whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-00000000000c"}`, "job-none")

	for _, tc := range []struct {
		h     *harness
		token string
		want  string
	}{{a, ownerA, "job-a"}, {b, ownerB, "job-b"}} {
		r := tc.h.do(call{method: "GET", path: "/api/v1/system/failed-jobs", token: tc.token})
		tc.h.expect(r, 200, "")
		if total := r.body["meta"].(map[string]any)["total"]; total != float64(1) || len(r.list()) != 1 ||
			r.list()[0].(map[string]any)["id"] != tc.want {
			t.Fatalf("%s: %s", tc.want, r.raw)
		}
		r = tc.h.do(call{method: "GET", path: "/api/v1/system/status", token: tc.token})
		tc.h.expect(r, 200, "")
		if n := r.data()["failed_jobs"]; n != float64(1) {
			t.Fatalf("%s: status failed_jobs = %v", tc.want, n)
		}
	}
	// Another tenant's job is not found, neither for retry nor delete.
	a.expect(a.do(call{method: "POST", path: "/api/v1/system/failed-jobs/critical/job-b/retry", token: ownerA}), 404, "NOT_FOUND")
	a.expect(a.do(call{method: "DELETE", path: "/api/v1/system/failed-jobs/critical/job-b", token: ownerA}), 404, "NOT_FOUND")
	a.expect(a.do(call{method: "DELETE", path: "/api/v1/system/failed-jobs/critical/job-none", token: ownerA}), 404, "NOT_FOUND")
	insp := queue.NewInspector(a.rdb)
	defer insp.Close()
	if info, err := insp.GetTaskInfo(queue.Critical, "job-b"); err != nil || info.State != asynq.TaskStateArchived {
		t.Fatalf("tenant B's job after A's attempts: %v %v", info, err)
	}

	jobs := system.NewFailedJobs(insp)
	all, total, err := jobs.List(system.PlatformScope(), "", 1, 50)
	if err != nil || total != 3 || len(all) != 3 {
		t.Fatalf("platform view: %d jobs, total %d, %v", len(all), total, err)
	}
	tenants := map[string]uuid.UUID{}
	for _, j := range all {
		tenants[j.ID] = j.TenantID
	}
	if tenants["job-a"] != a.tenantID || tenants["job-b"] != b.tenantID || tenants["job-none"] != uuid.Nil {
		t.Fatalf("platform view tenants %v", tenants)
	}
	if _, err := system.TenantScope(context.Background()); err == nil {
		t.Fatal("a context without a tenant must not get a scope")
	}
}
