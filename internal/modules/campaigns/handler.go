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
	rm := api.ReactionMap{}
	_ = json.Unmarshal(c.ReactionMap, &rm)
	return api.Campaign{ReactionMap: &rm, Id: c.ID, Name: c.Name, Status: api.CampaignStatus(c.Status), PhoneNumberId: c.PhoneNumberID,
		TemplateName: c.TemplateName, TemplateLanguage: c.TemplateLanguage, TemplateParams: params, Audience: aud,
		WindowSize: int(c.WindowSize), TotalRecipients: c.TotalRecipients, Counts: counts, ScheduledAt: c.ScheduledAt,
		StartedAt: c.StartedAt, FinishedAt: c.FinishedAt, LastError: c.LastError, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		OfferId: c.OfferID, Priority: api.CampaignPriority(c.Priority), MaxMps: c.MaxMps, PausedReason: c.PausedReason,
		PausedAt: c.PausedAt, PausedBy: c.PausedBy, ResumeAt: c.ResumeAt, SendRate: c.SendRate, EtaSeconds: eta(c, counts)}, nil
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
		Name             *string            `json:"name"`
		PhoneNumberID    *uuid.UUID         `json:"phone_number_id"`
		TemplateName     *string            `json:"template_name"`
		TemplateLanguage *string            `json:"template_language"`
		TemplateParams   *Params            `json:"template_params"`
		Audience         *Audience          `json:"audience"`
		WindowSize       *int32             `json:"window_size"`
		OfferID          *uuid.UUID         `json:"offer_id"`
		ClearOffer       bool               `json:"clear_offer"`
		Priority         *string            `json:"priority"`
		MaxMPS           *float64           `json:"max_mps"`
		ClearMaxMPS      bool               `json:"clear_max_mps"`
		ReactionMap      *map[string]string `json:"reaction_map"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return Input{}, apperr.Validation("", "Invalid campaign.")
	}
	return Input{Name: b.Name, PhoneNumberID: b.PhoneNumberID, TemplateName: b.TemplateName, TemplateLanguage: b.TemplateLanguage,
		Params: b.TemplateParams, Audience: b.Audience, WindowSize: b.WindowSize, OfferID: b.OfferID, ClearOffer: b.ClearOffer,
		Priority: b.Priority, MaxMPS: b.MaxMPS, ClearMaxMPS: b.ClearMaxMPS, ReactionMap: b.ReactionMap}, nil
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
	var req api.CampaignResumeRequest
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	o := ResumeOptions{Settings: settingsOf(req.Name, req.Priority, req.MaxMps, req.ClearMaxMps), At: req.ResumeAt}
	c, err := h.svc.ResumeWith(r.Context(), id, actor(r), o)
	h.write(w, r, http.StatusOK, c, err)
}

func settingsOf(name *string, priority *api.CampaignPriority, maxMPS *float64, clear *bool) Settings {
	st := Settings{Name: name, MaxMPS: maxMPS, ClearMaxMPS: clear != nil && *clear}
	if priority != nil {
		p := string(*priority)
		st.Priority = &p
	}
	return st
}

// UpdateCampaignSettings implements api.ServerInterface.
func (h *Handler) UpdateCampaignSettings(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req api.CampaignSettingsRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.UpdateSettings(r.Context(), id, settingsOf(req.Name, req.Priority, req.MaxMps, req.ClearMaxMps), actor(r))
	h.write(w, r, http.StatusOK, c, err)
}

// ListCampaignPauses implements api.ServerInterface.
func (h *Handler) ListCampaignPauses(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	rows, err := h.svc.Pauses(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.CampaignPause, len(rows))
	for i, p := range rows {
		detail, snap := map[string]any{}, map[string]int64{}
		_ = json.Unmarshal(p.Detail, &detail)
		_ = json.Unmarshal(p.Snapshot, &snap)
		var changes *map[string]any
		if len(p.Changes) > 0 {
			m := map[string]any{}
			if json.Unmarshal(p.Changes, &m) == nil {
				changes = &m
			}
		}
		out[i] = api.CampaignPause{Id: p.ID, Status: api.CampaignStatus(p.Status), Reason: p.Reason, Detail: detail, PausedAt: p.PausedAt,
			PausedBy: p.PausedBy, ResumeAt: p.ResumeAt, Snapshot: snap, ResumedAt: p.ResumedAt, ResumedBy: p.ResumedBy, Changes: changes}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// PreviewCampaignResume implements api.ServerInterface.
func (h *Handler) PreviewCampaignResume(w http.ResponseWriter, r *http.Request, id api.CampaignId) {
	var req api.CampaignSettingsRequest
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	p, err := h.svc.PreviewResume(r.Context(), id, settingsOf(req.Name, req.Priority, req.MaxMps, req.ClearMaxMps))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.CampaignResumePreview{CampaignId: id, Remaining: p.Remaining, Eligible: p.Eligible,
		Blocked: p.Blocked, Warned: p.Warned, BlockedReasons: apiReasons(p.BlockedReasons), WarnedReasons: apiReasons(p.WarnedReasons),
		ShareMps: p.ShareMPS, EstimatedSeconds: p.EstimatedSeconds, CapacityMps: p.Capacity.Ceiling, RateMps: p.Capacity.Rate,
		ReservedShare: p.Capacity.ReservedShare, Limit: apiLimit(p.Limit)})
}

// GetCampaignQueue implements api.ServerInterface.
func (h *Handler) GetCampaignQueue(w http.ResponseWriter, r *http.Request) {
	qs, err := h.svc.Queue(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, QueueToAPI(qs))
}

// QueueToAPI is the send queue view as the API returns it.
func QueueToAPI(qs []QueueNumber) []api.CampaignQueueNumber {
	out := make([]api.CampaignQueueNumber, len(qs))
	for i, n := range qs {
		cs := make([]api.CampaignQueueEntry, len(n.Campaigns))
		for j, c := range n.Campaigns {
			cs[j] = api.CampaignQueueEntry{Id: c.ID, Name: c.Name, Status: api.CampaignStatus(c.Status), Priority: api.CampaignPriority(c.Priority),
				Weight: c.Weight, MaxMps: c.MaxMPS, ShareMps: c.ShareMPS, RateMps: c.RateMPS, Remaining: c.Remaining,
				TotalRecipients: c.TotalRecipients, EtaSeconds: c.ETASeconds, PausedReason: c.PausedReason, ResumeAt: c.ResumeAt}
		}
		out[i] = api.CampaignQueueNumber{PhoneNumberId: n.PhoneNumberID, CeilingMps: n.Ceiling, RateMps: n.Rate, ReservedShare: n.ReservedShare,
			CampaignCapacityMps: n.Capacity.Campaigns, Limit: apiLimit(n.Limit), Campaigns: cs}
	}
	return out
}

func apiLimit(l limitState) api.PortfolioLimit {
	return api.PortfolioLimit{Tier: l.Tier, Limit: l.Limit, Used: l.Used, InFlight: l.InFlight, FreesAt: l.FreesAt}
}

func apiReasons(in []PreflightReason) []api.PreflightReason {
	out := make([]api.PreflightReason, len(in))
	for i, x := range in {
		out[i] = api.PreflightReason{Reason: api.PreflightReasonReason(x.Reason), Count: x.Count, Hard: x.Hard}
	}
	return out
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
	var class *string
	if p.Class != nil {
		c := string(*p.Class)
		class = &c
	}
	rows, err := h.svc.q.ListRecipients(r.Context(), db.ListRecipientsParams{CampaignID: id, State: state, Class: class, BeforeID: before, Lim: int32(limit + 1)}) //nolint:gosec // limit ≤ 200
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
			FailureClass: (*api.FailureClass)(x.FailureClass), Outcome: ptr(api.CampaignRecipientOutcome(Outcome(x.State, x.MessageStatus))),
			Resends: ptr(int(x.Resends)), NextAttemptAt: x.NextAttemptAt}
		if x.UnreachedClass != "" {
			out[i].UnreachedClass = ptr(api.CampaignRecipientUnreachedClass(x.UnreachedClass))
		}
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
		BlockedReasons: reasons(p.BlockedReasons), WarnedReasons: reasons(p.WarnedReasons), ByFailureClass: byClass(p.ByFailureClass)})
}

func byClass(m map[string]int64) *map[string]int64 {
	if m == nil {
		return nil
	}
	return &m
}
