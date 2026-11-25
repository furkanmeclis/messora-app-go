package realtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
)

// TopicPrefix marks outbox events published to Centrifugo: "rt:<channel>".
const TopicPrefix = "rt:"

// EventVersion is the payload version of every event below. Bump it when a
// field changes meaning; adding fields does not need a new version.
const EventVersion = 1

// Event types. Names follow "<area>.<event>": the area is the business
// object (message, conversation, campaign, automation.run, ai,
// notification), the event a past-tense verb (created, updated) or a noun
// for a stream of states (status, progress, activity). Clients ignore types
// they do not know, so adding a type needs no version bump.
const (
	// TypeMessageCreated: a message was received or queued for sending.
	TypeMessageCreated = "message.created"
	// TypeMessageStatus: a message's delivery state changed.
	TypeMessageStatus = "message.status"
	// TypeCampaignUpdated: a campaign's state or counters changed.
	TypeCampaignUpdated = "campaign.updated"
	// TypeConversationUpdated: a conversation got a message, or its
	// assignee, status or unread count changed.
	TypeConversationUpdated = "conversation.updated"
	// TypeOfferUpdated: a time-limited offer was created, edited, deleted
	// or changed status (published on campaign:all).
	TypeOfferUpdated = "offer.updated"

	// Reserved for Phase 16C producers (defined, not published yet).

	// TypeConversationHandoff: a conversation moved between the AI agent,
	// an automation and a person.
	TypeConversationHandoff = "conversation.handoff"
	// TypeConversationActivity: someone is typing in or viewing a
	// conversation.
	TypeConversationActivity = "conversation.activity"
	// TypeAutomationRunUpdated: an automation run started, advanced or ended.
	TypeAutomationRunUpdated = "automation.run.updated"
	// TypeCampaignProgress: frequent campaign counters.
	TypeCampaignProgress = "campaign.progress"
	// TypeAIStatus: the AI agent's state in a conversation.
	TypeAIStatus = "ai.status"
	// TypeNotificationCreated: a notice for one user or for everyone.
	TypeNotificationCreated = "notification.created"
)

// Envelope is the common head of every event:
//
//	type  event type, one of the constants above
//	v     payload version (EventVersion)
//	id    unique event ID; an event published to several channels keeps
//	      one ID, so a client that receives it twice drops the copy
//	ts    when the event was produced (UTC)
//
// Event structs embed it, so their own fields sit in the same JSON object.
type Envelope struct {
	Type string    `json:"type"`
	V    int       `json:"v"`
	ID   uuid.UUID `json:"id"`
	TS   time.Time `json:"ts"`
}

// NewEnvelope starts an event of type typ with a fresh ID.
func NewEnvelope(typ string) Envelope {
	return Envelope{Type: typ, V: EventVersion, ID: uuid.Must(uuid.NewV7()), TS: time.Now().UTC()}
}

func (e Envelope) envelope() Envelope { return e }

// Event is any struct embedding Envelope.
type Event interface{ envelope() Envelope }

// Publish records an event for channel in the caller's transaction; the
// worker's outbox relay delivers it to Centrifugo after commit.
func Publish(ctx context.Context, tx outbox.DBTX, channel string, event Event) error {
	if channel == "" {
		return fmt.Errorf("realtime: empty channel")
	}
	if h := event.envelope(); h.Type == "" || h.ID == uuid.Nil {
		return fmt.Errorf("realtime: event without type or id (start it with NewEnvelope)")
	}
	_, err := outbox.Write(ctx, tx, TopicPrefix+channel, event, 0)
	return err
}

// PublishAll records the same event (one ID) for every channel.
func PublishAll(ctx context.Context, tx outbox.DBTX, channels []string, event Event) error {
	for _, ch := range channels {
		if err := Publish(ctx, tx, ch, event); err != nil {
			return err
		}
	}
	return nil
}

// NotificationEvent is a notice for a user (on their personal channel).
// Kind says what happened (e.g. consent.opt_out_requested); the ids say
// where to look.
type NotificationEvent struct {
	Envelope
	Kind           string     `json:"kind"`
	ContactID      *uuid.UUID `json:"contact_id,omitempty"`
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
}

// NewNotificationEvent starts a notification of kind.
func NewNotificationEvent(kind string) NotificationEvent {
	return NotificationEvent{Envelope: NewEnvelope(TypeNotificationCreated), Kind: kind}
}

// MessageEvent announces a message or a change of its state. It names the
// message and its contact; clients fetch the content through the API.
type MessageEvent struct {
	Envelope
	MessageID     uuid.UUID `json:"message_id"`
	ContactID     uuid.UUID `json:"contact_id"`
	PhoneNumberID uuid.UUID `json:"phone_number_id"`
	Direction     string    `json:"direction"`
	MessageType   string    `json:"message_type"`
	Status        string    `json:"status"`
	At            time.Time `json:"at"`
	// ConversationID names the inbox thread of the message, when known.
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
}

// NewMessageEvent describes m for an event of type typ.
func NewMessageEvent(typ string, m db.Message) MessageEvent {
	return MessageEvent{Envelope: NewEnvelope(typ), MessageID: m.ID, ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID,
		Direction: m.Direction, MessageType: m.Type, Status: m.Status, At: m.UpdatedAt.UTC()}
}

