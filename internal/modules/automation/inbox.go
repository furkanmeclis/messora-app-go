package automation

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
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

// SourceAIUnverified is the provenance of contact fields AI steps write,
// as the run log and the timeline show it: collected by the AI, not
// confirmed by a person (contact_field_sources: source ai, verified false).
const SourceAIUnverified = "ai_unverified"

// CodeFieldKept is the apperr code a FieldWriter returns for a value it did
// not store because the field holds a value the AI may not replace (set by
// a person, an import or the API, or verified).
const CodeFieldKept = "FIELD_KEPT"

// FieldOrigin is where the values an AI step stores came from.
type FieldOrigin struct {
	// Source is SourceAIUnverified.
	Source string
	// UsageID is the AI request (ai_usage) that produced the values.
	UsageID *uuid.UUID
	// ConversationID is the conversation they were collected in.
	ConversationID *uuid.UUID
}

// FieldWriter stores contact fields with their provenance in the run's
// transaction. The conversations module implements it on top of the
// provenance store of AI customer intelligence (contact_field_sources,
// ai.MayAIWrite): it never overwrites a value a person, an import or the
// API set or one a person verified (apperr CodeFieldKept), and marks what
// it stores as collected by the AI, not verified. A value a field cannot
// take is an apperr validation error. Without a writer (SetFieldWriter)
// AI field steps fail.
type FieldWriter interface {
	SetFieldsWithSource(ctx context.Context, tx pgx.Tx, q *db.Queries, contactID uuid.UUID, fields map[string]string, origin FieldOrigin) error
}
