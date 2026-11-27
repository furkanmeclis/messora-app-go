package settings

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the settings operations.
type Handler struct {
	pool   *pgxpool.Pool
	store  *Store
	mail   *Mail
	stores *storage.Stores
	log    *slog.Logger
}

// NewHandler returns the settings handler. mail may be nil when e-mail is
// injected by tests; the SMTP operations then report it as unavailable.
func NewHandler(pool *pgxpool.Pool, store *Store, mail *Mail, stores *storage.Stores, log *slog.Logger) *Handler {
	return &Handler{pool: pool, store: store, mail: mail, stores: stores, log: log}
}

func logoToAPI(l *Logo) *api.LogoInfo {
	if l == nil {
		return nil
	}
	return &api.LogoInfo{ContentType: l.ContentType, Version: l.Version}
}

func toAPI(s Settings) api.Settings {
	roles := make([]api.RoleKey, len(s.MFARequiredRoles))
	for i, r := range s.MFARequiredRoles {
		roles[i] = api.RoleKey(r)
	}
	steps := make([]api.SetupStep, len(s.Setup.Steps))
	for i, st := range s.Setup.Steps {
		steps[i] = api.SetupStep(st)
	}
	locked := s.Locked
	if locked == nil {
		locked = []string{}
	}
	return api.Settings{OrganizationName: s.OrganizationName, DefaultTimezone: s.DefaultTimezone, MfaRequiredRoles: roles,
		SessionIdleHours: s.SessionIdleHours, SessionLifetimeDays: s.SessionLifetimeDays,
		DefaultLocale: api.Locale(s.DefaultLocale), DateFormat: api.DateFormat(s.DateFormat), TimeFormat: api.TimeFormat(s.TimeFormat),
		Logo: logoToAPI(s.Logo), Setup: api.SetupProgress{Completed: s.Setup.Completed, Steps: steps},
		Locked: locked}
}

// GetAppearance implements api.ServerInterface. Every page needs it, the
// sign-in page included, so it carries nothing confidential.
func (h *Handler) GetAppearance(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.Get(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.Appearance{OrganizationName: s.OrganizationName, DefaultLocale: api.Locale(s.DefaultLocale),
		DefaultTimezone: s.DefaultTimezone, DateFormat: api.DateFormat(s.DateFormat), TimeFormat: api.TimeFormat(s.TimeFormat),
		Logo: logoToAPI(s.Logo), SetupCompleted: s.Setup.Completed})
}

// GetSettings implements api.ServerInterface.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.Get(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, toAPI(s))
}

// UpdateSettings implements api.ServerInterface.
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req api.UpdateSettingsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	actor := principal.From(r.Context())
	p := Patch{OrganizationName: req.OrganizationName, DefaultTimezone: req.DefaultTimezone,
		SessionIdleHours: req.SessionIdleHours, SessionLifetimeDays: req.SessionLifetimeDays, SetupCompleted: req.SetupCompleted}
	if req.MfaRequiredRoles != nil {
		roles := make([]string, len(*req.MfaRequiredRoles))
		for i, rk := range *req.MfaRequiredRoles {
			roles[i] = string(rk)
		}
		p.MFARequiredRoles = &roles
	}
	if req.DefaultLocale != nil {
		v := string(*req.DefaultLocale)
		p.DefaultLocale = &v
	}
	if req.DateFormat != nil {
		v := string(*req.DateFormat)
		p.DateFormat = &v
	}
	if req.TimeFormat != nil {
		v := string(*req.TimeFormat)
		p.TimeFormat = &v
	}
	if req.SetupSteps != nil {
		steps := make([]string, len(*req.SetupSteps))
		for i, st := range *req.SetupSteps {
			steps[i] = string(st)
		}
		p.SetupSteps = &steps
	}
	var updated Settings
	err := inTx(r.Context(), h.pool, func(q *db.Queries) error {
		old, upd, err := h.store.Update(r.Context(), q, p, &actor.ID)
		if err != nil {
			return err
		}
		updated = upd
		return audit.Record(r.Context(), q, audit.Entry{Action: "settings.update", EntityType: "settings", Changes: changes(old, upd)})
	})
	h.store.Invalidate()
	if err != nil {
		httpx.WriteError(w, r, settingsError(err), h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, toAPI(updated))
}

func settingsError(err error) error {
	switch {
	case errors.Is(err, ErrLocked):
		return apperr.Conflict("SETTING_MANAGED_BY_ENV", strings.TrimPrefix(err.Error(), "settings: ")+".")
	case errors.Is(err, ErrSMTPManagedByEnv):
		return apperr.Conflict("SMTP_MANAGED_BY_ENV", "E-mail is configured by environment variables (SMTP_HOST).")
	case errors.Is(err, ErrInvalid):
		return apperr.Validation("", strings.TrimPrefix(err.Error(), "settings: invalid: "))
	}
	return err
}

