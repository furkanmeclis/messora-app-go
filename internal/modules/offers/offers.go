// Package offers keeps time-limited offers: what a campaign promotes, from
// when until when, with which coupon and image. Meta's limited-time offer
// templates count down to the offer's end (campaigns fill
// expiration_time_ms from it); the rest (start, product, description) is
// Messora's. See docs/WHATSAPP.md ("Offers").
//
//	draft ─publish→ scheduled ─starts_at→ active ─ends_at→ expired
//	                    └──────────ends_at────────────────↗
//
// The scheduler runs Tick every minute; ending an offer pauses the
// campaigns that use it (reason offer_expired).
package offers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// TypeTick is the scheduler task that moves offers between states.
const TypeTick = "offers:tick"

// Statuses.
const (
	StatusDraft     = "draft"
	StatusScheduled = "scheduled"
	StatusActive    = "active"
	StatusExpired   = "expired"
)

// NextStatus is the state machine: the status an offer in cur has at now.
// Drafts and expired offers stay as they are; published offers follow the
// clock.
func NextStatus(cur string, startsAt, endsAt, now time.Time) string {
	switch cur {
	case StatusScheduled, StatusActive:
		switch {
		case !endsAt.After(now):
			return StatusExpired
		case !startsAt.After(now):
			return StatusActive
		}
		return StatusScheduled
	}
	return cur
}

// Service manages offers.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	camp *campaigns.Service
	log  *slog.Logger
	now  func() time.Time
}

// New returns the service; campaigns using an offer that ends are paused
// through camp.
func New(pool *pgxpool.Pool, camp *campaigns.Service, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), camp: camp, log: log, now: time.Now}
}

// Fields are an offer's editable values.
type Fields struct {
	Title        string
	Description  string
	DiscountText string
	Product      string
	StartsAt     time.Time
	EndsAt       time.Time
	ImageURL     *string
	ImageMediaID *string
	CTAURL       *string
	CouponCode   *string
	TemplateID   *uuid.UUID
}

// Patch changes some fields; nil keeps the value, an empty string clears
// an optional text.
type Patch struct {
	Title, Description, DiscountText, Product *string
	StartsAt, EndsAt                          *time.Time
	ImageURL, ImageMediaID, CTAURL, Coupon    *string
	TemplateID                                *uuid.UUID
	ClearTemplate                             bool
	// Status: draft (unpublish), scheduled (publish) or expired (end now).
	Status *string
}

func blank(p *string) *string {
	if p == nil {
		return nil
	}
	if v := strings.TrimSpace(*p); v != "" {
		return &v
	}
	return nil
}

func tooLong(s string, n int) bool { return utf8.RuneCountInString(s) > n }

func webURL(field, raw string, httpsOnly bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && (httpsOnly || u.Scheme != "http")) {
		if httpsOnly {
			return apperr.Validation(field, "Use a public https:// address.")
		}
		return apperr.Validation(field, "Use a full web address (https://…).")
	}
	return nil
}

// check validates f; the template, when set, must exist, and a
// limited-time offer template limits the coupon to 15 characters.
func (s *Service) check(ctx context.Context, f *Fields) error {
	f.Title = strings.TrimSpace(f.Title)
	switch {
	case f.Title == "" || tooLong(f.Title, 120):
		return apperr.Validation("title", "The title is required (at most 120 characters).")
	case tooLong(f.Description, 2000):
		return apperr.Validation("description", "The description is at most 2000 characters.")
	case tooLong(f.DiscountText, 60):
		return apperr.Validation("discount_text", "The discount is at most 60 characters.")
	case tooLong(f.Product, 200):
		return apperr.Validation("product", "The product is at most 200 characters.")
	case f.StartsAt.IsZero() || f.EndsAt.IsZero():
		return apperr.Validation("starts_at", "Give the start and the end of the offer.")
	case !f.EndsAt.After(f.StartsAt):
		return apperr.Validation("ends_at", "The offer must end after it starts.")
	}
	f.ImageURL, f.ImageMediaID, f.CTAURL, f.CouponCode = blank(f.ImageURL), blank(f.ImageMediaID), blank(f.CTAURL), blank(f.CouponCode)
	if f.ImageURL != nil {
		if err := webURL("image_url", *f.ImageURL, true); err != nil {
			return err
		}
	}
	if f.CTAURL != nil {
		if err := webURL("cta_url", *f.CTAURL, false); err != nil {
			return err
		}
	}
	if f.CouponCode != nil && tooLong(*f.CouponCode, 20) {
		return apperr.Validation("coupon_code", "The coupon code is at most 20 characters.")
	}
	if f.TemplateID != nil {
		t, err := s.q.GetTemplate(ctx, *f.TemplateID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Validation("template_id", "Unknown template.")
		}
		if err != nil {
			return err
		}
		sh := campaigns.ShapeOf(t.Components, t.ParameterFormat, t.Category)
		if sh.LTO && f.CouponCode != nil && tooLong(*f.CouponCode, 15) {
			return apperr.Validation("coupon_code", "Limited-time offer templates take coupon codes of at most 15 characters.")
		}
	}
	return nil
}

