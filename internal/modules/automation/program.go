// Package automation runs user-defined reactions to contact events.
//
// An automation has one trigger and a list of steps. Steps may branch with
// conditions; on save they are compiled into a flat program (a list of
// instructions with forward jumps), so a run's position is a single
// instruction index that survives restarts and delays. Programs only jump
// forward, so every run terminates.
//
// The panel edits the steps as a tree on a canvas: every step has a stable
// id, the canvas stores node positions by id (the automation's layout), and
// each instruction keeps the id of its step so a run records the ids it went
// through (its trace).
package automation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Limits.
const (
	MaxSteps       = 50
	MaxDepth       = 4
	MaxDelaySecs   = 30 * 24 * 3600
	MaxMuteSecs    = 90 * 24 * 3600
	maxTextLen     = 4096
	maxKeywords    = 50
	maxInstruction = 200
	maxPromptLen   = 4000
	maxChoices     = 20
	// MaxRetries and MaxBackoffSecs bound a step's retry policy.
	MaxRetries     = 10
	MaxBackoffSecs = 3600
	// Window trigger: minutes before the 24-hour window closes.
	DefaultWindowMinutes = 60
	minWindowMinutes     = 5
	maxWindowMinutes     = 23 * 60
	// No-reply trigger: minutes without an answer.
	minNoReplyMinutes = 5
	maxNoReplyMinutes = 30 * 24 * 60
)

// Step is one action as edited by users.
type Step struct {
	// ID names the step for the canvas layout and run traces; unique within
	// an automation. Optional on input (see AssignIDs).
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	// reply; add_internal_note (the note); handoff_to_human (a note for
	// the person who takes the conversation)
	Text string `json:"text,omitempty"`
	// send_template
	Template *TemplateRef `json:"template,omitempty"`
	// add_tag, remove_tag
	Tag string `json:"tag,omitempty"`
	// add_to_group
	GroupID *uuid.UUID `json:"group_id,omitempty"`
	// set_field: name, email, var1…var5
	Field string `json:"field,omitempty"`
	Value string `json:"value,omitempty"`
	// condition
	If   *Condition `json:"if,omitempty"`
	Then []Step     `json:"then,omitempty"`
	Else []Step     `json:"else,omitempty"`
	// http_request
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	// send_email
	To      string `json:"to,omitempty"`
	Subject string `json:"subject,omitempty"`
	// delay, mute
	Seconds int `json:"seconds,omitempty"`
	// assign; handoff_to_human and ai_route_to_agent (nil = the team's
	// pool)
	AssigneeID *uuid.UUID `json:"assignee_id,omitempty"`
	// add_internal_note: the AI reads the note as an instruction
	ForAI bool `json:"for_ai,omitempty"`
	// handoff_to_ai: auto_reply (default) or awaiting_approval
	Mode string `json:"mode,omitempty"`

	// AI steps. Prompt is the instruction (with placeholders) of
	// ai_generate_message and an optional extra instruction elsewhere.
	Prompt string `json:"prompt,omitempty"`
	// ai_generate_message: "variable" (default) keeps the text in the run
	// variable, "send" also sends it as a reply.
	Output string `json:"output,omitempty"`
	// Variable is the run variable the result goes to ({{var:<name>}},
	// condition field var.<name>); each AI step has a default.
	Variable string `json:"variable,omitempty"`
	// ai_detect_intent: the intents to choose from ("other" is implied).
	Intents []string `json:"intents,omitempty"`
	// ai_extract_fields: the contact fields to fill.
	Fields []ExtractField `json:"fields,omitempty"`
	// ai_classify_segment: the tags or groups to choose one from.
	Tags     []string    `json:"tags,omitempty"`
	GroupIDs []uuid.UUID `json:"group_ids,omitempty"`
	// ai_build_profile: the contact attribute the profile goes to
	// (default ai_profile).
	Attribute string `json:"attribute,omitempty"`

	// Retry repeats a failed external step (HTTP, e-mail, sends, AI).
	Retry *RetryPolicy `json:"retry,omitempty"`
}

