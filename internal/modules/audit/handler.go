package audit

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Handler serves the audit log.
type Handler struct {
	q   db.Querier
	log *slog.Logger
}

// NewHandler returns the audit handler.
func NewHandler(q db.Querier, log *slog.Logger) *Handler { return &Handler{q: q, log: log} }

// ListAuditLogs implements api.ServerInterface. Pagination is keyset-based on
// the time-ordered UUIDv7 primary key; the cursor is the last ID returned.
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request, params api.ListAuditLogsParams) {
	limit := 50
	if params.Limit != nil {
		limit = *params.Limit
	}
	var before *uuid.UUID
	if params.Cursor != nil {
		id, err := uuid.Parse(*params.Cursor)
		if err != nil {
			httpx.WriteError(w, r, apperr.Validation("cursor", "Invalid cursor"), h.log)
			return
		}
		before = &id
	}
	rows, err := h.q.ListAuditLogs(r.Context(), db.ListAuditLogsParams{
		BeforeID: before, Action: params.Action, ActorID: params.ActorId,
		EntityType: params.EntityType, EntityID: params.EntityId, PageSize: int32(limit + 1), //nolint:gosec // limit ≤ 200 by schema
	})
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID.String()
	}
	out := make([]api.AuditLog, 0, len(rows))
	for _, a := range rows {
		var changes map[string]any
		_ = json.Unmarshal(a.Changes, &changes)
		if changes == nil {
			changes = map[string]any{}
		}
		var actorName *string
		if a.ActorName != "" {
			n := a.ActorName
			actorName = &n
		}
		var ip *string
		if a.Ip != nil {
			v := a.Ip.String()
			ip = &v
		}
		out = append(out, api.AuditLog{
			Id: a.ID, OccurredAt: a.OccurredAt, Action: a.Action, EntityType: a.EntityType, EntityId: a.EntityID,
			Ip: ip, RequestId: a.RequestID, Changes: changes,
			Actor: api.AuditActor{Type: api.AuditActorType(a.ActorType), Id: a.ActorID, Name: actorName},
		})
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}
