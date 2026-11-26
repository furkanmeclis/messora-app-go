package automation

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Inbox is what the handoff, note and conversation-aware AI steps need
// from the conversations module. That module imports this one, so it is
// connected at start-up (SetInbox); conversations.Service implements it.
type Inbox interface {
	// AutomationHandOff gives the conversation to the AI or to a person
	// (the assignee, else the team's pool) in the run's transaction and
	// tells whoever may take it. A handoff that does not apply (the AI is
	// unavailable, the conversation already has that owner) is reported
	// as Skipped, not as an error.
	AutomationHandOff(ctx context.Context, tx pgx.Tx, q *db.Queries, conversationID uuid.UUID, h HandoffRequest) (HandoffResult, error)
	// AutomationNote adds an internal note written by the automation.
	AutomationNote(ctx context.Context, tx pgx.Tx, q *db.Queries, conversationID uuid.UUID, body string, forAI bool) (uuid.UUID, error)
	// ComposeAIReply asks the number's AI profile for one reply in the
	// conversation, as an AI turn would (disclosure, handoff rules); the
	// usage is recorded with purpose automation and the run's id. Config
	// problems are *StepError (ai_disabled, ai_no_profile …).
	ComposeAIReply(ctx context.Context, conversationID, runID uuid.UUID) (AIReply, error)
	// ComposeSummary is the handoff summary of the conversation (AI, or
	// built from the messages when the AI cannot).
	ComposeSummary(ctx context.Context, conversationID, runID uuid.UUID) (Summary, error)
}

// HandoffRequest is a handoff an automation step makes.
type HandoffRequest struct {
	To         string // ai | human
	Mode       string // to ai: auto_reply (default) | awaiting_approval
	AssigneeID *uuid.UUID
	Note       string
	Summary    *Summary
}

// HandoffResult is what a handoff did.
type HandoffResult struct {
	HandoffID *uuid.UUID
	// Skipped is why nothing changed (ai_unavailable, already_owner …).
	Skipped string
}

// Summary is a handoff summary.
type Summary struct {
	Text       string
	Need       string
	NextAction string
	Source     string // ai | fallback
}

// AIReply is one reply composed by the AI for ai_auto_reply.
type AIReply struct {
	Text       string
	UsageID    *uuid.UUID
	Intent     string
	Confidence float64
	// Disclose: the text starts with the AI disclosure; the conversation
	// is marked disclosed when the message is queued.
	Disclose bool
	// Escalate: the profile's handoff rules say a person should answer
	// (Reason, Summary); nothing is sent.
	Escalate bool
	Reason   string
	Summary  *Summary
	// Fields the AI collected (profile data collection).
	Fields map[string]string
	// Skipped is why no reply was composed: the AI already answers the
	// conversation (ai_owns_conversation), the window is closed, the AI is
	// disabled for the contact.
	Skipped string
}

// SourceAIUnverified is the provenance of contact fields AI steps write:
// written by the AI, not confirmed by a person.
const SourceAIUnverified = "ai_unverified"

// FieldWriter stores contact fields with their provenance (source). The
// contacts module's provenance store (P7) implements it once it exists;
// until then directFields writes the contact columns and attributes, and
// the source is kept in the run log and on the conversation's timeline.
type FieldWriter interface {
	SetFieldsWithSource(ctx context.Context, q *db.Queries, contactID uuid.UUID, fields map[string]string, source string) error
}

// directFields writes fields straight to the contact: name, email,
// var1…var5, attr:<key>. Values that do not fit a field (an e-mail without
// @) are a validation error.
type directFields struct{}

func (directFields) SetFieldsWithSource(ctx context.Context, q *db.Queries, contactID uuid.UUID, fields map[string]string, _ string) error {
	p := db.UpdateContactParams{ID: contactID}
	columns := false
	for key, val := range fields {
		if attr, ok := strings.CutPrefix(key, "attr:"); ok {
			if err := q.SetContactAttribute(ctx, db.SetContactAttributeParams{ID: contactID, Key: attr, Value: val}); err != nil {
				return err
			}
			continue
		}
		v := val
		columns = true
		switch key {
		case "name":
			p.Name = &v
		case "email":
			if !strings.Contains(v, "@") || strings.ContainsAny(v, " \t") {
				return apperr.Validation("email", "not an e-mail address")
			}
			p.SetEmail, p.Email = true, &v
		case "var1":
			p.SetVar1, p.Var1 = true, &v
		case "var2":
			p.SetVar2, p.Var2 = true, &v
		case "var3":
			p.SetVar3, p.Var3 = true, &v
		case "var4":
			p.SetVar4, p.Var4 = true, &v
		case "var5":
			p.SetVar5, p.Var5 = true, &v
		}
	}
	if !columns {
		return nil
	}
	_, err := q.UpdateContact(ctx, p)
	return err
}
