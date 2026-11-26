package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/activity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Default run variables of the AI steps.
const (
	VarMessage   = "ai_message"
	VarSentiment = "sentiment"
	VarSummary   = "summary"
	VarIntent    = "intent"
	VarSegment   = "segment"
	// IntentOther is the intent when none of the configured ones fits.
	IntentOther = "other"
	// DefaultProfileAttribute is where ai_build_profile writes.
	DefaultProfileAttribute = "ai_profile"
	// contextMessages is how many messages of the thread AI steps read.
	contextMessages = 20
)

// aiAction is the AI profile action a step needs (allowed_actions): steps
// that change the contact need the profile's permission.
func aiAction(st *Step) string {
	switch st.Type {
	case "ai_extract_fields", "ai_build_profile":
		return "update_contact_fields"
	case "ai_classify_segment":
		return "add_tag"
	}
	// Handing over to a person is always allowed (as in AI turns).
	return ""
}

// aiConfigError explains a step that cannot run because of the AI
// configuration (shown on the run and the canvas).
func aiConfigError(code string) *StepError {
	msg := map[string]string{
		CodeAIDisabled:      "AI is turned off for the company: turn it on in Settings → AI, or remove the AI steps.",
		CodeAINotConfigured: "No AI provider or model is chosen: set one up in Settings → AI.",
		CodeAINoProfile:     "The conversation's number has no active AI profile: create one in Settings → AI.",
	}[code]
	if msg == "" {
		msg = "The AI cannot run this step."
	}
	return &StepError{Code: code, Message: msg}
}

// aiFailure converts an AI call error.
func aiFailure(err error) *StepError {
	switch ai.KindOf(err) {
	case ai.KindDisabled:
		return aiConfigError(CodeAIDisabled)
	case ai.KindNotConfigured:
		return &StepError{Code: CodeAINotConfigured, Message: "AI is not set up: " + err.Error()}
	}
	return &StepError{Code: CodeAIError, Message: "AI request failed: " + err.Error(), Retryable: ai.IsRetryable(err) || ai.KindOf(err) == ""}
}

