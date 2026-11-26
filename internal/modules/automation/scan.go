package automation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/shared/licensegate"
)

// noReplyLookback bounds how far back the no-reply scan looks beyond the
// waiting time: a conversation silent for longer than that (the scanner
// was down, the automation was enabled later) is not picked up anymore.
const noReplyLookback = 24 * time.Hour

// Candidate is a conversation a time-based trigger found.
type Candidate struct {
	ConversationID uuid.UUID
	ContactID      uuid.UUID
	PhoneNumberID  uuid.UUID
	// MessageID identifies the occasion: the customer message that opened
	// the window (window_closing) or the unanswered message (no_reply).
	MessageID uuid.UUID
}

// WindowCandidate decides whether a conversation from the window query
// fires window_closing: the window is still open at now (the query
// already asked for a window closing within the trigger's minutes) and —
// unless the trigger includes answered conversations — the customer's
// message is the thread's last one, i.e. unanswered.
func WindowCandidate(t Trigger, lastInbound *time.Time, lastDirection string, now time.Time) bool {
	if !messaging.WindowOpen(lastInbound, now) {
		return false
	}
	closes := lastInbound.Add(messaging.ServiceWindow)
	if closes.Sub(now) > time.Duration(t.WindowMinutes())*time.Minute {
		return false
	}
	return t.IncludeAnswered || lastDirection == "inbound"
}

// NoReplyRange is the window of last-message times the no-reply scan looks
// at: older than the waiting time, at most noReplyLookback older.
func NoReplyRange(t Trigger, now time.Time) (after, before time.Time) {
	before = now.Add(-time.Duration(t.Minutes) * time.Minute)
	return before.Add(-noReplyLookback), before
}

// WindowKey and NoReplyKey are the event keys of the scanned triggers: a
// window fires once per customer message that opened it, a silence once
// per unanswered message.
func WindowKey(conv, msg uuid.UUID) string { return "window:" + conv.String() + ":" + msg.String() }

// NoReplyKey see WindowKey.
func NoReplyKey(from string, conv, msg uuid.UUID) string {
	return "noreply:" + from + ":" + conv.String() + ":" + msg.String()
}

// Scan starts the runs of the time-based triggers (window_closing,
// no_reply) for every active automation. It runs every minute; event keys
// make it idempotent, so a scan that overlaps another or repeats after a
// crash starts nothing twice.
func (s *Service) Scan(ctx context.Context) (int, error) {
	if licensegate.Check(ctx) != nil {
		return 0, nil
	}
	autos, err := s.q.ListAutomationsByTypes(ctx, []string{trigger.WindowClosing, trigger.NoReply})
	if err != nil || len(autos) == 0 {
		return 0, err
	}
	now := s.now()
	started := 0
	for _, a := range autos {
		var t Trigger
		_ = json.Unmarshal(a.Trigger, &t)
		cands, err := s.candidates(ctx, a.TriggerType, t, now)
		if err != nil {
			return started, err
		}
		for _, c := range cands {
			e := trigger.Event{Type: a.TriggerType, ContactID: c.ContactID, ConversationID: &c.ConversationID,
				PhoneNumberID: &c.PhoneNumberID, MessageID: &c.MessageID}
			if a.TriggerType == trigger.WindowClosing {
				e.Key = WindowKey(c.ConversationID, c.MessageID)
			} else {
				e.Key = NoReplyKey(t.NoReplyFrom, c.ConversationID, c.MessageID)
			}
			if m, err := s.q.GetMessage(ctx, c.MessageID); err == nil {
				e.Body, e.MessageType, e.CampaignID = m.Body, m.Type, m.CampaignID
			}
			contact, err := s.q.GetContact(ctx, c.ContactID)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return started, err
			}
			ok, err := s.start(ctx, a, e, contact)
			if err != nil {
				return started, err
			}
			if ok {
				started++
			}
		}
	}
	return started, nil
}

func (s *Service) candidates(ctx context.Context, typ string, t Trigger, now time.Time) ([]Candidate, error) {
	var out []Candidate
	switch typ {
	case trigger.WindowClosing:
		rows, err := s.q.WindowClosingCandidates(ctx, db.WindowClosingCandidatesParams{OpenedAfter: now.Add(-messaging.ServiceWindow),
			OpenedBefore: now.Add(-messaging.ServiceWindow + time.Duration(t.WindowMinutes())*time.Minute), PhoneNumberID: t.PhoneNumberID})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if !WindowCandidate(t, r.LastInboundAt, r.LastDirection, now) {
				continue // closed already, or the team answered
			}
			out = append(out, Candidate{ConversationID: r.ID, ContactID: r.ContactID, PhoneNumberID: r.PhoneNumberID, MessageID: r.InboundMessageID})
		}
	case trigger.NoReply:
		after, before := NoReplyRange(t, now)
		dir := "outbound" // the customer did not answer our message
		if t.NoReplyFrom == "agent" {
			dir = "inbound"
		}
		rows, err := s.q.NoReplyCandidates(ctx, db.NoReplyCandidatesParams{After: after, Before: before, Direction: dir,
			IncludeCampaigns: t.IncludeCampaigns, PhoneNumberID: t.PhoneNumberID})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, Candidate{ConversationID: r.ID, ContactID: r.ContactID, PhoneNumberID: r.PhoneNumberID, MessageID: r.MessageID})
		}
	}
	return out, nil
}
