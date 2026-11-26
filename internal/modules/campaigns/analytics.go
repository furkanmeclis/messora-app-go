package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/metacost"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// Campaign analytics (docs/CAMPAIGN_ANALYTICS.md).
//
// Every recipient row mirrors its current message (status, timestamps,
// error, cost) through a database trigger in the transaction that changes
// the message, and campaign_counters is kept up to date by triggers on the
// recipient and message rows: reading a campaign's numbers is one row, never
// a count over its recipients. What the customer did afterwards (reply,
// reaction, opt-out) is attributed here, in the inbound and consent
// transactions, to the one campaign that reached them last.

// Reactions and where they came from. The system's labels (quick reply
// mapping, AI) are shown as "system calculation"; a manual label always
// wins.
const (
	ReactionPositive = "positive"
	ReactionNegative = "negative"
	ReactionNeutral  = "neutral"

	ReactionSourceQuickReply = "quick_reply"
	ReactionSourceAI         = "ai"
	ReactionSourceManual     = "manual"
)

// ValidReaction reports whether r is a reaction label.
func ValidReaction(r string) bool {
	return r == ReactionPositive || r == ReactionNegative || r == ReactionNeutral
}

// Counters is a campaign's incremental counters (one row).
type Counters = db.GetCampaignCountersRow

func counters(ctx context.Context, q *db.Queries, id uuid.UUID) (Counters, error) {
	k, err := q.GetCampaignCounters(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Created with the campaign; a missing row (restored backup) is
		// rebuilt from the recipient rows once.
		if _, err := q.RecomputeCampaignCounters(ctx, id); err != nil {
			return k, err
		}
		return q.GetCampaignCounters(ctx, id)
	}
	return k, err
}

// countsOf turns the counters into the counts map of the API and realtime
// events: recipients by state (recipients_<state>), messages by status (as
// before: only statuses that occur), and the analytics counters (a_*,
// always present). a_cost_micros is the per-message cost in millionths of
// the account's currency.
func countsOf(k Counters) map[string]int64 {
	out := map[string]int64{"recipients": k.Recipients}
	put := func(key string, v int64) {
		if v != 0 {
			out[key] = v
		}
	}
	put("recipients_pending", k.RPending)
	put("recipients_queued", k.RQueued)
	put("recipients_skipped", k.RSkipped)
	put("recipients_cancelled", k.RCancelled)
	put("recipients_retry", k.RRetry)
	for key, v := range map[string]int64{"pending": k.MPending, "sending": k.MSending, "unknown": k.MUnknown, "sent": k.MSent,
		"delivered": k.MDelivered, "read": k.MRead, "failed": k.MFailed, "held": k.MHeld, "cancelled": k.MCancelled,
		"deferred": k.MDeferred, "skipped": k.MSkipped} {
		put(key, v)
	}
	cost, _ := metacost.ParseMicros(strings.TrimLeft(k.Cost, "+"))
	for key, v := range map[string]int64{"a_pending": k.OPending, "a_sent": k.Sent, "a_delivered": k.Delivered, "a_read": k.Read,
		"a_failed": k.OFailed, "a_unknown": k.OUnknown, "a_blocked": k.OBlocked, "a_cancelled": k.OCancelled,
		"a_rejected": k.Rejected, "a_replied": k.Replied, "a_positive": k.Positive, "a_negative": k.Negative,
		"a_neutral": k.Neutral, "a_opted_out": k.OptedOut, "a_err_131048": k.Err131048, "a_err_131049": k.Err131049,
		"a_err_131050": k.Err131050, "a_priced": k.Priced, "a_unpriced": k.Unpriced, "a_cost_micros": int64(cost)} {
		out[key] = v
	}
	return out
}

// ReconcileCounters recomputes the counters of active and recently
// finished campaigns from their rows (drift repair) and evaluates spam
// protection for the running ones; it returns how many had drifted.
func (s *Service) ReconcileCounters(ctx context.Context) (int, error) {
	st, err := LoadAnalyticsSettings(ctx, s.q)
	if err != nil {
		return 0, err
	}
	ids, err := s.q.CampaignsToReconcile(ctx, int32(st.AttributionDays+1)) //nolint:gosec // bounded setting
	if err != nil {
		return 0, err
	}
	drifted := 0
	for _, id := range ids {
		d, err := s.q.RecomputeCampaignCounters(ctx, id)
		if err != nil {
			return drifted, err
		}
		if d {
			drifted++
			s.log.WarnContext(ctx, "campaign counters drifted; recomputed", "campaign_id", id)
		}
	}
	// The protection's state rules (number quality, template status) are
	// also checked here, in case a change came without an event.
	if err := s.ProtectActive(ctx); err != nil {
		return drifted, err
	}
	return drifted, s.expireExports(ctx)
}

