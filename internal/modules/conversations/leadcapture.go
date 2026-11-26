package conversations

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/activity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
)

// Profile statuses of a contact the AI builds (contacts.profile_status).
const (
	ProfileNew        = "new"
	ProfileCollecting = "collecting"
	ProfileComplete   = "complete"
)

// collect stores the fields an AI reply extracted (docs/AI.md "Müşteri
// bilgisi toplama"):
//   - only fields the profile's data collection lists (data minimisation),
//     only with "may update fields";
//   - each value is checked against the field's type (e-mail, phone,
//     select option …) and dropped when it does not fit;
//   - a value a person, an import or the API set — or a person verified —
//     is never overwritten (ai.MayAIWrite);
//   - stored values are marked "collected by the AI, not verified" in
//     contact_field_sources with the AI request that produced them.
//
// For a new contact whose profile the AI builds (LeadCapture) it then
// updates the profile progress; completed is true when this turn
// completed the required fields (once: the status becomes complete).
func (s *Service) collect(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, t *turn, reply ai.Reply) (bool, error) {
	p := t.profile
	if p == nil || !p.DataCollection.Enabled {
		return false, nil
	}
	contact, err := q.LockContact(ctx, c.ContactID)
	if err != nil {
		return false, err
	}
	capture := LeadCapture(*p, contact)
	if p.DataCollection.MayUpdateFields {
		sources, err := q.ListContactFieldSources(ctx, contact.ID)
		if err != nil {
			return false, err
		}
		bySource := map[string]*ai.FieldSource{}
		for _, fs := range sources {
			bySource[fs.Field] = &ai.FieldSource{Source: fs.Source, Verified: fs.Verified}
		}
		values := ContactValues(contact)
		changed := false
		for _, f := range p.DataCollection.Fields {
			v := reply.ExtractedFields[f.Key]
			if v == nil || strings.TrimSpace(*v) == "" {
				continue
			}
			val, ok := ai.ValidateValue(f, *v)
			if !ok {
				if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": "update_field",
					"field": f.Key, "label": f.Label, "status": "invalid"}); err != nil {
					return false, err
				}
				continue
			}
			if values[f.Key] == val {
				continue
			}
			if !ai.MayAIWrite(values[f.Key], bySource[f.Key]) {
				if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": "update_field",
					"field": f.Key, "label": f.Label, "status": "kept_existing"}); err != nil {
					return false, err
				}
				continue
			}
			if err := setContactField(ctx, q, contact.ID, f.Key, val); err != nil {
				return false, err
			}
			convID := c.ID
			if err := q.UpsertContactFieldSource(ctx, db.UpsertContactFieldSourceParams{ContactID: contact.ID, Field: f.Key,
				Source: ai.SourceAI, Verified: false, AiUsageID: t.usageID, ConversationID: &convID}); err != nil {
				return false, err
			}
			values[f.Key] = val
			changed = true
			if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": "update_field",
				"field": f.Key, "label": f.Label}); err != nil {
				return false, err
			}
		}
		if changed {
			if err := webhooks.ContactChanged(ctx, tx, webhooks.TypeContactUpdated, contact.ID); err != nil {
				return false, err
			}
			if contact, err = q.GetContact(ctx, contact.ID); err != nil {
				return false, err
			}
		}
	}
	if !capture {
		return false, nil
	}
	prog := ai.ProgressOf(p.DataCollection.Fields, ContactValues(contact))
	status := ProfileCollecting
	if prog.Complete() {
		status = ProfileComplete
	}
	raw, _ := json.Marshal(prog)
	if err := q.SetContactProfile(ctx, db.SetContactProfileParams{ID: contact.ID, ProfileStatus: &status, ProfileProgress: raw}); err != nil {
		return false, err
	}
	if status != ProfileComplete {
		return false, nil
	}
	_, err = activity.Record(ctx, tx, q, c.ID, activity.ProfileCompleted, activity.AI, activity.Data{"contact_id": contact.ID,
		"required": prog.Required, "handoff": p.DataCollection.HandoffWhenComplete})
	return err == nil, err
}
