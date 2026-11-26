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
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
)

// ReasonAutomation is the handoff reason of automation steps.
const ReasonAutomation Reason = "automation"

// The conversations module is the automation engine's inbox.
var _ automation.Inbox = (*Service)(nil)

// AutomationHandOff implements automation.Inbox: an automation step gives
// the conversation to the AI or to a person (the assignee, else the
// team's pool, waiting to be taken). It runs in the run's transaction and
// records the handoff, the timeline and the notice like a person's
// handoff does, with the system as the actor.
func (s *Service) AutomationHandOff(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, h automation.HandoffRequest) (automation.HandoffResult, error) {
	c, err := q.LockConversation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return automation.HandoffResult{Skipped: "no_conversation"}, nil
	}
	if err != nil {
		return automation.HandoffResult{}, err
	}
	contact, err := q.GetContact(ctx, c.ContactID)
	if err != nil {
		return automation.HandoffResult{}, err
	}
	prof, ok, why := s.availability(ctx, c, contact)
	cur := stateOf(c)
	var next State
	switch h.To {
	case "ai":
		mode := Mode(h.Mode)
		if mode == "" {
			mode = ModeAutoReply
		}
		next, err = Next(cur, Event{Action: HandToAI, Mode: mode, AIAvailable: ok})
		if errors.Is(err, ErrAIUnavailable) {
			return automation.HandoffResult{Skipped: "ai_unavailable_" + why}, nil
		}
		if err == nil && next == cur {
			return automation.HandoffResult{Skipped: "already_with_ai"}, nil
		}
	default:
		if h.AssigneeID != nil {
			if _, err := q.GetAssignableUser(ctx, *h.AssigneeID); errors.Is(err, pgx.ErrNoRows) {
				return automation.HandoffResult{Skipped: "assignee_unavailable"}, nil
			} else if err != nil {
				return automation.HandoffResult{}, err
			}
		}
		next, err = Next(cur, Event{Action: HandToHuman, TakeoverMode: takeoverMode(prof), AIAvailable: ok})
		next.PendingHandoff = true // a person has to take it
	}
	if err != nil {
		return automation.HandoffResult{Skipped: "invalid_transition"}, nil //nolint:nilerr // reported as a skipped step
	}
	toType, toUser := "human", h.AssigneeID
	switch {
	case h.To == "ai":
		toType, toUser = "ai", nil
	case h.AssigneeID == nil:
		toType = "pool"
	}
	from := c.AssigneeID
	if cur.Owner == OwnerAI {
		from = nil
	}
	params := db.InsertConversationHandoffParams{ConversationID: c.ID, FromType: string(cur.Owner), FromUserID: from, ToType: toType,
		ToUserID: toUser, Reason: string(ReasonAutomation), ReasonDetail: truncateRunes(h.Note, 500), SummarySource: "none",
		CollectedFields: []byte("{}"), ActorType: activity.ActorSystem}
	if sum := h.Summary; sum != nil {
		params.Summary, params.DetectedNeed, params.SuggestedNextAction = truncateRunes(sum.Text, 4000), truncateRunes(sum.Need, 1000), truncateRunes(sum.NextAction, 1000)
		if sum.Source == "ai" || sum.Source == "fallback" {
			params.SummarySource = sum.Source
		}
	}
	ho, err := q.InsertConversationHandoff(ctx, params)
	if err != nil {
		return automation.HandoffResult{}, err
	}
	var profileID *uuid.UUID
	if prof != nil && next.Mode != ModeOff {
		profileID = &prof.ID
	}
	updated, err := s.persist(ctx, tx, q, c, next, profileID, &ho.ID, string(ReasonAutomation))
	if err != nil {
		return automation.HandoffResult{}, err
	}
	if h.To != "ai" && !sameUser(c.AssigneeID, h.AssigneeID) && h.AssigneeID != nil {
		if updated, err = q.SetConversationAssignee(ctx, db.SetConversationAssigneeParams{ID: c.ID, AssigneeID: h.AssigneeID}); err != nil {
			return automation.HandoffResult{}, err
		}
		if _, err := q.InsertConversationEvent(ctx, db.InsertConversationEventParams{ConversationID: c.ID, Kind: "assigned",
			ActorType: activity.ActorSystem, AssigneeID: h.AssigneeID}); err != nil {
			return automation.HandoffResult{}, err
		}
		if _, err := activity.Record(ctx, tx, q, c.ID, activity.AssigneeChanged, activity.System, activity.Data{"assignee_id": h.AssigneeID,
			"by": "automation"}); err != nil {
			return automation.HandoffResult{}, err
		}
	}
	kind, data := activity.HandedToHuman, activity.Data{"by": "automation", "handoff_id": ho.ID, "to": "pool"}
	if h.To == "ai" {
		kind, data = activity.HandedToAI, activity.Data{"by": "automation", "handoff_id": ho.ID, "mode": next.Mode}
	} else if h.AssigneeID != nil {
		data["to"], data["assignee_id"] = "user", *h.AssigneeID
	}
	if h.Note != "" {
		data["note"] = activity.Preview(h.Note, 200)
	}
	if _, err := activity.Record(ctx, tx, q, c.ID, kind, activity.System, data); err != nil {
		return automation.HandoffResult{}, err
	}
	if h.To == "ai" {
		return automation.HandoffResult{HandoffID: &ho.ID}, s.scheduleUnanswered(ctx, tx, q, updated)
	}
	return automation.HandoffResult{HandoffID: &ho.ID}, s.notify(ctx, tx, q, updated, "conversation.handed_over", h.AssigneeID)
}