// ExtractField is a contact field an AI step fills: name, email,
// var1…var5 or attr:<key>.
type ExtractField struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
}

// RetryPolicy repeats a failed step up to Max more times, waiting Backoff
// seconds before the first retry and doubling the wait each time (at most
// an hour).
type RetryPolicy struct {
	Max     int `json:"max"`
	Backoff int `json:"backoff"`
}

// TemplateRef names an approved template and its variables (same
// placeholders as campaigns).
type TemplateRef struct {
	Name     string          `json:"name"`
	Language string          `json:"language"`
	Body     json.RawMessage `json:"body,omitempty"`
	Header   json.RawMessage `json:"header,omitempty"`
	// Buttons, Fallback and HeaderMedia mean what they mean in campaigns:
	// URL button suffixes by button position, values for empty
	// placeholders, and the image/video/document of a media header.
	Buttons     []string               `json:"buttons,omitempty"`
	Fallback    map[string]string      `json:"fallback,omitempty"`
	HeaderMedia *campaigns.HeaderMedia `json:"header_media,omitempty"`
	// Location, coupon code, offer end, carousel cards, one-time code and
	// Flow values, as in campaigns.
	campaigns.Extras
}

// Params is the template's values in the campaign format.
func (t TemplateRef) Params() campaigns.Params {
	return campaigns.Params{Header: t.Header, Body: t.Body, Buttons: t.Buttons, Fallback: t.Fallback, HeaderMedia: t.HeaderMedia, Extras: t.Extras}
}

// Condition compares a value of the event or contact.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}

// Trigger configures when an automation starts.
type Trigger struct {
	// message_received: match the text (any = every message).
	Match    string   `json:"match,omitempty"` // any | contains | exact | starts_with
	Keywords []string `json:"keywords,omitempty"`
	// Only this sending number (every trigger with a conversation).
	PhoneNumberID *uuid.UUID `json:"phone_number_id,omitempty"`
	// tag_added
	Tag string `json:"tag,omitempty"`
	// message_status
	Status string `json:"status,omitempty"` // delivered | read | failed
	// message_status, campaign_reply, reaction: only this campaign.
	CampaignID *uuid.UUID `json:"campaign_id,omitempty"`
	// subscription_request: stop | request | resume ("" = any)
	Subscription string `json:"subscription,omitempty"`
	// window_closing: minutes before the window closes (default 60);
	// no_reply: minutes without an answer.
	Minutes int `json:"minutes,omitempty"`
	// window_closing: also when the team answered last (default: only
	// when the customer's message is unanswered).
	IncludeAnswered bool `json:"include_answered,omitempty"`
	// no_reply: who did not answer — customer (after our message) or
	// agent (after the customer's).
	NoReplyFrom string `json:"no_reply_from,omitempty"`
	// no_reply from customer: campaign messages count as "our message"
	// too (default: only conversation messages).
	IncludeCampaigns bool `json:"include_campaigns,omitempty"`
	// reaction: positive | negative ("" = both)
	Reaction string `json:"reaction,omitempty"`
	// ai_escalation: only these handoff reasons (empty = any).
	Reasons []string `json:"reasons,omitempty"`
	// At most one run per contact within this many seconds (0 = no limit).
	CooldownSeconds int `json:"cooldown_seconds,omitempty"`
}

// WindowMinutes is the effective minutes of a window_closing trigger.
func (t Trigger) WindowMinutes() int {
	if t.Minutes == 0 {
		return DefaultWindowMinutes
	}
	return t.Minutes
}

// Instr is one compiled instruction.
type Instr struct {
	Op   string `json:"op"`             // action | branch | jump
	Step *Step  `json:"step,omitempty"` // action (no nested steps) or branch (If only)
	// Target is where a branch goes when its condition is false, or where
	// a jump goes.
	Target int `json:"target,omitempty"`
	// StepID is the id of the step the instruction comes from (the
	// condition for a branch; empty for jumps). Runs record it as their
	// trace. Programs compiled before step ids existed have none.
	StepID string `json:"step_id,omitempty"`
}

