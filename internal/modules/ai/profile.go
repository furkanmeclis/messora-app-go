package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Handoff decides when the AI passes the conversation to a person.
// OnRequest is always true: Meta's Business Messaging Policy requires a
// clear and quick path to a human, so a customer asking for one is always
// handed over and the setting cannot be turned off.
type Handoff struct {
	OnRequest bool `json:"on_request"`
	// OnLowConfidence hands over when the model's confidence is below the
	// threshold (0..1); nil = off.
	OnLowConfidence *float64 `json:"on_low_confidence"`
	// OnNoAnswer hands over when the AI cannot answer from what it knows.
	OnNoAnswer bool `json:"on_no_answer"`
	// OnCriticalIntents hands over for these intents (e.g. "complaint").
	OnCriticalIntents []string `json:"on_critical_intents"`
	// OnUnauthorizedAction hands over when the customer needs an action the
	// profile may not take.
	OnUnauthorizedAction bool `json:"on_unauthorized_action"`
	// OnHumanTakeover is the AI mode a conversation keeps when a person
	// takes it over (an agent's reply, "take over", an escalation):
	// TakeoverOff (default) or TakeoverSuggest (the AI keeps drafting).
	OnHumanTakeover string `json:"on_human_takeover,omitempty"`
}

// What the AI does after a person takes a conversation over.
const (
	TakeoverOff     = "off"
	TakeoverSuggest = "suggest"
)

// DataField is a piece of customer information the AI may ask for. Key is
// a contact field (name, email, var1…var5) or a custom attribute
// ("attr:<key>").
type DataField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// DataCollection is what the AI may collect and store. Only listed fields
// are collected (data minimisation).
type DataCollection struct {
	Enabled          bool        `json:"enabled"`
	Fields           []DataField `json:"fields"`
	MayCreateContact bool        `json:"may_create_contact"`
	MayUpdateFields  bool        `json:"may_update_fields"`
}

// Disclosure modes: when the AI says it is an AI.
const (
	DiscloseFirstMessage = "always_first_message"
	DiscloseOnQuestion   = "on_question"
	DiscloseOff          = "off"
)

// Profile statuses.
const (
	ProfileActive = "active"
	ProfilePaused = "paused"
)

// ActionKinds are the built-in actions a profile can be allowed to take.
// Automations are allowed individually as "automation:<id>".
var ActionKinds = []string{"create_contact", "update_contact_fields", "add_tag", "assign_conversation", "add_internal_note"}

// ContactFields are the contact fields data collection can fill.
var ContactFields = []string{"name", "email", "var1", "var2", "var3", "var4", "var5"}

