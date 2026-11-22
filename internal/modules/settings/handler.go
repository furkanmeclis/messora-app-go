package settings

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the settings operations.
type Handler struct {
	pool  *pgxpool.Pool
	store *Store
	log   *slog.Logger
}

// NewHandler returns the settings handler.
func NewHandler(pool *pgxpool.Pool, store *Store, log *slog.Logger) *Handler {
	return &Handler{pool: pool, store: store, log: log}
}

func toAPI(s Settings) api.Settings {
	roles := make([]api.RoleKey, len(s.MFARequiredRoles))
	for i, r := range s.MFARequiredRoles {
		roles[i] = api.RoleKey(r)
	}
	return api.Settings{OrganizationName: s.OrganizationName, DefaultTimezone: s.DefaultTimezone, MfaRequiredRoles: roles,
		SessionIdleHours: s.SessionIdleHours, SessionLifetimeDays: s.SessionLifetimeDays}
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
	p := Patch{OrganizationName: req.OrganizationName, DefaultTimezone: req.DefaultTimezone,
		SessionIdleHours: req.SessionIdleHours, SessionLifetimeDays: req.SessionLifetimeDays}
	if req.MfaRequiredRoles != nil {
		roles := make([]string, len(*req.MfaRequiredRoles))
		for i, rk := range *req.MfaRequiredRoles {
			roles[i] = string(rk)
		}
		p.MFARequiredRoles = &roles
	}
	actor := principal.From(r.Context())
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
		if errorsIsInvalid(err) {
			err = apperr.Validation("", err.Error())
		}
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, toAPI(updated))
}

func changes(a, b Settings) map[string]any {
	out := map[string]any{}
	if a.OrganizationName != b.OrganizationName {
		out["organization_name"] = []string{a.OrganizationName, b.OrganizationName}
	}
	if a.DefaultTimezone != b.DefaultTimezone {
		out["default_timezone"] = []string{a.DefaultTimezone, b.DefaultTimezone}
	}
	if !equalStrings(a.MFARequiredRoles, b.MFARequiredRoles) {
		out["mfa_required_roles"] = []any{a.MFARequiredRoles, b.MFARequiredRoles}
	}
	if a.SessionIdleHours != b.SessionIdleHours {
		out["session_idle_hours"] = []int{a.SessionIdleHours, b.SessionIdleHours}
	}
	if a.SessionLifetimeDays != b.SessionLifetimeDays {
		out["session_lifetime_days"] = []int{a.SessionLifetimeDays, b.SessionLifetimeDays}
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
