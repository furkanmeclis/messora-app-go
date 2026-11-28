package channels

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// WebhookFields are the webhook fields Messora processes.
var WebhookFields = []string{"messages", "message_template_status_update", "message_template_quality_update",
	"phone_number_quality_update", "account_update", "business_capability_update", "user_preferences"}

// WebhookKeyPath is the path of a tenant's keyed webhook address below the
// webhook base URL: <base>/meta/<key>. The API also serves it under
// /webhooks, so <PUBLIC_APP_URL>/webhooks/meta/<key> works through the
// edge proxy's /webhooks route.
const WebhookKeyPath = "/meta/"

// HostWebhookPath is the webhook at a tenant's own address.
const HostWebhookPath = "/webhooks/whatsapp"

// HandlerOptions are the public addresses the handler shows.
type HandlerOptions struct {
	PublicURL      string // PUBLIC_APP_URL, for the scheme of host addresses
	WebhookBaseURL string // config.MetaConfig.WebhookBaseURL
}

// Handler serves the channel and template operations.
type Handler struct {
	svc  *Service
	log  *slog.Logger
	opts HandlerOptions
}

// NewHandler returns the handler.
func NewHandler(svc *Service, opts HandlerOptions, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, opts: opts}
}

func accountToAPI(a db.WhatsappAccount) api.WhatsAppAccount {
	return api.WhatsAppAccount{Id: a.ID, Name: a.Name, WabaId: a.WabaID, AppId: a.AppID, ConnectionMethod: api.WhatsAppAccountConnectionMethod(a.ConnectionMethod),
		Status: api.WhatsAppAccountStatus(a.Status), LastError: a.LastError, AppSecretConfigured: len(a.AppSecretEnc) > 0,
		LastSyncedAt: a.LastSyncedAt, CreatedAt: a.CreatedAt, Token: tokenToAPI(a), Currency: a.Currency}
}

func tokenToAPI(a db.WhatsappAccount) api.WhatsAppTokenHealth {
	scopes := a.TokenScopes
	if scopes == nil {
		scopes = []string{}
	}
	return api.WhatsAppTokenHealth{Type: a.TokenType, ExpiresAt: a.TokenExpiresAt, DataAccessExpiresAt: a.TokenDataAccessExpiresAt,
		Scopes: scopes, CheckedAt: a.TokenCheckedAt, Warning: a.TokenWarning}
}

// CheckWhatsAppAccount implements api.ServerInterface.
func (h *Handler) CheckWhatsAppAccount(w http.ResponseWriter, r *http.Request, id api.AccountId) {
	acc, err := h.svc.CheckToken(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, accountToAPI(acc))
}

func templateToAPI(t db.MessageTemplate, account string) api.Template {
	comps := []map[string]any{}
	if json.Unmarshal(t.Components, &comps) != nil || comps == nil {
		comps = []map[string]any{}
	}
	out := api.Template{Id: t.ID, AccountId: t.AccountID, MetaTemplateId: &t.MetaTemplateID, Name: t.Name, Language: t.Language,
		Category: t.Category, Status: t.Status, ParameterFormat: t.ParameterFormat, Components: comps,
		RejectedReason: t.RejectedReason, QualityScore: &t.QualityScore, SyncedAt: &t.SyncedAt}
	if account != "" {
		out.AccountName = &account
	}
	return out
}

// ListWhatsAppAccounts implements api.ServerInterface.
func (h *Handler) ListWhatsAppAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListWhatsAppAccounts(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.WhatsAppAccount, len(rows))
	for i, a := range rows {
		out[i] = accountToAPI(a)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ConnectWhatsAppAccount implements api.ServerInterface.
func (h *Handler) ConnectWhatsAppAccount(w http.ResponseWriter, r *http.Request, _ api.ConnectWhatsAppAccountParams) {
	var req api.ConnectWhatsAppAccountRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	actor := principal.From(r.Context())
	in := AccountInput{Name: req.Name, WABAID: req.WabaId, AccessToken: deref(req.AccessToken), AppID: req.AppId, AppSecret: deref(req.AppSecret)}
	acc, err := h.svc.Connect(r.Context(), in, &actor.ID)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, accountToAPI(acc))
}