// AutomationNote implements automation.Inbox.
func (s *Service) AutomationNote(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, body string, forAI bool) (uuid.UUID, error) {
	n, err := q.InsertConversationNote(ctx, db.InsertConversationNoteParams{ConversationID: id, AuthorType: "system", Body: body, ForAi: forAI})
	if err != nil {
		return uuid.Nil, err
	}
	_, err = activity.Record(ctx, tx, q, id, activity.NoteAdded, activity.System, activity.Data{"note_id": n.ID, "for_ai": forAI,
		"preview": activity.Preview(body, 140), "by": "automation"})
	return n.ID, err
}

// automationTurn loads what an automation's AI step sees of a
// conversation: the context messages, the notes for the AI and the
// number's profile (nil when there is none or the AI is unavailable).
func (s *Service) automationTurn(ctx context.Context, id uuid.UUID) (*turn, bool, string, error) {
	c, err := s.q.GetConversation(ctx, id)
	if err != nil {
		return nil, false, "", err
	}
	t := &turn{conv: c, lang: language(ctx, s.q)}
	if t.contact, err = s.q.GetContact(ctx, c.ContactID); err != nil {
		return nil, false, "", err
	}
	prof, ok, why := s.availability(ctx, c, t.contact)
	t.profile = prof
	limit := int32(20)
	var st ai.Settings
	if s.ai != nil {
		if st, err = s.ai.Settings(ctx); err == nil {
			limit = int32(st.ContextSize()) //nolint:gosec // bounded by MaxContextMessages
		}
	}
	rows, err := s.q.ListThreadMessages(ctx, db.ListThreadMessagesParams{ContactID: c.ContactID, PhoneNumberID: c.PhoneNumberID, Lim: limit})
	if err != nil {
		return nil, false, "", err
	}
	slices.Reverse(rows)
	t.msgs = rows
	if len(rows) > 0 {
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i].Direction == "inbound" {
				t.msgID = rows[i].ID
				break
			}
		}
	}
	if ok {
		notes, err := s.q.ListAINotes(ctx, db.ListAINotesParams{ConversationID: c.ID, Lim: maxAINotes})
		if err != nil {
			return nil, false, "", err
		}
		slices.Reverse(notes)
		t.request = BuildTurn(TurnInput{Profile: *prof, Settings: st, Lang: t.lang, Messages: rows, Notes: notes, Contact: t.contact})
	}
	return t, ok, why, nil
}

