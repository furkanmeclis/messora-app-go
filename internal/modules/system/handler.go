package system

import (
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
)

// ListFailedJobs implements api.ServerInterface.
func (h *Handler) ListFailedJobs(w http.ResponseWriter, r *http.Request, p api.ListFailedJobsParams) {
	page, limit := 1, 50
	if p.Page != nil {
		page = *p.Page
	}
	if p.Limit != nil {
		limit = *p.Limit
	}
	var q string
	if p.Queue != nil {
		q = string(*p.Queue)
	}
	sc, err := TenantScope(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	jobs, total, err := h.jobs.List(sc, q, page, limit)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.FailedJob, len(jobs))
	for i, j := range jobs {
		out[i] = toAPI(j)
	}
	httpx.JSONTotal(w, r, http.StatusOK, out, total)
}

// RetryFailedJob implements api.ServerInterface.
func (h *Handler) RetryFailedJob(w http.ResponseWriter, r *http.Request, q api.QueuePath, id api.JobId) {
	sc, err := TenantScope(r.Context())
	var j FailedJob
	if err == nil {
		j, err = h.jobs.Retry(sc, string(q), id)
	}
	if err == nil {
		err = h.audit(r, "failed_job.retry", j)
	}
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.NoContent(w)
}

// DeleteFailedJob implements api.ServerInterface.
func (h *Handler) DeleteFailedJob(w http.ResponseWriter, r *http.Request, q api.QueuePath, id api.JobId) {
	sc, err := TenantScope(r.Context())
	var j FailedJob
	if err == nil {
		j, err = h.jobs.Delete(sc, string(q), id)
	}
	if err == nil {
		err = h.audit(r, "failed_job.delete", j)
	}
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.NoContent(w)
}

// audit records the action after the fact: the queue lives in Redis, so
// there is no transaction to share with the change.
func (h *Handler) audit(r *http.Request, action string, j FailedJob) error {
	changes := map[string]any{"queue": j.Queue, "job_id": j.ID, "task_type": j.Type}
	for k, v := range j.Refs {
		changes["ref_"+k] = v
	}
	return audit.Record(r.Context(), h.q, audit.Entry{Action: action, EntityType: "failed_job", Changes: changes})
}

func toAPI(j FailedJob) api.FailedJob {
	out := api.FailedJob{Id: j.ID, Queue: api.QueueName(j.Queue), Type: j.Type, LastError: j.LastError,
		Retried: j.Retried, MaxRetry: j.MaxRetry, Refs: j.Refs}
	if !j.FailedAt.IsZero() {
		t := j.FailedAt.UTC()
		out.FailedAt = &t
	}
	return out
}
