package channels

import (
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// ── Company side: Embedded Signup ──────────────────────────────────────────

func onboardingToAPI(o Onboarding) api.TechProviderOnboarding {
	r := o.Row
	out := api.TechProviderOnboarding{Id: r.ID, WabaId: r.WabaID, PhoneNumberId: r.PhoneNumberID, BusinessId: r.BusinessID,
		Name: r.Name, Status: api.TechProviderOnboardingStatus(r.Status), Step: api.TechProviderOnboardingStep(r.Step),
		AccountId: r.AccountID, Attempts: int(r.Attempts), ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage,
		PinGenerated: r.PinGenerated, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, CompletedAt: r.CompletedAt}
	if o.PIN != "" {
		pin := o.PIN
		out.Pin = &pin
	}
	return out
}

// GetTechProvider implements api.ServerInterface.
func (h *Handler) GetTechProvider(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.enabledTechProvider(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	rows, err := h.svc.Onboardings(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	list := make([]api.TechProviderOnboarding, len(rows))
	for i, row := range rows {
		list[i] = onboardingToAPI(Onboarding{Row: row})
	}
	version := cfg.GraphAPIVersion
	if version == "" {
		version = h.svc.wa.Version()
	}
	httpx.JSON(w, r, http.StatusOK, api.TechProvider{AppId: cfg.AppID, ConfigurationId: cfg.ConfigurationID, GraphApiVersion: version,
		Scopes: append([]string(nil), TechProviderScopes...), Onboardings: list})
}

// StartTechProviderOnboarding implements api.ServerInterface.
func (h *Handler) StartTechProviderOnboarding(w http.ResponseWriter, r *http.Request) {
	var req api.StartTechProviderOnboardingRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	actor := principal.From(r.Context())
	o, err := h.svc.StartOnboarding(r.Context(), OnboardingInput{Code: req.Code, WABAID: req.WabaId, PhoneNumberID: req.PhoneNumberId,
		BusinessID: deref(req.BusinessId), Name: deref(req.Name), PIN: deref(req.Pin)}, &actor.ID)
	h.writeOnboarding(w, r, o, err)
}

// RetryTechProviderOnboarding implements api.ServerInterface.
func (h *Handler) RetryTechProviderOnboarding(w http.ResponseWriter, r *http.Request, id api.OnboardingId) {
	var req api.RetryTechProviderOnboardingRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	o, err := h.svc.RetryOnboarding(r.Context(), id, deref(req.Code), deref(req.Pin))
	h.writeOnboarding(w, r, o, err)
}

// CancelTechProviderOnboarding implements api.ServerInterface.
func (h *Handler) CancelTechProviderOnboarding(w http.ResponseWriter, r *http.Request, id api.OnboardingId) {
	o, err := h.svc.CancelOnboarding(r.Context(), id)
	h.writeOnboarding(w, r, o, err)
}

func (h *Handler) writeOnboarding(w http.ResponseWriter, r *http.Request, o Onboarding, err error) {
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, onboardingToAPI(o))
}

// DisconnectTechProviderAccount implements api.ServerInterface.
func (h *Handler) DisconnectTechProviderAccount(w http.ResponseWriter, r *http.Request, id api.AccountId) {
	acc, warning, err := h.svc.DisconnectTechProvider(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := api.TechProviderDisconnect{Account: accountToAPI(acc)}
	if warning != "" {
		out.Warning = &warning
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ── Platform side ──────────────────────────────────────────────────────────

// tpCallbackURL is the platform webhook address shown to the platform
// admin and checked by the test.
func (h *Handler) tpCallbackURL() string { return h.opts.WebhookBaseURL + TechProviderWebhookPath }

func (h *Handler) platformTechProviderToAPI(c TechProviderConfig) api.PlatformTechProvider {
	n, last := Unrouted()
	out := api.PlatformTechProvider{Enabled: c.Enabled, Ready: c.Ready(), AppId: nonEmpty(c.AppID), ConfigurationId: nonEmpty(c.ConfigurationID),
		GraphApiVersion: nonEmpty(c.GraphAPIVersion), DefaultGraphApiVersion: h.svc.wa.Version(),
		AppSecretConfigured: c.AppSecret != "", SystemUserTokenConfigured: c.SystemUserToken != "",
		VerifyToken: nonEmpty(c.VerifyToken), CallbackUrl: h.tpCallbackURL(), Scopes: append([]string(nil), TechProviderScopes...),
		UnroutedEntries: n, LastUnroutedWabaId: nonEmpty(last), UpdatedAt: c.UpdatedAt}
	return out
}

// GetPlatformTechProvider implements api.ServerInterface.
// TODO(WP2-admin): the platform panel's "Meta / Tech Provider" form reads
// this (admin host, platform session).
func (h *Handler) GetPlatformTechProvider(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.TechProvider(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, h.platformTechProviderToAPI(c))
}

// UpdatePlatformTechProvider implements api.ServerInterface.
// TODO(WP2-admin): the platform panel's form writes this; WP2 runs platform
// work as messora_platform (tenant.Platform), which may write
// platform_settings. HandlerOptions.PlatformDB is that connection.
func (h *Handler) UpdatePlatformTechProvider(w http.ResponseWriter, r *http.Request) {
	var req api.UpdatePlatformTechProviderRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	writer := h.opts.PlatformDB
	if writer == nil {
		writer = h.svc.pool
	}
	p := TechProviderPatch{Enabled: req.Enabled, AppID: req.AppId, ConfigurationID: req.ConfigurationId, GraphAPIVersion: req.GraphApiVersion,
		AppSecret: req.AppSecret, SystemUserToken: req.SystemUserToken, RotateVerifyToken: req.RotateVerifyToken != nil && *req.RotateVerifyToken}
	c, _, err := h.svc.UpdateTechProvider(r.Context(), writer, p)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, h.platformTechProviderToAPI(c))
}

// TestPlatformTechProvider implements api.ServerInterface.
func (h *Handler) TestPlatformTechProvider(w http.ResponseWriter, r *http.Request) {
	checks, err := h.svc.TestTechProvider(r.Context(), h.tpCallbackURL())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := api.PlatformTechProviderTest{Ok: true, Checks: make([]api.PlatformTechProviderCheck, len(checks))}
	for i, c := range checks {
		params := c.Params
		if params == nil {
			params = map[string]string{}
		}
		out.Checks[i] = api.PlatformTechProviderCheck{Key: api.PlatformTechProviderCheckKey(c.Key), Status: api.PlatformTechProviderCheckStatus(c.Status),
			Code: c.Code, Params: params, Message: c.Message}
		out.Ok = out.Ok && c.Status != CheckError
	}
	httpx.JSON(w, r, http.StatusOK, out)
}
