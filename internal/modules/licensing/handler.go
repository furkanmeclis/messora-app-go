package licensing

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/license"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the licence operations.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

type licenseView struct {
	ID              string     `json:"id"`
	Customer        string     `json:"customer"`
	Plan            string     `json:"plan"`
	IssuedAt        time.Time  `json:"issued_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	UpdatesUntil    *time.Time `json:"updates_until"`
	CheckIn         string     `json:"check_in"`
	MaxUsers        int        `json:"max_users"`
	MaxPhoneNumbers int        `json:"max_phone_numbers"`
}

type releaseView struct {
	Version       string    `json:"version"`
	ReleasedAt    time.Time `json:"released_at"`
	Channel       string    `json:"channel"`
	Notes         string    `json:"notes,omitempty"`
	UpdateAllowed bool      `json:"update_allowed"`
}

type infoView struct {
	Status            license.Status `json:"status"`
	Reason            string         `json:"reason"`
	Until             *time.Time     `json:"until"`
	InstanceID        string         `json:"instance_id"`
	InstalledAt       time.Time      `json:"installed_at"`
	LastCheckInAt     *time.Time     `json:"last_check_in_at"`
	CheckInConfigured bool           `json:"check_in_configured"`
	CurrentVersion    string         `json:"current_version"`
	License           *licenseView   `json:"license"`
	LatestRelease     *releaseView   `json:"latest_release"`
}

func (h *Handler) view(r *http.Request) (infoView, error) {
	st, stored, err := h.svc.State(r.Context())
	if err != nil {
		return infoView{}, err
	}
	v := infoView{Status: st.Status, Reason: st.Reason, Until: st.Until, InstanceID: stored.InstanceID,
		InstalledAt: stored.InstalledAt, LastCheckInAt: stored.LastCheckIn, CheckInConfigured: h.svc.o.GateURL != "", CurrentVersion: h.svc.o.Version}
	if p := st.License; p != nil {
		v.License = &licenseView{ID: p.ID, Customer: p.Customer, Plan: p.Plan, IssuedAt: p.IssuedAt, ExpiresAt: p.ExpiresAt,
			UpdatesUntil: p.UpdatesUntil, CheckIn: p.CheckIn, MaxUsers: p.Limits.Users, MaxPhoneNumbers: p.Limits.PhoneNumbers}
	}
	if l := stored.Latest; l != nil {
		allowed := st.License != nil && st.License.UpdatesAllowed(*l)
		v.LatestRelease = &releaseView{Version: l.Version, ReleasedAt: l.ReleasedAt, Channel: l.Channel, Notes: l.Notes, UpdateAllowed: allowed}
	}
	return v, nil
}

// Summary is the short status every page shows (banners).
func (h *Handler) Summary(r *http.Request) api.LicenseSummary {
	st, _, err := h.svc.State(r.Context())
	if err != nil {
		return api.LicenseSummary{Status: api.LicenseStatusActive}
	}
	return api.LicenseSummary{Status: api.LicenseStatus(st.Status), Reason: st.Reason, Until: st.Until}
}

// GetLicense implements api.ServerInterface.
func (h *Handler) GetLicense(w http.ResponseWriter, r *http.Request) {
	v, err := h.view(r)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, v)
}

// SetLicense implements api.ServerInterface.
func (h *Handler) SetLicense(w http.ResponseWriter, r *http.Request) {
	p := principal.From(r.Context())
	if p.RoleKey != "owner" {
		httpx.WriteError(w, r, apperr.Forbidden("OWNER_ONLY", "Only the owner can change the licence."), h.log)
		return
	}
	var req api.SetLicenseRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	if err := h.svc.SetKey(r.Context(), req.Key); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	// Report right away so a revoked or replaced key shows at once.
	if err := h.svc.CheckIn(r.Context()); err != nil {
		h.log.WarnContext(r.Context(), "licence check-in after entering a key failed", "err", err)
	}
	h.GetLicense(w, r)
}
