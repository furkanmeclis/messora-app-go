package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
)

// EventVersion is the envelope version. Adding fields keeps the version;
// changing or removing one bumps it.
const EventVersion = 1

// Event types endpoints can subscribe to. docs/WEBHOOKS.md documents each
// payload; keep both in sync with the WebhookEventType enum of the API spec.
const (
	TypeMessageReceived      = "message.received"
	TypeMessageStatus        = "message.status"
	TypeConversationAssigned = "conversation.assigned"
	TypeConversationClosed   = "conversation.closed"
	TypeConversationReopened = "conversation.reopened"
	TypeCampaignCompleted    = "campaign.completed"
	TypeCampaignFailed       = "campaign.failed"
	TypeCampaignCancelled    = "campaign.cancelled"
	TypeContactCreated       = "contact.created"
	TypeContactUpdated       = "contact.updated"
	TypeContactOptedOut      = "contact.opted_out"
	TypeContactOptedIn       = "contact.opted_in"
	// TypeTest is sent by "send test event" to one endpoint; it cannot be
	// subscribed to.
	TypeTest = "webhook.test"
)

// EventType describes a subscribable event.
type EventType struct {
	Type        string
	Description string
}

// Catalogue lists every subscribable event type.
var Catalogue = []EventType{
	{TypeMessageReceived, "A customer sent a message (includes the text)."},
	{TypeMessageStatus, "A message's delivery state changed: sent, delivered, read, failed or unknown."},
	{TypeConversationAssigned, "A conversation was assigned to a user or unassigned."},
	{TypeConversationClosed, "A conversation was closed."},
	{TypeConversationReopened, "A conversation was reopened by a user or by a new customer message."},
	{TypeCampaignCompleted, "A campaign finished sending."},
	{TypeCampaignFailed, "A campaign stopped because of an error."},
	{TypeCampaignCancelled, "A campaign was cancelled."},
	{TypeContactCreated, "A contact was created in the panel or through the API."},
	{TypeContactUpdated, "A contact's fields, tags or groups were changed in the panel or through the API."},
	{TypeContactOptedOut, "A contact opted out of messages."},
	{TypeContactOptedIn, "A contact opted back in."},
}

// Known reports whether t is a subscribable event type.
func Known(t string) bool {
	for _, e := range Catalogue {
		if e.Type == t {
			return true
		}
	}
	return false
}

// Envelope is the JSON body every receiver gets.
type Envelope struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}

// emit records an event in the caller's transaction when at least one
// active endpoint subscribes to typ; build is only called then. The worker
// fans it out to endpoints after commit (task webhook:fanout).
func emit(ctx context.Context, tx outbox.DBTX, typ string, build func(q *db.Queries) (any, error)) error {
	q := db.New(tx)
	ok, err := q.WebhookTypeSubscribed(ctx, typ)
	if err != nil || !ok {
		return err
	}
	data, err := build(q)
	if err != nil {
		return err
	}
	_, err = record(ctx, tx, typ, data)
	return err
}

// record stores the envelope and schedules the fan-out.
func record(ctx context.Context, tx outbox.DBTX, typ string, data any) (uuid.UUID, error) {
	id, err := insertEvent(ctx, db.New(tx), typ, data)
	if err != nil {
		return uuid.Nil, fmt.Errorf("webhooks: record %s: %w", typ, err)
	}
	_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeFanOut), map[string]string{"event_id": id.String()}, 0)
	return id, err
}

// ContactRef identifies the customer of a message or conversation.
type ContactRef struct {
	ID    uuid.UUID `json:"id"`
	Phone string    `json:"phone"`
	Name  string    `json:"name"`
}

func contactRef(c db.Contact) ContactRef {
	name := c.Name
	if name == "" {
		name = c.ProfileName
	}
	return ContactRef{ID: c.ID, Phone: c.PhoneE164, Name: name}
}

// MessageData is the payload of message.received and message.status.
type MessageData struct {
	MessageID      uuid.UUID   `json:"message_id"`
	Wamid          *string     `json:"wamid"`
	ConversationID *uuid.UUID  `json:"conversation_id"`
	PhoneNumberID  uuid.UUID   `json:"phone_number_id"`
	Direction      string      `json:"direction"`
	MessageType    string      `json:"message_type"`
	Status         string      `json:"status"`
	Contact        ContactRef  `json:"contact"`
	CampaignID     *uuid.UUID  `json:"campaign_id,omitempty"`
	Text           *string     `json:"text,omitempty"`
	ReplyToWamid   *string     `json:"reply_to_wamid,omitempty"`
	Error          *ErrorField `json:"error,omitempty"`
	OccurredAt     time.Time   `json:"occurred_at"`
}

// ErrorField describes why a message failed.
type ErrorField struct {
	Code  *string `json:"code"`
	Title *string `json:"title"`
}

func messageData(ctx context.Context, q *db.Queries, m db.Message, conv *uuid.UUID) (MessageData, error) {
	c, err := q.GetContact(ctx, m.ContactID)
	if err != nil {
		return MessageData{}, err
	}
	d := MessageData{MessageID: m.ID, Wamid: m.Wamid, ConversationID: conv, PhoneNumberID: m.PhoneNumberID, Direction: m.Direction,
		MessageType: m.Type, Status: m.Status, Contact: contactRef(c), CampaignID: m.CampaignID, OccurredAt: m.UpdatedAt.UTC()}
	if m.ErrorCode != nil || m.ErrorTitle != nil {
		d.Error = &ErrorField{Code: m.ErrorCode, Title: m.ErrorTitle}
	}
	return d, nil
}

