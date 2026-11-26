package conversations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
)

// CampaignReplyClassifier is the AI classifier of free-text campaign
// replies (campaigns.ReplyClassifier). The campaigns module attributes the
// reply (one attribution window and rule for replies, opt-outs and AI
// usage) and calls it for the recipient's first reply; it schedules an
// ai:classify job unless the contact has AI turned off or the message is a
// consent keyword. The job's result becomes the recipient's reaction with
// source ai (ai.Service.SetClassificationSink).
func (s *Service) CampaignReplyClassifier() campaigns.ReplyClassifier {
	return replyClassifier{s}
}

type replyClassifier struct{ s *Service }

func (c replyClassifier) ClassifyReply(ctx context.Context, tx pgx.Tx, r campaigns.ReplyToClassify) error {
	if c.s.ai == nil {
		return nil
	}
	q := db.New(tx)
	contact, err := q.GetContact(ctx, r.ContactID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil || contact.AiDisabled {
		return err
	}
	// A consent keyword (STOP, İPTAL …) is an opt-out, counted as such.
	if set, err := consent.CachedSettings(ctx, q); err == nil && set.Keywords.Match(r.Text) != "" {
		return nil
	}
	in := ai.CampaignReply{CampaignID: r.CampaignID, RecipientID: r.RecipientID, ContactID: r.ContactID, MessageID: r.MessageID, Text: r.Text}
	if conv, err := q.GetConversationByThread(ctx, db.GetConversationByThreadParams{ContactID: r.ContactID, PhoneNumberID: r.PhoneNumberID}); err == nil {
		in.ConversationID = &conv.ID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return c.s.ai.EnqueueReplyClassification(ctx, tx, q, in)
}
