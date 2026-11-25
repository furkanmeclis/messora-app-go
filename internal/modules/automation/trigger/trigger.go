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
