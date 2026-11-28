package conversations

import (
	"context"
	"encoding/json"
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
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// TypeAIReply is the worker task of one AI turn (queue ai).
const TypeAIReply = "ai:reply"

// DefaultDebounce is how long an AI turn waits for more customer messages
// of a burst before answering them at once.
const DefaultDebounce = 3 * time.Second

// maxAINotes bounds the notes for the AI given to one turn (latest win).
const maxAINotes = 20

// SetAI connects the AI module (the AI reply loop is off without it).
func (s *Service) SetAI(a *ai.Service) { s.ai = a }

// SetDebounce changes the burst delay (tests).
func (s *Service) SetDebounce(d time.Duration) { s.debounce = d }

// AIReplyPayload is the payload of a TypeAIReply task.
type AIReplyPayload struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	MessageID      uuid.UUID `json:"message_id"`
}

func stateOf(c db.Conversation) State {
	return State{Owner: Owner(c.OwnerType), Mode: Mode(c.AiMode), PendingHandoff: c.PendingHandoffID != nil}
}

// takeoverMode is what the AI does once a person took over.
func takeoverMode(p *ai.Profile) Mode {
	if p != nil && p.Handoff.OnHumanTakeover == ai.TakeoverSuggest {
		return ModeSuggest
	}
	return ModeOff
}

// language is the workspace language of AI texts.
func language(ctx context.Context, q db.Querier) ai.Language {
	if strings.HasPrefix(consent.WorkspaceLocale(ctx, q), "tr") {
		return ai.LangTR
	}
	return ai.LangEN
}

// availability reports whether the AI can work in c: AI is on for the
// company, the number has an active profile and AI is allowed for the
// contact. why is a short reason when it cannot.
func (s *Service) availability(ctx context.Context, c db.Conversation, contact db.Contact) (*ai.Profile, bool, string) {
	if s.ai == nil {
		return nil, false, "disabled"
	}
	if contact.AiDisabled {
		return nil, false, "contact_disabled"
	}
	if !s.ai.Enabled(ctx) {
		return nil, false, "disabled"
	}
	p, err := s.ai.ProfileForNumber(ctx, c.PhoneNumberID)
	if err != nil {
		return nil, false, "no_profile"
	}
	if p.Status != ai.ProfileActive {
		return &p, false, "profile_paused"
	}
	return &p, true, ""
}

var unavailableMessages = map[string]string{
	"disabled":         "AI is turned off for the company.",
	"contact_disabled": "AI is disabled for this contact.",
	"no_profile":       "The conversation's number has no AI profile.",
	"profile_paused":   "The number's AI profile is paused.",
}

// persist stores a new ownership state and announces it.
func (s *Service) persist(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, next State, profileID, pending *uuid.UUID, reason string) (db.Conversation, error) {
	if !next.PendingHandoff {
		pending = nil
	}
	if profileID == nil {
		profileID = c.AiProfileID
	}
	var paused *string
	if next.Owner != OwnerAI && reason != "" {
		paused = &reason
	}
	updated, err := q.SetConversationOwnership(ctx, db.SetConversationOwnershipParams{ID: c.ID, OwnerType: string(next.Owner),
		AiMode: string(next.Mode), AiProfileID: profileID, AiPausedReason: paused, PendingHandoffID: pending})
	if err != nil {
		return updated, err
	}
	if err := realtime.PublishConversationChange(ctx, tx, c, updated); err != nil {
		return updated, err
	}
	return updated, realtime.PublishAll(ctx, tx, realtime.ConversationChannels(updated), realtime.HandoffEvent{
		Envelope: realtime.NewEnvelope(realtime.TypeConversationHandoff), ConversationID: updated.ID, HandoffID: pending,
		OwnerType: updated.OwnerType, AIMode: updated.AiMode, Reason: reason, Pending: updated.PendingHandoffID != nil,
		AssigneeID: updated.AssigneeID})
}

