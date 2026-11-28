package webhooks

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the outgoing webhook operations.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

func endpointToAPI(ep db.WebhookEndpoint) api.WebhookEndpoint {
	types := make([]api.WebhookEventType, len(ep.EventTypes))
	for i, t := range ep.EventTypes {
		types[i] = api.WebhookEventType(t)
	}
	return api.WebhookEndpoint{Id: ep.ID, Url: ep.Url, Description: ep.Description, EventTypes: types, Active: ep.Active,
		SecretLast4: ep.SecretLast4, DisabledReason: ep.DisabledReason, DisabledAt: ep.DisabledAt, FailureStreak: int(ep.FailureStreak),
		FailingSince: ep.FailingSince, LastSuccessAt: ep.LastSuccessAt, LastFailureAt: ep.LastFailureAt,
		PreviousSecretExpiresAt: activeUntil(ep.PreviousSecretExpiresAt), CreatedAt: ep.CreatedAt, UpdatedAt: ep.UpdatedAt}
}

// activeUntil hides an overlap that has already ended.
func activeUntil(t *time.Time) *time.Time {
	if t == nil || !time.Now().Before(*t) {
		return nil
	}
	return t
}

func intPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func deliveryToAPI(d db.ListWebhookDeliveriesRow) api.WebhookDelivery {
	out := api.WebhookDelivery{Id: d.ID, EventId: d.EventID, EventType: d.EventType, EndpointId: d.EndpointID,
		Status: api.WebhookDeliveryStatus(d.Status), Attempts: int(d.Attempts), LastAttemptAt: d.LastAttemptAt,
		LastStatusCode: intPtr(d.LastStatusCode), LastDurationMs: intPtr(d.LastDurationMs), LastError: d.LastError,
		LastResponseBody: d.LastResponseBody, DeliveredAt: d.DeliveredAt, CreatedAt: d.CreatedAt}
	if d.Status == "pending" || d.Status == "retrying" {
		next := d.NextAttemptAt
		out.NextAttemptAt = &next
	}
	return out
}

// deliveryRow adds the event type to a plain delivery row.
func (h *Handler) deliveryRow(r *http.Request, d db.WebhookDelivery) (db.ListWebhookDeliveriesRow, error) {
	ev, err := h.svc.q.GetOutgoingWebhookEvent(r.Context(), d.EventID)
	if err != nil {
		return db.ListWebhookDeliveriesRow{}, err
	}
	return db.ListWebhookDeliveriesRow{ID: d.ID, EventID: d.EventID, EndpointID: d.EndpointID, Status: d.Status, Attempts: d.Attempts,
		NextAttemptAt: d.NextAttemptAt, LockedUntil: d.LockedUntil, LastAttemptAt: d.LastAttemptAt, LastStatusCode: d.LastStatusCode,
		LastDurationMs: d.LastDurationMs, LastError: d.LastError, LastResponseBody: d.LastResponseBody, DeliveredAt: d.DeliveredAt,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, EventType: ev.Type}, nil
}

func (h *Handler) writeDelivery(w http.ResponseWriter, r *http.Request, status int, d db.WebhookDelivery, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	row, err := h.deliveryRow(r, d)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, status, deliveryToAPI(row))
}

func typesIn(in []api.WebhookEventType) *[]string {
	out := make([]string, len(in))
	for i, t := range in {
		out[i] = string(t)
	}
	return &out
}

func actor(r *http.Request) *uuid.UUID {
	p := principal.From(r.Context())
	if p == nil || p.Kind != principal.KindUser {
		return nil
	}
	return &p.ID
}

