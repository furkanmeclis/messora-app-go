package campaigns

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// PauseReasonOfferExpired is the pause reason when a campaign's offer ends.
const PauseReasonOfferExpired = "offer_expired"

// offerEnded reports whether o can no longer be promoted at now.
func offerEnded(o db.Offer, now time.Time) bool {
	return o.Status == "expired" || !o.EndsAt.After(now)
}

// campaignOffer loads the campaign's offer, nil when it has none (or it was
// deleted meanwhile).
func campaignOffer(ctx context.Context, q *db.Queries, c db.Campaign) (*db.Offer, error) {
	if c.OfferID == nil {
		return nil, nil
	}
	o, err := q.GetOffer(ctx, *c.OfferID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func offerExpiredDetail(o db.Offer) map[string]any {
	return map[string]any{"reason": PauseReasonOfferExpired, "offer_id": o.ID, "offer_title": o.Title, "ended_at": o.EndsAt.UTC()}
}

// OfferPlaceholders are the values an offer adds to a campaign's
// placeholders, e.g. a body variable mapped to {{offer_title}}.
func OfferPlaceholders(o db.Offer) map[string]string {
	out := map[string]string{
		"offer_title":       o.Title,
		"offer_description": o.Description,
		"offer_discount":    o.DiscountText,
		"offer_product":     o.Product,
		"offer_ends":        o.EndsAt.UTC().Format("2006-01-02 15:04 MST"),
	}
	if o.CouponCode != nil {
		out["offer_coupon"] = *o.CouponCode
	}
	if o.CtaUrl != nil {
		out["offer_url"] = *o.CtaUrl
	}
	return out
}

// ApplyOffer fills what the offer knows into the campaign's parameters for
// template shape sh:
//   - a limited-time offer's expiration_time_ms is always the offer's end;
//   - the copy code button takes the coupon unless the campaign set one;
//   - an image header without media takes the offer's image;
//   - placeholders {{offer_title}}, {{offer_discount}}, {{offer_coupon}},
//     {{offer_ends}}, {{offer_url}}, {{offer_product}} and
//     {{offer_description}} resolve to the offer's fields.
//
// Templates without these components (a normal marketing template with an
// image header) therefore carry the offer through their variables; only
// limited-time offer templates show WhatsApp's countdown.
func ApplyOffer(p Params, sh Shape, o db.Offer) Params {
	if sh.LTO {
		ms := o.EndsAt.UnixMilli()
		p.ExpirationTimeMS = &ms
	}
	if len(sh.CopyCode) > 0 && strings.TrimSpace(p.CouponCode) == "" && o.CouponCode != nil {
		p.CouponCode = *o.CouponCode
	}
	if sh.HeaderMedia == "image" && p.HeaderMedia == nil {
		switch {
		case o.ImageMediaID != nil && *o.ImageMediaID != "":
			p.HeaderMedia = &HeaderMedia{Type: "image", ID: *o.ImageMediaID}
		case o.ImageUrl != nil && *o.ImageUrl != "":
			p.HeaderMedia = &HeaderMedia{Type: "image", Link: *o.ImageUrl}
		}
	}
	fb := OfferPlaceholders(o)
	maps.Copy(fb, p.Fallback) // the campaign's own fallbacks win
	p.Fallback = fb
	return p
}

// pauseIfOfferEnded pauses a preparing campaign whose offer ended; stop
// reports that it did (or that the campaign is no longer preparing).
func (s *Service) pauseIfOfferEnded(ctx context.Context, c db.Campaign) (stop bool, err error) {
	o, err := campaignOffer(ctx, s.q, c)
	if err != nil || o == nil || !offerEnded(*o, time.Now()) {
		return false, err
	}
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		_, err := s.transition(ctx, q, tx, c.ID, "paused", []string{"preparing", "running"}, "paused", offerExpiredDetail(*o), nil, nil)
		if e, ok := apperr.As(err); ok && e.Code == "INVALID_TRANSITION" {
			return nil
		}
		return err
	})
	return true, err
}

// PauseForExpiredOffer pauses the running and preparing campaigns of an
// offer that just ended (the offers scheduler calls it). Scheduled ones
// pause the same way when they start; drafts are refused at start.
func (s *Service) PauseForExpiredOffer(ctx context.Context, offerID uuid.UUID) (int, error) {
	rows, err := s.q.CampaignsUsingOffer(ctx, &offerID)
	if err != nil {
		return 0, err
	}
	o, err := s.q.GetOffer(ctx, offerID)
	if err != nil {
		return 0, fmt.Errorf("offer: %w", err)
	}
	n := 0
	for _, r := range rows {
		if r.Status != "running" && r.Status != "preparing" {
			continue
		}
		err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
			_, err := s.transition(ctx, q, tx, r.ID, "paused", []string{"running", "preparing"}, "paused", offerExpiredDetail(o), nil, nil)
			if e, ok := apperr.As(err); ok && e.Code == "INVALID_TRANSITION" {
				return nil
			}
			if err == nil {
				n++
			}
			return err
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