// aiStep runs an AI step: checks, the model call (outside a transaction)
// and the effects (apply, in the recording transaction).
func (s *Service) aiStep(ctx context.Context, st *Step, rc runCtx) stepResult {
	if s.ai == nil {
		return stepResult{err: aiConfigError(CodeAINotConfigured)}
	}
	if !s.ai.Enabled(ctx) {
		return stepResult{err: aiConfigError(CodeAIDisabled)}
	}
	if rc.contact.AiDisabled {
		return stepResult{outcome: "skipped", detail: map[string]any{"reason": "contact_ai_disabled"}}
	}
	var prof *ai.Profile
	if rc.ev.PhoneNumberID != nil {
		p, err := s.ai.ProfileForNumber(ctx, *rc.ev.PhoneNumberID)
		switch {
		case err == nil:
			prof = &p
		case !errors.Is(err, ai.ErrNotFound):
			return stepResult{err: err}
		}
	}
	if need := aiAction(st); need != "" && prof != nil && !prof.Allows(need) {
		return stepResult{err: &StepError{Code: CodeAINotAllowed,
			Message: fmt.Sprintf("The AI profile %q does not allow %q: allow it in the profile or remove the step.", prof.Name, need)}}
	}
	switch st.Type {
	case "ai_auto_reply":
		return s.aiAutoReply(ctx, st, rc)
	case "ai_route_to_agent":
		return s.aiRoute(ctx, st, rc)
	}
	lang := consent.WorkspaceLocale(ctx, s.q)
	call := ai.Call{Purpose: ai.PurposeAutomation, ContactID: &rc.contact.ID, ConversationID: rc.ev.ConversationID, AutomationRunID: &rc.id}
	if prof != nil {
		call.ProfileID = &prof.ID
	}
	transcript := s.transcript(ctx, rc)
	business := ""
	if prof != nil && strings.TrimSpace(prof.SystemPrompt) != "" {
		business = "\n\nAbout the business (from its AI profile):\n" + truncateRunes(prof.SystemPrompt, 8000)
	}
	extra := ""
	if p := strings.TrimSpace(rc.fill(st.Prompt)); p != "" && st.Type != "ai_generate_message" {
		extra = "\n\nAdditional instruction: " + p
	}
	language := "Write in the customer's language; if unclear, in " + map[bool]string{true: "Turkish", false: "English"}[strings.HasPrefix(lang, "tr")] + "."
	user := func(task string) []ai.Message {
		return []ai.Message{{Role: ai.RoleUser, Text: "Conversation (oldest first):\n" + transcript + "\n\nLatest customer message: " + rc.ev.Body + "\n\n" + task}}
	}
	switch st.Type {
	case "ai_generate_message":
		call.Request = ai.Request{System: "You write one WhatsApp message on behalf of a business. Answer with the message text only — no quotes, no explanations. " +
			language + business, Messages: user("Write the message: " + rc.fill(st.Prompt)), MaxTokens: 800}
		res, err := s.ai.Generate(ctx, call)
		if err != nil {
			return stepResult{err: aiFailure(err)}
		}
		text := truncateRunes(strings.TrimSpace(res.Text), maxTextLen)
		if text == "" {
			return stepResult{err: &StepError{Code: CodeAIError, Message: "The AI returned an empty message.", Retryable: true}}
		}
		v := orDefault(st.Variable, VarMessage)
		r := stepResult{outcome: "ok", vars: map[string]string{v: text}, detail: usageDetail(res, map[string]any{"variable": v})}
		if st.Output == "send" {
			r.apply = func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult {
				return s.sendOnce(ctx, tx, q, rc, messaging.SendInput{PhoneNumberID: rc.ev.PhoneNumberID, To: rc.contact.PhoneE164, Type: "text",
					Text: &whatsapp.Text{Body: text}, Source: "automation", AIUsageID: usageRef(res)})
			}
		}
		return r
	case "ai_analyze_message":
		call.Request = ai.Request{System: "You analyse a customer's message for a business. " + language + business + extra, Messages: user(
			"Give the sentiment of the latest customer message (positive, neutral or negative) and a one or two sentence summary of what the customer wants."),
			Output: &ai.OutputSchema{Name: "messora_analysis", Schema: object(map[string]any{
				"sentiment": map[string]any{"type": "string", "enum": []string{"positive", "neutral", "negative"}},
				"summary":   map[string]any{"type": "string"},
			})}}
		var out struct {
			Sentiment string `json:"sentiment"`
			Summary   string `json:"summary"`
		}
		res, serr := s.structured(ctx, call, &out)
		if serr != nil {
			return stepResult{err: serr}
		}
		if !slices.Contains([]string{"positive", "neutral", "negative"}, out.Sentiment) {
			out.Sentiment = "neutral"
		}
		return stepResult{outcome: "ok", vars: map[string]string{VarSentiment: out.Sentiment, VarSummary: truncateRunes(out.Summary, 1000)},
			detail: usageDetail(res, map[string]any{"sentiment": out.Sentiment})}
	case "ai_detect_intent":
		options := append(slices.Clone(st.Intents), IntentOther)
		call.Request = ai.Request{System: "You classify what a customer wants. Choose exactly one intent from the list; choose \"other\" when none fits. " +
			business + extra, Messages: user("Intents: " + strings.Join(options, ", ") + ". Which intent does the latest customer message have?"),
			Output: &ai.OutputSchema{Name: "messora_intent", Schema: object(map[string]any{
				"intent":     map[string]any{"type": "string", "enum": options},
				"confidence": map[string]any{"type": "number"},
			})}}
		var out struct {
			Intent     string  `json:"intent"`
			Confidence float64 `json:"confidence"`
		}
		res, serr := s.structured(ctx, call, &out)
		if serr != nil {
			return stepResult{err: serr}
		}
		if !slices.Contains(options, out.Intent) {
			out.Intent = IntentOther
		}
		v := orDefault(st.Variable, VarIntent)
		return stepResult{outcome: "ok", vars: map[string]string{v: out.Intent},
			detail: usageDetail(res, map[string]any{"intent": out.Intent, "confidence": out.Confidence, "variable": v})}
	case "ai_extract_fields":
		props := map[string]any{}
		for i, f := range st.Fields {
			desc := f.Key
			if f.Label != "" {
				desc = f.Label + " (" + f.Key + ")"
			}
			props[fmt.Sprintf("f%d", i)] = map[string]any{"type": []string{"string", "null"}, "description": desc}
		}
		var list []string
		for i, f := range st.Fields {
			list = append(list, fmt.Sprintf("f%d = %s", i, orDefault(f.Label, f.Key)))
		}
		call.Purpose = ai.PurposeAutomation
		call.Request = ai.Request{System: "You extract facts the customer stated about themselves. Only use what the customer wrote; use null for anything not stated. Never guess." +
			extra, Messages: user("Fields: " + strings.Join(list, "; ") + "."), Output: &ai.OutputSchema{Name: "messora_fields",
			Schema: object(map[string]any{"fields": object(props)})}}
		var out struct {
			Fields map[string]*string `json:"fields"`
		}
		res, serr := s.structured(ctx, call, &out)
		if serr != nil {
			return stepResult{err: serr}
		}
		found := map[string]string{}
		for i, f := range st.Fields {
			if v := out.Fields[fmt.Sprintf("f%d", i)]; v != nil && strings.TrimSpace(*v) != "" {
				found[f.Key] = truncateRunes(strings.TrimSpace(*v), 500)
			}
		}
		r := stepResult{outcome: "ok", detail: usageDetail(res, nil)}
		r.apply = func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult {
			written, err := s.writeFields(ctx, tx, q, rc, found)
			if err != nil {
				return stepResult{err: err}
			}
			return stepResult{outcome: "ok", detail: map[string]any{"fields": written, "source": SourceAIUnverified}}
		}
		return r
	case "ai_classify_segment":
		options := slices.Clone(st.Tags)
		labels := options
		if len(st.GroupIDs) > 0 {
			options, labels = nil, nil
			for _, g := range st.GroupIDs {
				grp, err := s.q.GetContactGroup(ctx, g)
				if err != nil {
					continue // a deleted group is no longer a choice
				}
				options = append(options, g.String())
				labels = append(labels, g.String()+" = "+grp.Name)
			}
			if len(options) == 0 {
				return stepResult{outcome: "skipped", detail: map[string]any{"reason": "no_groups"}}
			}
		}
		choices := append(slices.Clone(options), "none")
		call.Request = ai.Request{System: "You put a customer into exactly one segment. Choose \"none\" when no segment fits." + business + extra,
			Messages: user("Segments: " + strings.Join(labels, "; ") + ". Which segment fits the customer?"),
			Output:   &ai.OutputSchema{Name: "messora_segment", Schema: object(map[string]any{"segment": map[string]any{"type": "string", "enum": choices}})}}
		var out struct {
			Segment string `json:"segment"`
		}
		res, serr := s.structured(ctx, call, &out)
		if serr != nil {
			return stepResult{err: serr}
		}
		v := orDefault(st.Variable, VarSegment)
		if !slices.Contains(options, out.Segment) {
			return stepResult{outcome: "ok", vars: map[string]string{v: "none"}, detail: usageDetail(res, map[string]any{"segment": "none"})}
		}
		choice := out.Segment
		r := stepResult{outcome: "ok", vars: map[string]string{v: choice}, detail: usageDetail(res, map[string]any{"segment": choice})}
		r.apply = func(ctx context.Context, _ pgx.Tx, q *db.Queries) stepResult {
			if len(st.GroupIDs) > 0 {
				g, _ := uuid.Parse(choice)
				if _, err := q.AddGroupMembers(ctx, db.AddGroupMembersParams{GroupID: g, ContactIds: []uuid.UUID{rc.contact.ID}}); err != nil {
					return stepResult{err: err}
				}
				return stepResult{outcome: "ok", detail: map[string]any{"group_id": g}}
			}
			t, err := q.UpsertTag(ctx, db.UpsertTagParams{Name: strings.TrimSpace(choice), Color: "gray"})
			if err == nil {
				err = q.AddContactTag(ctx, db.AddContactTagParams{ContactID: rc.contact.ID, TagID: t.ID})
			}
			if err != nil {
				return stepResult{err: err}
			}
			return stepResult{outcome: "ok", detail: map[string]any{"tag": choice}}
		}
		return r
	case "ai_build_profile":
		call.Request = ai.Request{System: "You keep a short customer profile for a business: who the customer is, what they are interested in and how they prefer to be served. " +
			"Only use what the conversation shows. " + language + extra, Messages: user("Write the customer's profile."),
			Output: &ai.OutputSchema{Name: "messora_profile", Schema: object(map[string]any{
				"summary":   map[string]any{"type": "string"},
				"interests": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			})}}
		var out struct {
			Summary   string   `json:"summary"`
			Interests []string `json:"interests"`
		}
		res, serr := s.structured(ctx, call, &out)
		if serr != nil {
			return stepResult{err: serr}
		}
		attr := orDefault(st.Attribute, DefaultProfileAttribute)
		found := map[string]string{}
		if t := strings.TrimSpace(out.Summary); t != "" {
			found["attr:"+attr] = truncateRunes(t, 500)
		}
		if len(out.Interests) > 0 {
			found["attr:"+attr+"_interests"] = truncateRunes(strings.Join(out.Interests, ", "), 500)
		}
		r := stepResult{outcome: "ok", vars: map[string]string{"profile": found["attr:"+attr]}, detail: usageDetail(res, map[string]any{"attribute": attr})}
		r.apply = func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult {
			written, err := s.writeFields(ctx, tx, q, rc, found)
			if err != nil {
				return stepResult{err: err}
			}
			return stepResult{outcome: "ok", detail: map[string]any{"fields": written, "source": SourceAIUnverified}}
		}
		return r
	}
	return stepResult{err: fmt.Errorf("unsupported step %q", st.Type)}
}