// AISteps are the AI step types. Before each runs, the engine checks that AI is on and that
// the number's AI profile allows what the step does.
var AISteps = []string{"ai_generate_message", "ai_analyze_message", "ai_detect_intent", "ai_extract_fields",
	"ai_classify_segment", "ai_auto_reply", "ai_route_to_agent", "ai_build_profile"}

// IsAIStep reports whether typ is an AI step.
func IsAIStep(typ string) bool { return slices.Contains(AISteps, typ) }

// retryable lists the steps a retry policy applies to.
func retryable(typ string) bool {
	return typ == "http_request" || typ == "send_email" || typ == "reply" || typ == "send_template" || IsAIStep(typ)
}

// EscalationReasons are the handoff reasons of an AI escalation.
var EscalationReasons = []string{"customer_request", "no_answer", "low_confidence", "critical_intent", "unauthorized_action", "window_closed", "ai_error"}

var (
	fields        = map[string]bool{"name": true, "email": true, "var1": true, "var2": true, "var3": true, "var4": true, "var5": true}
	condFields    = regexp.MustCompile(`^(message\.(body|type)|contact\.(name|email|phone|var[1-5]|tag|opted_out)|conversation\.window_open|event\.(tag|status|subscription|reaction|reason)|var\.[a-z][a-z0-9_]{0,39})$`)
	condOperators = map[string]bool{"contains": true, "not_contains": true, "equals": true, "not_equals": true, "starts_with": true, "is_set": true, "is_not_set": true, "is_true": true, "is_false": true}
	actionTypes   = map[string]bool{"reply": true, "send_template": true, "add_tag": true, "remove_tag": true, "add_to_group": true, "set_field": true,
		"http_request": true, "send_email": true, "delay": true, "assign": true, "close_conversation": true, "mute": true,
		"handoff_to_ai": true, "handoff_to_human": true, "add_internal_note": true}
	triggerMatches = map[string]bool{"": true, "any": true, "contains": true, "exact": true, "starts_with": true}
	variableRe     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	intentRe       = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
	attrKeyRe      = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
)

func init() {
	for _, t := range AISteps {
		actionTypes[t] = true
	}
}

func bad(path, msg string) error { return apperr.Validation(path, msg) }

// ValidateTrigger checks a trigger configuration for its type.
func ValidateTrigger(typ string, t Trigger) error {
	switch typ {
	case trigger.MessageReceived:
		if !triggerMatches[t.Match] {
			return bad("trigger.match", "Match must be any, contains, exact or starts_with.")
		}
		if t.Match != "" && t.Match != "any" && len(t.Keywords) == 0 {
			return bad("trigger.keywords", "Give at least one keyword.")
		}
		if len(t.Keywords) > maxKeywords {
			return bad("trigger.keywords", fmt.Sprintf("At most %d keywords.", maxKeywords))
		}
		for _, k := range t.Keywords {
			if strings.TrimSpace(k) == "" || len(k) > 200 {
				return bad("trigger.keywords", "Keywords are 1–200 characters.")
			}
		}
	case trigger.TagAdded:
		if strings.TrimSpace(t.Tag) == "" {
			return bad("trigger.tag", "Choose the tag.")
		}
	case trigger.MessageStatus:
		if t.Status != "delivered" && t.Status != "read" && t.Status != "failed" {
			return bad("trigger.status", "Status must be delivered, read or failed.")
		}
	case trigger.SubscriptionRequest:
		if t.Subscription != "" && t.Subscription != "stop" && t.Subscription != "request" && t.Subscription != "resume" {
			return bad("trigger.subscription", "Subscription is stop, request or resume.")
		}
	case trigger.WindowClosing:
		if m := t.WindowMinutes(); m < minWindowMinutes || m > maxWindowMinutes {
			return bad("trigger.minutes", "The window trigger fires 5 minutes to 23 hours before the window closes.")
		}
	case trigger.NoReply:
		if t.NoReplyFrom != "customer" && t.NoReplyFrom != "agent" {
			return bad("trigger.no_reply_from", "Choose who did not reply: customer or agent.")
		}
		if t.Minutes < minNoReplyMinutes || t.Minutes > maxNoReplyMinutes {
			return bad("trigger.minutes", "The waiting time is 5 minutes to 30 days.")
		}
	case trigger.Reaction:
		if t.Reaction != "" && t.Reaction != trigger.Positive && t.Reaction != trigger.Negative {
			return bad("trigger.reaction", "Reaction is positive or negative.")
		}
	case trigger.AIEscalation:
		for _, r := range t.Reasons {
			if !slices.Contains(EscalationReasons, r) {
				return bad("trigger.reasons", fmt.Sprintf("Unknown escalation reason %q.", r))
			}
		}
	case trigger.NewContact, trigger.ConversationClosed, trigger.CampaignReply:
	default:
		return bad("trigger_type", "Unknown trigger.")
	}
	if t.CooldownSeconds < 0 || t.CooldownSeconds > MaxMuteSecs {
		return bad("trigger.cooldown_seconds", "Cooldown is out of range.")
	}
	return nil
}