func changes(a, b Settings) map[string]any {
	out := map[string]any{}
	str := func(name, x, y string) {
		if x != y {
			out[name] = []string{x, y}
		}
	}
	str("organization_name", a.OrganizationName, b.OrganizationName)
	str("default_timezone", a.DefaultTimezone, b.DefaultTimezone)
	str("default_locale", a.DefaultLocale, b.DefaultLocale)
	str("date_format", a.DateFormat, b.DateFormat)
	str("time_format", a.TimeFormat, b.TimeFormat)
	if !slices.Equal(a.MFARequiredRoles, b.MFARequiredRoles) {
		out["mfa_required_roles"] = []any{a.MFARequiredRoles, b.MFARequiredRoles}
	}
	if a.SessionIdleHours != b.SessionIdleHours {
		out["session_idle_hours"] = []int{a.SessionIdleHours, b.SessionIdleHours}
	}
	if a.SessionLifetimeDays != b.SessionLifetimeDays {
		out["session_lifetime_days"] = []int{a.SessionLifetimeDays, b.SessionLifetimeDays}
	}
	if a.Setup.Completed != b.Setup.Completed {
		out["setup_completed"] = []bool{a.Setup.Completed, b.Setup.Completed}
	}
	return out
}

// Logo storage -------------------------------------------------------------

const maxLogoBytes = 2 << 20

// LogoKey is the public object of a logo version. The key changes with
// every upload, so a CDN never serves a stale logo.
func LogoKey(tenantID, version string) string {
	return storage.TenantPrefix(tenantID) + "branding/logo-" + version
}

// legacyLogoKey is where logos were kept before the object store (MEDIA_DIR/branding/logo).
const legacyLogoKey = "branding/logo"

func (h *Handler) currentLogo(r *http.Request) *Logo {
	if s, err := h.store.Get(r.Context()); err == nil {
		return s.Logo
	}
	return nil
}

func (h *Handler) removeLogo(r *http.Request, old *Logo, keep string) {
	if old == nil {
		return
	}
	key := LogoKey(storage.TenantKey(r.Context()), old.Version)
	if key == keep {
		return
	}
	if err := h.stores.Public.Delete(context.WithoutCancel(r.Context()), key); err != nil {
		h.log.WarnContext(r.Context(), "removing logo file", "error", err)
	}
}

// sniffLogo returns the image type, or "" for anything that is not a
// PNG, JPEG, WebP or SVG image.
func sniffLogo(b []byte) string {
	switch ct := http.DetectContentType(b); ct {
	case "image/png", "image/jpeg", "image/webp":
		return ct
	}
	head := strings.ToLower(string(b[:min(len(b), 1024)]))
	head = strings.TrimSpace(strings.TrimPrefix(head, "\ufeff"))
	if strings.HasPrefix(head, "<svg") || (strings.HasPrefix(head, "<?xml") && strings.Contains(head, "<svg")) {
		return "image/svg+xml"
	}
	return ""
}

// UploadLogo implements api.ServerInterface.
func (h *Handler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxLogoBytes+1))
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	if len(body) > maxLogoBytes {
		httpx.WriteError(w, r, apperr.Validation("", "The logo must be 2 MB or smaller."), h.log)
		return
	}
	ct := sniffLogo(body)
	if ct == "" {
		httpx.WriteError(w, r, apperr.Validation("", "The logo must be a PNG, JPEG, WebP or SVG image."), h.log)
		return
	}
	sum := sha256.Sum256(body)
	logo := &Logo{ContentType: ct, Version: hex.EncodeToString(sum[:8])}
	old := h.currentLogo(r)
	key := LogoKey(storage.TenantKey(r.Context()), logo.Version)
	if err := h.stores.Public.Put(r.Context(), key, bytes.NewReader(body), int64(len(body)), ct,
		storage.PutOptions{CacheControl: "public, max-age=31536000, immutable"}); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	if h.saveLogo(w, r, logo, "settings.logo_upload") {
		h.removeLogo(r, old, key)
	}
}

// DeleteLogo implements api.ServerInterface.
func (h *Handler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	old := h.currentLogo(r)
	if h.saveLogo(w, r, nil, "settings.logo_delete") {
		h.removeLogo(r, old, "")
		if storage.TenantKey(r.Context()) == storage.LegacyTenant {
			_ = h.stores.Private.Delete(context.WithoutCancel(r.Context()), legacyLogoKey)
		}
	}
}

// saveLogo stores the logo setting and answers with the settings; it
// reports whether the change was saved.
func (h *Handler) saveLogo(w http.ResponseWriter, r *http.Request, logo *Logo, action string) bool {
	actor := principal.From(r.Context())
	err := inTx(r.Context(), h.pool, func(q *db.Queries) error {
		if err := h.store.SetLogo(r.Context(), q, logo, &actor.ID); err != nil {
			return err
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: action, EntityType: "settings"})
	})
	h.store.Invalidate()
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return false
	}
	s, err := h.store.Get(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return true
	}
	httpx.JSON(w, r, http.StatusOK, toAPI(s))
	return true
}

