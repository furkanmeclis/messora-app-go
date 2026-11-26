package messaging

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// engagementTriggers raises the automation triggers about how a customer
// responded to us: a positive or negative reaction (emoji) to a message,
// and a reply to a campaign message (it quotes the campaign message, or
// the campaign message is the thread's latest outbound one of the last 72
// hours and the customer had not written since).
func (s *Service) engagementTriggers(ctx context.Context, tx pgx.Tx, q *db.Queries, pn db.PhoneNumber, c db.Contact, conv db.Conversation,
	row db.Message, m *whatsapp.InboundMessage) error {
	if m.Type == "reaction" {
		if m.Reaction == nil {
			return nil
		}
		pol := trigger.Polarity(m.Reaction.Emoji)
		if pol == "" {
			return nil // removed or neutral
		}
		var campaign *uuid.UUID
		if m.Reaction.MessageID != "" {
			id, err := q.CampaignOfWAMID(ctx, &m.Reaction.MessageID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			campaign = id
		}
		return trigger.EmitReaction(ctx, tx, q, trigger.ReactionEvent{Key: m.ID, ContactID: c.ID, ConversationID: &conv.ID,
			PhoneNumberID: &pn.ID, MessageID: &row.ID, CampaignID: campaign, Reaction: pol, Source: "emoji", Body: m.Reaction.Emoji})
	}
	if !opensWindow(m.Type) {
		return nil
	}
	ok, err := q.HasActiveAutomation(ctx, trigger.CampaignReply)
	if err != nil || !ok {
		return err
	}
	id, err := q.CampaignRepliedTo(ctx, db.CampaignRepliedToParams{ReplyToWamid: row.ReplyToWamid, ContactID: c.ID,
		PhoneNumberID: pn.ID, MessageID: row.ID})
	if err != nil || id == uuid.Nil {
		return err
	}
	return trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.CampaignReply, Key: "campaign_reply:" + m.ID, ContactID: c.ID,
		ConversationID: &conv.ID, PhoneNumberID: &pn.ID, MessageID: &row.ID, MessageType: row.Type, Body: row.Body, CampaignID: &id})
}
