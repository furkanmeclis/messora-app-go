package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// VerifySignature checks X-Hub-Signature-256 ("sha256=<hex>") over the raw
// request body against any of the app secrets, in constant time.
func VerifySignature(body []byte, header string, appSecrets ...string) bool {
	got, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	sig, err := hex.DecodeString(got)
	if err != nil || len(sig) != sha256.Size {
		return false
	}
	for _, secret := range appSecrets {
		if secret == "" {
			continue
		}
		m := hmac.New(sha256.New, []byte(secret))
		m.Write(body)
		if hmac.Equal(m.Sum(nil), sig) {
			return true
		}
	}
	return false
}

// Sign returns the X-Hub-Signature-256 value for body (used by tests and the
// mock server).
func Sign(body []byte, appSecret string) string {
	m := hmac.New(sha256.New, []byte(appSecret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// VerifyChallenge answers Meta's subscription handshake: it returns the
// challenge when mode is "subscribe" and the token matches.
func VerifyChallenge(mode, token, challenge, expected string) (string, bool) {
	if mode != "subscribe" || expected == "" || challenge == "" {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return "", false
	}
	return challenge, true
}

// Notification is a webhook POST body.
type Notification struct {
	Object string  `json:"object"` // "whatsapp_business_account"
	Entry  []Entry `json:"entry"`
}

// Entry groups changes for one WABA.
type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

// Change is one event; Field selects how Value is interpreted.
type Change struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

// MessagesValue is the value of field "messages".
type MessagesValue struct {
	MessagingProduct string `json:"messaging_product"`
	Metadata         struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		PhoneNumberID      string `json:"phone_number_id"`
	} `json:"metadata"`
	Contacts []struct {
		WaID    string `json:"wa_id"`
		UserID  string `json:"user_id"`
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
	} `json:"contacts"`
	Messages []InboundMessage `json:"messages"`
	Statuses []Status         `json:"statuses"`
	Errors   []WebhookError   `json:"errors"`
}

// InboundMessage is a message from a customer. The raw JSON is kept so new
// types are stored even before Messora understands them.
type InboundMessage struct {
	From       string `json:"from"`
	FromUserID string `json:"from_user_id"`
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Type       string `json:"type"`
	Context    *struct {
		From string `json:"from"`
		ID   string `json:"id"`
	} `json:"context"`
	Text *struct {
		Body string `json:"body"`
	} `json:"text"`
	Image    *InboundMedia `json:"image"`
	Video    *InboundMedia `json:"video"`
	Audio    *InboundMedia `json:"audio"`
	Document *InboundMedia `json:"document"`
	Sticker  *InboundMedia `json:"sticker"`
	Location *Location     `json:"location"`
	Button   *struct {
		Payload string `json:"payload"`
		Text    string `json:"text"`
	} `json:"button"`
	Interactive *struct {
		Type        string `json:"type"`
		ButtonReply *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"button_reply"`
		ListReply *struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"list_reply"`
	} `json:"interactive"`
	Reaction *Reaction       `json:"reaction"`
	Referral json.RawMessage `json:"referral"`
	Errors   []WebhookError  `json:"errors"`
}

// InboundMedia references customer media (ID valid for 7 days).
type InboundMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
	Voice    bool   `json:"voice"`
	Animated bool   `json:"animated"`
}

// Media returns the media object for media-bearing types.
func (m *InboundMessage) Media() *InboundMedia {
	switch m.Type {
	case "image":
		return m.Image
	case "video":
		return m.Video
	case "audio":
		return m.Audio
	case "document":
		return m.Document
	case "sticker":
		return m.Sticker
	}
	return nil
}

// Body is a plain-text rendering used for search, automation keywords and
// previews.
func (m *InboundMessage) Body() string {
	switch {
	case m.Text != nil:
		return m.Text.Body
	case m.Button != nil:
		return m.Button.Text
	case m.Interactive != nil && m.Interactive.ButtonReply != nil:
		return m.Interactive.ButtonReply.Title
	case m.Interactive != nil && m.Interactive.ListReply != nil:
		return m.Interactive.ListReply.Title
	case m.Reaction != nil:
		return m.Reaction.Emoji
	case m.Location != nil:
		return strings.TrimSpace(m.Location.Name + " " + m.Location.Address)
	}
	if md := m.Media(); md != nil {
		return md.Caption
	}
	return ""
}

// Time parses the Unix timestamp string.
func (m *InboundMessage) Time() time.Time { return parseUnix(m.Timestamp) }

// Status is a delivery update for an outbound message.
type Status struct {
	ID                    string          `json:"id"` // wamid
	Status                string          `json:"status"`
	Timestamp             string          `json:"timestamp"`
	RecipientID           string          `json:"recipient_id"`
	BizOpaqueCallbackData string          `json:"biz_opaque_callback_data"`
	Errors                []WebhookError  `json:"errors"`
	Pricing               json.RawMessage `json:"pricing"`
	Conversation          json.RawMessage `json:"conversation"`
}

// Time parses the Unix timestamp string.
func (s *Status) Time() time.Time { return parseUnix(s.Timestamp) }

// WebhookError describes a failure inside a webhook.
type WebhookError struct {
	Code      int    `json:"code"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	ErrorData struct {
		Details string `json:"details"`
	} `json:"error_data"`
}

// TemplateStatusValue is the value of field "message_template_status_update".
type TemplateStatusValue struct {
	Event                   string      `json:"event"`
	MessageTemplateID       json.Number `json:"message_template_id"`
	MessageTemplateName     string      `json:"message_template_name"`
	MessageTemplateLanguage string      `json:"message_template_language"`
	Reason                  string      `json:"reason"`
}

// QualityValue is the value of field "phone_number_quality_update".
type QualityValue struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	Event              string `json:"event"`
	CurrentLimit       string `json:"current_limit"`
}

func parseUnix(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}
