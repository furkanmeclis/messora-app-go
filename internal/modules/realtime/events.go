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

// Event types.
const (
	// TypeMessageCreated: a message was received or queued for sending.
	TypeMessageCreated = "message.created"
	// TypeMessageStatus: a message's delivery state changed.
	TypeMessageStatus = "message.status"
	// TypeCampaignUpdated: a campaign's state or counters changed.
	TypeCampaignUpdated = "campaign.updated"
)

// Publish records an event for channel in the caller's transaction; the
// worker's outbox relay delivers it to Centrifugo after commit.
func Publish(ctx context.Context, tx outbox.DBTX, channel string, event any) error {
	if channel == "" {
		return fmt.Errorf("realtime: empty channel")
	}
	_, err := outbox.Write(ctx, tx, TopicPrefix+channel, event, 0)
	return err
}

// MessageEvent announces a message or a change of its state. It names the
// message and its contact; clients fetch the content through the API.
type MessageEvent struct {
	Type          string    `json:"type"`
	V             int       `json:"v"`
	MessageID     uuid.UUID `json:"message_id"`
	ContactID     uuid.UUID `json:"contact_id"`
	PhoneNumberID uuid.UUID `json:"phone_number_id"`
	Direction     string    `json:"direction"`
	MessageType   string    `json:"message_type"`
	Status        string    `json:"status"`
	At            time.Time `json:"at"`
}

// NewMessageEvent describes m for an event of type typ.
func NewMessageEvent(typ string, m db.Message) MessageEvent {
	return MessageEvent{Type: typ, V: EventVersion, MessageID: m.ID, ContactID: m.ContactID, PhoneNumberID: m.PhoneNumberID,
		Direction: m.Direction, MessageType: m.Type, Status: m.Status, At: m.UpdatedAt.UTC()}
}

// CampaignEvent announces campaign progress; producers publish it to both
// CampaignsChannel and CampaignChannel(id).
type CampaignEvent struct {
	Type       string           `json:"type"`
	V          int              `json:"v"`
	CampaignID uuid.UUID        `json:"campaign_id"`
	Status     string           `json:"status"`
	Counts     map[string]int64 `json:"counts,omitempty"`
	At         time.Time        `json:"at"`
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
