// Package activity writes a conversation's activity timeline
// (conversation_activity): one typed stream next to every change —
// handoffs, assignee changes, notes, AI actions and replies, automation
// runs, sent and failed messages, escalations. Each item is written in the
// caller's transaction and announced on the conversation's realtime
// channel (conversation.activity), so the inbox timeline updates live.
//
// It is a leaf package: messaging, automation and conversations all write
// through it.
package activity

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Kinds of timeline items (conversation_activity.kind).
const (
	HandedToAI          = "handed_to_ai"
	HandedToHuman       = "handed_to_human"
	AIModeChanged       = "ai_mode_changed"
	AssigneeChanged     = "assignee_changed"
	Closed              = "closed"
	Reopened            = "reopened"
	NoteAdded           = "note_added"
	AIAction            = "ai_action"
	AIReplySent         = "ai_reply_sent"
	AIDraftCreated      = "ai_draft_created"
	AIDraftDecided      = "ai_draft_decided"
	AIError             = "ai_error"
	Escalation          = "escalation"
	HandoffAccepted     = "handoff_accepted"
	AutomationStarted   = "automation_started"
	AutomationCompleted = "automation_completed"
	AutomationFailed    = "automation_failed"
	MessageSent         = "message_sent"
	MessageFailed       = "message_failed"
	ContactAIDisabled   = "contact_ai_disabled"
	ContactAIEnabled    = "contact_ai_enabled"
	// ProfileCompleted: the AI collected every required field of a new contact.
	ProfileCompleted = "profile_completed"
)

// Actor types.
const (
	ActorUser   = "user"
	ActorAPIKey = "api_key"
	ActorSystem = "system"
	ActorAI     = "ai"
)

// Actor is who did something.
type Actor struct {
	Type string
	ID   *uuid.UUID
}

// System is an automatic change (a trigger, the platform).
var System = Actor{Type: ActorSystem}

// AI is the AI assistant.
var AI = Actor{Type: ActorAI}

// Of returns the actor of a principal (System for nil).
func Of(p *principal.Principal) Actor {
	if p == nil {
		return System
	}
	id := p.ID
	return Actor{Type: string(p.Kind), ID: &id}
}

// Data is the kind-specific detail of an item.
type Data map[string]any

// Record appends an item to the conversation's timeline and announces it,
// in the caller's transaction.
func Record(ctx context.Context, tx pgx.Tx, q *db.Queries, conversationID uuid.UUID, kind string, actor Actor, data Data) (db.ConversationActivity, error) {
	if data == nil {
		data = Data{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return db.ConversationActivity{}, err
	}
	a, err := q.InsertConversationActivity(ctx, db.InsertConversationActivityParams{ConversationID: conversationID, Kind: kind,
		ActorType: actor.Type, ActorID: actor.ID, Data: raw})
	if err != nil {
		return a, err
	}
	return a, realtime.Publish(ctx, tx, realtime.ConversationChannel(conversationID), realtime.ActivityEvent{
		Envelope: realtime.NewEnvelope(realtime.TypeConversationActivity), ConversationID: conversationID,
		ActivityID: a.ID, Kind: kind, At: a.CreatedAt.UTC()})
}

// AnnounceRun publishes the state of an automation run tied to a
// conversation (automation.run.updated) and records its milestones in the
// timeline: started (started = true), completed, failed or cancelled.
// Progress in between is only published. Runs without a conversation are
// not announced.
func AnnounceRun(ctx context.Context, tx pgx.Tx, q *db.Queries, runID uuid.UUID, started bool) error {
	r, err := q.GetAutomationRunSummary(ctx, runID)
	if err != nil || r.ConversationID == nil {
		return err
	}
	if err := realtime.Publish(ctx, tx, realtime.ConversationChannel(*r.ConversationID), realtime.AutomationRunEvent{
		Envelope: realtime.NewEnvelope(realtime.TypeAutomationRunUpdated), RunID: r.ID, AutomationID: r.AutomationID,
		ConversationID: *r.ConversationID, Status: r.Status, StartedBy: r.StartedBy, StepsDone: r.StepsDone, FinishedAt: r.FinishedAt}); err != nil {
		return err
	}
	var milestone string
	switch {
	case started:
		milestone = AutomationStarted
	case r.Status == "completed":
		milestone = AutomationCompleted
	case r.Status == "failed" || r.Status == "cancelled":
		milestone = AutomationFailed
	default:
		return nil
	}
	d := Data{"automation_id": r.AutomationID, "automation_name": r.AutomationName, "run_id": r.ID, "started_by": r.StartedBy,
		"steps": r.StepsDone, "status": r.Status}
	if r.Error != nil {
		d["error"] = *r.Error
	}
	actor := System
	if milestone == AutomationStarted && r.StartedBy == ActorAI {
		actor = AI
	}
	_, err = Record(ctx, tx, q, *r.ConversationID, milestone, actor, d)
	return err
}

// Preview shortens a text for a timeline item.
func Preview(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
