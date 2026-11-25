package offers

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the offer endpoints.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func toAPI(o db.Offer) api.Offer {
	return api.Offer{Id: o.ID, Title: o.Title, Description: o.Description, DiscountText: o.DiscountText, Product: o.Product,
		StartsAt: o.StartsAt, EndsAt: o.EndsAt, ImageUrl: o.ImageUrl, ImageMediaId: o.ImageMediaID, CtaUrl: o.CtaUrl,
		CouponCode: o.CouponCode, TemplateId: o.TemplateID, Status: api.OfferStatus(o.Status), CreatedBy: o.CreatedBy,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (h *Handler) write(w http.ResponseWriter, r *http.Request, status int, o db.Offer, err error) {
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	httpx.JSON(w, r, status, toAPI(o))
}

// ListOffers implements api.ServerInterface.
func (h *Handler) ListOffers(w http.ResponseWriter, r *http.Request, p api.ListOffersParams) {
	var status *string
	if p.Status != nil {
		s := string(*p.Status)
		status = &s
	}
	rows, err := h.svc.List(r.Context(), status)
	if err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	out := make([]api.Offer, len(rows))
	for i, o := range rows {
		out[i] = toAPI(o)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateOffer implements api.ServerInterface.
func (h *Handler) CreateOffer(w http.ResponseWriter, r *http.Request, _ api.CreateOfferParams) {
	var req api.OfferInput
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	f := Fields{Title: req.Title, Description: deref(req.Description), DiscountText: deref(req.DiscountText), Product: deref(req.Product),
		StartsAt: req.StartsAt, EndsAt: req.EndsAt, ImageURL: req.ImageUrl, ImageMediaID: req.ImageMediaId, CTAURL: req.CtaUrl,
		CouponCode: req.CouponCode, TemplateID: req.TemplateId}
	var actor *uuid.UUID
	if p := principal.From(r.Context()); p != nil && p.Kind == principal.KindUser {
		actor = &p.ID
	}
	o, err := h.svc.Create(r.Context(), f, req.Status != nil && *req.Status == api.OfferInputStatusScheduled, actor)
	h.write(w, r, http.StatusCreated, o, err)
}

// GetOffer implements api.ServerInterface.
func (h *Handler) GetOffer(w http.ResponseWriter, r *http.Request, id api.OfferId) {
	o, err := h.svc.Get(r.Context(), id)
	h.write(w, r, http.StatusOK, o, err)
}

// UpdateOffer implements api.ServerInterface.
func (h *Handler) UpdateOffer(w http.ResponseWriter, r *http.Request, id api.OfferId) {
	var req api.OfferUpdate
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	p := Patch{Title: req.Title, Description: req.Description, DiscountText: req.DiscountText, Product: req.Product,
		StartsAt: req.StartsAt, EndsAt: req.EndsAt, ImageURL: req.ImageUrl, ImageMediaID: req.ImageMediaId, CTAURL: req.CtaUrl,
		Coupon: req.CouponCode, TemplateID: req.TemplateId, ClearTemplate: req.ClearTemplate != nil && *req.ClearTemplate}
	if req.Status != nil {
		s := string(*req.Status)
		p.Status = &s
	}
	o, err := h.svc.Update(r.Context(), id, p)
	h.write(w, r, http.StatusOK, o, err)
}

// DeleteOffer implements api.ServerInterface.
func (h *Handler) DeleteOffer(w http.ResponseWriter, r *http.Request, id api.OfferId) {
	if err := h.svc.Delete(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err, h.log)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
