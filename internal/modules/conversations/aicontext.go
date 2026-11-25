package conversations

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
)

// TurnInput is what one AI turn of a conversation sees.
type TurnInput struct {
	Profile  ai.Profile
	Settings ai.Settings
	Lang     ai.Language
	// Messages of the thread, oldest first.
	Messages []db.Message
	// Notes for the AI, oldest first. They become operator instructions.
	Notes   []db.ConversationNote
	Contact db.Contact
}

// BuildTurn composes the model request of an AI turn:
//   - the profile's instructions and the platform rules (ai.Instructions),
//     then what the business knows about the customer;
//   - the thread: customer messages as user content, the business's
//     messages (AI or person) as assistant content;
//   - notes for the AI as operator notes (ai.RoleOperatorNote), placed in
//     time among the messages. Notes are never user content: the
//     providers send them as system/developer instructions.
func BuildTurn(in TurnInput) ai.Request {
	var b strings.Builder
	b.WriteString(ai.Instructions(in.Profile, in.Settings, in.Lang))
	b.WriteString("\n## The customer (from the business's records; use it to help, never read it out)\n")
	c := in.Contact
	field := func(k, v string) {
		if v = strings.TrimSpace(v); v != "" {
			fmt.Fprintf(&b, "- %s: %s\n", k, v)
		}
	}
	field("name", c.Name)
	field("whatsapp_name", c.ProfileName)
	field("phone", c.PhoneE164)
	field("email", deref(c.Email))
	for i, v := range []*string{c.Var1, c.Var2, c.Var3, c.Var4, c.Var5} {
		field(fmt.Sprintf("var%d", i+1), deref(v))
	}
	var attrs map[string]any
	if len(c.Attributes) > 0 && json.Unmarshal(c.Attributes, &attrs) == nil {
		keys := make([]string, 0, len(attrs))
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			field("attr:"+k, fmt.Sprint(attrs[k]))
		}
	}

	msgs := make([]ai.Message, 0, len(in.Messages)+len(in.Notes))
	ni := 0
	addNotesBefore := func(m *db.Message) {
		for ni < len(in.Notes) && (m == nil || !in.Notes[ni].CreatedAt.After(messageTime(*m))) {
			msgs = append(msgs, ai.Message{Role: ai.RoleOperatorNote, Text: in.Notes[ni].Body})
			ni++
		}
	}
	for i := range in.Messages {
		m := in.Messages[i]
		addNotesBefore(&m)
		text := messageText(m)
		if text == "" {
			continue
		}
		role := ai.RoleAssistant
		if m.Direction == "inbound" {
			role = ai.RoleUser
		}
		msgs = append(msgs, ai.Message{Role: role, Text: text})
	}
	addNotesBefore(nil)

	var fields []ai.DataField
	if in.Profile.DataCollection.Enabled {
		fields = in.Profile.DataCollection.Fields
	}
	return ai.Request{System: b.String(), Messages: msgs, MaxTokens: 2048,
		Output: &ai.OutputSchema{Name: ai.ReplySchemaName, Schema: ai.ReplySchema(fields)}}
}

// messageTime is when a message happened in the thread.
func messageTime(m db.Message) time.Time {
	if m.Direction == "inbound" {
		return m.OccurredAt
	}
	return m.CreatedAt
}

// messageText is what the model reads of a message: its text, or a short
// description of media and other kinds.
func messageText(m db.Message) string {
	body := strings.TrimSpace(m.Body)
	switch m.Type {
	case "text", "interactive", "button":
		return body
	case "reaction":
		return ""
	}
	if strings.HasPrefix(body, "[template] ") {
		return body
	}
	if body != "" {
		return "[" + m.Type + "] " + body
	}
	return "[" + m.Type + "]"
}

// summarySchema is the structured answer of the handoff summary request.
var summarySchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary":               map[string]any{"type": "string", "description": "What happened in the conversation, in two or three sentences."},
		"detected_need":         map[string]any{"type": "string", "description": "What the customer needs, in one sentence."},
		"suggested_next_action": map[string]any{"type": "string", "description": "What the agent should do next, in one sentence."},
	},
	"required":             []any{"summary", "detected_need", "suggested_next_action"},
	"additionalProperties": false,
}

// Summary is the handoff summary for the agent.
type Summary struct {
	Text       string `json:"summary"`
	Need       string `json:"detected_need"`
	NextAction string `json:"suggested_next_action"`
	Source     string `json:"-"` // ai | fallback
}

// summaryRequest asks the model for a handoff summary of the same context
// the turn saw, in the workspace language.
func summaryRequest(turn ai.Request, reason Reason, lang ai.Language) ai.Request {
	language := "English"
	if lang == ai.LangTR {
		language = "Turkish"
	}
	sys := "You hand a WhatsApp customer conversation over to a person on the business's team. Summarise it for them in " + language +
		". Reason for the handover: " + string(reason) + ". Internal notes are context; do not quote them. Answer with the JSON object of the schema."
	return ai.Request{System: sys, Messages: turn.Messages, MaxTokens: 600,
		Output: &ai.OutputSchema{Name: "messora_handoff_summary", Schema: summarySchema}}
}

// FallbackSummary is the deterministic summary used when the AI cannot
// write one: the customer's last messages, the detected intent and the
// next action the reason suggests.
func FallbackSummary(msgs []db.Message, reason Reason, intent string, lang ai.Language) Summary {
	var last []string
	for i := len(msgs) - 1; i >= 0 && len(last) < 3; i-- {
		if msgs[i].Direction == "inbound" {
			if t := messageText(msgs[i]); t != "" {
				last = append([]string{"“" + preview(t) + "”"}, last...)
			}
		}
	}
	tr := lang == ai.LangTR
	s := Summary{Source: "fallback", Need: strings.ReplaceAll(intent, "_", " ")}
	switch {
	case len(last) == 0 && tr:
		s.Text = "Müşteri mesajı yok."
	case len(last) == 0:
		s.Text = "No customer message."
	case tr:
		s.Text = "Müşterinin son mesajları: " + strings.Join(last, " · ")
	default:
		s.Text = "The customer's last messages: " + strings.Join(last, " · ")
	}
	next := map[Reason][2]string{
		ReasonCustomerRequest:    {"Müşteri bir temsilciyle görüşmek istiyor; sohbeti devralıp yanıtlayın.", "The customer asked for a person; take over and answer."},
		ReasonWindowClosed:       {"24 saatlik pencere kapalı; onaylı bir şablonla yazın.", "The 24-hour window is closed; write with an approved template."},
		ReasonAIError:            {"AI yanıt veremedi; müşteriyi siz yanıtlayın.", "The AI could not answer; answer the customer yourself."},
		ReasonLowConfidence:      {"AI yanıtından emin değildi; yanıtı kontrol edip müşteriye dönün.", "The AI was unsure; check and answer the customer."},
		ReasonCriticalIntent:     {"Kritik bir konu; müşteriyle ilgilenin.", "A critical matter; take care of the customer."},
		ReasonUnauthorizedAction: {"Müşterinin istediği işlem AI'ın yetkisinde değil; işlemi siz yapın.", "The customer needs an action the AI may not take; do it yourself."},
		ReasonNoAnswer:           {"AI cevap bulamadı; müşteriyi yanıtlayın.", "The AI found no answer; answer the customer."},
		ReasonManual:             {"Sohbeti devralın.", "Take the conversation over."},
	}[reason]
	if tr {
		s.NextAction = next[0]
	} else {
		s.NextAction = next[1]
	}
	return s
}
