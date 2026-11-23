package realtime

import (
	"log/slog"
	"net/http"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the realtime token operations.
type Handler struct {
	issuer *Issuer
	log    *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(issuer *Issuer, log *slog.Logger) *Handler { return &Handler{issuer: issuer, log: log} }

// CreateRealtimeConnectionToken implements api.ServerInterface.
func (h *Handler) CreateRealtimeConnectionToken(w http.ResponseWriter, r *http.Request) {
	t, err := h.issuer.Connection(principal.From(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.RealtimeToken{Token: t.Token, ExpiresAt: t.ExpiresAt.UTC()})
}

// CreateRealtimeSubscriptionToken implements api.ServerInterface.
func (h *Handler) CreateRealtimeSubscriptionToken(w http.ResponseWriter, r *http.Request) {
	var req api.RealtimeSubscriptionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	t, err := h.issuer.SubscriptionContext(r.Context(), principal.From(r.Context()), req.Channel)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.RealtimeToken{Token: t.Token, ExpiresAt: t.ExpiresAt.UTC()})
}
