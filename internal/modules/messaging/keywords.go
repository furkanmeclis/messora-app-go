package messaging

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// inboundConsent applies what a new customer message says about consent,
// in the webhook's transaction:
//   - any window-opening message proves the number reachable again;
//   - a stop word opts the contact out of marketing at once, a resubscribe
//     word subscribes them again (each optionally confirmed by a reply
//     inside the window the message opened);
//   - an opt-out phrase marks the request for an agent to confirm and
//     notifies the assignee (or the owners and admins).
//
// Each keyword also raises the subscription_request automation trigger.
func (s *Service) inboundConsent(ctx context.Context, tx pgx.Tx, q *db.Queries, pn db.PhoneNumber, c db.Contact,
	conv db.Conversation, row db.Message) error {
	if opensWindow(row.Type) {
		if _, err := consent.ClearUndeliverable(ctx, q, c.ID); err != nil {
			return err
		}
	}
	if row.Type != "text" && row.Type != "button" {
		return nil
	}
	set, err := consent.CachedSettings(ctx, q)
	if err != nil {
		return err
	}
	kind := set.Keywords.Match(row.Body)
	if kind == "" {
		return nil
	}
	reason := "Keyword: " + truncate(row.Body, 200)
	cur := consent.StateOf(c)
	var res consent.Result
	var event string
	switch kind {
	case consent.KindStop:
		res, err = consent.Apply(ctx, tx, q, consent.Change{ContactID: c.ID, Categories: []string{consent.CatMarketing},
			Status: consent.OptedOut, Source: consent.SourceKeyword, Reason: reason})
		event = "opted_out"
	case consent.KindRequest:
		if cur.Marketing.Status == consent.OptedOut {
			break // already out; nothing to confirm
		}
		res, err = consent.Apply(ctx, tx, q, consent.Change{ContactID: c.ID, Categories: []string{consent.CatMarketing},
			Status: consent.OptOutRequested, Source: consent.SourceKeyword, Reason: reason})
		event = "opt_out_requested"
	case consent.KindResume:
		cats := []string{consent.CatMarketing}
		if cur.Service.Status == consent.OptedOut || cur.Service.Status == consent.OptOutRequested {
			cats = append(cats, consent.CatService)
		}
		res, err = consent.Apply(ctx, tx, q, consent.Change{ContactID: c.ID, Categories: cats,
			Status: consent.Resubscribed, Source: consent.SourceKeyword, Reason: reason})
		event = "resubscribed"
	}
	if err != nil {
		return err
	}
	if len(res.Changed) > 0 {
		if _, err := q.InsertConversationEvent(ctx, db.InsertConversationEventParams{ConversationID: conv.ID, Kind: event, ActorType: "system"}); err != nil {
			return err
		}
		if kind == consent.KindRequest {
			if err := s.notifyOptOutRequest(ctx, tx, q, c.ID, conv); err != nil {
				return err
			}
		}
		if set.Confirm && (kind == consent.KindStop || kind == consent.KindResume) {
			if err := s.confirmKeyword(ctx, tx, q, pn, c, kind, set); err != nil {
				return err
			}
		}
	}
	return trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.SubscriptionRequest, Key: "sub:" + deref(row.Wamid),
		ContactID: c.ID, ConversationID: &conv.ID, PhoneNumberID: &pn.ID, MessageID: &row.ID, MessageType: row.Type,
		Body: row.Body, Subscription: kind})
}

// confirmKeyword answers a stop or resume word inside the window the
// customer's message just opened. A refusal (restricted licence, a
// validation problem) never fails the webhook: the consent change stands.
func (s *Service) confirmKeyword(ctx context.Context, tx pgx.Tx, q *db.Queries, pn db.PhoneNumber, c db.Contact, kind string, set consent.Settings) error {
	msg := set.StopMessage
	if kind == consent.KindResume {
		msg = set.ResumeMessage
	}
	text := msg.For(consent.WorkspaceLocale(ctx, q))
	if text == "" {
		return nil
	}
	_, err := s.QueueTx(ctx, tx, SendInput{PhoneNumberID: &pn.ID, To: c.PhoneE164, Type: "text",
		Text: &whatsapp.Text{Body: text}, Source: "automation"})
	if _, ok := apperr.As(err); ok {
		s.log.WarnContext(ctx, "consent confirmation not sent", "contact_id", c.ID, "error", err)
		return nil
	}
	return err
}

// notifyOptOutRequest tells the conversation's assignee, or every active
// owner and admin when nobody is assigned, that a customer asked to stop.
func (s *Service) notifyOptOutRequest(ctx context.Context, tx pgx.Tx, q *db.Queries, contactID uuid.UUID, conv db.Conversation) error {
	var users []uuid.UUID
	if conv.AssigneeID != nil {
		users = append(users, *conv.AssigneeID)
	} else {
		all, err := q.ListUsersWithRole(ctx)
		if err != nil {
			return err
		}
		for _, u := range all {
			if u.Status == "active" && (u.RoleKey == "owner" || u.RoleKey == "admin") {
				users = append(users, u.ID)
			}
		}
	}
	ev := realtime.NewNotificationEvent("consent.opt_out_requested")
	ev.ContactID, ev.ConversationID = &contactID, &conv.ID
	channels := make([]string, len(users))
	for i, u := range users {
		channels[i] = realtime.PersonalChannel(u)
	}
	return realtime.PublishAll(ctx, tx, channels, ev)
}