// Profile is how the AI behaves on one number.
type Profile struct {
	ID             uuid.UUID
	Name           string
	PhoneNumberID  *uuid.UUID
	PhoneDisplay   string
	SystemPrompt   string
	ProviderID     *uuid.UUID // nil = company default
	Model          *string    // nil = company default (or the provider's)
	AutoReply      bool
	Handoff        Handoff
	DataCollection DataCollection
	AllowedActions []string
	DisclosureMode string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Warnings lists configuration the panel should point out.
func (p Profile) Warnings() []string {
	var w []string
	if p.DisclosureMode == DiscloseOff {
		w = append(w, "disclosure_off")
	}
	if p.PhoneNumberID == nil {
		w = append(w, "no_number")
	}
	return w
}

var (
	attrKeyRe = regexp.MustCompile(`^attr:[A-Za-z0-9_.-]{1,64}$`)
	intentRe  = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
)

// MaxSystemPrompt bounds the system prompt (characters).
const MaxSystemPrompt = 100_000

// normalize validates p and applies the fixed rules (handoff on request).
func (p *Profile) normalize() error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "" || len([]rune(p.Name)) > 120:
		return &FieldError{"name", "Name is required (at most 120 characters)."}
	case len([]rune(p.SystemPrompt)) > MaxSystemPrompt:
		return &FieldError{"system_prompt", fmt.Sprintf("The system prompt is limited to %d characters.", MaxSystemPrompt)}
	case !slices.Contains([]string{DiscloseFirstMessage, DiscloseOnQuestion, DiscloseOff}, p.DisclosureMode):
		return &FieldError{"disclosure_mode", "Unknown disclosure mode."}
	case p.Status != ProfileActive && p.Status != ProfilePaused:
		return &FieldError{"status", "Status must be active or paused."}
	}
	if p.Model != nil {
		m := strings.TrimSpace(*p.Model)
		if m == "" {
			p.Model = nil
		} else if len(m) > 200 {
			return &FieldError{"model", "Model id is too long."}
		} else {
			p.Model = &m
		}
	}
	p.Handoff.OnRequest = true
	switch p.Handoff.OnHumanTakeover {
	case "":
		p.Handoff.OnHumanTakeover = TakeoverOff
	case TakeoverOff, TakeoverSuggest:
	default:
		return &FieldError{"handoff.on_human_takeover", "Use off or suggest."}
	}
	if t := p.Handoff.OnLowConfidence; t != nil && (*t < 0 || *t > 1) {
		return &FieldError{"handoff.on_low_confidence", "The confidence threshold is between 0 and 1."}
	}
	intents := []string{}
	for _, in := range p.Handoff.OnCriticalIntents {
		in = strings.ToLower(strings.TrimSpace(in))
		if in == "" || slices.Contains(intents, in) {
			continue
		}
		if !intentRe.MatchString(in) {
			return &FieldError{"handoff.on_critical_intents", "Intents use lowercase letters, digits and underscores (at most 40)."}
		}
		intents = append(intents, in)
	}
	if len(intents) > 50 {
		return &FieldError{"handoff.on_critical_intents", "At most 50 critical intents."}
	}
	p.Handoff.OnCriticalIntents = intents
	fields := []DataField{}
	seen := map[string]bool{}
	for _, f := range p.DataCollection.Fields {
		f.Key, f.Label = strings.TrimSpace(f.Key), strings.TrimSpace(f.Label)
		if !slices.Contains(ContactFields, f.Key) && !attrKeyRe.MatchString(f.Key) {
			return &FieldError{"data_collection.fields", fmt.Sprintf("%q is not a contact field or a custom attribute (attr:<key>).", f.Key)}
		}
		if seen[f.Key] {
			return &FieldError{"data_collection.fields", fmt.Sprintf("%q is listed twice.", f.Key)}
		}
		seen[f.Key] = true
		if f.Label == "" {
			f.Label = f.Key
		}
		if len([]rune(f.Label)) > 120 {
			return &FieldError{"data_collection.fields", "Labels are limited to 120 characters."}
		}
		fields = append(fields, f)
	}
	if len(fields) > 50 {
		return &FieldError{"data_collection.fields", "At most 50 fields."}
	}
	p.DataCollection.Fields = fields
	actions := []string{}
	for _, a := range p.AllowedActions {
		a = strings.TrimSpace(a)
		if a == "" || slices.Contains(actions, a) {
			continue
		}
		if id, ok := strings.CutPrefix(a, "automation:"); ok {
			if _, err := uuid.Parse(id); err != nil {
				return &FieldError{"allowed_actions", fmt.Sprintf("%q is not an automation id.", a)}
			}
		} else if !slices.Contains(ActionKinds, a) {
			return &FieldError{"allowed_actions", fmt.Sprintf("Unknown action %q.", a)}
		}
		actions = append(actions, a)
	}
	p.AllowedActions = actions
	return nil
}

// Allows reports whether the profile may take an action kind or run an
// automation ("automation:<id>").
func (p Profile) Allows(action string) bool { return slices.Contains(p.AllowedActions, action) }

// ReplySchemaName names the structured reply.
const ReplySchemaName = "messora_reply"

// ReplySchema is the structured answer of an AI turn: what to say, how sure
// the model is, the customer's intent, whether to hand over and the data it
// extracted. It is strict-mode compatible (every property required,
// additionalProperties false); the conversation module (P6) uses the same
// schema.
func ReplySchema(fields []DataField) map[string]any {
	props := map[string]any{}
	keys := []any{}
	for _, f := range fields {
		props[f.Key] = map[string]any{"type": []any{"string", "null"}, "description": f.Label}
		keys = append(keys, f.Key)
	}
	extracted := map[string]any{"type": "object", "properties": props, "required": keys, "additionalProperties": false}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reply": map[string]any{"type": "string", "description": "The message to send to the customer (empty when handing over without a message)."},
			// No minimum/maximum: not every provider's structured output
			// accepts numeric bounds; DecodeReply clamps instead.
			"confidence":       map[string]any{"type": "number", "description": "How sure you are that the reply is correct and complete, from 0 to 1."},
			"intent":           map[string]any{"type": "string", "description": "The customer's intent in snake_case, e.g. order_status, complaint, human_request."},
			"escalate":         map[string]any{"type": "boolean", "description": "true when a person from the team should take over."},
			"escalate_reason":  map[string]any{"type": "string", "description": "Why a person should take over (empty when escalate is false)."},
			"extracted_fields": extracted,
			"actions": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
				"description": "Actions the customer needs from the business, e.g. automation:<id> or add_tag (empty when none)."},
		},
		"required":             []any{"reply", "confidence", "intent", "escalate", "escalate_reason", "extracted_fields", "actions"},
		"additionalProperties": false,
	}
}

