package conversations

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// The conversations module stores the contact fields of automation AI
// steps with the same provenance rules as the AI's own data collection.
var _ automation.FieldWriter = (*Service)(nil)

var aiFieldKey = regexp.MustCompile(`^(name|email|var[1-5]|attr:[A-Za-z0-9_.-]{1,64})$`)

// SetFieldsWithSource implements automation.FieldWriter on the provenance
// store (contact_field_sources, docs/AI.md "What is stored"):
//   - a value is checked against its field (an e-mail must be one; special
//     categories of personal data and secrets are never stored);
//   - a value a person, an import or the API set, or one a person
//     verified, is never overwritten (ai.MayAIWrite): apperr
//     automation.CodeFieldKept;
//   - a stored value is marked source ai, not verified, with the AI request
//     (ai_usage) and the conversation that produced it.
func (s *Service) SetFieldsWithSource(ctx context.Context, tx pgx.Tx, q *db.Queries, contactID uuid.UUID, fields map[string]string,
	origin automation.FieldOrigin) error {
	contact, err := q.LockContact(ctx, contactID)
	if err != nil {
		return err
	}
	sources, err := q.ListContactFieldSources(ctx, contactID)
	if err != nil {
		return err
	}
	bySource := map[string]*ai.FieldSource{}
	for _, fs := range sources {
		bySource[fs.Field] = &ai.FieldSource{Source: fs.Source, Verified: fs.Verified}
	}
	values := ContactValues(contact)
	changed := false
	var kept []string
	for key, raw := range fields {
		if !aiFieldKey.MatchString(key) || ai.SensitiveField(key, "") {
			return apperr.Validation(key, "not a field the AI may fill")
		}
		val, ok := ai.ValidateValue(ai.DataField{Key: key}, raw)
		if !ok || strings.TrimSpace(val) == "" {
			return apperr.Validation(key, "the value does not fit the field")
		}
		if values[key] == val {
			continue
		}
		if !ai.MayAIWrite(values[key], bySource[key]) {
			kept = append(kept, key)
			continue
		}
		if err := setContactField(ctx, q, contactID, key, val); err != nil {
			return err
		}
		if err := q.UpsertContactFieldSource(ctx, db.UpsertContactFieldSourceParams{ContactID: contactID, Field: key,
			Source: ai.SourceAI, Verified: false, AiUsageID: origin.UsageID, ConversationID: origin.ConversationID}); err != nil {
			return err
		}
		values[key], changed = val, true
	}
	if changed {
		if err := webhooks.ContactChanged(ctx, tx, webhooks.TypeContactUpdated, contactID); err != nil {
			return err
		}
	}
	if len(kept) > 0 {
		return apperr.Conflict(automation.CodeFieldKept, "the field holds a value the AI may not replace: "+strings.Join(kept, ", "))
	}
	return nil
}
