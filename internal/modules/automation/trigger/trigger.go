// Package trigger lets other modules report events that automations may
// react to. It only writes an outbox task in the caller's transaction, so it
// has no dependency on the automation engine (which imports those modules).
//
// Loop protection: events caused by an automation are never emitted, so one
// automation cannot start another (or itself). Callers acting on behalf of
// an automation simply do not call Emit.
package trigger

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
)

// TypeEvaluate is the worker task that matches an event against automations.
const TypeEvaluate = "automation:evaluate"

// Trigger types.
const (
	MessageReceived    = "message_received"
	NewContact         = "new_contact"
	TagAdded           = "tag_added"
	MessageStatus      = "message_status"
	ConversationClosed = "conversation_closed"
	// SubscriptionRequest: an inbound message matched a consent keyword
	// (Event.Subscription: stop, request or resume).
	SubscriptionRequest = "subscription_request"
	// WindowClosing: the customer service window of a conversation closes
	// within the automation's minutes (found by the scanner, see
	// automation.Service.Scan).
	WindowClosing = "window_closing"
	// NoReply: nobody answered for a while — the customer after our
	// message, or the team after the customer's (scanner).
	NoReply = "no_reply"
	// CampaignReply: a customer answered a campaign message
	// (Event.CampaignID).
	CampaignReply = "campaign_reply"
	// Reaction: a customer reacted to a message (Event.Reaction positive or
	// negative, Event.Source, Event.CampaignID of the message); see
	// EmitReaction.
	Reaction = "reaction"
	// AIEscalation: the AI handed a conversation to a person (Event.Reason
	// is the handoff reason).
	AIEscalation = "ai_escalation"
)

// Event is something that happened to a contact.
type Event struct {
	Type string `json:"type"`
	// Key identifies the event: the same real-world event always has the
	// same key (e.g. the WhatsApp message ID), so redelivered webhooks do
	// not start a second run.
	Key            string     `json:"key"`
	ContactID      uuid.UUID  `json:"contact_id"`
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
	PhoneNumberID  *uuid.UUID `json:"phone_number_id,omitempty"`
	MessageID      *uuid.UUID `json:"message_id,omitempty"`
	MessageType    string     `json:"message_type,omitempty"`
	Body           string     `json:"body,omitempty"`
	Tag            string     `json:"tag,omitempty"`
	Status         string     `json:"status,omitempty"`
	CampaignID     *uuid.UUID `json:"campaign_id,omitempty"`
	// Subscription is the keyword kind of a subscription_request.
	Subscription string `json:"subscription,omitempty"`
	// Reaction is positive or negative (reaction); Source is where it came
	// from (emoji, button …).
	Reaction string `json:"reaction,omitempty"`
	Source   string `json:"source,omitempty"`
	// Reason is the handoff reason of an ai_escalation.
	Reason string `json:"reason,omitempty"`
}

// Checker reports whether any active automation listens to a trigger type
// (db.Queries satisfies it).
type Checker interface {
	HasActiveAutomation(ctx context.Context, triggerType string) (bool, error)
}

// Emit schedules evaluation of e in the caller's transaction, but only when
// an active automation listens to its type: a large campaign produces
// hundreds of thousands of status events that nobody may care about.
func Emit(ctx context.Context, tx outbox.DBTX, q Checker, e Event) error {
	ok, err := q.HasActiveAutomation(ctx, e.Type)
	if err != nil || !ok {
		return err
	}
	_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeEvaluate), e, 0)
	return err
}

// Reaction polarities.
const (
	Positive = "positive"
	Negative = "negative"
)

// ReactionEvent is a customer's reaction. The contract is shared with the
// analytics module (P3): whoever records a reaction calls EmitReaction in
// the transaction that stores it.
type ReactionEvent struct {
	// Key identifies the reaction (e.g. the wamid of the reaction message).
	Key            string
	ContactID      uuid.UUID
	ConversationID *uuid.UUID
	PhoneNumberID  *uuid.UUID
	// MessageID is the stored reaction message, if any.
	MessageID *uuid.UUID
	// CampaignID is the campaign of the message reacted to, if any.
	CampaignID *uuid.UUID
	// Reaction is Positive or Negative; anything else is not emitted.
	Reaction string
	// Source is where the reaction came from: "emoji" (a WhatsApp
	// reaction), "button" (a quick-reply button), "text" …
	Source string
	// Body is the emoji or text, for {{message}}.
	Body string
}

// EmitReaction raises the reaction trigger, only for a positive or a
// negative reaction (a neutral or removed one starts nothing).
func EmitReaction(ctx context.Context, tx outbox.DBTX, q Checker, r ReactionEvent) error {
	if r.Reaction != Positive && r.Reaction != Negative {
		return nil
	}
	return Emit(ctx, tx, q, Event{Type: Reaction, Key: "reaction:" + r.Key, ContactID: r.ContactID, ConversationID: r.ConversationID,
		PhoneNumberID: r.PhoneNumberID, MessageID: r.MessageID, CampaignID: r.CampaignID, MessageType: "reaction",
		Body: r.Body, Reaction: r.Reaction, Source: r.Source})
}

var (
	positiveEmoji = []string{"👍", "❤", "😍", "🥰", "😊", "🙏", "👏", "🔥", "💯", "✅", "😀", "😃", "😄", "😁", "🤩", "💚", "💙", "💜", "👌", "🎉"}
	negativeEmoji = []string{"👎", "😡", "😠", "🤬", "😢", "😞", "😭", "💔", "🙄", "😒", "❌", "🤮", "😤", "😩"}
)

// Polarity classifies a reaction emoji: Positive, Negative or "" (neutral
// or unknown). Skin-tone and variation modifiers after the emoji are
// ignored.
func Polarity(emoji string) string {
	e := strings.TrimSpace(emoji)
	if e == "" {
		return ""
	}
	for _, p := range positiveEmoji {
		if strings.HasPrefix(e, p) {
			return Positive
		}
	}
	for _, n := range negativeEmoji {
		if strings.HasPrefix(e, n) {
			return Negative
		}
	}
	return ""
}
