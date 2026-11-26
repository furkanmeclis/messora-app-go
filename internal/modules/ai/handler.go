package ai

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the AI operations (permissions ai.read / ai.manage are
// enforced by the API guard from the OpenAPI security requirements).
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// fail renders service errors. Provider messages are already scrubbed of
// keys.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var fe *FieldError
	var ae *Error
	switch {
	case errors.As(err, &fe):
		err = apperr.Validation(fe.Field, fe.Message)
	case errors.Is(err, ErrNotFound):
		err = apperr.NotFound("Not found.")
	case errors.As(err, &ae):
		err = apiError(ae)
	}
	httpx.WriteError(w, r, err, h.log)
}

// apiError maps a typed AI error to the API. Codes: AI_DISABLED,
// AI_NOT_CONFIGURED, AI_RATE_LIMITED, AI_QUOTA_EXCEEDED, AI_AUTH_FAILED,
// AI_PROVIDER_UNAVAILABLE, AI_BAD_REQUEST, AI_REFUSED, AI_INVALID_OUTPUT.
func apiError(e *Error) *apperr.Error {
	msg := e.Message
	if e.Provider != "" && msg != "" {
		msg = string(e.Provider) + ": " + msg
	}
	switch e.Kind {
	case KindDisabled:
		return apperr.Conflict("AI_DISABLED", "AI is turned off for the company. Turn it on in Settings → AI.")
	case KindNotConfigured:
		return apperr.Conflict("AI_NOT_CONFIGURED", "AI is not configured: "+msg)
	case KindRateLimited:
		return apperr.RateLimited("AI_RATE_LIMITED", "The AI provider is rate limiting requests: "+msg, e.RetryAfter)
	}
	code := map[ErrorKind]string{KindQuotaExceeded: "AI_QUOTA_EXCEEDED", KindAuth: "AI_AUTH_FAILED", KindOverloaded: "AI_PROVIDER_UNAVAILABLE",
		KindUnavailable: "AI_PROVIDER_UNAVAILABLE", KindBadRequest: "AI_BAD_REQUEST", KindRefusal: "AI_REFUSED", KindInvalidOutput: "AI_INVALID_OUTPUT"}[e.Kind]
	if code == "" {
		code = "AI_PROVIDER_ERROR"
	}
	return &apperr.Error{Status: http.StatusBadGateway, Code: code, Message: "The AI provider request failed: " + msg}
}

func actorID(r *http.Request) *uuid.UUID {
	if p := principal.From(r.Context()); p != nil {
		id := p.ID
		return &id
	}
	return nil
}

// ---------------------------------------------------------------- settings

func (h *Handler) settingsToAPI(s Settings) api.AISettings {
	return api.AISettings{Enabled: s.Enabled, DefaultProviderId: s.DefaultProviderID, DefaultModel: s.DefaultModel,
		Disclosure: api.AITexts{Tr: s.Disclosure.TR, En: s.Disclosure.EN}, DataSharingNotice: api.AITexts{Tr: s.DataSharingNotice.TR, En: s.DataSharingNotice.EN},
		FakeProviderAvailable: h.svc.FakeAllowed(), MaxRepliesPerHour: s.RepliesPerHour(), ContextMessages: s.ContextSize(),
		ClassifyCampaignReplies: s.ClassifyCampaignReplies, CampaignAttributionDays: s.AttributionDays(), MonthlyBudget: nonEmpty(s.MonthlyBudget)}
}

// GetAISettings implements api.ServerInterface.
func (h *Handler) GetAISettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Settings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, h.settingsToAPI(s))
}