// OnInbound is messaging's inbound hook: on the first customer message it
// sets the conversation's AI mode from the number's AI profile, and for a
// conversation the AI answers (or drafts in) it schedules an AI turn after
// the debounce delay. It runs in the webhook transaction.
func (s *Service) OnInbound(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, m db.Message) error {
	if s.ai == nil || m.Type == "reaction" || m.Type == "unsupported" || m.Type == "system" {
		return nil
	}
	contact, err := q.GetContact(ctx, c.ContactID)
	if err != nil {
		return err
	}
	if contact.AiDisabled {
		return nil
	}
	// A consent keyword (STOP, İPTAL …) is answered by the consent rules,
	// never by the AI.
	if set, err := consent.CachedSettings(ctx, q); err == nil && set.Keywords.Match(m.Body) != "" {
		return nil
	}
	if c.AiModeSetAt == nil && c.OwnerType == string(OwnerNone) {
		if p, ok, _ := s.availability(ctx, c, contact); ok {
			next := State{Owner: OwnerAI, Mode: ModeAutoReply}
			kind := activity.HandedToAI
			if !p.AutoReply {
				next, kind = State{Owner: OwnerNone, Mode: ModeSuggest}, activity.AIModeChanged
			}
			if c, err = s.persist(ctx, tx, q, c, next, &p.ID, nil, ""); err != nil {
				return err
			}
			if _, err := activity.Record(ctx, tx, q, c.ID, kind, activity.System, activity.Data{"mode": next.Mode, "by": "profile",
				"profile_id": p.ID, "profile_name": p.Name}); err != nil {
				return err
			}
		}
	}
	if c.OwnerType != string(OwnerAI) && c.AiMode != string(ModeSuggest) {
		return nil
	}
	_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeAIReply), AIReplyPayload{ConversationID: c.ID, MessageID: m.ID}, s.debounce)
	return err
}

// scheduleUnanswered starts an AI turn for the customer's last message
// when nobody answered it yet (the AI was just given the conversation).
func (s *Service) scheduleUnanswered(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation) error {
	latest, err := q.LatestInboundMessage(ctx, db.LatestInboundMessageParams{ContactID: c.ContactID, PhoneNumberID: c.PhoneNumberID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if (c.AiHandledMessageID != nil && *c.AiHandledMessageID == latest.ID) || !messaging.WindowOpen(c.LastInboundAt, time.Now()) {
		return nil
	}
	// Unanswered: nothing was written after it. Message ids follow insertion
	// order; Meta's timestamps have one-second precision, so
	// last_message_id cannot tell.
	last, err := q.ListThreadMessages(ctx, db.ListThreadMessagesParams{ContactID: c.ContactID, PhoneNumberID: c.PhoneNumberID, Lim: 1})
	if err != nil {
		return err
	}
	if len(last) == 0 || last[0].ID != latest.ID {
		return nil
	}
	_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeAIReply), AIReplyPayload{ConversationID: c.ID, MessageID: latest.ID}, 0)
	return err
}

// setAIStatus records and announces the AI's state in a conversation.
func (s *Service) setAIStatus(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, status, detail string) (db.Conversation, error) {
	var d *string
	if detail != "" {
		d = &detail
	}
	c, err := q.SetConversationAIStatus(ctx, db.SetConversationAIStatusParams{ID: id, AiStatus: status, AiStatusDetail: d})
	if err != nil {
		return c, err
	}
	return c, realtime.Publish(ctx, tx, realtime.ConversationChannel(id), realtime.AIStatusEvent{
		Envelope: realtime.NewEnvelope(realtime.TypeAIStatus), ConversationID: id, Status: status, Detail: detail})
}

