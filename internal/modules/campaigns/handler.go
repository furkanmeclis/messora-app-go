package campaigns

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the campaign operations.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

func (h *Handler) toAPI(r *http.Request, c db.Campaign) (api.Campaign, error) {
	var params api.CampaignParams
	_ = json.Unmarshal(c.TemplateParams, &params)
	var aud api.CampaignAudience
	_ = json.Unmarshal(c.Audience, &aud)
	counts, err := h.svc.Stats(r.Context(), c.ID)
	if err != nil {
		return api.Campaign{}, err
	}
	return api.Campaign{Id: c.ID, Name: c.Name, Status: api.CampaignStatus(c.Status), PhoneNumberId: c.PhoneNumberID,
		TemplateName: c.TemplateName, TemplateLanguage: c.TemplateLanguage, TemplateParams: params, Audience: aud,
		WindowSize: int(c.WindowSize), TotalRecipients: c.TotalRecipients, Counts: counts, ScheduledAt: c.ScheduledAt,
		StartedAt: c.StartedAt, FinishedAt: c.FinishedAt, LastError: c.LastError, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}, nil
}

func (h *Handler) write(w http.ResponseWriter, r *http.Request, status int, c db.Campaign, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.toAPI(r, c)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, out)
}

// input converts request bodies via JSON so the oneOf parameter shapes stay
// raw for the renderer.
func input(body any) (Input, error) {
	raw, _ := json.Marshal(body)
	var b struct {
		Name             *string    `json:"name"`
		PhoneNumberID    *uuid.UUID `json:"phone_number_id"`
		TemplateName     *string    `json:"template_name"`
		TemplateLanguage *string    `json:"template_language"`
		TemplateParams   *Params    `json:"template_params"`
		Audience         *Audience  `json:"audience"`
		WindowSize       *int32     `json:"window_size"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return Input{}, apperr.Validation("", "Invalid campaign.")
	}
	return Input{Name: b.Name, PhoneNumberID: b.PhoneNumberID, TemplateName: b.TemplateName, TemplateLanguage: b.TemplateLanguage,
		Params: b.TemplateParams, Audience: b.Audience, WindowSize: b.WindowSize}, nil
}

func actor(r *http.Request) *uuid.UUID {
	p := principal.From(r.Context())
	if p == nil {
		return nil
	}
	return &p.ID
}

// ListCampaigns implements api.ServerInterface.
func (h *Handler) ListCampaigns(w http.ResponseWriter, r *http.Request, p api.ListCampaignsParams) {
	limit := 50
	if p.Limit != nil {
		limit = *p.Limit
	}
	var before *uuid.UUID
	if p.Cursor != nil {
		id, err := uuid.Parse(*p.Cursor)
		if err != nil {
			h.fail(w, r, apperr.Validation("cursor", "Invalid cursor"))
			return
		}
		before = &id
	}
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	rows, err := h.svc.q.ListCampaigns(r.Context(), db.ListCampaignsParams{Status: status, BeforeID: before, Lim: int32(limit + 1)}) //nolint:gosec // limit ≤ 200 (spec)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	out := make([]api.Campaign, len(rows))
	for i, c := range rows {
		if out[i], err = h.toAPI(r, c); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// CreateCampaign implements api.ServerInterface.
func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request, _ api.CreateCampaignParams) {
	var req api.CreateCampaignRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in, err := input(req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Create(r.Context(), in, actor(r))
	h.write(w, r, http.StatusCreated, c, err)
}

// GetCampaign implements api.ServerInterface.
func (h *Handler) GetCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	c, err := h.svc.q.GetCampaign(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Campaign not found.")
	}
	h.write(w, r, http.StatusOK, c, err)
}

// UpdateCampaign implements api.ServerInterface.
func (h *Handler) UpdateCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req api.UpdateCampaignRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in, err := input(req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Update(r.Context(), id, in)
	h.write(w, r, http.StatusOK, c, err)
}

// DeleteCampaign implements api.ServerInterface.
func (h *Handler) DeleteCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// StartCampaign implements api.ServerInterface.
func (h *Handler) StartCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId, _ api.StartCampaignParams) {
	var req api.StartCampaignJSONBody
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	c, err := h.svc.Start(r.Context(), id, req.ScheduledAt, actor(r))
	h.write(w, r, http.StatusOK, c, err)
}

// PauseCampaign implements api.ServerInterface.
func (h *Handler) PauseCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	c, err := h.svc.Pause(r.Context(), id, actor(r), "")
	h.write(w, r, http.StatusOK, c, err)
}

// ResumeCampaign implements api.ServerInterface.
func (h *Handler) ResumeCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	c, err := h.svc.Resume(r.Context(), id, actor(r))
	h.write(w, r, http.StatusOK, c, err)
}

// CancelCampaign implements api.ServerInterface.
func (h *Handler) CancelCampaign(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	c, err := h.svc.Cancel(r.Context(), id, actor(r))
	h.write(w, r, http.StatusOK, c, err)
}

// ListCampaignEvents implements api.ServerInterface.
func (h *Handler) ListCampaignEvents(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	if _, err := h.svc.q.GetCampaign(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		h.fail(w, r, apperr.NotFound("Campaign not found."))
		return
	}
	rows, err := h.svc.q.ListCampaignEvents(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.CampaignEventItem, len(rows))
	for i, e := range rows {
		detail := map[string]any{}
		_ = json.Unmarshal(e.Detail, &detail)
		out[i] = api.CampaignEventItem{Id: e.ID, Kind: e.Kind, Detail: detail, ActorId: e.ActorID, CreatedAt: e.CreatedAt}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ListCampaignRecipients implements api.ServerInterface.
func (h *Handler) ListCampaignRecipients(w http.ResponseWriter, r *http.Request, id api.CampaignId, p api.ListCampaignRecipientsParams) {
	limit := 50
	if p.Limit != nil {
		limit = *p.Limit
	}
	var before *uuid.UUID
	if p.Cursor != nil {
		c, err := uuid.Parse(*p.Cursor)
		if err != nil {
			h.fail(w, r, apperr.Validation("cursor", "Invalid cursor"))
			return
		}
		before = &c
	}
	var state *string
	if p.State != nil {
		s := string(*p.State)
		state = &s
	}
	rows, err := h.svc.q.ListRecipients(r.Context(), db.ListRecipientsParams{CampaignID: id, State: state, BeforeID: before, Lim: int32(limit + 1)}) //nolint:gosec // limit ≤ 200
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	out := make([]api.CampaignRecipient, len(rows))
	for i, x := range rows {
		out[i] = api.CampaignRecipient{Id: x.ID, ContactId: x.ContactID, Phone: x.PhoneE164, State: api.CampaignRecipientState(x.State),
			SkipReason: x.SkipReason, MessageId: x.MessageID, MessageStatus: x.MessageStatus, ErrorCode: x.ErrorCode, ErrorTitle: x.ErrorTitle,
			FailureClass: (*api.FailureClass)(x.FailureClass)}
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// PreflightCampaign implements api.ServerInterface.
func (h *Handler) PreflightCampaign(w http.ResponseWriter, r *http.Request) {
	var req api.CampaignPreflightRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	raw, _ := json.Marshal(req.Audience)
	var a Audience
	if err := json.Unmarshal(raw, &a); err != nil {
		h.fail(w, r, apperr.Validation("audience", "Invalid audience."))
		return
	}
	p, err := h.svc.Preflight(r.Context(), a, req.PhoneNumberId, req.TemplateName, req.TemplateLanguage)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reasons := func(in []PreflightReason) []api.PreflightReason {
		out := make([]api.PreflightReason, len(in))
		for i, x := range in {
			out[i] = api.PreflightReason{Reason: api.PreflightReasonReason(x.Reason), Count: x.Count, Hard: x.Hard}
		}
		return out
	}
	httpx.JSON(w, r, http.StatusOK, api.CampaignPreflight{Category: api.CampaignPreflightCategory(p.Category),
		UnknownMarketing: api.CampaignPreflightUnknownMarketing(p.UnknownMarketing), Total: p.Total, Eligible: p.Eligible,
		Warned: p.Warned, Blocked: p.Blocked, HardBlocked: p.HardBlocked,
		BlockedReasons: reasons(p.BlockedReasons), WarnedReasons: reasons(p.WarnedReasons)})
}