// PublishMessage announces a message event on the channels of its
// conversation c (ConversationChannels). A message without a conversation
// goes to InboxChannel only, which needs conversations.read_all.
func PublishMessage(ctx context.Context, tx outbox.DBTX, ev MessageEvent, c *db.Conversation) error {
	if c == nil {
		ev.ConversationID = nil
		return Publish(ctx, tx, InboxChannel, ev)
	}
	ev.ConversationID = &c.ID
	return PublishAll(ctx, tx, ConversationChannels(*c), ev)
}

// ConversationEvent announces a change of a conversation.
type ConversationEvent struct {
	Envelope
	ConversationID uuid.UUID  `json:"conversation_id"`
	ContactID      uuid.UUID  `json:"contact_id"`
	PhoneNumberID  uuid.UUID  `json:"phone_number_id"`
	Status         string     `json:"status"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	UnreadCount    int32      `json:"unread_count"`
	LastMessageAt  time.Time  `json:"last_message_at"`
	At             time.Time  `json:"at"`
}

// NewConversationEvent describes c.
func NewConversationEvent(c db.Conversation) ConversationEvent {
	return ConversationEvent{Envelope: NewEnvelope(TypeConversationUpdated), ConversationID: c.ID, ContactID: c.ContactID,
		PhoneNumberID: c.PhoneNumberID, Status: c.Status, AssigneeID: c.AssigneeID, UnreadCount: c.UnreadCount,
		LastMessageAt: c.LastMessageAt.UTC(), At: c.UpdatedAt.UTC()}
}

// AudienceChannel is the channel of the users with conversations.read (but
// not read_all) who see a conversation with this assignee: the unassigned
// pool, or the assignee's personal channel.
func AudienceChannel(assignee *uuid.UUID) string {
	if assignee == nil {
		return UnassignedChannel
	}
	return PersonalChannel(*assignee)
}

// ConversationChannels lists the channels that carry the events of c:
// InboxChannel (conversations.read_all), AudienceChannel(c.AssigneeID) and
// ConversationChannel(c.ID). A user who may read only some conversations
// is never sent the IDs of the others.
func ConversationChannels(c db.Conversation) []string {
	return []string{InboxChannel, AudienceChannel(c.AssigneeID), ConversationChannel(c.ID)}
}

// PublishConversation announces c on ConversationChannels(c).
func PublishConversation(ctx context.Context, tx outbox.DBTX, c db.Conversation) error {
	return PublishConversationChange(ctx, tx, c, c)
}

// PublishConversationChange announces after, the new state of before. When
// the assignee changed, the previous audience (the previous assignee, or the
// unassigned pool) gets the new state too, so its clients drop the
// conversation.
func PublishConversationChange(ctx context.Context, tx outbox.DBTX, before, after db.Conversation) error {
	channels := ConversationChannels(after)
	if prev := AudienceChannel(before.AssigneeID); prev != channels[1] {
		channels = append(channels, prev)
	}
	return PublishAll(ctx, tx, channels, NewConversationEvent(after))
}

// CampaignEvent announces campaign progress on CampaignsChannel and
// CampaignChannel(id).
type CampaignEvent struct {
	Envelope
	CampaignID uuid.UUID        `json:"campaign_id"`
	Status     string           `json:"status"`
	Counts     map[string]int64 `json:"counts,omitempty"`
	At         time.Time        `json:"at"`
}

// OfferEvent announces a change of a time-limited offer. Deleted is set
// when the offer is gone.
type OfferEvent struct {
	Envelope
	OfferID uuid.UUID `json:"offer_id"`
	Status  string    `json:"status"`
	EndsAt  time.Time `json:"ends_at"`
	Deleted bool      `json:"deleted,omitempty"`
}

// PublishOffer announces o on the campaign list channel: offers drive
// campaigns, and whoever plans campaigns watches that channel.
func PublishOffer(ctx context.Context, tx outbox.DBTX, o db.Offer, deleted bool) error {
	return Publish(ctx, tx, CampaignsChannel, OfferEvent{Envelope: NewEnvelope(TypeOfferUpdated), OfferID: o.ID, Status: o.Status,
		EndsAt: o.EndsAt.UTC(), Deleted: deleted})
}

// NewCampaignEvent describes campaign c with its counters.
func NewCampaignEvent(c db.Campaign, counts map[string]int64) CampaignEvent {
	return CampaignEvent{Envelope: NewEnvelope(TypeCampaignUpdated), CampaignID: c.ID, Status: c.Status, Counts: counts, At: time.Now().UTC()}
}

// PublishCampaign announces ev on CampaignsChannel and its own channel.
func PublishCampaign(ctx context.Context, tx outbox.DBTX, ev CampaignEvent) error {
	return PublishAll(ctx, tx, []string{CampaignsChannel, CampaignChannel(ev.CampaignID)}, ev)
}

// publishClient is the part of the Centrifugo client the publisher needs.
type publishClient interface {
	Publish(ctx context.Context, channel string, data []byte, idempotencyKey string) error
}

// Publisher relays "rt:<channel>" outbox events to Centrifugo. The outbox
// ID is the idempotency key, so a publication repeated after a lost reply is
// dropped by Centrifugo instead of reaching clients twice.
func Publisher(c publishClient) outbox.Publisher {
	return outbox.PublisherFunc(func(ctx context.Context, e outbox.Event) error {
		channel := strings.TrimPrefix(e.Topic, TopicPrefix)
		if channel == "" {
			return fmt.Errorf("realtime: outbox event %d has no channel", e.ID)
		}
		return c.Publish(ctx, channel, e.Payload, fmt.Sprintf("outbox:%d", e.ID))
	})
}