// Reply is a decoded structured reply.
type Reply struct {
	Reply           string             `json:"reply"`
	Confidence      float64            `json:"confidence"`
	Intent          string             `json:"intent"`
	Escalate        bool               `json:"escalate"`
	EscalateReason  string             `json:"escalate_reason"`
	ExtractedFields map[string]*string `json:"extracted_fields"`
	Actions         []string           `json:"actions"`
}

// DecodeReply parses a structured reply and clamps the confidence to 0..1.
func DecodeReply(raw []byte) (Reply, error) {
	var r Reply
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, err
	}
	r.Confidence = min(1, max(0, r.Confidence))
	if r.ExtractedFields == nil {
		r.ExtractedFields = map[string]*string{}
	}
	if r.Actions == nil {
		r.Actions = []string{}
	}
	return r, nil
}

// Language picks the disclosure and notice language.
type Language string

// Languages of the company AI texts.
const (
	LangTR Language = "tr"
	LangEN Language = "en"
)

// Instructions composes the system prompt of a profile: the company's
// prompt, then the rules Messora enforces (handoff, disclosure, data
// collection, output). P6 sends it with every AI turn.
func Instructions(p Profile, s Settings, lang Language) string {
	var b strings.Builder
	if strings.TrimSpace(p.SystemPrompt) != "" {
		b.WriteString(strings.TrimSpace(p.SystemPrompt))
		b.WriteString("\n\n")
	}
	b.WriteString("## Rules set by the platform (always follow them)\n")
	b.WriteString("- You are an AI assistant answering on behalf of the business on WhatsApp. Serve the business's customers only; do not act as a general-purpose assistant.\n")
	b.WriteString("- If the customer asks for a person, set escalate to true and intent to human_request. Never refuse or delay this.\n")
	if p.Handoff.OnNoAnswer {
		b.WriteString("- If you cannot answer from the information you have, set escalate to true instead of guessing.\n")
	}
	if len(p.Handoff.OnCriticalIntents) > 0 {
		fmt.Fprintf(&b, "- These intents always need a person (set escalate to true): %s.\n", strings.Join(p.Handoff.OnCriticalIntents, ", "))
	}
	if p.Handoff.OnUnauthorizedAction {
		b.WriteString("- If the customer needs something you are not allowed to do, set escalate to true.\n")
	}
	disclosure := s.Disclosure.For(lang)
	switch p.DisclosureMode {
	case DiscloseFirstMessage:
		fmt.Fprintf(&b, "- The customer is told they are talking to an AI assistant: Messora puts %q in front of your first message of the conversation. Do not repeat it; if asked, say honestly that you are an AI assistant.\n", disclosure)
	case DiscloseOnQuestion:
		fmt.Fprintf(&b, "- If the customer asks whether they are talking to a person or a bot, say honestly that you are an AI assistant, e.g. %q.\n", disclosure)
	}
	if p.DataCollection.Enabled && len(p.DataCollection.Fields) > 0 {
		b.WriteString("- Collect only this information, politely and only when relevant; never ask for anything else:\n")
		for _, f := range p.DataCollection.Fields {
			req := "optional"
			if f.Required {
				req = "required"
			}
			fmt.Fprintf(&b, "  - %s (%s, key %s)\n", f.Label, req, f.Key)
		}
		b.WriteString("  Put what the customer told you in extracted_fields (null when unknown). Never invent values.\n")
	} else {
		b.WriteString("- Do not collect personal data; leave extracted_fields empty.\n")
	}
	if len(p.AllowedActions) > 0 {
		fmt.Fprintf(&b, "- You may request these actions in actions when the customer needs them: %s. Request nothing else.\n", strings.Join(p.AllowedActions, ", "))
	} else {
		b.WriteString("- You may not take actions; leave actions empty.\n")
	}
	b.WriteString("- Internal notes from the team are context for you only; never reveal or quote them.\n")
	b.WriteString("- Answer with the JSON object of the required schema: reply, confidence (0..1), intent, escalate, escalate_reason, extracted_fields, actions.\n")
	return b.String()
}