func (s *Service) withTx(ctx context.Context, fn func(tx pgx.Tx, q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// turn is one AI turn in progress.
type turn struct {
	conv    db.Conversation
	msgID   uuid.UUID
	contact db.Contact
	profile *ai.Profile
	lang    ai.Language
	msgs    []db.Message // context, oldest first
	request ai.Request
	intent  string
	usageID *uuid.UUID // the AI request of the reply
}

// HandleAIReply runs the AI turn for customer message msgID (task
// TypeAIReply). It is idempotent: a turn acts at most once per message
// (ai_handled_message_id is claimed in the transaction that sends, drafts
// or escalates), and a message followed by a newer one is skipped — the
// newer message's turn answers the burst. It never drops a turn silently:
// when the AI cannot answer a conversation it owns, a person gets it.
func (s *Service) HandleAIReply(ctx context.Context, convID, msgID uuid.UUID) error {
	if s.ai == nil {
		return nil
	}
	c, err := s.q.GetConversation(ctx, convID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	aiOwned := c.OwnerType == string(OwnerAI)
	mode := Mode(c.AiMode)
	if (!aiOwned && mode != ModeSuggest) || (c.AiHandledMessageID != nil && *c.AiHandledMessageID == msgID) {
		return nil
	}
	latest, err := s.q.LatestInboundMessage(ctx, db.LatestInboundMessageParams{ContactID: c.ContactID, PhoneNumberID: c.PhoneNumberID})
	if err != nil {
		return err
	}
	if latest.ID != msgID {
		return nil // a newer customer message has its own turn
	}
	t := &turn{conv: c, msgID: msgID, lang: language(ctx, s.q)}
	if t.contact, err = s.q.GetContact(ctx, c.ContactID); err != nil {
		return err
	}
	prof, ok, why := s.availability(ctx, c, t.contact)
	t.profile = prof
	if !ok {
		return s.fail(ctx, t, ReasonAIError, why)
	}
	st, err := s.ai.Settings(ctx)
	if err != nil {
		return err
	}
	n, err := s.q.CountConversationAIRequests(ctx, &c.ID)
	if err != nil {
		return err
	}
	if n >= int64(st.RepliesPerHour()) {
		return s.fail(ctx, t, ReasonAIError, "rate_limited")
	}
	// The monthly AI budget pauses AI replies once it is used up; the
	// people who manage AI are told once per month.
	if b, err := s.ai.BudgetStatus(ctx); err != nil {
		return err
	} else if b.Exceeded {
		if err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error { return s.ai.NotifyBudgetExceeded(ctx, tx, q, b) }); err != nil {
			return err
		}
		return s.fail(ctx, t, ReasonAIBudget, "budget_exceeded")
	}
	if aiOwned && mode == ModeAutoReply && !messaging.WindowOpen(c.LastInboundAt, time.Now()) {
		return s.fail(ctx, t, ReasonWindowClosed, "")
	}
	if err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		_, err := s.setAIStatus(ctx, tx, q, c.ID, "thinking", "")
		return err
	}); err != nil {
		return err
	}

	rows, err := s.q.ListThreadMessages(ctx, db.ListThreadMessagesParams{ContactID: c.ContactID, PhoneNumberID: c.PhoneNumberID,
		Lim: int32(st.ContextSize())}) //nolint:gosec // bounded by MaxContextMessages
	if err != nil {
		return err
	}
	slices.Reverse(rows)
	t.msgs = rows
	notes, err := s.q.ListAINotes(ctx, db.ListAINotesParams{ConversationID: c.ID, Lim: maxAINotes})
	if err != nil {
		return err
	}
	slices.Reverse(notes)
	t.request = BuildTurn(TurnInput{Profile: *prof, Settings: st, Lang: t.lang, Messages: rows, Notes: notes, Contact: t.contact})
	purpose := ai.PurposeReply
	if !aiOwned {
		purpose = ai.PurposeSuggest
	}
	res, gerr := s.ai.Generate(ctx, ai.Call{Purpose: purpose, ProfileID: &prof.ID, ConversationID: &c.ID, ContactID: &c.ContactID, Request: t.request})
	if gerr != nil {
		kind := string(ai.KindOf(gerr))
		if kind == "" {
			kind = "error"
		}
		s.log.WarnContext(ctx, "ai turn failed", "conversation_id", c.ID, "kind", kind)
		if ai.KindOf(gerr) == ai.KindCreditsExhausted {
			// AI with Messora's keys pauses until the monthly reset or a
			// top-up; the conversation goes to a person, the people who
			// manage AI are told once per month.
			if err := s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error { return s.ai.NotifyCreditsExhausted(ctx, tx, q) }); err != nil {
				return err
			}
			return s.fail(ctx, t, ReasonAICredits, kind)
		}
		return s.fail(ctx, t, ReasonAIError, kind)
	}
	raw := res.Structured
	if len(raw) == 0 {
		raw = []byte(res.Text)
	}
	reply, err := ai.DecodeReply(raw)
	if err != nil {
		return s.fail(ctx, t, ReasonAIError, string(ai.KindInvalidOutput))
	}
	t.intent = reply.Intent
	var usageID *uuid.UUID
	if res.UsageID != uuid.Nil {
		usageID = &res.UsageID
	}
	t.usageID = usageID
	if !aiOwned {
		return s.draft(ctx, t, "suggestion", reply, usageID)
	}
	d := Decide(reply, *prof, customerBurst(rows))
	if d.Escalate {
		sum := s.summarize(ctx, t, d.Reason)
		return s.escalate(ctx, t, d.Reason, d.Detail, sum, reply.ExtractedFields)
	}
	if mode == ModeAwaitingApproval {
		return s.draft(ctx, t, "approval", reply, usageID, d)
	}
	return s.send(ctx, t, reply, usageID, d)
}