// ListWebhookEventTypes implements api.ServerInterface.
func (h *Handler) ListWebhookEventTypes(w http.ResponseWriter, r *http.Request) {
	out := make([]api.WebhookEventTypeInfo, len(Catalogue))
	for i, e := range Catalogue {
		out[i] = api.WebhookEventTypeInfo{Type: api.WebhookEventType(e.Type), Description: e.Description}
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// ListWebhookEndpoints implements api.ServerInterface.
func (h *Handler) ListWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
	eps, err := h.svc.List(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.WebhookEndpoint, len(eps))
	for i, ep := range eps {
		out[i] = endpointToAPI(ep)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateWebhookEndpoint implements api.ServerInterface.
func (h *Handler) CreateWebhookEndpoint(w http.ResponseWriter, r *http.Request, _ api.CreateWebhookEndpointParams) {
	var req api.CreateWebhookEndpointRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	ep, secret, err := h.svc.Create(r.Context(), EndpointInput{URL: &req.Url, Description: req.Description,
		EventTypes: typesIn(req.EventTypes), Active: req.Active}, actor(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, api.WebhookEndpointSecret{Endpoint: endpointToAPI(ep), Secret: secret})
}

// GetWebhookEndpoint implements api.ServerInterface.
func (h *Handler) GetWebhookEndpoint(w http.ResponseWriter, r *http.Request, id api.EndpointId) {
	ep, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, endpointToAPI(ep))
}

// UpdateWebhookEndpoint implements api.ServerInterface.
func (h *Handler) UpdateWebhookEndpoint(w http.ResponseWriter, r *http.Request, id api.EndpointId) {
	var req api.UpdateWebhookEndpointRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in := EndpointInput{URL: req.Url, Description: req.Description, Active: req.Active}
	if req.EventTypes != nil {
		in.EventTypes = typesIn(*req.EventTypes)
	}
	ep, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, endpointToAPI(ep))
}

// DeleteWebhookEndpoint implements api.ServerInterface.
func (h *Handler) DeleteWebhookEndpoint(w http.ResponseWriter, r *http.Request, id api.EndpointId) {
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

// RotateWebhookSecret implements api.ServerInterface.
func (h *Handler) RotateWebhookSecret(w http.ResponseWriter, r *http.Request, id api.EndpointId) {
	grace := 24
	if r.ContentLength != 0 {
		var req api.RotateWebhookSecretRequest
		if err := httpx.Decode(r, &req); err != nil && r.ContentLength > 0 {
			h.fail(w, r, err)
			return
		} else if err == nil && req.GraceHours != nil {
			grace = *req.GraceHours
		}
	}
	ep, secret, err := h.svc.RotateSecret(r.Context(), id, time.Duration(grace)*time.Hour)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, api.WebhookEndpointSecret{Endpoint: endpointToAPI(ep), Secret: secret})
}

// SendWebhookTest implements api.ServerInterface.
func (h *Handler) SendWebhookTest(w http.ResponseWriter, r *http.Request, id api.EndpointId) {
	d, err := h.svc.SendTest(r.Context(), id)
	h.writeDelivery(w, r, http.StatusCreated, d, err)
}

// ListWebhookDeliveries implements api.ServerInterface.
func (h *Handler) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request, id api.EndpointId, p api.ListWebhookDeliveriesParams) {
	limit := 50
	if p.Limit != nil {
		limit = *p.Limit
	}
	var before *uuid.UUID
	if p.Cursor != nil {
		c, err := uuid.Parse(*p.Cursor)
		if err != nil {
			h.fail(w, r, apperr.Validation("cursor", "Invalid cursor"))
			return
		}
		before = &c
	}
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	rows, err := h.svc.Deliveries(r.Context(), id, status, before, int32(limit+1)) //nolint:gosec // limit ≤ 200 (spec)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	out := make([]api.WebhookDelivery, len(rows))
	for i, d := range rows {
		out[i] = deliveryToAPI(d)
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// RedeliverWebhook implements api.ServerInterface.
func (h *Handler) RedeliverWebhook(w http.ResponseWriter, r *http.Request, id api.DeliveryId) {
	d, err := h.svc.Redeliver(r.Context(), id)
	h.writeDelivery(w, r, http.StatusOK, d, err)
}