// Compile validates steps and flattens them into a program.
func Compile(steps []Step) ([]Instr, error) { return CompileDraft(steps, nil) }

// CompileDraft is Compile for an automation that still waits for setup:
// the steps named in open may lack their required values (an installed
// preset whose template is not chosen yet). Such an automation cannot be
// enabled until the values are filled.
func CompileDraft(steps []Step, open map[string]bool) ([]Instr, error) {
	if len(steps) == 0 {
		return nil, bad("steps", "Add at least one step.")
	}
	if err := ValidateIDs(steps); err != nil {
		return nil, err
	}
	c := &compiler{open: open}
	if err := c.block(steps, "steps", 1); err != nil {
		return nil, err
	}
	if c.count > MaxSteps {
		return nil, bad("steps", fmt.Sprintf("At most %d steps.", MaxSteps))
	}
	if len(c.out) > maxInstruction {
		return nil, bad("steps", "The automation is too large.")
	}
	return c.out, nil
}

type compiler struct {
	out   []Instr
	count int
	open  map[string]bool
}

func (c *compiler) block(steps []Step, path string, depth int) error {
	if depth > MaxDepth {
		return bad(path, fmt.Sprintf("Conditions can be nested at most %d levels.", MaxDepth))
	}
	for i := range steps {
		s := steps[i]
		p := fmt.Sprintf("%s[%d]", path, i)
		c.count++
		if s.Type == "condition" {
			if err := validateCondition(s.If, p+".if"); err != nil {
				return err
			}
			if len(s.Then) == 0 && len(s.Else) == 0 {
				return bad(p, "A condition needs steps in then or else.")
			}
			branch := len(c.out)
			c.out = append(c.out, Instr{Op: "branch", Step: &Step{ID: s.ID, Type: "condition", If: s.If}, StepID: s.ID})
			if err := c.block(s.Then, p+".then", depth+1); err != nil {
				return err
			}
			if len(s.Else) == 0 {
				c.out[branch].Target = len(c.out)
				continue
			}
			jump := len(c.out)
			c.out = append(c.out, Instr{Op: "jump"})
			c.out[branch].Target = len(c.out)
			if err := c.block(s.Else, p+".else", depth+1); err != nil {
				return err
			}
			c.out[jump].Target = len(c.out)
			continue
		}
		if err := validateAction(&s, p); err != nil && (!c.open[s.ID] || !actionTypes[s.Type]) {
			return err
		}
		s.Then, s.Else, s.If = nil, nil, nil
		c.out = append(c.out, Instr{Op: "action", Step: &s, StepID: s.ID})
	}
	return nil
}