// customerBurst is the text of the customer's messages since the business
// last wrote (what this turn answers).
func customerBurst(msgs []db.Message) string {
	var parts []string
	for i := len(msgs) - 1; i >= 0 && msgs[i].Direction == "inbound"; i-- {
		parts = append([]string{msgs[i].Body}, parts...)
	}
	return strings.Join(parts, "\n")
}

// fail ends a turn the AI could not do: a conversation the AI owns goes to
// a person (escalation with the reason), a suggestion turn shows the error.
func (s *Service) fail(ctx context.Context, t *turn, reason Reason, detail string) error {
	if t.conv.OwnerType == string(OwnerAI) {
		return s.escalate(ctx, t, reason, detail, FallbackSummary(t.msgs, reason, t.intent, t.lang), nil)
	}
	return s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := q.SetConversationAIHandled(ctx, db.SetConversationAIHandledParams{ID: t.conv.ID, MessageID: &t.msgID}); err != nil {
			return err
		}
		if _, err := s.setAIStatus(ctx, tx, q, t.conv.ID, "error", detail); err != nil {
			return err
		}
		_, err := activity.Record(ctx, tx, q, t.conv.ID, activity.AIError, activity.AI, activity.Data{"reason": string(reason), "detail": detail})
		return err
	})
}

// summarize asks the AI for the handoff summary; when it cannot (or the
// turn failed because of the AI), the summary is built from the messages.
func (s *Service) summarize(ctx context.Context, t *turn, reason Reason) Summary {
	fallback := FallbackSummary(t.msgs, reason, t.intent, t.lang)
	if reason == ReasonAIError || t.profile == nil || len(t.request.Messages) == 0 {
		return fallback
	}
	res, err := s.ai.Generate(ctx, ai.Call{Purpose: ai.PurposeSummarize, ProfileID: &t.profile.ID, ConversationID: &t.conv.ID,
		ContactID: &t.conv.ContactID, Request: summaryRequest(t.request, reason, t.lang)})
	if err != nil {
		return fallback
	}
	raw := res.Structured
	if len(raw) == 0 {
		raw = []byte(res.Text)
	}
	var sum Summary
	if json.Unmarshal(raw, &sum) != nil || strings.TrimSpace(sum.Text) == "" {
		return fallback
	}
	sum.Source = "ai"
	if sum.Need == "" {
		sum.Need = fallback.Need
	}
	if sum.NextAction == "" {
		sum.NextAction = fallback.NextAction
	}
	return sum
}

// lockTurn locks the conversation and checks that the turn may still act:
// the AI still owns it (or still drafts in it) and the message is not
// handled yet. It claims the message. A turn that may not act clears the
// "thinking" status it set.
func (s *Service) lockTurn(ctx context.Context, tx pgx.Tx, q *db.Queries, t *turn, needAI bool) (db.Conversation, bool, error) {
	c, err := q.LockConversation(ctx, t.conv.ID)
	if err != nil {
		return c, false, err
	}
	if (c.AiHandledMessageID != nil && *c.AiHandledMessageID == t.msgID) ||
		(needAI && c.OwnerType != string(OwnerAI)) || (!needAI && c.AiMode != string(ModeSuggest)) {
		// Done already, or a person took over meanwhile.
		if c.AiStatus == "thinking" {
			_, err = s.setAIStatus(ctx, tx, q, c.ID, "idle", "")
		}
		return c, false, err
	}
	return c, true, q.SetConversationAIHandled(ctx, db.SetConversationAIHandledParams{ID: c.ID, MessageID: &t.msgID})
}