// UpdateAISettings implements api.ServerInterface.
func (h *Handler) UpdateAISettings(w http.ResponseWriter, r *http.Request) {
	var req api.UpdateAISettingsRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in := Settings{Enabled: req.Enabled, DefaultProviderID: req.DefaultProviderId, DefaultModel: req.DefaultModel,
		Disclosure: Texts{TR: req.Disclosure.Tr, EN: req.Disclosure.En}, DataSharingNotice: Texts{TR: req.DataSharingNotice.Tr, EN: req.DataSharingNotice.En}}
	// Left out: keep the stored values.
	old, err := h.svc.Settings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	in.MaxRepliesPerHour, in.ContextMessages = old.MaxRepliesPerHour, old.ContextMessages
	in.MonthlyBudget, in.ClassifyCampaignReplies, in.CampaignAttributionDays = old.MonthlyBudget, old.ClassifyCampaignReplies, old.CampaignAttributionDays
	if req.MonthlyBudget != nil {
		in.MonthlyBudget = *req.MonthlyBudget // "" removes the budget
	}
	if req.ClassifyCampaignReplies != nil {
		in.ClassifyCampaignReplies = *req.ClassifyCampaignReplies
	}
	if req.CampaignAttributionDays != nil {
		in.CampaignAttributionDays = *req.CampaignAttributionDays
	}
	if req.MaxRepliesPerHour != nil {
		in.MaxRepliesPerHour = *req.MaxRepliesPerHour
	}
	if req.ContextMessages != nil {
		in.ContextMessages = *req.ContextMessages
	}
	s, err := h.svc.SaveSettings(r.Context(), in, actorID(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, h.settingsToAPI(s))
}

// --------------------------------------------------------------- providers

// providerToAPI never includes the key: only its last four characters.
func providerToAPI(p db.AiProvider) api.AIProvider {
	return api.AIProvider{Id: p.ID, Kind: api.AIProviderKind(p.Kind), Name: p.Name, KeyLast4: p.KeyLast4, BaseUrl: p.BaseUrl,
		Status: api.AIProviderStatus(p.Status), StatusDetail: p.StatusDetail, CheckedAt: p.CheckedAt, ModelsSyncedAt: p.ModelsSyncedAt,
		Enabled: p.Enabled, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

// ListAIProviders implements api.ServerInterface.
func (h *Handler) ListAIProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListProviders(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AIProvider, len(rows))
	for i, p := range rows {
		out[i] = providerToAPI(p)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateAIProvider implements api.ServerInterface.
func (h *Handler) CreateAIProvider(w http.ResponseWriter, r *http.Request) {
	var req api.CreateAIProviderRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreateProvider(r.Context(), ProviderInput{Kind: Kind(req.Kind), Name: &req.Name, APIKey: req.ApiKey,
		BaseURL: req.BaseUrl, Enabled: req.Enabled}, actorID(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, providerToAPI(p))
}

// UpdateAIProvider implements api.ServerInterface.
func (h *Handler) UpdateAIProvider(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	var req api.UpdateAIProviderRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.UpdateProvider(r.Context(), id, ProviderInput{Name: req.Name, APIKey: req.ApiKey, BaseURL: req.BaseUrl, Enabled: req.Enabled})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, providerToAPI(p))
}

// DeleteAIProvider implements api.ServerInterface.
func (h *Handler) DeleteAIProvider(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	if err := h.svc.DeleteProvider(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// CheckAIProvider implements api.ServerInterface.
func (h *Handler) CheckAIProvider(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	p, err := h.svc.CheckProvider(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, providerToAPI(p))
}

// SyncAIModels implements api.ServerInterface.
func (h *Handler) SyncAIModels(w http.ResponseWriter, r *http.Request, id api.ProviderId) {
	n, err := h.svc.SyncModels(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.AIModelSync{Models: n})
}

// ListAIModels implements api.ServerInterface.
func (h *Handler) ListAIModels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListModels(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AIModel, len(rows))
	for i, v := range rows {
		m := v.Model
		out[i] = api.AIModel{ProviderId: m.ProviderID, ProviderName: m.ProviderName, ProviderKind: api.AIProviderKind(m.ProviderKind),
			ModelId: m.ModelID, DisplayName: m.DisplayName, ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutput, Lifecycle: m.Lifecycle,
			SyncedAt: m.SyncedAt, Capabilities: api.AICapabilities{Tools: v.Capabilities.Tools, StructuredOutput: v.Capabilities.StructuredOutput,
				Vision: v.Capabilities.Vision, Thinking: v.Capabilities.Thinking}}
		if v.Price != nil {
			p := priceToAPI(*v.Price)
			out[i].Price = &p
		}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ------------------------------------------------------------------ prices

func priceToAPI(r db.GetAIPriceRow) api.AIPrice {
	rules, _ := ParseRules(r.Rules)
	out := api.AIPrice{Id: r.ID, ProviderKind: api.AIProviderKind(r.ProviderKind), ModelPattern: r.ModelPattern, Currency: r.Currency,
		Input: trimZeros(r.Input), Output: trimZeros(r.Output), CachedInput: trimmed(r.CachedInput), CacheWrite5m: trimmed(r.CacheWrite5m),
		CacheWrite1h: trimmed(r.CacheWrite1h), Rules: rulesToAPI(rules), EffectiveFrom: openapi_types.Date{Time: r.EffectiveFrom.Time},
		Source: api.AIPriceSource(r.Source), SourceUrl: r.SourceUrl, Notes: r.Notes, Confirmed: r.ConfirmedAt != nil,
		ConfirmedAt: r.ConfirmedAt, ConfirmedBy: r.ConfirmedBy, UpdatedAt: r.UpdatedAt}
	if r.EffectiveTo.Valid {
		out.EffectiveTo = &openapi_types.Date{Time: r.EffectiveTo.Time}
	}
	return out
}

func trimmed(s string) *string {
	if s == "" {
		return nil
	}
	t := trimZeros(s)
	return &t
}

func rulesToAPI(r Rules) api.AIPriceRules {
	var out api.AIPriceRules
	if len(r.Thresholds) > 0 {
		ts := make([]api.AIPriceThreshold, len(r.Thresholds))
		for i, t := range r.Thresholds {
			ts[i] = api.AIPriceThreshold{InputTokensOver: t.InputTokensOver, Input: t.Input, CachedInput: t.CachedInput,
				CacheWrite5m: t.CacheWrite5m, CacheWrite1h: t.CacheWrite1h, Output: t.Output}
		}
		out.Thresholds = &ts
	}
	if len(r.Windows) > 0 {
		ws := make([]api.AIPriceWindow, len(r.Windows))
		for i, w := range r.Windows {
			ws[i] = api.AIPriceWindow{Start: w.Start, End: w.End, Multiplier: w.Multiplier, Input: w.Input, CachedInput: w.CachedInput,
				CacheWrite5m: w.CacheWrite5m, CacheWrite1h: w.CacheWrite1h, Output: w.Output}
			if w.Label != "" {
				ws[i].Label = ptr(w.Label)
			}
			if w.TimeZone != "" {
				ws[i].TimeZone = ptr(w.TimeZone)
			}
			if len(w.Weekdays) > 0 {
				wd := append([]int(nil), w.Weekdays...)
				ws[i].Weekdays = &wd
			}
		}
		out.Windows = &ws
	}
	return out
}

func rulesFromAPI(r *api.AIPriceRules) Rules {
	var out Rules
	if r == nil {
		return out
	}
	if r.Thresholds != nil {
		for _, t := range *r.Thresholds {
			out.Thresholds = append(out.Thresholds, Threshold{InputTokensOver: t.InputTokensOver, Rates: Rates{Input: t.Input,
				CachedInput: t.CachedInput, CacheWrite5m: t.CacheWrite5m, CacheWrite1h: t.CacheWrite1h, Output: t.Output}})
		}
	}
	if r.Windows != nil {
		for _, w := range *r.Windows {
			win := Window{Label: deref(w.Label), Start: w.Start, End: w.End, TimeZone: deref(w.TimeZone), Multiplier: w.Multiplier,
				Rates: Rates{Input: w.Input, CachedInput: w.CachedInput, CacheWrite5m: w.CacheWrite5m, CacheWrite1h: w.CacheWrite1h, Output: w.Output}}
			if w.Weekdays != nil {
				win.Weekdays = *w.Weekdays
			}
			out.Windows = append(out.Windows, win)
		}
	}
	return out
}

// ListAIPrices implements api.ServerInterface.
func (h *Handler) ListAIPrices(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListPrices(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AIPrice, len(rows))
	for i, p := range rows {
		out[i] = priceToAPI(p)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateAIPrice implements api.ServerInterface.
func (h *Handler) CreateAIPrice(w http.ResponseWriter, r *http.Request) {
	var req api.CreateAIPriceRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in := PriceInput{Kind: Kind(req.ProviderKind), Pattern: req.ModelPattern, Currency: deref(req.Currency), Input: req.Input,
		CachedInput: req.CachedInput, CacheWrite5m: req.CacheWrite5m, CacheWrite1h: req.CacheWrite1h, Output: req.Output,
		Rules: rulesFromAPI(req.Rules), EffectiveFrom: req.EffectiveFrom.Time, SourceURL: deref(req.SourceUrl), Notes: deref(req.Notes)}
	if req.EffectiveTo != nil {
		in.EffectiveTo = &req.EffectiveTo.Time
	}
	p, err := h.svc.CreatePrice(r.Context(), in, actorID(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, priceToAPI(p))
}

// UpdateAIPrice implements api.ServerInterface. Fields left out keep their
// value; nullable rates sent as null are cleared.
func (h *Handler) UpdateAIPrice(w http.ResponseWriter, r *http.Request, id api.PriceId) {
	var raw map[string]json.RawMessage
	if err := httpx.Decode(r, &raw); err != nil {
		h.fail(w, r, err)
		return
	}
	b, _ := json.Marshal(raw)
	var req api.UpdateAIPriceRequest
	if err := json.Unmarshal(b, &req); err != nil {
		h.fail(w, r, apperr.Validation("", "Request body is not valid JSON for this operation"))
		return
	}
	cur, err := h.svc.GetPrice(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	in := PriceFromRow(cur)
	has := func(k string) bool { _, ok := raw[k]; return ok }
	if req.ModelPattern != nil {
		in.Pattern = *req.ModelPattern
	}
	if req.Currency != nil {
		in.Currency = *req.Currency
	}
	if req.Input != nil {
		in.Input = *req.Input
	}
	if req.Output != nil {
		in.Output = *req.Output
	}
	if has("cached_input") {
		in.CachedInput = req.CachedInput
	}
	if has("cache_write_5m") {
		in.CacheWrite5m = req.CacheWrite5m
	}
	if has("cache_write_1h") {
		in.CacheWrite1h = req.CacheWrite1h
	}
	if req.Rules != nil {
		in.Rules = rulesFromAPI(req.Rules)
	}
	if req.EffectiveFrom != nil {
		in.EffectiveFrom = req.EffectiveFrom.Time
	}
	if has("effective_to") {
		in.EffectiveTo = nil
		if req.EffectiveTo != nil {
			in.EffectiveTo = &req.EffectiveTo.Time
		}
	}
	if req.SourceUrl != nil {
		in.SourceURL = *req.SourceUrl
	}
	if req.Notes != nil {
		in.Notes = *req.Notes
	}
	confirm := req.Confirm != nil && *req.Confirm
	p, err := h.svc.UpdatePrice(r.Context(), id, in, confirm, actorID(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, priceToAPI(p))
}

// DeleteAIPrice implements api.ServerInterface.
func (h *Handler) DeleteAIPrice(w http.ResponseWriter, r *http.Request, id api.PriceId) {
	if err := h.svc.DeletePrice(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ImportAIPrices implements api.ServerInterface: multipart with "file"
// (LiteLLM JSON) and an optional "effective_from" (YYYY-MM-DD) before it.
func (h *Handler) ImportAIPrices(w http.ResponseWriter, r *http.Request) {
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" || params["boundary"] == "" {
		h.fail(w, r, apperr.Validation("file", "Upload the JSON as multipart/form-data in a field named file."))
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		h.fail(w, r, apperr.Validation("file", "Upload the JSON as multipart/form-data in a field named file."))
		return
	}
	now := h.svc.opts.Now()
	effective := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			h.fail(w, r, apperr.Validation("file", "The upload could not be read."))
			return
		}
		switch part.FormName() {
		case "effective_from":
			b, _ := io.ReadAll(io.LimitReader(part, 64))
			t, err := time.Parse(time.DateOnly, strings.TrimSpace(string(b)))
			if err != nil {
				h.fail(w, r, apperr.Validation("effective_from", "Use a date like 2026-12-01."))
				return
			}
			effective = t
		case "file":
			res, err := ParseLiteLLM(part, effective)
			if err != nil {
				h.fail(w, r, apperr.Validation("file", err.Error()))
				return
			}
			if err := h.svc.ImportPrices(r.Context(), res); err != nil {
				h.fail(w, r, err)
				return
			}
			httpx.JSON(w, r, http.StatusOK, api.AIPriceImport{Prices: len(res.Prices), Skipped: res.Skipped})
			return
		}
	}
	h.fail(w, r, apperr.Validation("file", "The upload has no file field."))
}

// ---------------------------------------------------------------- profiles

func profileToAPI(p Profile) api.AIProfile {
	warnings := []api.AIProfileWarnings{}
	for _, w := range p.Warnings() {
		warnings = append(warnings, api.AIProfileWarnings(w))
	}
	fields := make([]api.AIDataField, len(p.DataCollection.Fields))
	for i, f := range p.DataCollection.Fields {
		fields[i] = api.AIDataField{Key: f.Key, Label: f.Label, Required: f.Required, Type: ptr(api.AIDataFieldType(f.TypeOf()))}
		if len(f.Options) > 0 {
			opts := f.Options
			fields[i].Options = &opts
		}
	}
	var low *float32
	if p.Handoff.OnLowConfidence != nil {
		v := float32(*p.Handoff.OnLowConfidence)
		low = &v
	}
	return api.AIProfile{Id: p.ID, Name: p.Name, PhoneNumberId: p.PhoneNumberID, PhoneDisplay: p.PhoneDisplay, SystemPrompt: p.SystemPrompt,
		ProviderId: p.ProviderID, Model: p.Model, AutoReply: p.AutoReply,
		Handoff: api.AIHandoff{OnRequest: true, OnLowConfidence: low, OnNoAnswer: p.Handoff.OnNoAnswer,
			OnCriticalIntents: p.Handoff.OnCriticalIntents, OnUnauthorizedAction: p.Handoff.OnUnauthorizedAction,
			OnHumanTakeover: ptr(api.AIHandoffOnHumanTakeover(takeoverOf(p.Handoff)))},
		DataCollection: api.AIDataCollection{Enabled: p.DataCollection.Enabled, Fields: fields,
			MayCreateContact: p.DataCollection.MayCreateContact, MayUpdateFields: p.DataCollection.MayUpdateFields,
			HandoffWhenComplete: ptr(p.DataCollection.HandoffWhenComplete)},
		AllowedActions: p.AllowedActions, DisclosureMode: api.AIDisclosureMode(p.DisclosureMode), Status: api.AIProfileStatus(p.Status),
		Warnings: warnings, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func profileFromAPI(in api.AIProfileInput) Profile {
	p := Profile{Name: in.Name, PhoneNumberID: in.PhoneNumberId, SystemPrompt: deref(in.SystemPrompt), ProviderID: in.ProviderId,
		Model: in.Model, DisclosureMode: DiscloseFirstMessage, Status: ProfileActive, AllowedActions: []string{}}
	if in.AutoReply != nil {
		p.AutoReply = *in.AutoReply
	}
	if in.Handoff != nil {
		p.Handoff = Handoff{OnNoAnswer: in.Handoff.OnNoAnswer, OnCriticalIntents: in.Handoff.OnCriticalIntents,
			OnUnauthorizedAction: in.Handoff.OnUnauthorizedAction}
		if in.Handoff.OnHumanTakeover != nil {
			p.Handoff.OnHumanTakeover = string(*in.Handoff.OnHumanTakeover)
		}
		if in.Handoff.OnLowConfidence != nil {
			// float32 → two decimals, so 0.7 stays 0.7.
			v := float64(int(*in.Handoff.OnLowConfidence*100+0.5)) / 100
			p.Handoff.OnLowConfidence = &v
		}
	}
	if in.DataCollection != nil {
		p.DataCollection = DataCollection{Enabled: in.DataCollection.Enabled, MayCreateContact: in.DataCollection.MayCreateContact,
			MayUpdateFields: in.DataCollection.MayUpdateFields}
		if in.DataCollection.HandoffWhenComplete != nil {
			p.DataCollection.HandoffWhenComplete = *in.DataCollection.HandoffWhenComplete
		}
		for _, f := range in.DataCollection.Fields {
			df := DataField{Key: f.Key, Label: f.Label, Required: f.Required}
			if f.Type != nil {
				df.Type = string(*f.Type)
			}
			if f.Options != nil {
				df.Options = *f.Options
			}
			p.DataCollection.Fields = append(p.DataCollection.Fields, df)
		}
	}
	if in.AllowedActions != nil {
		p.AllowedActions = *in.AllowedActions
	}
	if in.DisclosureMode != nil {
		p.DisclosureMode = string(*in.DisclosureMode)
	}
	if in.Status != nil {
		p.Status = string(*in.Status)
	}
	return p
}

// ListAIProfiles implements api.ServerInterface.
func (h *Handler) ListAIProfiles(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListProfiles(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AIProfile, len(rows))
	for i, p := range rows {
		out[i] = profileToAPI(p)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// GetAIProfile implements api.ServerInterface.
func (h *Handler) GetAIProfile(w http.ResponseWriter, r *http.Request, id api.ProfileId) {
	p, err := h.svc.GetProfile(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, profileToAPI(p))
}

// CreateAIProfile implements api.ServerInterface.
func (h *Handler) CreateAIProfile(w http.ResponseWriter, r *http.Request) {
	var req api.AIProfileInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreateProfile(r.Context(), profileFromAPI(req), actorID(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, profileToAPI(p))
}

// UpdateAIProfile implements api.ServerInterface.
func (h *Handler) UpdateAIProfile(w http.ResponseWriter, r *http.Request, id api.ProfileId) {
	var req api.AIProfileInput
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.UpdateProfile(r.Context(), id, profileFromAPI(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, profileToAPI(p))
}

// DeleteAIProfile implements api.ServerInterface.
func (h *Handler) DeleteAIProfile(w http.ResponseWriter, r *http.Request, id api.ProfileId) {
	if err := h.svc.DeleteProfile(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// TestAIProfile implements api.ServerInterface.
func (h *Handler) TestAIProfile(w http.ResponseWriter, r *http.Request, id api.ProfileId) {
	var req api.TestAIProfileRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	msgs := make([]Message, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = Message{Role: Role(m.Role), Text: m.Text}
	}
	lang := LangTR
	if req.Language != nil {
		lang = Language(*req.Language)
	}
	res, err := h.svc.TestProfile(r.Context(), id, msgs, lang)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	u := res.Usage
	out := api.AITestResult{Text: res.Text, Model: res.Model, ProviderId: res.ProviderID, ProviderName: res.ProviderName,
		Usage: api.AIUsageTokens{InputTokens: u.InputTokens, CachedInputTokens: u.CachedInputTokens, CacheWriteTokens: u.CacheWriteTokens,
			CacheWrite1hTokens: u.CacheWrite1hTokens, OutputTokens: u.OutputTokens, ReasoningTokens: u.ReasoningTokens},
		Cost: res.Cost, Estimated: res.Estimated, LatencyMs: res.Latency.Milliseconds(), Instructions: res.Instructions}
	if res.Cost != nil {
		c := trimZeros(*res.Cost)
		out.Cost, out.Currency = &c, &res.Currency
	}
	if rp := res.Reply; rp != nil {
		fields := rp.ExtractedFields
		if fields == nil {
			fields = map[string]*string{}
		}
		out.Reply = &api.AIReply{Reply: rp.Reply, Confidence: float32(rp.Confidence), Intent: rp.Intent, Escalate: rp.Escalate,
			EscalateReason: rp.EscalateReason, ExtractedFields: fields}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// takeoverOf is the stored takeover mode (off for profiles saved before it
// existed).
func takeoverOf(h Handoff) string {
	if h.OnHumanTakeover == TakeoverSuggest {
		return TakeoverSuggest
	}
	return TakeoverOff
}
