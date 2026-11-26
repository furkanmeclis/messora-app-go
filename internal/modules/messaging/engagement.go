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

// engagementTriggers raises the automation reaction trigger for a positive
// or negative WhatsApp emoji reaction to a message. A reaction to a campaign
// message uses the recipient's key (trigger.RecipientReactionKey), shared
// with the campaign's quick-reply, AI and manual labels, so it starts an
// automation once per recipient and polarity. The campaign_reply trigger is
// raised by the campaigns module with the analytics attribution (see
// campaigns.RecordInbound).
func (s *Service) engagementTriggers(ctx context.Context, tx pgx.Tx, q *db.Queries, pn db.PhoneNumber, c db.Contact, conv db.Conversation,
	row db.Message, m *whatsapp.InboundMessage) error {
	if m.Type != "reaction" || m.Reaction == nil {
		return nil
	}
	pol := trigger.Polarity(m.Reaction.Emoji)
	if pol == "" {
		return nil // removed or neutral
	}
	key := m.ID
	var campaign *uuid.UUID
	if m.Reaction.MessageID != "" {
		r, err := q.CampaignRecipientOfWAMID(ctx, &m.Reaction.MessageID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		campaign = r.CampaignID
		if r.RecipientID != nil {
			key = trigger.RecipientReactionKey(*r.RecipientID, pol)
		}
	}
	return trigger.EmitReaction(ctx, tx, q, trigger.ReactionEvent{Key: key, ContactID: c.ID, ConversationID: &conv.ID,
		PhoneNumberID: &pn.ID, MessageID: &row.ID, CampaignID: campaign, Reaction: pol, Source: "emoji", Body: m.Reaction.Emoji})
}