// escalate hands the conversation of a turn to a person.
func (s *Service) escalate(ctx context.Context, t *turn, reason Reason, detail string, sum Summary, fields map[string]*string) error {
	return s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		c, ok, err := s.lockTurn(ctx, tx, q, t, true)
		if err != nil || !ok {
			return err
		}
		_, err = s.escalateTx(ctx, tx, q, c, reason, detail, sum, fields, t.profile)
		return err
	})
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// escalateTx records an escalation of conversation c (owned by the AI):
// the handoff with the summary, the new ownership (a person, waiting),
// the AI status, the timeline, the audit log, and notices to the people
// who may take it — the assignee, or everyone who reads conversations.
func (s *Service) escalateTx(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, reason Reason, detail string, sum Summary,
	fields map[string]*string, prof *ai.Profile) (db.Conversation, error) {
	next, err := Next(stateOf(c), Event{Action: Escalate, TakeoverMode: takeoverMode(prof), AIAvailable: prof != nil})
	if err != nil {
		return c, err
	}
	collected := map[string]string{}
	for k, v := range fields {
		if v != nil && strings.TrimSpace(*v) != "" {
			collected[k] = truncateRunes(strings.TrimSpace(*v), 500)
		}
	}
	raw, _ := json.Marshal(collected)
	to := "pool"
	if c.AssigneeID != nil {
		to = "human"
	}
	source := sum.Source
	if source == "" {
		source = "none"
	}
	h, err := q.InsertConversationHandoff(ctx, db.InsertConversationHandoffParams{ConversationID: c.ID, FromType: "ai", ToType: to,
		ToUserID: c.AssigneeID, Reason: string(reason), ReasonDetail: truncateRunes(detail, 500), Summary: truncateRunes(sum.Text, 4000),
		SummarySource: source, DetectedNeed: truncateRunes(sum.Need, 1000), SuggestedNextAction: truncateRunes(sum.NextAction, 1000),
		CollectedFields: raw, ActorType: activity.ActorAI})
	if err != nil {
		return c, err
	}
	var profileID *uuid.UUID
	if prof != nil {
		profileID = &prof.ID
	}
	updated, err := s.persist(ctx, tx, q, c, next, profileID, &h.ID, string(reason))
	if err != nil {
		return updated, err
	}
	status, statusDetail := "idle", ""
	if reason == ReasonAIError {
		status, statusDetail = "error", detail
	}
	if updated, err = s.setAIStatus(ctx, tx, q, c.ID, status, statusDetail); err != nil {
		return updated, err
	}
	if _, err := activity.Record(ctx, tx, q, c.ID, activity.Escalation, activity.AI, activity.Data{"handoff_id": h.ID,
		"reason": string(reason), "detail": detail, "to": to}); err != nil {
		return updated, err
	}
	// Automations may react to the escalation (trigger ai_escalation).
	if err := trigger.Emit(ctx, tx, q, trigger.Event{Type: trigger.AIEscalation, Key: "escalation:" + h.ID.String(), ContactID: c.ContactID,
		ConversationID: &c.ID, PhoneNumberID: &c.PhoneNumberID, Reason: string(reason), Body: sum.Text}); err != nil {
		return updated, err
	}
	if err := audit.Record(ctx, q, audit.Entry{Action: "conversation.escalate", EntityType: "conversation", EntityID: &c.ID,
		Changes: map[string]any{"reason": string(reason), "handoff_id": h.ID, "owner_type": []any{c.OwnerType, updated.OwnerType}}}); err != nil {
		return updated, err
	}
	return updated, s.notify(ctx, tx, q, updated, "conversation.escalated", nil)
}

// notify tells the people who may take conversation c about it: target,
// else its assignee, else everyone who can read conversations (a user
// with conversations.read alone sees unassigned conversations, so the
// pool may be told).
func (s *Service) notify(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, kind string, target *uuid.UUID) error {
	var users []uuid.UUID
	switch {
	case target != nil:
		users = []uuid.UUID{*target}
	case c.AssigneeID != nil:
		users = []uuid.UUID{*c.AssigneeID}
	default:
		rows, err := q.ListConversationReaders(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if !slices.Contains(users, r.ID) {
				users = append(users, r.ID)
			}
		}
	}
	ev := realtime.NewNotificationEvent(kind)
	ev.ContactID, ev.ConversationID = &c.ContactID, &c.ID
	channels := make([]string, len(users))
	for i, u := range users {
		channels[i] = realtime.PersonalChannel(u)
	}
	return realtime.PublishAll(ctx, tx, channels, ev)
}

