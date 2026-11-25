package conversations

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/activity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// ownershipChange runs fn on a visible, locked conversation in a
// transaction.
func (s *Service) ownershipChange(ctx context.Context, p *principal.Principal, id uuid.UUID,
	fn func(tx pgx.Tx, q *db.Queries, c db.Conversation, contact db.Contact) error) error {
	return s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := s.load(ctx, q, p, id); err != nil {
			return err
		}
		c, err := q.LockConversation(ctx, id)
		if err != nil {
			return err
		}
		contact, err := q.GetContact(ctx, c.ContactID)
		if err != nil {
			return err
		}
		return fn(tx, q, c, contact)
	})
}

func transitionError(err error, why string) error {
	switch {
	case errors.Is(err, ErrAIUnavailable):
		msg := unavailableMessages[why]
		if msg == "" {
			msg = "The AI is not available for this conversation."
		}
		return apperr.Conflict("AI_UNAVAILABLE", msg)
	case errors.Is(err, ErrTransition):
		return apperr.Conflict("INVALID_TRANSITION", "This change does not apply to the conversation's current owner.")
	}
	return err
}

// acceptPending marks the conversation's pending escalation as taken.
func acceptPending(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, p *principal.Principal) error {
	if c.PendingHandoffID == nil || p.Kind != principal.KindUser {
		return nil
	}
	h, err := q.AcceptConversationHandoff(ctx, db.AcceptConversationHandoffParams{ID: *c.PendingHandoffID, AcceptedBy: &p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = activity.Record(ctx, tx, q, c.ID, activity.HandoffAccepted, activity.Of(p), activity.Data{"handoff_id": h.ID, "reason": h.Reason})
	return err
}

// SetAIMode sets the AI mode of a conversation (see Next for what each
// mode means for the owner).
func (s *Service) SetAIMode(ctx context.Context, p *principal.Principal, id uuid.UUID, mode Mode) error {
	if !mode.Valid() {
		return apperr.Validation("mode", "Use off, suggest, auto_reply or awaiting_approval.")
	}
	return s.ownershipChange(ctx, p, id, func(tx pgx.Tx, q *db.Queries, c db.Conversation, contact db.Contact) error {
		prof, ok, why := s.availability(ctx, c, contact)
		cur := stateOf(c)
		next, err := Next(cur, Event{Action: SetMode, Mode: mode, TakeoverMode: takeoverMode(prof), AIAvailable: ok})
		if err != nil {
			return transitionError(err, why)
		}
		if next == cur {
			return nil
		}
		return s.applyManual(ctx, tx, q, p, c, cur, next, prof, nil, false, "")
	})
}

// applyManual persists a transition a person made and records it: a
// handoff when the owner changes, the timeline, the audit log, and the AI
// turn for an unanswered message when the AI got work.
func (s *Service) applyManual(ctx context.Context, tx pgx.Tx, q *db.Queries, p *principal.Principal, c db.Conversation, cur, next State,
	prof *ai.Profile, assignee *uuid.UUID, pending bool, note string) error {
	var profileID *uuid.UUID
	if prof != nil && next.Mode != ModeOff {
		profileID = &prof.ID
	}
	actor := activity.Of(p)
	var handoffID *uuid.UUID
	if next.Owner != cur.Owner && (next.Owner == OwnerAI || cur.Owner == OwnerAI || pending) {
		toType, toUser := "human", assignee
		switch {
		case next.Owner == OwnerAI:
			toType, toUser = "ai", nil
		case assignee == nil:
			toType = "pool"
		}
		params := db.InsertConversationHandoffParams{ConversationID: c.ID, FromType: string(cur.Owner), FromUserID: c.AssigneeID,
			ToType: toType, ToUserID: toUser, Reason: string(ReasonManual), ReasonDetail: truncateRunes(note, 500), SummarySource: "none",
			CollectedFields: []byte("{}"), ActorType: actor.Type, ActorID: actor.ID}
		if cur.Owner == OwnerAI {
			params.FromUserID = nil
		}
		if !pending && p.Kind == principal.KindUser {
			now := time.Now()
			params.AcceptedBy, params.AcceptedAt = &p.ID, &now
		}
		h, err := q.InsertConversationHandoff(ctx, params)
		if err != nil {
			return err
		}
		handoffID = &h.ID
	} else if next.Owner == cur.Owner && cur.Owner == OwnerHuman && pending {
		// A person passing their conversation on: waiting for the next one.
		h, err := q.InsertConversationHandoff(ctx, db.InsertConversationHandoffParams{ConversationID: c.ID, FromType: string(cur.Owner),
			FromUserID: c.AssigneeID, ToType: map[bool]string{true: "pool", false: "human"}[assignee == nil], ToUserID: assignee,
			Reason: string(ReasonManual), ReasonDetail: truncateRunes(note, 500), SummarySource: "none", CollectedFields: []byte("{}"),
			ActorType: actor.Type, ActorID: actor.ID})
		if err != nil {
			return err
		}
		handoffID = &h.ID
	}
	if cur.PendingHandoff && !pending {
		if err := acceptPending(ctx, tx, q, c, p); err != nil {
			return err
		}
	}
	next.PendingHandoff = pending
	updated, err := s.persist(ctx, tx, q, c, next, profileID, handoffID, "manual")
	if err != nil {
		return err
	}
	kind, action, data := activity.AIModeChanged, "conversation.ai_mode", activity.Data{"from": cur.Mode, "mode": next.Mode}
	switch {
	case next.Owner == OwnerAI && cur.Owner != OwnerAI:
		kind, action = activity.HandedToAI, "conversation.handoff"
	case cur.Owner == OwnerAI && next.Owner != OwnerAI, pending:
		kind, action = activity.HandedToHuman, "conversation.handoff"
		data["to"] = "pool"
		if assignee != nil {
			data["to"], data["assignee_id"] = "user", *assignee
		}
	case cur.Owner == next.Owner && cur.Mode == next.Mode:
		// Only a pending escalation was taken (recorded by acceptPending).
		return audit.Record(ctx, q, audit.Entry{Action: "conversation.takeover", EntityType: "conversation", EntityID: &c.ID,
			Changes: map[string]any{"handoff_id": c.PendingHandoffID}})
	}
	if handoffID != nil {
		data["handoff_id"] = *handoffID
	}
	if note != "" {
		data["note"] = activity.Preview(note, 200)
	}
	if _, err := activity.Record(ctx, tx, q, c.ID, kind, actor, data); err != nil {
		return err
	}
	if err := audit.Record(ctx, q, audit.Entry{Action: action, EntityType: "conversation", EntityID: &c.ID,
		Changes: map[string]any{"owner_type": []any{cur.Owner, next.Owner}, "ai_mode": []any{cur.Mode, next.Mode}}}); err != nil {
		return err
	}
	if next.Owner == OwnerAI || next.Mode == ModeSuggest {
		return s.scheduleUnanswered(ctx, tx, q, updated)
	}
	return nil
}

// HandOff gives the conversation to the AI (to "ai", mode auto_reply or
// awaiting_approval) or to a person: assignee, or the team's pool with
// nil. Giving it to someone else needs conversations.assign.
func (s *Service) HandOff(ctx context.Context, p *principal.Principal, id uuid.UUID, to string, mode Mode, assignee *uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	switch to {
	case "ai":
		if mode == "" {
			mode = ModeAutoReply
		}
		if !mode.AIOwned() {
			return apperr.Validation("mode", "Hand to the AI with auto_reply or awaiting_approval.")
		}
		return s.SetAIMode(ctx, p, id, mode)
	case "human":
	default:
		return apperr.Validation("to", "Use ai or human.")
	}
	if assignee != nil && (p.Kind != principal.KindUser || *assignee != p.ID) && !p.Has(PermAssign) {
		return apperr.Forbidden("FORBIDDEN", "Giving a conversation to someone else needs conversations.assign.")
	}
	return s.ownershipChange(ctx, p, id, func(tx pgx.Tx, q *db.Queries, c db.Conversation, contact db.Contact) error {
		if assignee != nil {
			if _, err := q.GetAssignableUser(ctx, *assignee); errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("assignee_id", "Choose an active user who can read conversations.")
			} else if err != nil {
				return err
			}
		}
		prof, ok, _ := s.availability(ctx, c, contact)
		cur := stateOf(c)
		next, err := Next(cur, Event{Action: HandToHuman, TakeoverMode: takeoverMode(prof), AIAvailable: ok})
		if err != nil {
			return transitionError(err, "")
		}
		self := assignee != nil && p.Kind == principal.KindUser && *assignee == p.ID
		if err := s.applyManual(ctx, tx, q, p, c, cur, next, prof, assignee, !self, note); err != nil {
			return err
		}
		if !sameUser(c.AssigneeID, assignee) {
			if _, err = s.reassign(ctx, tx, q, p, c, assignee); err != nil {
				return err
			}
		}
		updated, err := q.GetConversation(ctx, c.ID)
		if err != nil {
			return err
		}
		if self {
			return nil
		}
		return s.notify(ctx, tx, q, updated, "conversation.handed_over", assignee)
	})
}

// reassign changes the assignee inside an ownership change and records it
// like Assign does.
func (s *Service) reassign(ctx context.Context, tx pgx.Tx, q *db.Queries, p *principal.Principal, c db.Conversation, assignee *uuid.UUID) (db.Conversation, error) {
	updated, err := q.SetConversationAssignee(ctx, db.SetConversationAssigneeParams{ID: c.ID, AssigneeID: assignee})
	if err != nil {
		return c, err
	}
	kind := "assigned"
	if assignee == nil {
		kind = "unassigned"
	}
	if _, err := q.InsertConversationEvent(ctx, db.InsertConversationEventParams{ConversationID: c.ID, Kind: kind, ActorType: string(p.Kind),
		ActorID: &p.ID, AssigneeID: assignee}); err != nil {
		return updated, err
	}
	data := activity.Data{"assignee_id": assignee}
	if assignee != nil {
		if u, err := q.GetAssignableUser(ctx, *assignee); err == nil {
			data["assignee_name"] = u.Name
		}
	}
	if _, err := activity.Record(ctx, tx, q, c.ID, activity.AssigneeChanged, activity.Of(p), data); err != nil {
		return updated, err
	}
	if err := audit.Record(ctx, q, audit.Entry{Action: "conversation." + kind, EntityType: "conversation", EntityID: &c.ID,
		Changes: map[string]any{"assignee_id": []any{c.AssigneeID, assignee}}}); err != nil {
		return updated, err
	}
	return updated, realtime.PublishConversationChange(ctx, tx, c, updated)
}

// TakeOver makes the caller the conversation's owner and assignee and
// accepts a pending escalation.
func (s *Service) TakeOver(ctx context.Context, p *principal.Principal, id uuid.UUID) error {
	return s.ownershipChange(ctx, p, id, func(tx pgx.Tx, q *db.Queries, c db.Conversation, contact db.Contact) error {
		prof, ok, _ := s.availability(ctx, c, contact)
		cur := stateOf(c)
		next, err := Next(cur, Event{Action: Takeover, TakeoverMode: takeoverMode(prof), AIAvailable: ok})
		if err != nil {
			return transitionError(err, "")
		}
		self := p.Kind == principal.KindUser
		if self && (c.AssigneeID == nil || *c.AssigneeID != p.ID) {
			if c, err = s.reassign(ctx, tx, q, p, c, &p.ID); err != nil {
				return err
			}
		}
		if next == cur {
			return nil
		}
		var assignee *uuid.UUID
		if self {
			assignee = &p.ID
		}
		return s.applyManual(ctx, tx, q, p, c, cur, next, prof, assignee, false, "")
	})
}

// SetContactAI allows or disables the AI for a contact. Disabling it takes
// the contact's conversations from the AI (to the team's pool) and stops
// suggestions.
func (s *Service) SetContactAI(ctx context.Context, p *principal.Principal, contactID uuid.UUID, disabled bool) (db.Contact, error) {
	var out db.Contact
	err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		old, err := q.GetContact(ctx, contactID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Contact not found.")
		}
		if err != nil {
			return err
		}
		if out, err = q.SetContactAIDisabled(ctx, db.SetContactAIDisabledParams{ID: contactID, AiDisabled: disabled}); err != nil {
			return err
		}
		if old.AiDisabled == disabled {
			return nil
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: "contact.ai", EntityType: "contact", EntityID: &contactID,
			Changes: map[string]any{"ai_disabled": []any{old.AiDisabled, disabled}}}); err != nil {
			return err
		}
		convs, err := q.LockConversationsOfContact(ctx, contactID)
		if err != nil {
			return err
		}
		kind := activity.ContactAIEnabled
		if disabled {
			kind = activity.ContactAIDisabled
		}
		for _, c := range convs {
			if disabled {
				cur := stateOf(c)
				next, err := Next(cur, Event{Action: AIUnavailable})
				if err != nil {
					return err
				}
				if next != cur {
					pending := cur.Owner == OwnerAI && c.AssigneeID == nil
					if err := s.applyManual(ctx, tx, q, p, c, cur, next, nil, c.AssigneeID, pending, "contact_ai_disabled"); err != nil {
						return err
					}
				}
			}
			if _, err := activity.Record(ctx, tx, q, c.ID, kind, activity.Of(p), nil); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// AddNote adds an internal note. Notes never reach the customer; with
// forAI the AI reads it as an instruction from the team.
func (s *Service) AddNote(ctx context.Context, p *principal.Principal, id uuid.UUID, body string, forAI bool) (db.ConversationNote, error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 4000 {
		return db.ConversationNote{}, apperr.Validation("body", "A note has 1 to 4000 characters.")
	}
	var n db.ConversationNote
	err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := s.load(ctx, q, p, id); err != nil {
			return err
		}
		authorType, author := "system", (*uuid.UUID)(nil)
		if p.Kind == principal.KindUser {
			authorType, author = "user", &p.ID
		}
		var err error
		if n, err = q.InsertConversationNote(ctx, db.InsertConversationNoteParams{ConversationID: id, AuthorType: authorType, AuthorID: author,
			Body: body, ForAi: forAI}); err != nil {
			return err
		}
		_, err = activity.Record(ctx, tx, q, id, activity.NoteAdded, activity.Of(p), activity.Data{"note_id": n.ID, "for_ai": forAI,
			"preview": activity.Preview(body, 140)})
		return err
	})
	return n, err
}

