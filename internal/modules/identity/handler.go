package identity

import (
	"log/slog"
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler implements the identity operations of api.ServerInterface.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the HTTP handler for s.
func NewHandler(s *Service, log *slog.Logger) *Handler { return &Handler{svc: s, log: log} }

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

// decode reads the body into v; on failure it writes the error and returns false.
func decode[T any](h *Handler, w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		h.fail(w, r, err)
		return v, false
	}
	return v, true
}

// ── Setup ──────────────────────────────────────────────────────────────────

// GetSetupStatus implements api.ServerInterface.
func (h *Handler) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	req, err := h.svc.SetupRequired(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.SetupStatus{SetupRequired: req})
}

// CompleteSetup implements api.ServerInterface.
func (h *Handler) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.CompleteSetupRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.CompleteSetup(r.Context(), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, out)
}

// ── Auth ───────────────────────────────────────────────────────────────────

// GetAuthCapabilities implements api.ServerInterface.
func (h *Handler) GetAuthCapabilities(w http.ResponseWriter, r *http.Request) {
	setup, err := h.svc.SetupRequired(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.AuthCapabilities{PasswordReset: h.svc.PasswordResetEnabled(r.Context()), SetupRequired: setup})
}

// Login implements api.ServerInterface.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.LoginRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.Login(r.Context(), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// LoginMfa implements api.ServerInterface.
func (h *Handler) LoginMfa(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.LoginMfaRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.LoginMFA(r.Context(), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// Logout implements api.ServerInterface.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), principal.From(r.Context())); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// Reauthenticate implements api.ServerInterface.
func (h *Handler) Reauthenticate(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.ReauthRequest](h, w, r)
	if !ok {
		return
	}
	until, err := h.svc.Reauthenticate(r.Context(), principal.From(r.Context()), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.ReauthResult{ValidUntil: until})
}

// RequestPasswordReset implements api.ServerInterface.
func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.ForgotPasswordRequest](h, w, r)
	if !ok {
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), req); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// ResetPassword implements api.ServerInterface.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.ResetPasswordRequest](h, w, r)
	if !ok {
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ── Me ─────────────────────────────────────────────────────────────────────

// GetMe implements api.ServerInterface.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	me, err := h.svc.Me(r.Context(), principal.From(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, me)
}

// UpdateMe implements api.ServerInterface.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.UpdateMeRequest](h, w, r)
	if !ok {
		return
	}
	me, err := h.svc.UpdateMe(r.Context(), principal.From(r.Context()), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, me)
}

// ChangePassword implements api.ServerInterface.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.ChangePasswordRequest](h, w, r)
	if !ok {
		return
	}
	if err := h.svc.ChangePassword(r.Context(), principal.From(r.Context()), req); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// ListMySessions implements api.ServerInterface.
func (h *Handler) ListMySessions(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSessions(r.Context(), principal.From(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// RevokeOtherSessions implements api.ServerInterface.
func (h *Handler) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeOtherSessions(r.Context(), principal.From(r.Context())); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// RevokeMySession implements api.ServerInterface.
func (h *Handler) RevokeMySession(w http.ResponseWriter, r *http.Request, sessionID api.SessionId) {
	if err := h.svc.RevokeSession(r.Context(), principal.From(r.Context()), sessionID); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// StartTotpSetup implements api.ServerInterface.
func (h *Handler) StartTotpSetup(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.StartTOTPSetup(r.Context(), principal.From(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ConfirmTotp implements api.ServerInterface.
func (h *Handler) ConfirmTotp(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.TotpConfirmRequest](h, w, r)
	if !ok {
		return
	}
	codes, err := h.svc.ConfirmTOTP(r.Context(), principal.From(r.Context()), req.Code)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.RecoveryCodes{Codes: codes})
}

// DisableTotp implements api.ServerInterface.
func (h *Handler) DisableTotp(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DisableTOTP(r.Context(), principal.From(r.Context())); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// RegenerateRecoveryCodes implements api.ServerInterface.
func (h *Handler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	codes, err := h.svc.RegenerateRecoveryCodes(r.Context(), principal.From(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.RecoveryCodes{Codes: codes})
}

// ── Users and invitations ──────────────────────────────────────────────────

// ListUsers implements api.ServerInterface.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListUsers(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// UpdateUser implements api.ServerInterface.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request, userID api.UserId) {
	req, ok := decode[api.UpdateUserRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.UpdateUser(r.Context(), principal.From(r.Context()), userID, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ListInvitations implements api.ServerInterface.
func (h *Handler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListInvitations(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateInvitation implements api.ServerInterface.
func (h *Handler) CreateInvitation(w http.ResponseWriter, r *http.Request, _ api.CreateInvitationParams) {
	req, ok := decode[api.CreateInvitationRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.CreateInvitation(r.Context(), principal.From(r.Context()), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, out)
}

// RevokeInvitation implements api.ServerInterface.
func (h *Handler) RevokeInvitation(w http.ResponseWriter, r *http.Request, id api.InvitationId) {
	if err := h.svc.RevokeInvitation(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// LookupInvitation implements api.ServerInterface.
func (h *Handler) LookupInvitation(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.InvitationTokenRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.LookupInvitation(r.Context(), req.Token)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// AcceptInvitation implements api.ServerInterface.
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[api.AcceptInvitationRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.AcceptInvitation(r.Context(), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, out)
}

// ListRoles implements api.ServerInterface.
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListRoles(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ── API keys ───────────────────────────────────────────────────────────────

// ListApiKeys implements api.ServerInterface.
func (h *Handler) ListApiKeys(w http.ResponseWriter, r *http.Request) { //nolint:revive // name fixed by the generated interface
	out, err := h.svc.ListAPIKeys(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateApiKey implements api.ServerInterface.
func (h *Handler) CreateApiKey(w http.ResponseWriter, r *http.Request, _ api.CreateApiKeyParams) { //nolint:revive // generated name
	req, ok := decode[api.CreateApiKeyRequest](h, w, r)
	if !ok {
		return
	}
	out, err := h.svc.CreateAPIKey(r.Context(), principal.From(r.Context()), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, out)
}

// RevokeApiKey implements api.ServerInterface.
func (h *Handler) RevokeApiKey(w http.ResponseWriter, r *http.Request, id api.ApiKeyId) { //nolint:revive // generated name
	if err := h.svc.RevokeAPIKey(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// GetApiKeySelf implements api.ServerInterface.
func (h *Handler) GetApiKeySelf(w http.ResponseWriter, r *http.Request) { //nolint:revive // generated name
	out, err := h.svc.APIKeySelf(r.Context(), principal.From(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, out)
}