// disclosed returns the reply with the AI disclosure in front when the
// profile discloses in the first message and the conversation has not had
// it yet.
func (s *Service) disclosed(ctx context.Context, q *db.Queries, c db.Conversation, p *ai.Profile, lang ai.Language, text string) (string, bool) {
	if p == nil || p.DisclosureMode != ai.DiscloseFirstMessage || c.AiDisclosedAt != nil {
		return text, false
	}
	st, err := s.ai.Settings(ctx)
	if err != nil {
		return text, false
	}
	d := strings.TrimSpace(st.Disclosure.For(lang))
	if d == "" {
		return text, false
	}
	return d + "\n\n" + text, true
}

// send delivers an auto reply through the normal send path (window and
// consent rules apply) and runs the actions the reply asked for.
func (s *Service) send(ctx context.Context, t *turn, reply ai.Reply, usageID *uuid.UUID, d Decision) error {
	return s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		c, ok, err := s.lockTurn(ctx, tx, q, t, true)
		if err != nil || !ok {
			return err
		}
		text, disclose := s.disclosed(ctx, q, c, t.profile, t.lang, strings.TrimSpace(reply.Reply))
		if len([]rune(text)) > 4096 {
			text = string([]rune(text)[:4096])
		}
		sp, err := tx.Begin(ctx)
		if err != nil {
			return err
		}
		m, err := s.msg.QueueTx(ctx, sp, messaging.SendInput{PhoneNumberID: &c.PhoneNumberID, To: t.contact.PhoneE164, Type: "text",
			Text: &whatsapp.Text{Body: text}, Source: "ai", SentBy: activity.ActorAI, AIUsageID: usageID})
		if ae, isApp := apperr.As(err); isApp {
			_ = sp.Rollback(ctx)
			reason, detail := ReasonAIError, ae.Code
			if ae.Code == "OUTSIDE_SERVICE_WINDOW" {
				reason, detail = ReasonWindowClosed, ""
			}
			_, err := s.escalateTx(ctx, tx, q, c, reason, detail, FallbackSummary(t.msgs, reason, reply.Intent, t.lang), reply.ExtractedFields, t.profile)
			return err
		}
		if err != nil {
			return err
		}
		if err := sp.Commit(ctx); err != nil {
			return err
		}
		if disclose {
			if err := q.MarkConversationAIDisclosed(ctx, c.ID); err != nil {
				return err
			}
		}
		if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIReplySent, activity.AI, activity.Data{"message_id": m.ID,
			"confidence": reply.Confidence, "intent": reply.Intent}); err != nil {
			return err
		}
		completed, err := s.act(ctx, tx, q, c, t, reply, d)
		if err != nil {
			return err
		}
		if completed && t.profile != nil && t.profile.DataCollection.HandoffWhenComplete {
			if c, err = q.GetConversation(ctx, c.ID); err != nil {
				return err
			}
			sum := FallbackSummary(t.msgs, ReasonProfileComplete, reply.Intent, t.lang)
			_, err = s.escalateTx(ctx, tx, q, c, ReasonProfileComplete, "", sum, reply.ExtractedFields, t.profile)
			return err
		}
		_, err = s.setAIStatus(ctx, tx, q, c.ID, "idle", "")
		return err
	})
}