// MessageReceived emits message.received for a stored customer message.
// The payload carries the message text: integrations need it, so receivers
// must treat webhook bodies as customer data.
func MessageReceived(ctx context.Context, tx outbox.DBTX, m db.Message, conversationID uuid.UUID) error {
	return emit(ctx, tx, TypeMessageReceived, func(q *db.Queries) (any, error) {
		d, err := messageData(ctx, q, m, &conversationID)
		if err != nil {
			return nil, err
		}
		body := m.Body
		d.Text, d.ReplyToWamid, d.OccurredAt = &body, m.ReplyToWamid, m.OccurredAt.UTC()
		return d, nil
	})
}

// MessageStatus emits message.status after a message's state changed.
func MessageStatus(ctx context.Context, tx outbox.DBTX, m db.Message, conversationID *uuid.UUID) error {
	return emit(ctx, tx, TypeMessageStatus, func(q *db.Queries) (any, error) {
		return messageData(ctx, q, m, conversationID)
	})
}

// Actor is who caused a change; type is user, api_key or system.
type Actor struct {
	Type string     `json:"type"`
	ID   *uuid.UUID `json:"id"`
}

// ConversationData is the payload of conversation.* events.
type ConversationData struct {
	ConversationID uuid.UUID  `json:"conversation_id"`
	Contact        ContactRef `json:"contact"`
	PhoneNumberID  uuid.UUID  `json:"phone_number_id"`
	Status         string     `json:"status"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	Actor          Actor      `json:"actor"`
}

// ConversationChanged emits the event matching a conversation history kind
// (assigned, unassigned, closed, reopened); other kinds are ignored.
func ConversationChanged(ctx context.Context, tx outbox.DBTX, kind string, c db.Conversation, actorType string, actorID *uuid.UUID) error {
	var typ string
	switch kind {
	case "assigned", "unassigned":
		typ = TypeConversationAssigned
	case "closed":
		typ = TypeConversationClosed
	case "reopened":
		typ = TypeConversationReopened
	default:
		return nil
	}
	return emit(ctx, tx, typ, func(q *db.Queries) (any, error) {
		ct, err := q.GetContact(ctx, c.ContactID)
		if err != nil {
			return nil, err
		}
		return ConversationData{ConversationID: c.ID, Contact: contactRef(ct), PhoneNumberID: c.PhoneNumberID, Status: c.Status,
			AssigneeID: c.AssigneeID, Actor: Actor{Type: actorType, ID: actorID}}, nil
	})
}

// CampaignData is the payload of campaign.* events.
type CampaignData struct {
	CampaignID      uuid.UUID        `json:"campaign_id"`
	Name            string           `json:"name"`
	Status          string           `json:"status"`
	TotalRecipients int64            `json:"total_recipients"`
	Counts          map[string]int64 `json:"counts"`
	LastError       *string          `json:"last_error"`
	StartedAt       *time.Time       `json:"started_at"`
	FinishedAt      *time.Time       `json:"finished_at"`
}

// CampaignFinished emits campaign.completed, .failed or .cancelled when c is
// in that state; other states are ignored.
func CampaignFinished(ctx context.Context, tx outbox.DBTX, c db.Campaign) error {
	typ := map[string]string{"completed": TypeCampaignCompleted, "failed": TypeCampaignFailed, "cancelled": TypeCampaignCancelled}[c.Status]
	if typ == "" {
		return nil
	}
	return emit(ctx, tx, typ, func(q *db.Queries) (any, error) {
		counts := map[string]int64{}
		ms, err := q.CampaignMessageStats(ctx, &c.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			counts[m.Status] = m.N
		}
		rs, err := q.CampaignRecipientStats(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			counts["recipients_"+r.State] = r.N
		}
		return CampaignData{CampaignID: c.ID, Name: c.Name, Status: c.Status, TotalRecipients: c.TotalRecipients, Counts: counts,
			LastError: c.LastError, StartedAt: c.StartedAt, FinishedAt: c.FinishedAt}, nil
	})
}

// ContactData is the payload of contact.* events.
type ContactData struct {
	ContactID  uuid.UUID  `json:"contact_id"`
	Phone      string     `json:"phone"`
	Name       string     `json:"name"`
	Email      *string    `json:"email"`
	OptedOut   bool       `json:"opted_out"`
	Source     string     `json:"source"`
	UpdatedAt  time.Time  `json:"updated_at"`
	OptedOutAt *time.Time `json:"opted_out_at,omitempty"`
}

// ContactChanged emits typ (one of the contact.* types) for the contact id,
// read in the caller's transaction so the payload shows the new state.
func ContactChanged(ctx context.Context, tx outbox.DBTX, typ string, id uuid.UUID) error {
	return emit(ctx, tx, typ, func(q *db.Queries) (any, error) {
		c, err := q.GetContact(ctx, id)
		if err != nil {
			return nil, err
		}
		return ContactData{ContactID: c.ID, Phone: c.PhoneE164, Name: c.Name, Email: c.Email, OptedOut: c.OptedOutAt != nil,
			Source: c.Source, UpdatedAt: c.UpdatedAt.UTC(), OptedOutAt: c.OptedOutAt}, nil
	})
}