// ComposeAIReply implements automation.Inbox: one reply of the number's AI
// profile, decided by the profile's handoff rules. A conversation the AI
// owns is skipped (the AI reply loop answers it already).
func (s *Service) ComposeAIReply(ctx context.Context, id, runID uuid.UUID) (automation.AIReply, error) {
	if s.ai == nil {
		return automation.AIReply{}, &automation.StepError{Code: automation.CodeAINotConfigured, Message: "AI is not available on this server."}
	}
	t, ok, why, err := s.automationTurn(ctx, id)
	if err != nil {
		return automation.AIReply{}, err
	}
	if t.conv.OwnerType == string(OwnerAI) {
		return automation.AIReply{Skipped: "ai_owns_conversation"}, nil
	}
	if !ok {
		switch why {
		case "contact_disabled":
			return automation.AIReply{Skipped: "contact_ai_disabled"}, nil
		case "disabled":
			return automation.AIReply{}, &automation.StepError{Code: automation.CodeAIDisabled,
				Message: "AI is turned off for the company: turn it on in Settings → AI, or remove the AI steps."}
		}
		return automation.AIReply{}, &automation.StepError{Code: automation.CodeAINoProfile, Message: unavailableMessages[why] +
			" Create or resume the number's AI profile in Settings → AI."}
	}
	if !messaging.WindowOpen(t.conv.LastInboundAt, time.Now()) {
		return automation.AIReply{Skipped: "window_closed"}, nil
	}
	res, err := s.ai.Generate(ctx, ai.Call{Purpose: ai.PurposeAutomation, ProfileID: &t.profile.ID, ConversationID: &t.conv.ID,
		ContactID: &t.conv.ContactID, AutomationRunID: &runID, Request: t.request})
	if err != nil {
		return automation.AIReply{}, err
	}
	raw := res.Structured
	if len(raw) == 0 {
		raw = []byte(res.Text)
	}
	reply, err := ai.DecodeReply(raw)
	if err != nil {
		return automation.AIReply{}, &automation.StepError{Code: automation.CodeAIError, Message: "The AI answered in an unexpected format.", Retryable: true}
	}
	out := automation.AIReply{Intent: reply.Intent, Confidence: reply.Confidence}
	if res.UsageID != uuid.Nil {
		out.UsageID = &res.UsageID
	}
	t.intent = reply.Intent
	if d := Decide(reply, *t.profile, customerBurst(t.msgs)); d.Escalate {
		sum := s.summarize(ctx, t, d.Reason)
		out.Escalate, out.Reason = true, string(d.Reason)
		out.Summary = &automation.Summary{Text: sum.Text, Need: sum.Need, NextAction: sum.NextAction, Source: sum.Source}
		return out, nil
	}
	text, disclose := s.disclosed(ctx, s.q, t.conv, t.profile, t.lang, strings.TrimSpace(reply.Reply))
	if text == "" {
		return automation.AIReply{}, &automation.StepError{Code: automation.CodeAIError, Message: "The AI returned an empty reply.", Retryable: true}
	}
	out.Text, out.Disclose = truncateRunes(text, 4096), disclose
	p := t.profile
	// Like the AI's own turns (collect): the profile's data collection with
	// "may update fields"; the writer applies the provenance rules.
	if p.DataCollection.Enabled && p.DataCollection.MayUpdateFields {
		out.Fields = map[string]string{}
		for _, f := range p.DataCollection.Fields {
			if v := reply.ExtractedFields[f.Key]; v != nil && strings.TrimSpace(*v) != "" {
				// Checked against the field's type, as in collect.
				if val, ok := ai.ValidateValue(f, *v); ok {
					out.Fields[f.Key] = val
				}
			}
		}
	}
	return out, nil
}

// ComposeSummary implements automation.Inbox: the AI's handoff summary,
// or the one built from the messages when the AI cannot write it.
func (s *Service) ComposeSummary(ctx context.Context, id, _ uuid.UUID) (automation.Summary, error) {
	t, ok, _, err := s.automationTurn(ctx, id)
	if err != nil {
		return automation.Summary{}, err
	}
	var sum Summary
	if ok {
		sum = s.summarize(ctx, t, ReasonAutomation)
	} else {
		sum = FallbackSummary(t.msgs, ReasonAutomation, "", t.lang)
	}
	if sum.NextAction == "" {
		sum.NextAction = map[bool]string{true: "Sohbeti devralın.", false: "Take the conversation over."}[t.lang == ai.LangTR]
	}
	return automation.Summary{Text: sum.Text, Need: sum.Need, NextAction: sum.NextAction, Source: sum.Source}, nil
}