// aiAutoReply sends one reply of the number's AI profile.
func (s *Service) aiAutoReply(ctx context.Context, st *Step, rc runCtx) stepResult {
	if s.inbox == nil || rc.ev.ConversationID == nil {
		return stepResult{outcome: "skipped", detail: map[string]any{"reason": "no_conversation"}}
	}
	reply, err := s.inbox.ComposeAIReply(ctx, *rc.ev.ConversationID, rc.id)
	if err != nil {
		var se *StepError
		if errors.As(err, &se) {
			return stepResult{err: se}
		}
		return stepResult{err: aiFailure(err)}
	}
	if reply.Skipped != "" {
		return stepResult{outcome: "skipped", detail: map[string]any{"reason": reply.Skipped}}
	}
	detail := map[string]any{"intent": reply.Intent, "confidence": reply.Confidence}
	if reply.UsageID != nil {
		detail["ai_usage_id"] = *reply.UsageID
	}
	if reply.Escalate {
		// The profile's rules hand this one to a person.
		detail["escalated"], detail["reason"] = true, reply.Reason
		r := stepResult{outcome: "ok", detail: detail}
		r.apply = func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult {
			return s.inboxStep(ctx, tx, q, &Step{Type: "handoff_to_human", Text: reply.Reason}, rc, reply.Summary)
		}
		return r
	}
	r := stepResult{outcome: "ok", detail: detail, vars: map[string]string{VarMessage: reply.Text}}
	r.apply = func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult {
		res := s.sendOnce(ctx, tx, q, rc, messaging.SendInput{PhoneNumberID: rc.ev.PhoneNumberID, To: rc.contact.PhoneE164, Type: "text",
			Text: &whatsapp.Text{Body: reply.Text}, Source: "ai", SentBy: activity.ActorAI, AIUsageID: reply.UsageID})
		if res.outcome == "ok" && res.detail["deduplicated"] == nil {
			if reply.Disclose {
				if err := q.MarkConversationAIDisclosed(ctx, *rc.ev.ConversationID); err != nil {
					return stepResult{err: err}
				}
			}
			if len(reply.Fields) > 0 {
				if _, err := s.writeFields(ctx, tx, q, rc, reply.Fields); err != nil {
					return stepResult{err: err}
				}
			}
		}
		return res
	}
	return r
}