// draft stores an AI reply for a person: a suggestion (suggest mode) or a
// reply waiting for approval.
func (s *Service) draft(ctx context.Context, t *turn, kind string, reply ai.Reply, usageID *uuid.UUID, decision ...Decision) error {
	return s.withTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		c, ok, err := s.lockTurn(ctx, tx, q, t, kind == "approval")
		if err != nil || !ok {
			return err
		}
		body := strings.TrimSpace(reply.Reply)
		if kind == "approval" {
			body, _ = s.disclosed(ctx, q, c, t.profile, t.lang, body)
		}
		if body != "" {
			if err := q.SupersedeAIDrafts(ctx, c.ID); err != nil {
				return err
			}
			dr, err := q.InsertAIDraft(ctx, db.InsertAIDraftParams{ConversationID: c.ID, MessageID: &t.msgID, Kind: kind,
				Body: truncateRunes(body, 4096), Confidence: reply.Confidence, Intent: truncateRunes(reply.Intent, 100), AiUsageID: usageID})
			if err != nil {
				return err
			}
			if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIDraftCreated, activity.AI, activity.Data{"draft_id": dr.ID,
				"kind": kind, "confidence": reply.Confidence, "intent": reply.Intent}); err != nil {
				return err
			}
		}
		if len(decision) > 0 {
			if _, err := s.act(ctx, tx, q, c, t, reply, decision[0]); err != nil {
				return err
			}
		}
		_, err = s.setAIStatus(ctx, tx, q, c.ID, "idle", "")
		return err
	})
}

// act applies what an AI reply asked for and the profile allows: collected
// contact fields and automations. Refused actions are recorded. completed
// reports that this turn completed the profile of a new contact.
func (s *Service) act(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, t *turn, reply ai.Reply, d Decision) (bool, error) {
	completed, err := s.collect(ctx, tx, q, c, t, reply)
	if err != nil {
		return false, err
	}
	for _, a := range d.Actions {
		id, ok := strings.CutPrefix(a, "automation:")
		if !ok {
			if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": a, "status": "requested"}); err != nil {
				return false, err
			}
			continue
		}
		autoID, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		if err := s.startAutomation(ctx, tx, q, c, t.msgID, autoID); err != nil {
			return false, err
		}
	}
	for _, a := range d.Refused {
		if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": a, "status": "not_allowed"}); err != nil {
			return false, err
		}
	}
	return completed, nil
}

// setContactField stores a field the AI collected.
func setContactField(ctx context.Context, q *db.Queries, contactID uuid.UUID, key, val string) error {
	if attr, ok := strings.CutPrefix(key, "attr:"); ok {
		return q.SetContactAttribute(ctx, db.SetContactAttributeParams{ID: contactID, Key: attr, Value: val})
	}
	p := db.UpdateContactParams{ID: contactID}
	switch key {
	case "name":
		p.Name = &val
	case "email":
		p.SetEmail, p.Email = true, &val
	case "var1":
		p.SetVar1, p.Var1 = true, &val
	case "var2":
		p.SetVar2, p.Var2 = true, &val
	case "var3":
		p.SetVar3, p.Var3 = true, &val
	case "var4":
		p.SetVar4, p.Var4 = true, &val
	case "var5":
		p.SetVar5, p.Var5 = true, &val
	default:
		return nil
	}
	_, err := q.UpdateContact(ctx, p)
	return err
}

// startAutomation runs an automation the AI asked for (and the profile
// allows) for the conversation, once per customer message.
func (s *Service) startAutomation(ctx context.Context, tx pgx.Tx, q *db.Queries, c db.Conversation, msgID, autoID uuid.UUID) error {
	a, err := q.GetAutomation(ctx, autoID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !a.Active) {
		_, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": "start_automation",
			"automation_id": autoID, "status": "unavailable"})
		return err
	}
	if err != nil {
		return err
	}
	key := "ai:" + msgID.String() + ":" + autoID.String()
	ev := trigger.Event{Type: "ai_action", Key: key, ContactID: c.ContactID, ConversationID: &c.ID, PhoneNumberID: &c.PhoneNumberID, MessageID: &msgID}
	raw, _ := json.Marshal(ev)
	runID, err := q.InsertAutomationRun(ctx, db.InsertAutomationRunParams{AutomationID: a.ID, AutomationVersion: a.Version, EventKey: key,
		Event: raw, ContactID: &c.ContactID, ConversationID: &c.ID, StartedBy: activity.ActorAI})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // already started for this message
	}
	if err != nil {
		return err
	}
	if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(automation.TypeRun), map[string]string{"run_id": runID.String()}, 0); err != nil {
		return err
	}
	if _, err := activity.Record(ctx, tx, q, c.ID, activity.AIAction, activity.AI, activity.Data{"action": "start_automation",
		"automation_id": a.ID, "automation_name": a.Name, "run_id": runID}); err != nil {
		return err
	}
	return activity.AnnounceRun(ctx, tx, q, runID, true)
}
