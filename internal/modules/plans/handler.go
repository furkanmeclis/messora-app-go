package plans

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/credits"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the plan operations of workspaces and of the platform
// admin panel, and the panel's AI credit and e-mail sender settings.
type Handler struct {
	svc  *Service
	mail *settings.Mail // nil when a test injects its own mailer
	log  *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, mail *settings.Mail, log *slog.Logger) *Handler {
	return &Handler{svc: svc, mail: mail, log: log}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var fe *ai.FieldError
	var ce *credits.FieldError
	switch {
	case errors.As(err, &fe):
		err = apperr.Validation(fe.Field, fe.Message)
	case errors.As(err, &ce):
		err = apperr.Validation(ce.Field, ce.Message)
	case errors.Is(err, settings.ErrInvalid):
		err = apperr.Validation("", err.Error())
	}
	httpx.WriteError(w, r, err, h.log)
}

func respond[T any](h *Handler, w http.ResponseWriter, r *http.Request, status int, v T, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, v)
}

func decode[T any](h *Handler, w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		h.fail(w, r, err)
		return v, false
	}
	return v, true
}

// ── Workspace ──────────────────────────────────────────────────────────────

// GetPlan implements api.ServerInterface.
func (h *Handler) GetPlan(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Overview(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// ListPlans implements api.ServerInterface.
func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Catalog(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// CreatePlanChangeRequest implements api.ServerInterface.
func (h *Handler) CreatePlanChangeRequest(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.CreatePlanChangeRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.RequestChange(r.Context(), principal.From(r.Context()), req)
	respond(h, w, r, http.StatusCreated, v, err)
}

// ── Platform: plans ────────────────────────────────────────────────────────

// ListPlatformPlans implements api.ServerInterface.
func (h *Handler) ListPlatformPlans(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.ListPlans(r.Context())
	respond(h, w, r, http.StatusOK, v, err)
}

// CreatePlatformPlan implements api.ServerInterface.
func (h *Handler) CreatePlatformPlan(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.PlatformPlanInput](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.CreatePlan(r.Context(), req)
	respond(h, w, r, http.StatusCreated, v, err)
}

// UpdatePlatformPlan implements api.ServerInterface.
func (h *Handler) UpdatePlatformPlan(w http.ResponseWriter, r *http.Request, key api.PlanKey) {
	req, ok := decode[api.PlatformPlanPatch](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.UpdatePlan(r.Context(), key, req)
	respond(h, w, r, http.StatusOK, v, err)
}

// GetPlatformTenantPlan implements api.ServerInterface.
func (h *Handler) GetPlatformTenantPlan(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	v, err := h.svc.TenantPlan(r.Context(), id)
	respond(h, w, r, http.StatusOK, v, err)
}

// SetPlatformTenantPlan implements api.ServerInterface.
func (h *Handler) SetPlatformTenantPlan(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.SetTenantPlanRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.SetTenantPlan(r.Context(), id, req)
	respond(h, w, r, http.StatusOK, v, err)
}

// GrantPlatformTenantAICredits implements api.ServerInterface.
func (h *Handler) GrantPlatformTenantAICredits(w http.ResponseWriter, r *http.Request, id api.TenantId) {
	req, ok := decode[api.GrantAICreditsRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.Grant(r.Context(), principal.From(r.Context()), id, req)
	respond(h, w, r, http.StatusCreated, v, err)
}

// ListPlatformPlanRequests implements api.ServerInterface.
func (h *Handler) ListPlatformPlanRequests(w http.ResponseWriter, r *http.Request, p api.ListPlatformPlanRequestsParams) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	v, err := h.svc.Requests(r.Context(), status)
	respond(h, w, r, http.StatusOK, v, err)
}

// ResolvePlatformPlanRequest implements api.ServerInterface.
func (h *Handler) ResolvePlatformPlanRequest(w http.ResponseWriter, r *http.Request, id api.RequestId) {
	req, ok := decode[api.ResolvePlanRequest](h, w, r)
	if !ok {
		return
	}
	v, err := h.svc.Resolve(r.Context(), principal.From(r.Context()), id, req)
	respond(h, w, r, http.StatusOK, v, err)
}

// ── Platform: AI credits and the default model ─────────────────────────────

func (h *Handler) aiSettings(r *http.Request) (api.PlatformAISettings, error) {
	ctx := pctx(r.Context())
	st, err := credits.ReadSettings(ctx, h.svc.q)
	if err != nil {
		return api.PlatformAISettings{}, err
	}
	out := api.PlatformAISettings{UsdPerCredit: st.USDPerCredit, Markup: st.Markup, UnpricedCallCredits: st.UnpricedCallCredits, Models: []api.AIModel{}}
	if h.svc.o.AI != nil {
		def, err := h.svc.o.AI.PlatformDefaults(ctx)
		if err != nil {
			return out, err
		}
		out.DefaultProviderId, out.DefaultModel = def.DefaultProviderID, def.DefaultModel
		models, err := h.svc.o.AI.PlatformModels(ctx)
		if err != nil {
			return out, err
		}
		for _, m := range models {
			out.Models = append(out.Models, ai.ModelToAPI(m))
		}
	}
	return out, nil
}

// GetPlatformAISettings implements api.ServerInterface.
func (h *Handler) GetPlatformAISettings(w http.ResponseWriter, r *http.Request) {
	v, err := h.aiSettings(r)
	respond(h, w, r, http.StatusOK, v, err)
}

// UpdatePlatformAISettings implements api.ServerInterface.
func (h *Handler) UpdatePlatformAISettings(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.UpdatePlatformAISettingsRequest](h, w, r)
	if !ok {
		return
	}
	ctx := pctx(r.Context())
	err := h.svc.platformTx(r.Context(), func(q *db.Queries) error {
		st, err := credits.ReadSettings(ctx, q)
		if err != nil {
			return err
		}
		changes := map[string]any{}
		if req.UsdPerCredit != nil {
			st.USDPerCredit, changes["usd_per_credit"] = *req.UsdPerCredit, *req.UsdPerCredit
		}
		if req.Markup != nil {
			st.Markup, changes["markup"] = *req.Markup, *req.Markup
		}
		if req.UnpricedCallCredits != nil {
			st.UnpricedCallCredits, changes["unpriced_call_credits"] = *req.UnpricedCallCredits, *req.UnpricedCallCredits
		}
		if err := credits.SaveSettings(ctx, q, st); err != nil {
			return err
		}
		if h.svc.o.AI != nil && (req.DefaultModel != nil || req.DefaultProviderId != nil) {
			def, err := h.svc.o.AI.PlatformDefaults(ctx)
			if err != nil {
				return err
			}
			def.DefaultProviderID = req.DefaultProviderId
			if req.DefaultModel != nil {
				def.DefaultModel = *req.DefaultModel
			}
			if req.DefaultProviderId == nil && req.DefaultModel == nil {
				def.DefaultModel = ""
			}
			if err := h.svc.o.AI.SavePlatformDefaults(ctx, q, def); err != nil {
				return err
			}
			changes["default_provider_id"], changes["default_model"] = def.DefaultProviderID, def.DefaultModel
		}
		return audit.Record(ctx, q, audit.Entry{Action: "platform.ai_settings_update", EntityType: "platform_settings", Changes: changes})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if h.svc.o.Credits != nil {
		h.svc.o.Credits.InvalidateSettings()
	}
	h.GetPlatformAISettings(w, r)
}

// ── Platform: the e-mail sender ────────────────────────────────────────────

func (h *Handler) mailUnavailable(w http.ResponseWriter, r *http.Request) bool {
	if h.mail == nil {
		httpx.WriteError(w, r, apperr.Unavailable("MAIL_UNAVAILABLE", "E-mail settings are not available."), h.log)
		return true
	}
	return false
}

func platformEmailToAPI(v settings.PlatformSMTPView) api.PlatformEmail {
	tls := v.TLS
	if tls == "" {
		tls = "starttls"
	}
	out := api.PlatformEmail{Source: api.PlatformEmailSource(v.Source), Host: v.Host, Port: v.Port, Username: v.Username, From: v.From,
		Tls: api.SmtpTls(tls), PasswordSet: v.PasswordSet}
	if v.ReplyTo != "" {
		out.ReplyTo = &v.ReplyTo
	}
	out.Env.Configured, out.Env.Host, out.Env.From = v.EnvConfigured, v.EnvHost, v.EnvFrom
	return out
}

// GetPlatformEmail implements api.ServerInterface.
func (h *Handler) GetPlatformEmail(w http.ResponseWriter, r *http.Request) {
	if h.mailUnavailable(w, r) {
		return
	}
	v, err := h.mail.PlatformView(pctx(r.Context()), h.svc.q)
	respond(h, w, r, http.StatusOK, platformEmailToAPI(v), err)
}

func smtpInput(in api.SmtpSettingsInput) settings.SMTPInput {
	return settings.SMTPInput{Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, From: in.From, TLS: string(in.Tls)}
}

// UpdatePlatformEmail implements api.ServerInterface.
func (h *Handler) UpdatePlatformEmail(w http.ResponseWriter, r *http.Request) {
	if h.mailUnavailable(w, r) {
		return
	}
	req, ok := decode[api.SmtpSettingsInput](h, w, r)
	if !ok {
		return
	}
	ctx := pctx(r.Context())
	err := h.svc.platformTx(r.Context(), func(q *db.Queries) error {
		c, err := h.mail.SavePlatform(ctx, q, smtpInput(req))
		if err != nil {
			return err
		}
		// The password is never written to the audit log.
		return audit.Record(ctx, q, audit.Entry{Action: "platform.email_update", EntityType: "platform_settings",
			Changes: map[string]any{"host": c.Host, "port": c.Port, "username": c.Username, "from": c.From, "tls": c.TLS,
				"password_changed": req.Password != nil}})
	})
	h.mail.Invalidate()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.GetPlatformEmail(w, r)
}

// TestPlatformEmail implements api.ServerInterface.
func (h *Handler) TestPlatformEmail(w http.ResponseWriter, r *http.Request) {
	if h.mailUnavailable(w, r) {
		return
	}
	req, ok := decode[api.SmtpTestRequest](h, w, r)
	if !ok {
		return
	}
	var in *settings.SMTPInput
	if req.Config != nil {
		v := smtpInput(*req.Config)
		in = &v
	}
	if err := h.mail.TestPlatform(pctx(r.Context()), h.svc.q, in, string(req.To)); err != nil {
		if errors.Is(err, settings.ErrInvalid) {
			h.fail(w, r, err)
			return
		}
		httpx.WriteError(w, r, apperr.Validation("", "The test e-mail could not be sent: "+err.Error()), h.log)
		return
	}
	httpx.NoContent(w)
}