func validateCondition(c *Condition, p string) error {
	if c == nil {
		return bad(p, "The condition is missing.")
	}
	if !condFields.MatchString(c.Field) {
		return bad(p+".field", "Unknown field.")
	}
	if !condOperators[c.Operator] {
		return bad(p+".operator", "Unknown operator.")
	}
	if c.Field == "contact.tag" && c.Operator != "equals" && c.Operator != "not_equals" && c.Operator != "contains" && c.Operator != "not_contains" {
		return bad(p+".operator", "Tag conditions are has (equals) or has not (not_equals).")
	}
	needsValue := c.Operator != "is_set" && c.Operator != "is_not_set" && c.Operator != "is_true" && c.Operator != "is_false"
	if needsValue && c.Value == "" {
		return bad(p+".value", "The condition needs a value.")
	}
	if len(c.Value) > 500 {
		return bad(p+".value", "Value is too long.")
	}
	return nil
}

// validField reports whether key is a contact field an AI step may fill.
func validField(key string) bool {
	if a, ok := strings.CutPrefix(key, "attr:"); ok {
		return attrKeyRe.MatchString(a)
	}
	return fields[key]
}

func validateAction(s *Step, p string) error {
	if !actionTypes[s.Type] {
		return bad(p+".type", fmt.Sprintf("Unknown step %q.", s.Type))
	}
	if s.Retry != nil {
		if !retryable(s.Type) {
			return bad(p+".retry", "Only HTTP, e-mail, message and AI steps can be retried.")
		}
		if s.Retry.Max < 0 || s.Retry.Max > MaxRetries {
			return bad(p+".retry.max", fmt.Sprintf("Retries are 0 to %d.", MaxRetries))
		}
		if s.Retry.Max > 0 && (s.Retry.Backoff < 1 || s.Retry.Backoff > MaxBackoffSecs) {
			return bad(p+".retry.backoff", "The wait before a retry is 1 second to 1 hour.")
		}
	}
	if s.Variable != "" && !variableRe.MatchString(s.Variable) {
		return bad(p+".variable", "A variable name is lower-case letters, digits and _ (up to 40), starting with a letter.")
	}
	if len([]rune(s.Prompt)) > maxPromptLen {
		return bad(p+".prompt", fmt.Sprintf("The instruction is limited to %d characters.", maxPromptLen))
	}
	switch s.Type {
	case "reply":
		if strings.TrimSpace(s.Text) == "" || len([]rune(s.Text)) > maxTextLen {
			return bad(p+".text", "Reply text is required (up to 4096 characters).")
		}
	case "send_template":
		if s.Template == nil || s.Template.Name == "" || s.Template.Language == "" {
			return bad(p+".template", "Template name and language are required.")
		}
	case "add_tag", "remove_tag":
		if t := strings.TrimSpace(s.Tag); t == "" || len([]rune(t)) > 60 {
			return bad(p+".tag", "Tag is required (up to 60 characters).")
		}
	case "add_to_group":
		if s.GroupID == nil {
			return bad(p+".group_id", "Choose the group.")
		}
	case "set_field":
		if !fields[s.Field] {
			return bad(p+".field", "Field must be name, email or var1–var5.")
		}
		if len([]rune(s.Value)) > 500 {
			return bad(p+".value", "Value is limited to 500 characters.")
		}
	case "http_request":
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return bad(p+".url", "Use an https:// URL.")
		}
		switch s.Method {
		case "", "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			return bad(p+".method", "Unsupported method.")
		}
		if len(s.Headers) > 20 || len(s.Body) > 64<<10 {
			return bad(p, "Too many headers or body too large.")
		}
		for k := range s.Headers {
			if strings.EqualFold(k, "host") || strings.ContainsAny(k, "\r\n: ") {
				return bad(p+".headers", "Invalid header name.")
			}
		}
	case "send_email":
		if !strings.Contains(s.To, "@") || len(s.To) > 254 || strings.TrimSpace(s.Subject) == "" || strings.TrimSpace(s.Body) == "" {
			return bad(p, "E-mail needs a recipient, subject and body.")
		}
	case "delay":
		if s.Seconds < 1 || s.Seconds > MaxDelaySecs {
			return bad(p+".seconds", "Delay is 1 second to 30 days.")
		}
	case "mute":
		if s.Seconds < 60 || s.Seconds > MaxMuteSecs {
			return bad(p+".seconds", "Mute is 1 minute to 90 days.")
		}
	case "add_internal_note":
		if strings.TrimSpace(s.Text) == "" || len([]rune(s.Text)) > 4000 {
			return bad(p+".text", "The note is required (up to 4000 characters).")
		}
	case "handoff_to_human":
		if len([]rune(s.Text)) > 500 {
			return bad(p+".text", "The handoff note is limited to 500 characters.")
		}
	case "handoff_to_ai":
		if s.Mode != "" && s.Mode != "auto_reply" && s.Mode != "awaiting_approval" {
			return bad(p+".mode", "Mode is auto_reply or awaiting_approval.")
		}
	case "ai_generate_message":
		if strings.TrimSpace(s.Prompt) == "" {
			return bad(p+".prompt", "Write what the message should say.")
		}
		if s.Output != "" && s.Output != "variable" && s.Output != "send" {
			return bad(p+".output", "Output is variable or send.")
		}
	case "ai_detect_intent":
		if len(s.Intents) == 0 || len(s.Intents) > maxChoices {
			return bad(p+".intents", fmt.Sprintf("Give 1 to %d intents.", maxChoices))
		}
		for _, in := range s.Intents {
			if !intentRe.MatchString(in) || in == "other" {
				return bad(p+".intents", "An intent is lower-case letters, digits and _ (up to 40); \"other\" is implied.")
			}
		}
	case "ai_extract_fields":
		if len(s.Fields) == 0 || len(s.Fields) > maxChoices {
			return bad(p+".fields", fmt.Sprintf("Choose 1 to %d fields.", maxChoices))
		}
		for _, f := range s.Fields {
			if !validField(f.Key) || len([]rune(f.Label)) > 120 {
				return bad(p+".fields", "Fields are name, email, var1–var5 or attr:<key>.")
			}
		}
	case "ai_classify_segment":
		if (len(s.Tags) == 0) == (len(s.GroupIDs) == 0) {
			return bad(p+".tags", "Give the tags or the groups to choose from (not both).")
		}
		if len(s.Tags) > maxChoices || len(s.GroupIDs) > maxChoices {
			return bad(p+".tags", fmt.Sprintf("At most %d choices.", maxChoices))
		}
		for _, t := range s.Tags {
			if t = strings.TrimSpace(t); t == "" || len([]rune(t)) > 60 {
				return bad(p+".tags", "Tags are 1–60 characters.")
			}
		}
	case "ai_build_profile":
		if s.Attribute != "" && !attrKeyRe.MatchString(s.Attribute) {
			return bad(p+".attribute", "The attribute key is letters, digits, _ . - (up to 64).")
		}
	}
	return nil
}

