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
)

// Step is one action as edited by users.
type Step struct {
	// ID names the step for the canvas layout and run traces; unique within
	// an automation. Optional on input (see AssignIDs).
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	// reply
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
	// assign
	AssigneeID *uuid.UUID `json:"assignee_id,omitempty"`
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
	// message_received / new_contact: only this sending number.
	PhoneNumberID *uuid.UUID `json:"phone_number_id,omitempty"`
	// tag_added
	Tag string `json:"tag,omitempty"`
	// message_status
	Status     string     `json:"status,omitempty"` // delivered | read | failed
	CampaignID *uuid.UUID `json:"campaign_id,omitempty"`
	// At most one run per contact within this many seconds (0 = no limit).
	CooldownSeconds int `json:"cooldown_seconds,omitempty"`
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

var (
	fields         = map[string]bool{"name": true, "email": true, "var1": true, "var2": true, "var3": true, "var4": true, "var5": true}
	condFields     = regexp.MustCompile(`^(message\.(body|type)|contact\.(name|email|phone|var[1-5]|tag|opted_out)|conversation\.window_open|event\.(tag|status))$`)
	condOperators  = map[string]bool{"contains": true, "not_contains": true, "equals": true, "not_equals": true, "starts_with": true, "is_set": true, "is_not_set": true, "is_true": true, "is_false": true}
	actionTypes    = map[string]bool{"reply": true, "send_template": true, "add_tag": true, "remove_tag": true, "add_to_group": true, "set_field": true, "http_request": true, "send_email": true, "delay": true, "assign": true, "close_conversation": true, "mute": true}
	triggerMatches = map[string]bool{"": true, "any": true, "contains": true, "exact": true, "starts_with": true}
)

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
	case trigger.NewContact, trigger.ConversationClosed:
	default:
		return bad("trigger_type", "Unknown trigger.")
	}
	if t.CooldownSeconds < 0 || t.CooldownSeconds > MaxMuteSecs {
		return bad("trigger.cooldown_seconds", "Cooldown is out of range.")
	}
	return nil
}

// Compile validates steps and flattens them into a program.
func Compile(steps []Step) ([]Instr, error) {
	if len(steps) == 0 {
		return nil, bad("steps", "Add at least one step.")
	}
	if err := ValidateIDs(steps); err != nil {
		return nil, err
	}
	c := &compiler{}
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
		if err := validateAction(&s, p); err != nil {
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

func validateAction(s *Step, p string) error {
	if !actionTypes[s.Type] {
		return bad(p+".type", fmt.Sprintf("Unknown step %q.", s.Type))
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
	}
	return nil
}

// MatchTrigger decides whether an event starts an automation.
func MatchTrigger(t Trigger, e trigger.Event) bool {
	if t.PhoneNumberID != nil && (e.PhoneNumberID == nil || *e.PhoneNumberID != *t.PhoneNumberID) {
		return false
	}
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
		return t.Status == e.Status && (t.CampaignID == nil || (e.CampaignID != nil && *e.CampaignID == *t.CampaignID))
	}
	return true
}