// Notes lists a conversation's internal notes, newest first.
func (s *Service) Notes(ctx context.Context, p *principal.Principal, id uuid.UUID) ([]db.ListConversationNotesRow, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return nil, err
	}
	return s.q.ListConversationNotes(ctx, db.ListConversationNotesParams{ConversationID: id, Lim: 100})
}

// Handoffs lists a conversation's handoffs, newest first.
func (s *Service) Handoffs(ctx context.Context, p *principal.Principal, id uuid.UUID) ([]db.ListConversationHandoffsRow, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return nil, err
	}
	return s.q.ListConversationHandoffs(ctx, db.ListConversationHandoffsParams{ConversationID: id, Lim: 50})
}

// Activity returns a page of the timeline before the cursor item: the
// latest limit items, oldest first, and whether older ones exist.
func (s *Service) Activity(ctx context.Context, p *principal.Principal, id uuid.UUID, before *uuid.UUID, limit int) ([]db.ListConversationActivityRow, bool, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return nil, false, err
	}
	limit = min(max(limit, 1), 200)
	rows, err := s.q.ListConversationActivity(ctx, db.ListConversationActivityParams{ConversationID: id, BeforeID: before,
		Lim: int32(limit + 1)}) //nolint:gosec // bounded above
	if err != nil {
		return nil, false, err
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	slices.Reverse(rows)
	return rows, more, nil
}