// ProtectActive evaluates spam protection for every sending campaign
// (periodically and after the numbers' signals were synced from Meta).
func (s *Service) ProtectActive(ctx context.Context) error {
	active, err := s.q.ActiveCampaignIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range active {
		if err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
			_, err := s.Protect(ctx, tx, q, id, ProtectEvent{})
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// ── Attribution ──────────────────────────────────────────────────────────

// attribution is the recipient an action may be credited to: the contact's
// latest reached recipient.
type attribution struct {
	RecipientID   uuid.UUID
	CampaignID    uuid.UUID
	PhoneNumberID uuid.UUID
	SentAt        time.Time
}

// attributable reports whether something the contact did at `at` (through
// number, nil = any) belongs to campaign recipient a: the campaign must be
// the last one that reached the contact before at (the caller picks the
// latest), at must be after the send and within the window, and a reply
// must come in on the number the campaign sent from.
func attributable(a attribution, at time.Time, number *uuid.UUID, window time.Duration) bool {
	// Meta's timestamps have whole seconds; the send time is compared at
	// the same precision.
	sent := a.SentAt.Truncate(time.Second)
	if at.Before(sent) || at.Sub(sent) > window {
		return false
	}
	return number == nil || *number == a.PhoneNumberID
}

func latestReached(ctx context.Context, q *db.Queries, contact uuid.UUID, at time.Time) (*attribution, db.LatestReachedRecipientRow, error) {
	// Sends within the second of at count as before it (whole-second
	// timestamps from Meta).
	until := at.Truncate(time.Second).Add(time.Second - time.Nanosecond)
	r, err := q.LatestReachedRecipient(ctx, db.LatestReachedRecipientParams{ContactID: &contact, At: &until})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, r, nil
	}
	if err != nil || r.SentAt == nil {
		return nil, r, err
	}
	return &attribution{RecipientID: r.ID, CampaignID: r.CampaignID, PhoneNumberID: r.PhoneNumberID, SentAt: *r.SentAt}, r, nil
}

// ReplyClassifier labels a free-text reply to a campaign (P7: an AI
// classification). It is called inside the inbound transaction and must
// only enqueue work; the result is stored later with SetReaction(…,
// ReactionSourceAI).
type ReplyClassifier interface {
	ClassifyReply(ctx context.Context, tx pgx.Tx, r ReplyToClassify) error
}

// ReplyToClassify is a reply waiting for a reaction label.
type ReplyToClassify struct {
	CampaignID, RecipientID, MessageID uuid.UUID
	ContactID, PhoneNumberID           uuid.UUID
	Text                               string
}

// SetReplyClassifier installs the AI classification hook (nil: none).
func (s *Service) SetReplyClassifier(c ReplyClassifier) { s.classifier = c }

// RecordInbound is told, inside the webhook transaction, about a new
// inbound message. The first message from the contact on the campaign's
// number after the campaign reached them, and before another campaign did
// or the attribution window ended, makes the recipient "replied"; a quick
// reply button sets its reaction from the campaign's mapping. The
// campaign's spam protection is evaluated afterwards (the message may have
// been an opt-out).
func (s *Service) RecordInbound(ctx context.Context, tx pgx.Tx, q *db.Queries, m db.Message) error {
	at := m.OccurredAt
	a, row, err := latestReached(ctx, q, m.ContactID, at)
	if err != nil || a == nil {
		return err
	}
	st, err := CachedAnalyticsSettings(ctx, q)
	if err != nil {
		return err
	}
	if !attributable(*a, at, &m.PhoneNumberID, st.AttributionWindow()) {
		return nil
	}
	first, err := q.SetRecipientReplied(ctx, db.SetRecipientRepliedParams{ID: a.RecipientID, At: &at, MessageID: &m.ID})
	if err != nil {
		return err
	}
	payload, text := quickReply(m)
	switch {
	case payload != "" || text != "":
		key := payload
		if key == "" {
			key = text
		}
		if err := q.SetRecipientReplyPayload(ctx, db.SetRecipientReplyPayloadParams{ID: a.RecipientID, Payload: &key}); err != nil {
			return err
		}
		if r := ReactionFor(row.ReactionMap, payload, text); r != "" {
			if _, err := q.SetRecipientReaction(ctx, db.SetRecipientReactionParams{ID: a.RecipientID, CampaignID: a.CampaignID,
				Reaction: &r, Source: ReactionSourceQuickReply}); err != nil {
				return err
			}
		}
	case first > 0 && s.classifier != nil && strings.TrimSpace(m.Body) != "":
		if err := s.classifier.ClassifyReply(ctx, tx, ReplyToClassify{CampaignID: a.CampaignID, RecipientID: a.RecipientID,
			MessageID: m.ID, ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID, Text: m.Body}); err != nil {
			return err
		}
	}
	if _, err := s.Protect(ctx, tx, q, a.CampaignID, ProtectEvent{Inbound: true}); err != nil {
		return err
	}
	return s.PublishProgress(ctx, tx, q, a.CampaignID)
}

// quickReply returns the payload and text of a tapped template quick reply
// button (or an interactive reply button).
func quickReply(m db.Message) (payload, text string) {
	if m.Type != "button" && m.Type != "interactive" {
		return "", ""
	}
	var in whatsapp.InboundMessage
	if json.Unmarshal(m.Payload, &in) != nil {
		return "", ""
	}
	switch {
	case in.Button != nil:
		return strings.TrimSpace(in.Button.Payload), strings.TrimSpace(in.Button.Text)
	case in.Interactive != nil && in.Interactive.ButtonReply != nil:
		return strings.TrimSpace(in.Interactive.ButtonReply.ID), strings.TrimSpace(in.Interactive.ButtonReply.Title)
	}
	return "", ""
}

// ReactionFor looks a tapped button up in a campaign's mapping: by payload
// first, then by the button's text (case-insensitive); "" = unmapped.
func ReactionFor(raw []byte, payload, text string) string {
	m := map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	if payload != "" {
		if r := m[payload]; ValidReaction(r) {
			return r
		}
	}
	if text != "" {
		if r := m[text]; ValidReaction(r) {
			return r
		}
		for k, r := range m {
			if strings.EqualFold(k, text) && ValidReaction(r) {
				return r
			}
		}
	}
	return ""
}

// RecordOptOut is told, inside the consent transaction, that a contact
// opted out of marketing (or chose Stop in WhatsApp) at `at`: the campaign
// that reached them last, within the attribution window, counts it.
func (s *Service) RecordOptOut(ctx context.Context, tx pgx.Tx, q *db.Queries, contact uuid.UUID, at time.Time) error {
	a, _, err := latestReached(ctx, q, contact, at)
	if err != nil || a == nil {
		return err
	}
	st, err := CachedAnalyticsSettings(ctx, q)
	if err != nil {
		return err
	}
	if !attributable(*a, at, nil, st.AttributionWindow()) {
		return nil
	}
	n, err := q.SetRecipientOptedOut(ctx, db.SetRecipientOptedOutParams{ID: a.RecipientID, At: &at})
	if err != nil || n == 0 {
		return err
	}
	if tx == nil {
		return nil
	}
	if _, err := s.Protect(ctx, tx, q, a.CampaignID, ProtectEvent{OptOut: true}); err != nil {
		return err
	}
	return s.PublishProgress(ctx, tx, q, a.CampaignID)
}

// SetReaction labels a recipient's reaction (nil clears it). source is
// manual (an operator; overrides everything), ai (P7) or quick_reply. A
// system label never replaces a manual one; it reports whether the row
// changed.
func (s *Service) SetReaction(ctx context.Context, campaignID, recipientID uuid.UUID, reaction *string, source string) (bool, error) {
	changed := false
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		n, err := q.SetRecipientReaction(ctx, db.SetRecipientReactionParams{ID: recipientID, CampaignID: campaignID, Reaction: reaction, Source: source})
		if err != nil || n == 0 {
			return err
		}
		changed = true
		return s.PublishProgress(ctx, tx, q, campaignID)
	})
	return changed, err
}