// aiRoute hands the conversation to a person with the AI's summary.
func (s *Service) aiRoute(ctx context.Context, st *Step, rc runCtx) stepResult {
	if s.inbox == nil || rc.ev.ConversationID == nil {
		return stepResult{outcome: "skipped", detail: map[string]any{"reason": "no_conversation"}}
	}
	sum, err := s.inbox.ComposeSummary(ctx, *rc.ev.ConversationID, rc.id)
	if err != nil {
		return stepResult{err: err}
	}
	r := stepResult{outcome: "ok", vars: map[string]string{VarSummary: sum.Text}, detail: map[string]any{"summary_source": sum.Source}}
	r.apply = func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult {
		return s.inboxStep(ctx, tx, q, &Step{Type: "handoff_to_human", AssigneeID: st.AssigneeID, Text: st.Text}, rc, &sum)
	}
	return r
}

// writeFields stores fields the AI found (provenance ai_unverified) and
// records them on the conversation's timeline. Values a field cannot take
// are left out.
func (s *Service) writeFields(ctx context.Context, tx pgx.Tx, q *db.Queries, rc runCtx, found map[string]string) ([]string, error) {
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var written []string
	for _, k := range keys {
		err := s.fields.SetFieldsWithSource(ctx, q, rc.contact.ID, map[string]string{k: found[k]}, SourceAIUnverified)
		if _, ok := apperr.As(err); ok {
			continue
		}
		if err != nil {
			return nil, err
		}
		written = append(written, k)
		if rc.ev.ConversationID != nil {
			if _, err := activity.Record(ctx, tx, q, *rc.ev.ConversationID, activity.AIAction, activity.AI, activity.Data{"action": "update_field",
				"field": k, "source": SourceAIUnverified, "by": "automation"}); err != nil {
				return nil, err
			}
		}
	}
	if written == nil {
		written = []string{}
	}
	return written, nil
}