// Create stores an offer: a draft, or published (status scheduled), which
// is active at once when it already started.
func (s *Service) Create(ctx context.Context, f Fields, publish bool, actor *uuid.UUID) (db.Offer, error) {
	if err := s.check(ctx, &f); err != nil {
		return db.Offer{}, err
	}
	now := s.now()
	if !f.EndsAt.After(now) {
		return db.Offer{}, apperr.Validation("ends_at", "The offer must end in the future.")
	}
	status := StatusDraft
	if publish {
		status = NextStatus(StatusScheduled, f.StartsAt, f.EndsAt, now)
	}
	var o db.Offer
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		var err error
		o, err = q.InsertOffer(ctx, db.InsertOfferParams{Title: f.Title, Description: f.Description, DiscountText: f.DiscountText,
			Product: f.Product, StartsAt: f.StartsAt, EndsAt: f.EndsAt, ImageUrl: f.ImageURL, ImageMediaID: f.ImageMediaID,
			CtaUrl: f.CTAURL, CouponCode: f.CouponCode, TemplateID: f.TemplateID, Status: status, CreatedBy: actor})
		if err != nil {
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: "offer.create", EntityType: "offer", EntityID: &o.ID,
			Changes: map[string]any{"title": o.Title, "status": o.Status, "ends_at": o.EndsAt}}); err != nil {
			return err
		}
		return realtime.PublishOffer(ctx, tx, o, false)
	})
	return o, err
}

// Get returns one offer.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Offer, error) {
	o, err := s.q.GetOffer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperr.NotFound("Offer not found.")
	}
	return o, err
}