// GetLogo implements api.ServerInterface.
func (h *Handler) GetLogo(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.Get(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	if s.Logo == nil {
		httpx.WriteError(w, r, apperr.NotFound("No logo has been uploaded."), h.log)
		return
	}
	// Streamed, never redirected to the CDN: the panel's BFF fetches this
	// route server-side, and an SVG must keep the sandboxing policy below.
	// PUBLIC_ASSET_BASE_URL serves the same object to e-mails and Meta
	// (LogoKey, storage.Store.PublicURL).
	key := LogoKey(storage.TenantKey(r.Context()), s.Logo.Version)
	rc, _, err := h.stores.Public.Get(r.Context(), key)
	if errors.Is(err, storage.ErrNotFound) && storage.TenantKey(r.Context()) == storage.LegacyTenant {
		rc, _, err = h.stores.Private.Get(r.Context(), legacyLogoKey)
	}
	if errors.Is(err, storage.ErrNotFound) {
		httpx.WriteError(w, r, apperr.NotFound("No logo has been uploaded."), h.log)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	defer func() { _ = rc.Close() }()
	w.Header().Set("Content-Type", s.Logo.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An SVG opened directly must not run scripts.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", `"`+s.Logo.Version+`"`)
	if r.Header.Get("If-None-Match") == `"`+s.Logo.Version+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = io.Copy(w, rc)
}

// E-mail -------------------------------------------------------------------

func smtpToAPI(v SMTPView) api.SmtpSettings {
	tls := v.TLS
	if tls == "" {
		tls = "starttls"
	}
	return api.SmtpSettings{Host: v.Host, Port: v.Port, Username: v.Username, From: v.From, Tls: api.SmtpTls(tls),
		PasswordSet: v.PasswordSet, ManagedByEnv: v.ManagedByEnv, Enabled: v.Enabled}
}

func smtpInput(in api.SmtpSettingsInput) SMTPInput {
	return SMTPInput{Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password, From: in.From, TLS: string(in.Tls)}
}

func (h *Handler) mailUnavailable(w http.ResponseWriter, r *http.Request) bool {
	if h.mail == nil {
		httpx.WriteError(w, r, apperr.Unavailable("MAIL_UNAVAILABLE", "E-mail settings are not available."), h.log)
		return true
	}
	return false
}

// GetSmtpSettings implements api.ServerInterface.
func (h *Handler) GetSmtpSettings(w http.ResponseWriter, r *http.Request) {
	if h.mailUnavailable(w, r) {
		return
	}
	v, err := h.mail.View(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, smtpToAPI(v))
}

// UpdateSmtpSettings implements api.ServerInterface.
func (h *Handler) UpdateSmtpSettings(w http.ResponseWriter, r *http.Request) {
	if h.mailUnavailable(w, r) {
		return
	}
	var req api.SmtpSettingsInput
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	actor := principal.From(r.Context())
	err := inTx(r.Context(), h.pool, func(q *db.Queries) error {
		c, err := h.mail.Prepare(r.Context(), q, smtpInput(req))
		if err != nil {
			return err
		}
		if err := h.mail.Save(r.Context(), q, c, &actor.ID); err != nil {
			return err
		}
		// The password is never written to the audit log.
		return audit.Record(r.Context(), q, audit.Entry{Action: "settings.smtp_update", EntityType: "settings",
			Changes: map[string]any{"host": c.Host, "port": c.Port, "username": c.Username, "from": c.From, "tls": c.TLS,
				"password_changed": req.Password != nil}})
	})
	h.mail.Invalidate()
	if err != nil {
		httpx.WriteError(w, r, settingsError(err), h.log)
		return
	}
	h.GetSmtpSettings(w, r)
}

// TestSmtpSettings implements api.ServerInterface.
func (h *Handler) TestSmtpSettings(w http.ResponseWriter, r *http.Request) {
	if h.mailUnavailable(w, r) {
		return
	}
	var req api.SmtpTestRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	var in *SMTPInput
	if req.Config != nil {
		v := smtpInput(*req.Config)
		in = &v
	}
	s, err := h.store.Get(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	if err := h.mail.Test(r.Context(), in, string(req.To), s.OrganizationName); err != nil {
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrSMTPManagedByEnv) {
			httpx.WriteError(w, r, settingsError(err), h.log)
			return
		}
		// The server's answer helps fix the configuration; it holds no secret.
		httpx.WriteError(w, r, apperr.Validation("", "The test e-mail could not be sent: "+err.Error()), h.log)
		return
	}
	httpx.NoContent(w)
}

func inTx(ctx context.Context, pool *pgxpool.Pool, fn func(*db.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
