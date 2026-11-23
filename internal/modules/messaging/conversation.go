package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
)

// Conversations (the inbox threads) are derived from messages: every message
// write updates its thread in the same transaction, so the inbox can never
// disagree with the message table.

// opensWindow reports whether an inbound message of type typ opens Meta's
// customer service window. Reactions and system notices do not.
func opensWindow(typ string) bool {
	return typ != "reaction" && typ != "unsupported" && typ != "system"
}

// WindowOpen reports whether free-form messages may be sent in a thread
// whose last window-opening customer message arrived at lastInbound.
func WindowOpen(lastInbound *time.Time, now time.Time) bool {
	return lastInbound != nil && now.Sub(*lastInbound) <= ServiceWindow
}

// WindowExpiresAt is when the customer service window of a thread closes, or
// nil if the customer never wrote.
func WindowExpiresAt(lastInbound *time.Time) *time.Time {
	if lastInbound == nil {
		return nil
	}
	t := lastInbound.Add(ServiceWindow)
	return &t
}

// threadWindowOpen checks the 24-hour window of the thread between contact
// and the sending number.
func threadWindowOpen(ctx context.Context, q *db.Queries, contactID, phoneNumberID uuid.UUID) (bool, error) {
	c, err := q.GetConversationByThread(ctx, db.GetConversationByThreadParams{ContactID: contactID, PhoneNumberID: phoneNumberID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return WindowOpen(c.LastInboundAt, time.Now()), nil
}

// recordNew updates the thread of a newly stored message and announces the
// message and the conversation. A customer message reopens a closed
// conversation; that is recorded in its history.
func recordNew(ctx context.Context, tx pgx.Tx, q *db.Queries, m db.Message) (db.Conversation, error) {
	inbound := m.Direction == "inbound"
	reopened := false
	if inbound {
		prev, err := q.LockConversationByThread(ctx, db.LockConversationByThreadParams{ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID})
		switch {
		case err == nil:
			reopened = prev.Status == "closed"
		case !errors.Is(err, pgx.ErrNoRows):
			return db.Conversation{}, err
		}
	}
	at := m.OccurredAt
	if !inbound {
		at = m.CreatedAt
	}
	c, err := q.TouchConversation(ctx, db.TouchConversationParams{ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID,
		MessageID: &m.ID, At: at, OpensWindow: inbound && opensWindow(m.Type), Inbound: inbound})
	if err != nil {
		return c, err
	}
	if reopened {
		if _, err := q.InsertConversationEvent(ctx, db.InsertConversationEventParams{ConversationID: c.ID, Kind: "reopened", ActorType: "system"}); err != nil {
			return c, err
		}
	}
	if err := publishMessage(ctx, tx, realtime.TypeMessageCreated, m, &c.ID); err != nil {
		return c, err
	}
	return c, realtime.PublishConversation(ctx, tx, c)
}

// announceStatus publishes a delivery state change of m to the inbox and to
// its conversation.
func announceStatus(ctx context.Context, tx pgx.Tx, q *db.Queries, m db.Message) error {
	var conv *uuid.UUID
	c, err := q.GetConversationByThread(ctx, db.GetConversationByThreadParams{ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID})
	switch {
	case err == nil:
		conv = &c.ID
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	return publishMessage(ctx, tx, realtime.TypeMessageStatus, m, conv)
}

func publishMessage(ctx context.Context, tx pgx.Tx, typ string, m db.Message, conv *uuid.UUID) error {
	ev := realtime.NewMessageEvent(typ, m)
	ev.ConversationID = conv
	if err := realtime.Publish(ctx, tx, realtime.InboxChannel, ev); err != nil {
		return err
	}
	if conv == nil {
		return nil
	}
	return realtime.Publish(ctx, tx, realtime.ConversationChannel(*conv), ev)
}

// RecordCampaignMessage keeps the thread of a campaign message current, in
// the caller's transaction. It does not announce each message on the inbox
// channel: campaign progress is published on the campaign's own channel, and
// a 100k-recipient campaign must not flood every open inbox.
func RecordCampaignMessage(ctx context.Context, q *db.Queries, m db.Message) error {
	_, err := q.TouchConversation(ctx, db.TouchConversationParams{ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID,
		MessageID: &m.ID, At: m.CreatedAt})
	return err
}
