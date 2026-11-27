package platform

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/system"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler implements the platform operations of api.ServerInterface.
type Handler struct {
	svc *Service
	log *slog.Logger
	// The catalogue reads are the same as the companies' (read-only there).
	listRates, listPrices http.HandlerFunc
	jobs                  *system.FailedJobs
}

// NewHandler returns the HTTP handler for s. listRates and listPrices serve
// the Meta rate card and the AI price table.
func NewHandler(s *Service, listRates, listPrices http.HandlerFunc, jobs *system.FailedJobs, log *slog.Logger) *Handler {
	return &Handler{svc: s, log: log, listRates: listRates, listPrices: listPrices, jobs: jobs}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

func decode[T any](h *Handler, w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		h.fail(w, r, err)
		return v, false
	}
	return v, true
}

func respond[T any](h *Handler, w http.ResponseWriter, r *http.Request, status int, v T, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, v)
}

// GetPlatformOverview implements api.ServerInterface.
func (h *Handler) GetPlatformOverview(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Overview(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// ListPlatformTenants implements api.ServerInterface.
func (h *Handler) ListPlatformTenants(w http.ResponseWriter, r *http.Request, params api.ListPlatformTenantsParams) {
	out, next, err := h.svc.ListTenants(r.Context(), params)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// CreatePlatformTenant implements api.ServerInterface.
func (h *Handler) CreatePlatformTenant(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.CreatePlatformTenantRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.CreateTenant(r.Context(), principal.From(r.Context()), req)
	respond(h, w, r, http.StatusCreated, v, err)
}

// GetPlatformTenant implements api.ServerInterface.
func (h *Handler) GetPlatformTenant(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	v, err := h.svc.Tenant(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// UpdatePlatformTenant implements api.ServerInterface. trial_ends_at: null
// clears the trial end; leaving it out keeps it.
func (h *Handler) UpdatePlatformTenant(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	raw, ok := decode[map[string]json.RawMessage](h, w, r)
	if !ok {
		return
	}
	b, _ := json.Marshal(raw)
	var req api.UpdatePlatformTenantRequest
	if err := json.Unmarshal(b, &req); err != nil {
		h.fail(w, r, apperr.Validation("", "Request body is not valid JSON for this operation"))
		return
	}
	p := TenantPatch{Name: req.Name}
	if req.SubscriptionStatus != nil {
		v := string(*req.SubscriptionStatus)
		p.Subscription = &v
	}
	if _, set := raw["trial_ends_at"]; set {
		p.SetTrial, p.TrialEndsAt = true, req.TrialEndsAt
		if p.TrialEndsAt != nil {
			t := p.TrialEndsAt.UTC().Truncate(time.Second)
			p.TrialEndsAt = &t
		}
	}
	v, err := h.svc.UpdateTenant(r.Context(), id, p)
	respond(h, w, r, http.StatusOK, v, err)
}

// SuspendPlatformTenant implements api.ServerInterface.
func (h *Handler) SuspendPlatformTenant(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.SuspendTenantRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.Suspend(r.Context(), id, req.Reason)
	respond(h, w, r, http.StatusOK, v, err)
}

// UnsuspendPlatformTenant implements api.ServerInterface.
func (h *Handler) UnsuspendPlatformTenant(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	v, err := h.svc.Unsuspend(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// InvitePlatformTenantOwner implements api.ServerInterface.
func (h *Handler) InvitePlatformTenantOwner(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.InviteOwnerRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.InviteOwner(r.Context(), id, string(req.Email))
	respond(h, w, r, http.StatusCreated, v, err)
}

// AddPlatformTenantDomain implements api.ServerInterface.
func (h *Handler) AddPlatformTenantDomain(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.AddDomainRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.AddDomain(r.Context(), id, req.Hostname)
	respond(h, w, r, http.StatusCreated, v, err)
}

// VerifyPlatformDomain implements api.ServerInterface.
func (h *Handler) VerifyPlatformDomain(w http.ResponseWriter, r *http.Request, id api.DomainId) {
	v, err := h.svc.VerifyDomain(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// SetPlatformPrimaryDomain implements api.ServerInterface.
func (h *Handler) SetPlatformPrimaryDomain(w http.ResponseWriter, r *http.Request, id api.DomainId) {
	v, err := h.svc.SetPrimaryDomain(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// DeletePlatformDomain implements api.ServerInterface.
func (h *Handler) DeletePlatformDomain(w http.ResponseWriter, r *http.Request, id api.DomainId) {
	if err := h.svc.DeleteDomain(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ListPlatformUsers implements api.ServerInterface.
func (h *Handler) ListPlatformUsers(w http.ResponseWriter, r *http.Request, params api.ListPlatformUsersParams) {
	out, next, err := h.svc.SearchUsers(r.Context(), params.Q, params.Cursor, params.Limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// GetPlatformUser implements api.ServerInterface.
func (h *Handler) GetPlatformUser(w http.ResponseWriter, r *http.Request, id api.UserId) {
	v, err := h.svc.User(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// DisablePlatformUser implements api.ServerInterface.
func (h *Handler) DisablePlatformUser(w http.ResponseWriter, r *http.Request, id api.UserId) {
	v, err := h.svc.SetUserStatus(r.Context(), principal.From(r.Context()), id, "disabled")
	respond(h, w, r, http.StatusOK, v, err)
}

// EnablePlatformUser implements api.ServerInterface.
func (h *Handler) EnablePlatformUser(w http.ResponseWriter, r *http.Request, id api.UserId) {
	v, err := h.svc.SetUserStatus(r.Context(), principal.From(r.Context()), id, "active")
	respond(h, w, r, http.StatusOK, v, err)
}

// ForcePlatformUserPasswordReset implements api.ServerInterface.
func (h *Handler) ForcePlatformUserPasswordReset(w http.ResponseWriter, r *http.Request, id api.UserId) {
	v, err := h.svc.ForcePasswordReset(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// StartSupportSession implements api.ServerInterface.
func (h *Handler) StartSupportSession(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.StartSupportSessionRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.StartSupport(r.Context(), principal.From(r.Context()), id, req)
	respond(h, w, r, http.StatusCreated, v, err)
}

// ListSupportSessions implements api.ServerInterface.
func (h *Handler) ListSupportSessions(w http.ResponseWriter, r *http.Request, params api.ListSupportSessionsParams) {
	v, err := h.svc.ListGrants(r.Context(), params.TenantId)
	respond(h, w, r, http.StatusOK, v, err)
}

// ListSupportAccess implements api.ServerInterface: the company's own view
// of support sessions (row-level security keeps it to the company).
func (h *Handler) ListSupportAccess(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.ListGrants(r.Context(), nil)
	respond(h, w, r, http.StatusOK, v, err)
}

// EndSupportSession implements api.ServerInterface.
func (h *Handler) EndSupportSession(w http.ResponseWriter, r *http.Request, id api.GrantId) {
	if err := h.svc.EndSupport(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// GetPlatformSettings implements api.ServerInterface.
func (h *Handler) GetPlatformSettings(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Settings(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// UpdatePlatformSettings implements api.ServerInterface.
func (h *Handler) UpdatePlatformSettings(w http.ResponseWriter, r *http.Request) {
	raw, ok := decode[map[string]json.RawMessage](h, w, r)
	if !ok {
		return
	}
	b, _ := json.Marshal(raw)
	var req api.UpdatePlatformSettingsRequest
	if err := json.Unmarshal(b, &req); err != nil {
		h.fail(w, r, apperr.Validation("", "Request body is not valid JSON for this operation"))
		return
	}
	p := SettingsPatch{SignupEnabled: req.SignupEnabled}
	if _, set := raw["reply_to"]; set {
		p.SetReplyTo = true
		if req.ReplyTo != nil {
			v := string(*req.ReplyTo)
			p.ReplyTo = &v
		}
	}
	v, err := h.svc.UpdateSettings(r.Context(), p)
	respond(h, w, r, http.StatusOK, v, err)
}

// ListPlatformAuditLogs implements api.ServerInterface.
func (h *Handler) ListPlatformAuditLogs(w http.ResponseWriter, r *http.Request, params api.ListPlatformAuditLogsParams) {
	out, next, err := h.svc.AuditLogs(r.Context(), params.TenantId, params.Cursor, params.Limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// ListPlatformMetaRates implements api.ServerInterface.
func (h *Handler) ListPlatformMetaRates(w http.ResponseWriter, r *http.Request) { h.listRates(w, r) }

// ListPlatformAIPrices implements api.ServerInterface.
func (h *Handler) ListPlatformAIPrices(w http.ResponseWriter, r *http.Request) { h.listPrices(w, r) }

// ListPlatformFailedJobs implements api.ServerInterface: every company's
// dead letters (system.PlatformScope).
func (h *Handler) ListPlatformFailedJobs(w http.ResponseWriter, r *http.Request, p api.ListPlatformFailedJobsParams) {
	page, limit := 1, 50
	if p.Page != nil {
		page = *p.Page
	}
	if p.Limit != nil {
		limit = *p.Limit
	}
	q := ""
	if p.Queue != nil {
		q = string(*p.Queue)
	}
	jobs, total, err := h.jobs.List(system.PlatformScope(), q, page, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.PlatformFailedJob, len(jobs))
	for i, j := range jobs {
		v := system.ToAPI(j)
		out[i] = api.PlatformFailedJob{Id: v.Id, Queue: v.Queue, Type: v.Type, LastError: v.LastError, FailedAt: v.FailedAt,
			Retried: v.Retried, MaxRetry: v.MaxRetry, Refs: v.Refs}
		if j.TenantID != uuid.Nil {
			id := j.TenantID
			out[i].TenantId = &id
		}
	}
	httpx.JSONTotal(w, r, http.StatusOK, out, total)
}

// RetryPlatformFailedJob implements api.ServerInterface.
func (h *Handler) RetryPlatformFailedJob(w http.ResponseWriter, r *http.Request, q api.QueuePath, id api.JobId) {
	j, err := h.jobs.Retry(system.PlatformScope(), string(q), id)
	if err == nil {
		err = h.auditJob(r, "failed_job.retry", j)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// DeletePlatformFailedJob implements api.ServerInterface.
func (h *Handler) DeletePlatformFailedJob(w http.ResponseWriter, r *http.Request, q api.QueuePath, id api.JobId) {
	j, err := h.jobs.Delete(system.PlatformScope(), string(q), id)
	if err == nil {
		err = h.auditJob(r, "failed_job.delete", j)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) auditJob(r *http.Request, action string, j system.FailedJob) error {
	var t *uuid.UUID
	if j.TenantID != uuid.Nil {
		id := j.TenantID
		t = &id
	}
	return h.svc.tx(r.Context(), func(q *db.Queries) error {
		return audit.Record(pctx(r.Context()), q, audit.Entry{Action: action, EntityType: "failed_job", Tenant: t,
			Changes: map[string]any{"queue": j.Queue, "job_id": j.ID, "task_type": j.Type}})
	})
}