// MatchTrigger decides whether an event starts an automation.
func MatchTrigger(t Trigger, e trigger.Event) bool {
	if t.PhoneNumberID != nil && (e.PhoneNumberID == nil || *e.PhoneNumberID != *t.PhoneNumberID) {
		return false
	}
	campaign := t.CampaignID == nil || (e.CampaignID != nil && *e.CampaignID == *t.CampaignID)
	switch e.Type {
	case trigger.MessageReceived:
		if t.Match == "" || t.Match == "any" {
			return true
		}
		body := strings.ToLower(strings.TrimSpace(e.Body))
		for _, k := range t.Keywords {
			k = strings.ToLower(strings.TrimSpace(k))
			switch t.Match {
			case "exact":
				if body == k {
					return true
				}
			case "starts_with":
				if strings.HasPrefix(body, k) {
					return true
				}
			case "contains":
				if strings.Contains(body, k) {
					return true
				}
			}
		}
		return false
	case trigger.TagAdded:
		return strings.EqualFold(t.Tag, e.Tag)
	case trigger.MessageStatus:
		return t.Status == e.Status && campaign
	case trigger.SubscriptionRequest:
		return t.Subscription == "" || t.Subscription == e.Subscription
	case trigger.CampaignReply:
		return campaign
	case trigger.Reaction:
		return (t.Reaction == "" || t.Reaction == e.Reaction) && campaign
	case trigger.AIEscalation:
		return len(t.Reasons) == 0 || slices.Contains(t.Reasons, e.Reason)
	}
	return true
}
