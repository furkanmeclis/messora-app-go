package whatsapp

import (
	"encoding/json"
	"strings"
	"time"
)

// Message is the body of POST /{phone-number-id}/messages. Exactly one of the
// type-specific fields is set, matching Type.
type Message struct {
	MessagingProduct string `json:"messaging_product"` // always "whatsapp"
	RecipientType    string `json:"recipient_type"`    // "individual"
	To               string `json:"to"`
	Type             string `json:"type"`

	Context *Context `json:"context,omitempty"`
	// BizOpaqueCallbackData is echoed in status webhooks; Messora puts its
	// own message ID here to reconcile sends whose outcome was ambiguous.
	BizOpaqueCallbackData string `json:"biz_opaque_callback_data,omitempty"`

	Text        *Text           `json:"text,omitempty"`
	Template    *Template       `json:"template,omitempty"`
	Image       *Media          `json:"image,omitempty"`
	Video       *Media          `json:"video,omitempty"`
	Audio       *Media          `json:"audio,omitempty"`
	Document    *Media          `json:"document,omitempty"`
	Sticker     *Media          `json:"sticker,omitempty"`
	Location    *Location       `json:"location,omitempty"`
	Reaction    *Reaction       `json:"reaction,omitempty"`
	Interactive json.RawMessage `json:"interactive,omitempty"`
}

// Context marks a reply to an earlier message.
type Context struct {
	MessageID string `json:"message_id"`
}

// Text message.
type Text struct {
	Body       string `json:"body"`
	PreviewURL bool   `json:"preview_url,omitempty"`
}

// Media by uploaded ID or public link.
type Media struct {
	ID       string `json:"id,omitempty"`
	Link     string `json:"link,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// Location message.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Reaction to a message (empty Emoji removes it).
type Reaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// Template message.
type Template struct {
	Name       string              `json:"name"`
	Language   TemplateLanguage    `json:"language"`
	Components []TemplateComponent `json:"components,omitempty"`
}

// TemplateLanguage selects the translation.
type TemplateLanguage struct {
	Code string `json:"code"`
}

// TemplateComponent fills header, body or button parameters.
type TemplateComponent struct {
	Type       string              `json:"type"`               // header | body | button
	SubType    string              `json:"sub_type,omitempty"` // url | quick_reply (buttons)
	Index      string              `json:"index,omitempty"`    // button index, "0"…
	Parameters []TemplateParameter `json:"parameters"`
}

// TemplateParameter is one variable value.
type TemplateParameter struct {
	Type          string `json:"type"` // text | image | video | document | payload
	ParameterName string `json:"parameter_name,omitempty"`
	Text          string `json:"text,omitempty"`
	Payload       string `json:"payload,omitempty"`
	Image         *Media `json:"image,omitempty"`
	Video         *Media `json:"video,omitempty"`
	Document      *Media `json:"document,omitempty"`
}

// SendResult is Meta's acceptance of a message (not delivery).
type SendResult struct {
	MessageID string
	WaID      string
	Status    string // accepted | held_for_quality_assessment | paused (paced templates)
}

type sendResponse struct {
	Contacts []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID            string `json:"id"`
		MessageStatus string `json:"message_status"`
	} `json:"messages"`
}

// PhoneNumber as returned by GET /{waba-id}/phone_numbers.
type PhoneNumber struct {
	ID                     string `json:"id"`
	DisplayPhoneNumber     string `json:"display_phone_number"`
	VerifiedName           string `json:"verified_name"`
	QualityRating          string `json:"quality_rating"`
	CodeVerificationStatus string `json:"code_verification_status"`
	Status                 string `json:"status"`
	MessagingLimit         string `json:"whatsapp_business_manager_messaging_limit"`
	Throughput             struct {
		Level string `json:"level"`
	} `json:"throughput"`
}

// ThroughputMPS converts the throughput level to messages per second:
// STANDARD is Meta's default 80, HIGH the upgraded 1,000. Unknown levels
// (NOT_APPLICABLE, missing) return 0.
func (p PhoneNumber) ThroughputMPS() int {
	switch strings.ToUpper(p.Throughput.Level) {
	case "STANDARD":
		return 80
	case "HIGH":
		return 1000
	}
	return 0
}

// PricingQuery selects pricing_analytics data points.
type PricingQuery struct {
	Start, End  time.Time
	Granularity string   // DAILY | HALF_HOUR | MONTHLY
	Dimensions  []string // COUNTRY, PHONE, PRICING_CATEGORY, PRICING_TYPE, TIER
	MetricTypes []string // COST, VOLUME
}

// PricingDataPoint is one pricing_analytics data point. Cost is nil when
// Meta does not report it (accounts on a partner's credit line).
type PricingDataPoint struct {
	Start           int64    `json:"start"`
	End             int64    `json:"end"`
	PhoneNumber     string   `json:"phone_number,omitempty"`
	Country         string   `json:"country,omitempty"`
	PricingType     string   `json:"pricing_type,omitempty"`
	PricingCategory string   `json:"pricing_category,omitempty"`
	Tier            string   `json:"tier,omitempty"`
	Volume          int64    `json:"volume"`
	Cost            *float64 `json:"cost,omitempty"`
}

// TemplateInfo as returned by GET /{waba-id}/message_templates.
type TemplateInfo struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Language        string          `json:"language"`
	Category        string          `json:"category"`
	Status          string          `json:"status"`
	ParameterFormat string          `json:"parameter_format"`
	QualityScore    json.RawMessage `json:"quality_score,omitempty"`
	RejectedReason  string          `json:"rejected_reason"`
	Components      json.RawMessage `json:"components"`
}

// MediaInfo as returned by GET /{media-id}.
type MediaInfo struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	FileSize int64  `json:"file_size"`
}
