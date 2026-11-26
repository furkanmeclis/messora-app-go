package ai

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

// TypeAttributionReconcile retries the campaign attribution of recent AI
// usage rows (periodic, queue ai).
const TypeAttributionReconcile = "ai:attribution_reconcile"

// DefaultAttributionWindow is the attribution window until
// SetAttributionWindow installs the company's setting.
const DefaultAttributionWindow = 7 * 24 * time.Hour

// SetAttributionWindow installs where the campaign attribution window comes
// from: the campaign protection setting attribution_days
// (docs/CAMPAIGN_ANALYTICS.md "Attribution"), the single window of replies,
// opt-outs and AI usage. Call it before the service is used.
func (s *Service) SetAttributionWindow(f func(context.Context) (time.Duration, error)) { s.window = f }

// AttributionWindow is the company's campaign attribution window.
func (s *Service) AttributionWindow(ctx context.Context) (time.Duration, error) {
	if s.window == nil {
		return DefaultAttributionWindow, nil
	}
	w, err := s.window(ctx)
	if err != nil || w <= 0 {
		return DefaultAttributionWindow, err
	}
	return w, nil
}

// CampaignSend is a campaign message that reached a contact.
type CampaignSend struct {
	CampaignID uuid.UUID
	SentAt     time.Time
}

// Attribute is the campaign attribution rule (docs/AI.md "Maliyet ve
// kullanım"): an AI request at time at belongs to the campaign whose
// message reached the contact last at or before at — so a later campaign
// reaching the same contact takes over from then on — provided that
// message is at most window old. Exactly one campaign or none: the result
// is nil when no campaign reached the contact within the window ("kampanya
// dışı").
func Attribute(sends []CampaignSend, at time.Time, window time.Duration) *uuid.UUID {
	var best *CampaignSend
	for i := range sends {
		s := &sends[i]
		if s.SentAt.After(at) {
			continue
		}
		if best == nil || s.SentAt.After(best.SentAt) {
			best = s
		}
	}
	if best == nil || at.Sub(best.SentAt) > window {
		return nil
	}
	id := best.CampaignID
	return &id
}

// attributeCampaign finds the campaign a request of conversation convID at
// time at belongs to (nil = none).
func attributeCampaign(ctx context.Context, q db.Querier, convID uuid.UUID, at time.Time, window time.Duration) (*uuid.UUID, error) {
	th, err := q.ConversationThreadOf(ctx, convID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return CampaignAt(ctx, q, th.ContactID, th.PhoneNumberID, at, window)
}

// CampaignAt is the campaign the contact's activity on the number at time
// at belongs to under the attribution rule (nil = none). Replies and
// opt-outs follow the same rule in the campaigns module.
func CampaignAt(ctx context.Context, q db.Querier, contactID, phoneNumberID uuid.UUID, at time.Time, window time.Duration) (*uuid.UUID, error) {
	row, err := q.LatestCampaignSend(ctx, db.LatestCampaignSendParams{ContactID: contactID, PhoneNumberID: phoneNumberID, At: &at})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return Attribute([]CampaignSend{{CampaignID: row.CampaignID, SentAt: row.SentAt}}, at, window), nil
}

// ReconcileAttribution retries the attribution of usage rows of the last
// attribution window that have no campaign yet (a campaign message whose
// send was recorded after the AI request, a restored conversation…). A row
// that has a campaign is never changed. It returns how many rows got one.
func (s *Service) ReconcileAttribution(ctx context.Context) (int, error) {
	window, err := s.AttributionWindow(ctx)
	if err != nil {
		return 0, err
	}
	since := s.opts.Now().Add(-window - 24*time.Hour)
	after := uuid.Nil
	n := 0
	for {
		rows, err := s.q.ListUnattributedAIUsage(ctx, db.ListUnattributedAIUsageParams{Since: since, After: after, Lim: 500})
		if err != nil {
			return n, err
		}
		for _, r := range rows {
			after = r.ID
			c, err := attributeCampaign(ctx, s.q, r.ConversationID, r.CreatedAt, window)
			if err != nil {
				return n, err
			}
			if c == nil {
				continue
			}
			k, err := s.q.SetAIUsageCampaign(ctx, db.SetAIUsageCampaignParams{ID: r.ID, CampaignID: c})
			if err != nil {
				return n, err
			}
			n += int(k)
		}
		if len(rows) < 500 {
			return n, nil
		}
	}
}