// List returns offers, latest end first.
func (s *Service) List(ctx context.Context, status *string) ([]db.Offer, error) {
	return s.q.ListOffers(ctx, status)
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// Update applies p. Expired offers are final.
func (s *Service) Update(ctx context.Context, id uuid.UUID, p Patch) (db.Offer, error) {
	var o db.Offer
	ended := false
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		cur, err := q.LockOffer(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Offer not found.")
		}
		if err != nil {
			return err
		}
		if cur.Status == StatusExpired {
			return apperr.Conflict("OFFER_EXPIRED", "This offer has ended and can no longer be changed; create a new one.")
		}
		f := Fields{Title: cur.Title, Description: cur.Description, DiscountText: cur.DiscountText, Product: cur.Product,
			StartsAt: cur.StartsAt, EndsAt: cur.EndsAt, ImageURL: cur.ImageUrl, ImageMediaID: cur.ImageMediaID, CTAURL: cur.CtaUrl,
			CouponCode: cur.CouponCode, TemplateID: cur.TemplateID}
		set(&f.Title, p.Title)
		set(&f.Description, p.Description)
		set(&f.DiscountText, p.DiscountText)
		set(&f.Product, p.Product)
		set(&f.StartsAt, p.StartsAt)
		set(&f.EndsAt, p.EndsAt)
		if p.ImageURL != nil {
			f.ImageURL = p.ImageURL
		}
		if p.ImageMediaID != nil {
			f.ImageMediaID = p.ImageMediaID
		}
		if p.CTAURL != nil {
			f.CTAURL = p.CTAURL
		}
		if p.Coupon != nil {
			f.CouponCode = p.Coupon
		}
		if p.TemplateID != nil {
			f.TemplateID = p.TemplateID
		}
		if p.ClearTemplate {
			f.TemplateID = nil
		}
		now := s.now()
		status := cur.Status
		if p.Status != nil {
			switch *p.Status {
			case StatusDraft:
				status = StatusDraft
			case StatusScheduled:
				status = StatusScheduled
			case StatusExpired:
				// End now: the countdown of messages already sent keeps
				// Meta's time; new sends stop.
				status = StatusExpired
				if f.EndsAt.After(now) {
					f.EndsAt = now
				}
				if !f.StartsAt.Before(f.EndsAt) {
					f.StartsAt = f.EndsAt.Add(-time.Second)
				}
			default:
				return apperr.Validation("status", "status is draft, scheduled or expired.")
			}
		}
		if err := s.check(ctx, &f); err != nil {
			return err
		}
		if status != StatusExpired {
			if !f.EndsAt.After(now) && (p.EndsAt != nil || p.Status != nil) {
				return apperr.Validation("ends_at", "The offer must end in the future (or end it now with status expired).")
			}
			status = NextStatus(status, f.StartsAt, f.EndsAt, now)
		}
		o, err = q.UpdateOffer(ctx, db.UpdateOfferParams{ID: id, Title: f.Title, Description: f.Description, DiscountText: f.DiscountText,
			Product: f.Product, StartsAt: f.StartsAt, EndsAt: f.EndsAt, ImageUrl: f.ImageURL, ImageMediaID: f.ImageMediaID,
			CtaUrl: f.CTAURL, CouponCode: f.CouponCode, TemplateID: f.TemplateID, Status: status})
		if err != nil {
			return err
		}
		changes := map[string]any{}
		if cur.Status != o.Status {
			changes["status"] = []string{cur.Status, o.Status}
		}
		if !cur.StartsAt.Equal(o.StartsAt) {
			changes["starts_at"] = []time.Time{cur.StartsAt, o.StartsAt}
		}
		if !cur.EndsAt.Equal(o.EndsAt) {
			changes["ends_at"] = []time.Time{cur.EndsAt, o.EndsAt}
		}
		if cur.Title != o.Title {
			changes["title"] = []string{cur.Title, o.Title}
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: "offer.update", EntityType: "offer", EntityID: &o.ID, Changes: changes}); err != nil {
			return err
		}
		ended = o.Status == StatusExpired
		return realtime.PublishOffer(ctx, tx, o, false)
	})
	if err == nil && ended {
		err = s.campaignsEnded(ctx, o.ID)
	}
	return o, err
}

// Delete removes an offer; campaigns keep running with their own values.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		o, err := q.LockOffer(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Offer not found.")
		}
		if err != nil {
			return err
		}
		if _, err := q.DeleteOffer(ctx, id); err != nil {
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: "offer.delete", EntityType: "offer", EntityID: &id,
			Changes: map[string]any{"title": o.Title}}); err != nil {
			return err
		}
		return realtime.PublishOffer(ctx, tx, o, true)
	})
}

// Tick moves published offers whose start or end has come; it returns how
// many changed. Campaigns using an offer that ended are paused.
func (s *Service) Tick(ctx context.Context) (int, error) {
	var expired []uuid.UUID
	n := 0
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		ended, err := q.ExpireDueOffers(ctx)
		if err != nil {
			return err
		}
		started, err := q.ActivateDueOffers(ctx)
		if err != nil {
			return err
		}
		for _, o := range append(ended, started...) {
			if err := realtime.PublishOffer(ctx, tx, o, false); err != nil {
				return err
			}
		}
		for _, o := range ended {
			expired = append(expired, o.ID)
		}
		n = len(ended) + len(started)
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, id := range expired {
		if err := s.campaignsEnded(ctx, id); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (s *Service) campaignsEnded(ctx context.Context, id uuid.UUID) error {
	if s.camp == nil {
		return nil
	}
	paused, err := s.camp.PauseForExpiredOffer(ctx, id)
	if err != nil {
		return fmt.Errorf("pause campaigns of offer %s: %w", id, err)
	}
	if paused > 0 {
		s.log.InfoContext(ctx, "offer ended; campaigns paused", "offer_id", id, "campaigns", paused)
	}
	return nil
}

func (s *Service) tx(ctx context.Context, fn func(pgx.Tx, *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
