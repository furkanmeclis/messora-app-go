package campaigns

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
)

// Automation triggers about how a recipient responded to a campaign
// (docs/AUTOMATIONS.md "Reactions" and "Campaign replies"). They are raised
// here, in the transaction that records the response, so the campaign an
// automation sees is always the one the analytics attributed it to.

// emitReaction raises the reaction trigger after a recipient's label was set
// to positive or negative (neutral and cleared labels start nothing).
func emitReaction(ctx context.Context, tx pgx.Tx, q *db.Queries, campaignID, recipientID uuid.UUID, reaction, source string) error {
	if tx == nil || (reaction != ReactionPositive && reaction != ReactionNegative) {
		return nil
	}
	t, err := q.RecipientAutomationTarget(ctx, recipientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil || t.ContactID == nil {
		return err
	}
	return trigger.EmitReaction(ctx, tx, q, trigger.ReactionEvent{Key: trigger.RecipientReactionKey(recipientID, reaction),
		ContactID: *t.ContactID, ConversationID: t.ConversationID, PhoneNumberID: &t.PhoneNumberID, MessageID: t.ReplyMessageID,
		CampaignID: &campaignID, Reaction: reaction, Source: source})
}

// emitCampaignReply raises the campaign_reply trigger for the recipient's
// first reply, the one the analytics count (attribution window, the
// campaign that reached the contact last, the campaign's number).
func emitCampaignReply(ctx context.Context, tx pgx.Tx, q *db.Queries, a attribution, m db.Message) error {
	if tx == nil || m.Type == "reaction" || m.Type == "unsupported" || m.Type == "system" {
		return nil
	}
	ok, err := q.HasActiveAutomation(ctx, trigger.CampaignReply)
	if err != nil || !ok {
		return err
	}
	t, err := q.RecipientAutomationTarget(ctx, a.RecipientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.CampaignReply, Key: "campaign_reply:" + a.RecipientID.String(),
		ContactID: m.ContactID, ConversationID: t.ConversationID, PhoneNumberID: &m.PhoneNumberID, MessageID: &m.ID,
		MessageType: m.Type, Body: m.Body, CampaignID: &a.CampaignID})
}
