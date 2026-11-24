package channels

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// WebhookFields are the webhook fields Messora processes.
var WebhookFields = []string{"messages", "message_template_status_update", "phone_number_quality_update"}

// Handler serves the channel and template operations.
type Handler struct {
	svc         *Service
	log         *slog.Logger
	callbackURL string
	verifyToken string
}

// NewHandler returns the handler.
func NewHandler(svc *Service, publicURL, verifyToken string, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, callbackURL: publicURL + "/webhooks/whatsapp", verifyToken: verifyToken}
}

func accountToAPI(a db.WhatsappAccount) api.WhatsAppAccount {
	return api.WhatsAppAccount{Id: a.ID, Name: a.Name, WabaId: a.WabaID, AppId: a.AppID,
		Status: api.WhatsAppAccountStatus(a.Status), LastError: a.LastError, AppSecretConfigured: len(a.AppSecretEnc) > 0,
		LastSyncedAt: a.LastSyncedAt, CreatedAt: a.CreatedAt, Token: tokenToAPI(a)}
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
	in := AccountInput{Name: req.Name, WABAID: req.WabaId, AccessToken: deref(req.AccessToken), AppID: deref(req.AppId), AppSecret: deref(req.AppSecret)}
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
			MessagingLimit: p.MessagingLimit, ThroughputLevel: &p.ThroughputLevel, IsDefault: p.IsDefault}
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

// GetWebhookSetup implements api.ServerInterface.
func (h *Handler) GetWebhookSetup(w http.ResponseWriter, r *http.Request) {
	secrets, err := h.svc.AppSecrets(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	fields := append([]string(nil), WebhookFields...)
	httpx.JSON(w, r, http.StatusOK, api.WebhookSetup{CallbackUrl: h.callbackURL, VerifyToken: h.verifyToken,
		SignatureVerification: len(secrets) > 0, Fields: &fields})
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