// AutomationRuns lists the automation runs of a conversation, newest first.
func (s *Service) AutomationRuns(ctx context.Context, p *principal.Principal, id uuid.UUID) ([]db.ListConversationAutomationRunsRow, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return nil, err
	}
	return s.q.ListConversationAutomationRuns(ctx, db.ListConversationAutomationRunsParams{ConversationID: &id, Lim: 50})
}

// Drafts lists AI drafts, newest first.
func (s *Service) Drafts(ctx context.Context, p *principal.Principal, id uuid.UUID, pendingOnly bool) ([]db.ConversationAiDraft, error) {
	if _, err := s.load(ctx, s.q, p, id); err != nil {
		return nil, err
	}
	return s.q.ListAIDrafts(ctx, db.ListAIDraftsParams{ConversationID: id, OnlyPending: pendingOnly, Lim: 20})
}

var errDraftDecided = apperr.Conflict("DRAFT_DECIDED", "Someone already decided on this draft.")

// AcceptDraft sends an AI draft, as written or with the edited body.
func (s *Service) AcceptDraft(ctx context.Context, p *principal.Principal, id, draftID uuid.UUID, body *string) (db.ConversationAiDraft, error) {
	var out db.ConversationAiDraft
	err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		c, err := s.load(ctx, q, p, id)
		if err != nil {
			return err
		}
		d, err := q.LockAIDraft(ctx, db.LockAIDraftParams{ID: draftID, ConversationID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Draft not found.")
		}
		if err != nil {
			return err
		}
		if d.Status != "pending" {
			return errDraftDecided
		}
		text, status := d.Body, "accepted"
		if body != nil && strings.TrimSpace(*body) != "" && strings.TrimSpace(*body) != strings.TrimSpace(d.Body) {
			text, status = strings.TrimSpace(*body), "edited"
		}
		contact, err := q.GetContact(ctx, c.ContactID)
		if err != nil {
			return err
		}
		in := messaging.SendInput{PhoneNumberID: &c.PhoneNumberID, To: contact.PhoneE164, Type: "text", Text: &whatsapp.Text{Body: text},
			CreatedBy: &p.ID, Source: "panel", SentBy: string(p.Kind)}
		if d.Kind == "approval" {
			in.Source = "ai"
		}
		if status == "accepted" {
			in.SentBy, in.AIUsageID = activity.ActorAI, d.AiUsageID
		}
		m, err := s.msg.QueueTx(ctx, tx, in)
		if err != nil {
			return err
		}
		if d.Kind == "approval" {
			if err := q.MarkConversationAIDisclosed(ctx, c.ID); err != nil {
				return err
			}
		}
		decider := (*uuid.UUID)(nil)
		if p.Kind == principal.KindUser {
			decider = &p.ID
		}
		if out, err = q.DecideAIDraft(ctx, db.DecideAIDraftParams{ID: d.ID, Status: status, DecidedBy: decider, SentMessageID: &m.ID}); err != nil {
			return err
		}
		_, err = activity.Record(ctx, tx, q, c.ID, activity.AIDraftDecided, activity.Of(p), activity.Data{"draft_id": d.ID, "kind": d.Kind,
			"status": status, "message_id": m.ID})
		return err
	})
	return out, err
}

// RejectDraft discards an AI draft.
func (s *Service) RejectDraft(ctx context.Context, p *principal.Principal, id, draftID uuid.UUID) (db.ConversationAiDraft, error) {
	var out db.ConversationAiDraft
	err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if _, err := s.load(ctx, q, p, id); err != nil {
			return err
		}
		d, err := q.LockAIDraft(ctx, db.LockAIDraftParams{ID: draftID, ConversationID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("Draft not found.")
		}
		if err != nil {
			return err
		}
		if d.Status != "pending" {
			return errDraftDecided
		}
		decider := (*uuid.UUID)(nil)
		if p.Kind == principal.KindUser {
			decider = &p.ID
		}
		if out, err = q.DecideAIDraft(ctx, db.DecideAIDraftParams{ID: d.ID, Status: "rejected", DecidedBy: decider}); err != nil {
			return err
		}
		_, err = activity.Record(ctx, tx, q, id, activity.AIDraftDecided, activity.Of(p), activity.Data{"draft_id": d.ID, "kind": d.Kind,
			"status": "rejected"})
		return err
	})
	return out, err
}
