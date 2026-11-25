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
	ID      string   `json:"id"`   // WABA ID
	Time    int64    `json:"time"` // set on account-level changes
	Changes []Change `json:"changes"`
}

// At is when Meta triggered the change (now when not sent).
func (e Entry) At() time.Time {
	if e.Time <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(e.Time, 0).UTC()
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
	ID                    string         `json:"id"` // wamid
	Status                string         `json:"status"`
	Timestamp             string         `json:"timestamp"`
	RecipientID           string         `json:"recipient_id"`
	BizOpaqueCallbackData string         `json:"biz_opaque_callback_data"`
	Errors                []WebhookError `json:"errors"`
	// Pricing is present on either the "sent" or the "delivered" status,
	// never both (and not on read/failed).
	Pricing      *Pricing        `json:"pricing"`
	Conversation json.RawMessage `json:"conversation"`
}

// Pricing is the status webhook's pricing object. It carries no amount.
type Pricing struct {
	// Billable is deprecated by Meta (type and category decide).
	Billable     *bool  `json:"billable"`
	PricingModel string `json:"pricing_model"` // PMP (CBP before 2025-07-01)
	// Type: regular (charged) | free_customer_service | free_entry_point |
	// free_group_customer_service.
	Type string `json:"type"`
	// Category: marketing | utility | authentication |
	// authentication-international | service | marketing_lite |
	// referral_conversion | group_marketing | group_utility | group_service.
	Category string `json:"category"`
	Subtype  string `json:"subtype"` // free_tier | paid_exempt (free service only)
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
// Events seen: ONBOARDING, THROUGHPUT_UPGRADE (the number moved to the
// higher throughput) and, in older payloads, FLAGGED/UNFLAGGED/UPGRADE/
// DOWNGRADE. current_limit/old_limit are being replaced by
// max_daily_conversations_per_business (the portfolio's messaging limit).
type QualityValue struct {
	DisplayPhoneNumber               string `json:"display_phone_number"`
	Event                            string `json:"event"`
	CurrentLimit                     string `json:"current_limit"`
	OldLimit                         string `json:"old_limit"`
	MaxDailyConversationsPerBusiness string `json:"max_daily_conversations_per_business"`
}

// MessagingLimit is the portfolio limit the event reports, preferring the
// current field.
func (v QualityValue) MessagingLimit() string {
	if v.MaxDailyConversationsPerBusiness != "" {
		return v.MaxDailyConversationsPerBusiness
	}
	if v.Event == "THROUGHPUT_UPGRADE" {
		return "" // current_limit is the throughput tier here, not the limit
	}
	return v.CurrentLimit
}

// AccountUpdateValue is the value of field "account_update". Only the
// fields Messora acts on are decoded; the raw value is stored.
type AccountUpdateValue struct {
	Event           string `json:"event"`
	PhoneNumber     string `json:"phone_number"`
	RestrictionInfo []struct {
		RestrictionType string `json:"restriction_type"`
		Expiration      int64  `json:"expiration"`
	} `json:"restriction_info"`
	BanInfo *struct {
		WABABanState string `json:"waba_ban_state"`
		WABABanDate  string `json:"waba_ban_date"`
	} `json:"ban_info"`
	ViolationInfo *struct {
		ViolationType string `json:"violation_type"`
	} `json:"violation_info"`
}

// TemplateQualityValue is the value of field "message_template_quality_update".
type TemplateQualityValue struct {
	PreviousQualityScore    string      `json:"previous_quality_score"`
	NewQualityScore         string      `json:"new_quality_score"` // GREEN | YELLOW | RED | UNKNOWN
	MessageTemplateID       json.Number `json:"message_template_id"`
	MessageTemplateName     string      `json:"message_template_name"`
	MessageTemplateLanguage string      `json:"message_template_language"`
}

// CapabilityValue is the value of field "business_capability_update".
type CapabilityValue struct {
	// MaxDailyConversationsPerBusiness is documented as a tier string
	// (TIER_2K) but Meta's own example sends a number; both are accepted.
	MaxDailyConversationsPerBusiness json.RawMessage `json:"max_daily_conversations_per_business"`
	MaxDailyConversationPerPhone     *int64          `json:"max_daily_conversation_per_phone"`
	MaxPhoneNumbersPerBusiness       *int64          `json:"max_phone_numbers_per_business"`
	MaxPhoneNumbersPerWABA           *int64          `json:"max_phone_numbers_per_waba"`
}

// MessagingLimit returns the portfolio limit as a tier name (TIER_250 …
// TIER_UNLIMITED), or "" when the event does not carry one.
func (v CapabilityValue) MessagingLimit() string {
	if len(v.MaxDailyConversationsPerBusiness) > 0 {
		var s string
		if json.Unmarshal(v.MaxDailyConversationsPerBusiness, &s) == nil && s != "" {
			return s
		}
		var n int64
		if json.Unmarshal(v.MaxDailyConversationsPerBusiness, &n) == nil {
			return LimitTier(n)
		}
	}
	if v.MaxDailyConversationPerPhone != nil {
		return LimitTier(*v.MaxDailyConversationPerPhone)
	}
	return ""
}

// LimitTier names a numeric messaging limit (-1 = unlimited).
func LimitTier(n int64) string {
	switch n {
	case -1:
		return "TIER_UNLIMITED"
	case 2000:
		return "TIER_2K"
	case 10000:
		return "TIER_10K"
	case 100000:
		return "TIER_100K"
	}
	if n <= 0 {
		return ""
	}
	return "TIER_" + strconv.FormatInt(n, 10)
}

// UserPreferencesValue is the value of field "user_preferences": a user
// stopped or resumed marketing messages from the business in WhatsApp.
type UserPreferencesValue struct {
	Metadata struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		PhoneNumberID      string `json:"phone_number_id"`
	} `json:"metadata"`
	UserPreferences []UserPreference `json:"user_preferences"`
}

// UserPreference is one preference change.
type UserPreference struct {
	WaID         string      `json:"wa_id"`   // omitted for username-only users
	UserID       string      `json:"user_id"` // business-scoped user ID, when enabled
	ParentUserID string      `json:"parent_user_id"`
	Detail       string      `json:"detail"`
	Category     string      `json:"category"` // marketing_messages
	Value        string      `json:"value"`    // stop | resume
	Timestamp    json.Number `json:"timestamp"`
}

// Time parses the Unix timestamp.
func (p UserPreference) Time() time.Time { return parseUnix(p.Timestamp.String()) }

func parseUnix(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}