// structured runs a call with a structured output and decodes it into out.
func (s *Service) structured(ctx context.Context, call ai.Call, out any) (ai.Result, *StepError) {
	res, err := s.ai.Generate(ctx, call)
	if err != nil {
		return res, aiFailure(err)
	}
	raw := res.Structured
	if len(raw) == 0 {
		raw = []byte(res.Text)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return res, &StepError{Code: CodeAIError, Message: "The AI answered in an unexpected format.", Retryable: true}
	}
	return res, nil
}

// transcript is the recent messages of the run's thread, oldest first.
func (s *Service) transcript(ctx context.Context, rc runCtx) string {
	if rc.ev.PhoneNumberID == nil {
		return "(no conversation)"
	}
	rows, err := s.q.ListThreadMessages(ctx, db.ListThreadMessagesParams{ContactID: rc.contact.ID, PhoneNumberID: *rc.ev.PhoneNumberID, Lim: contextMessages})
	if err != nil || len(rows) == 0 {
		return "(no messages)"
	}
	slices.Reverse(rows)
	var b strings.Builder
	for _, m := range rows {
		who := "Customer"
		if m.Direction == "outbound" {
			who = "Business"
		}
		body := strings.TrimSpace(m.Body)
		if body == "" {
			body = "[" + m.Type + "]"
		}
		fmt.Fprintf(&b, "%s: %s\n", who, truncateRunes(body, 1000))
	}
	return strings.TrimSpace(b.String())
}

// object is a strict-mode JSON schema object with every property required.
func object(props map[string]any) map[string]any {
	req := make([]string, 0, len(props))
	for k := range props {
		req = append(req, k)
	}
	slices.Sort(req)
	return map[string]any{"type": "object", "properties": props, "required": req, "additionalProperties": false}
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func usageRef(res ai.Result) *uuid.UUID {
	if res.UsageID == uuid.Nil {
		return nil
	}
	return &res.UsageID
}

func usageDetail(res ai.Result, d map[string]any) map[string]any {
	if d == nil {
		d = map[string]any{}
	}
	if res.UsageID != uuid.Nil {
		d["ai_usage_id"] = res.UsageID
	}
	if res.Model != "" {
		d["model"] = res.Model
	}
	return d
}