// UpdateWhatsAppAccount implements api.ServerInterface.
func (h *Handler) UpdateWhatsAppAccount(w http.ResponseWriter, r *http.Request, id api.AccountId) {
	var req api.UpdateWhatsAppAccountRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	var status *string
	if req.Status != nil {
		s := string(*req.Status)
		status = &s
	}
	acc, err := h.svc.Update(r.Context(), id, UpdateInput{Name: req.Name, AccessToken: req.AccessToken, AppSecret: req.AppSecret, AppID: req.AppId, Status: status})
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, accountToAPI(acc))
}

// DisconnectWhatsAppAccount implements api.ServerInterface.
func (h *Handler) DisconnectWhatsAppAccount(w http.ResponseWriter, r *http.Request, id api.AccountId) {
	if err := h.svc.Disconnect(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.NoContent(w)
}

// SyncWhatsAppAccount implements api.ServerInterface.
func (h *Handler) SyncWhatsAppAccount(w http.ResponseWriter, r *http.Request, id api.AccountId) {
	n, t, err := h.svc.Sync(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.SyncResult{PhoneNumbers: n, Templates: t})
}

// ListPhoneNumbers implements api.ServerInterface.
func (h *Handler) ListPhoneNumbers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListPhoneNumbers(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.PhoneNumber, len(rows))
	for i, p := range rows {
		out[i] = api.PhoneNumber{Id: p.ID, AccountId: p.AccountID, AccountName: p.AccountName, PhoneNumberId: p.PhoneNumberID,
			DisplayPhoneNumber: p.DisplayPhoneNumber, VerifiedName: p.VerifiedName, QualityRating: p.QualityRating,
			MessagingLimit: p.MessagingLimit, ThroughputLevel: &p.ThroughputLevel, IsDefault: p.IsDefault,
			SignalsCheckedAt: p.SignalsCheckedAt}
		if p.ThroughputMps != nil {
			mps := int(*p.ThroughputMps)
			out[i].ThroughputMps = &mps
		}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// SetDefaultPhoneNumber implements api.ServerInterface.
func (h *Handler) SetDefaultPhoneNumber(w http.ResponseWriter, r *http.Request, id api.PhoneNumberId) {
	if err := h.svc.SetDefaultNumber(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.NoContent(w)
}

// hostCallbackURL is the webhook at the address of the request (the
// tenant's own host), with PUBLIC_APP_URL's scheme.
func (h *Handler) hostCallbackURL(r *http.Request) string {
	host := storage.RequestHost(r.Context())
	u, err := url.Parse(h.opts.PublicURL)
	if host == "" || err != nil || u.Scheme == "" {
		return h.opts.PublicURL + HostWebhookPath
	}
	return u.Scheme + "://" + host + HostWebhookPath
}

func (h *Handler) connectionToAPI(r *http.Request, c Connection) (api.MetaConnection, error) {
	secrets, err := h.svc.AppSecrets(r.Context())
	if err != nil {
		return api.MetaConnection{}, err
	}
	row := c.Row
	return api.MetaConnection{
		Method:                api.MetaConnectionMethod(row.Method),
		CallbackUrl:           h.opts.WebhookBaseURL + WebhookKeyPath + c.WebhookKey,
		HostCallbackUrl:       h.hostCallbackURL(r),
		VerifyToken:           c.VerifyToken,
		Fields:                append([]string(nil), WebhookFields...),
		SignatureVerification: len(secrets) > 0,
		TechProviderEnabled:   h.svc.TechProviderEnabled(r.Context()),
		WebhookKeyRotatedAt:   row.WebhookKeyRotatedAt,
		PreviousKeyExpiresAt:  futureOrNil(row.PreviousKeyExpiresAt),
		VerifyTokenRotatedAt:  row.VerifyTokenRotatedAt,
		Health: api.MetaWebhookHealth{LastWebhookAt: row.LastWebhookAt, SignatureFailures: row.SignatureFailures,
			LastSignatureFailureAt: row.LastSignatureFailureAt, WabaMismatches: row.WabaMismatches,
			LastWabaMismatchAt: row.LastWabaMismatchAt, LastMismatchWabaId: row.LastMismatchWabaID},
	}, nil
}

func futureOrNil(t *time.Time) *time.Time {
	if t == nil || !t.After(time.Now()) {
		return nil
	}
	return t
}

func (h *Handler) writeConnection(w http.ResponseWriter, r *http.Request, c Connection, err error) {
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out, err := h.connectionToAPI(r, c)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// GetMetaConnection implements api.ServerInterface.
func (h *Handler) GetMetaConnection(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Connection(r.Context())
	h.writeConnection(w, r, c, err)
}

// RotateMetaWebhookKey implements api.ServerInterface.
func (h *Handler) RotateMetaWebhookKey(w http.ResponseWriter, r *http.Request) {
	var req api.RotateMetaWebhookKeyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	c, err := h.svc.RotateWebhookKey(r.Context(), time.Duration(req.GraceHours)*time.Hour)
	h.writeConnection(w, r, c, err)
}

// RotateMetaVerifyToken implements api.ServerInterface.
func (h *Handler) RotateMetaVerifyToken(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.RotateVerifyToken(r.Context())
	h.writeConnection(w, r, c, err)
}

// TestWhatsAppAccount implements api.ServerInterface.
func (h *Handler) TestWhatsAppAccount(w http.ResponseWriter, r *http.Request, id api.AccountId) {
	res, err := h.svc.TestConnection(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	checks := make([]api.MetaConnectionCheck, len(res.Checks))
	for i, c := range res.Checks {
		params := c.Params
		if params == nil {
			params = map[string]string{}
		}
		checks[i] = api.MetaConnectionCheck{Key: api.MetaConnectionCheckKey(c.Key), Status: api.MetaConnectionCheckStatus(c.Status),
			Code: c.Code, Params: params, Message: c.Message}
	}
	httpx.JSON(w, r, http.StatusOK, api.MetaConnectionTest{Ok: res.OK(), Checks: checks, PhoneNumbers: res.PhoneNumbers,
		Subscribed: res.Subscribed, SubscribedNow: res.SubscribedNow, Account: accountToAPI(res.Account)})
}

// ListTemplates implements api.ServerInterface.
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request, p api.ListTemplatesParams) {
	rows, err := h.svc.q.ListTemplates(r.Context(), db.ListTemplatesParams{AccountID: p.AccountId, Status: p.Status})
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.Template, len(rows))
	for i, t := range rows {
		out[i] = templateToAPI(db.MessageTemplate{ID: t.ID, AccountID: t.AccountID, MetaTemplateID: t.MetaTemplateID, Name: t.Name,
			Language: t.Language, Category: t.Category, Status: t.Status, ParameterFormat: t.ParameterFormat, Components: t.Components,
			RejectedReason: t.RejectedReason, QualityScore: t.QualityScore, SyncedAt: t.SyncedAt}, t.AccountName)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateTemplate implements api.ServerInterface.
func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request, _ api.CreateTemplateParams) {
	var req api.CreateTemplateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	comps, _ := json.Marshal(req.Components)
	in := TemplateInput{AccountID: req.AccountId, Name: req.Name, Language: req.Language, Category: string(req.Category), Components: comps}
	if req.ParameterFormat != nil {
		in.ParameterFormat = string(*req.ParameterFormat)
	}
	t, err := h.svc.CreateTemplate(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, templateToAPI(t, ""))
}

// DeleteTemplate implements api.ServerInterface.
func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request, id api.TemplateId) {
	if err := h.svc.DeleteTemplate(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.NoContent(w)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// UploadTemplateSample implements api.ServerInterface; the body is the file.
func (h *Handler) UploadTemplateSample(w http.ResponseWriter, r *http.Request, p api.UploadTemplateSampleParams) {
	handle, mime, size, err := h.svc.UploadTemplateSample(r.Context(), p.AccountId, p.Filename, r.Header.Get("Content-Type"), r.Body)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, api.TemplateSample{Handle: handle, MimeType: mime, Size: size})
}
