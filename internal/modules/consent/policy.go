// Package consent decides whether a contact may receive a message and keeps
// the contact's consent per category (marketing, service), the Meta-sourced
// reachability flags and the append-only consent history.
//
// The decision itself (Decide) is pure: it sees the contact's consent, the
// message category and the company policy, and nothing else, so every send
// path (campaigns, API, inbox, automations) applies the same rules. See
// docs/CONSENT.md.
package consent

import (
	"net/http"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Status is a contact's consent for one category.
type Status string

// Statuses.
const (
	OptedIn         Status = "opted_in"
	Unknown         Status = "unknown"
	OptOutRequested Status = "opt_out_requested"
	OptedOut        Status = "opted_out"
	Resubscribed    Status = "resubscribed"
)

// Statuses lists every status, for validation.
var Statuses = []Status{OptedIn, Unknown, OptOutRequested, OptedOut, Resubscribed}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	for _, v := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}

// Consent categories (what a contact agrees to receive).
const (
	CatMarketing = "marketing"
	CatService   = "service"
)

// Message categories (what is being sent). Utility and authentication
// templates are business-initiated but not marketing: they need service
// consent. "service" is a free-form message inside the customer service
// window.
const (
	Marketing      = "marketing"
	Utility        = "utility"
	Authentication = "authentication"
	Service        = "service"
)

// MessageCategory maps a template category from Meta (MARKETING, UTILITY,
// AUTHENTICATION) to a message category. Anything unknown is treated as
// marketing, the strictest.
func MessageCategory(templateCategory string) string {
	switch templateCategory {
	case "UTILITY", "utility":
		return Utility
	case "AUTHENTICATION", "authentication":
		return Authentication
	}
	return Marketing
}

// CategoryStatus is the consent of one category.
type CategoryStatus struct {
	Status    Status
	UpdatedAt *time.Time
}

// State is everything Decide needs to know about a contact.
type State struct {
	Marketing CategoryStatus
	Service   CategoryStatus
	// MetaMarketingStoppedAt: the customer chose "Stop" for this business's
	// marketing in WhatsApp.
	MetaMarketingStoppedAt *time.Time
	// UndeliverableAt: Meta could not deliver to this number (131026).
	UndeliverableAt *time.Time
	// LastInboundAt is the customer's last window-opening message in the
	// thread being replied to (service messages only).
	LastInboundAt *time.Time
}

// StateOf reads a contact's consent. A contact whose legacy opted_out_at is
// set counts as opted out of both categories, whatever the category columns
// say (rows written before consent per category, or by hand).
func StateOf(c db.Contact) State {
	s := State{
		Marketing:              CategoryStatus{Status: Status(c.MarketingConsent), UpdatedAt: c.MarketingConsentAt},
		Service:                CategoryStatus{Status: Status(c.ServiceConsent), UpdatedAt: c.ServiceConsentAt},
		MetaMarketingStoppedAt: c.MetaMarketingStoppedAt,
		UndeliverableAt:        c.UndeliverableAt,
	}
	if !s.Marketing.Status.Valid() {
		s.Marketing.Status = Unknown
	}
	if !s.Service.Status.Valid() {
		s.Service.Status = Unknown
	}
	if c.OptedOutAt != nil {
		for _, cs := range []*CategoryStatus{&s.Marketing, &s.Service} {
			if cs.Status != OptedOut {
				cs.Status, cs.UpdatedAt = OptedOut, c.OptedOutAt
			}
		}
	}
	return s
}

// Policy is the company's configurable part of the rules.
type Policy struct {
	// BlockUnknownMarketing refuses marketing to contacts whose marketing
	// consent is unknown (default: allow with a warning).
	BlockUnknownMarketing bool
}

// Verdict is the outcome of a decision.
type Verdict string

// Verdicts.
const (
	Allow Verdict = "allow"
	Warn  Verdict = "warn"
	Block Verdict = "block"
)

// Reasons.
const (
	ReasonOptedOut             = "opted_out"
	ReasonMetaMarketingStopped = "meta_marketing_stopped"
	ReasonUndeliverable        = "undeliverable"
	ReasonOptOutRequested      = "opt_out_requested"
	ReasonConsentUnknown       = "consent_unknown"
	// ReasonRepliedAfterOptOut: the customer opted out of service messages
	// but wrote again afterwards; replying in that conversation is allowed.
	ReasonRepliedAfterOptOut = "replied_after_opt_out"
)

// Decision is the answer for one contact and message.
type Decision struct {
	Verdict Verdict
	Reason  string
	// Hard blocks can never be overridden: not by the company policy, an
	// API call or an automation.
	Hard bool
}

// Allowed reports whether the message may be sent (allow or warn).
func (d Decision) Allowed() bool { return d.Verdict != Block }

func allow() Decision                  { return Decision{Verdict: Allow} }
func warn(reason string) Decision      { return Decision{Verdict: Warn, Reason: reason} }
func hardBlock(reason string) Decision { return Decision{Verdict: Block, Reason: reason, Hard: true} }
func softBlock(reason string) Decision { return Decision{Verdict: Block, Reason: reason} }

// Decide applies the rules:
//
//   - Undeliverable numbers get nothing (hard) until they write again.
//   - Marketing: opted out or stopped in WhatsApp → hard block; opt-out
//     requested → block (pending confirmation); unknown → warn, or block when
//     the company chose so; opted in / resubscribed → allow.
//   - Utility and authentication need service consent: opted out → hard
//     block; opt-out requested → warn.
//   - Service (a reply inside the customer's window): opted out → hard block,
//     unless the customer wrote again after opting out; opt-out requested →
//     warn. Marketing consent does not matter for replies.
func Decide(s State, category string, p Policy) Decision {
	if s.UndeliverableAt != nil {
		return hardBlock(ReasonUndeliverable)
	}
	switch category {
	case Utility, Authentication:
		switch s.Service.Status {
		case OptedOut:
			return hardBlock(ReasonOptedOut)
		case OptOutRequested:
			return warn(ReasonOptOutRequested)
		}
		return allow()
	case Service:
		switch s.Service.Status {
		case OptedOut:
			// Meta's message timestamps have whole seconds: a message in the
			// same second as the opt-out counts as after it.
			if s.LastInboundAt != nil && (s.Service.UpdatedAt == nil || !s.LastInboundAt.Before(s.Service.UpdatedAt.Truncate(time.Second))) {
				return warn(ReasonRepliedAfterOptOut)
			}
			return hardBlock(ReasonOptedOut)
		case OptOutRequested:
			return warn(ReasonOptOutRequested)
		}
		return allow()
	}
	// Marketing (and anything unrecognised).
	switch s.Marketing.Status {
	case OptedOut:
		return hardBlock(ReasonOptedOut)
	}
	if s.MetaMarketingStoppedAt != nil {
		return hardBlock(ReasonMetaMarketingStopped)
	}
	switch s.Marketing.Status {
	case OptOutRequested:
		return softBlock(ReasonOptOutRequested)
	case Unknown:
		if p.BlockUnknownMarketing {
			return softBlock(ReasonConsentUnknown)
		}
		return warn(ReasonConsentUnknown)
	}
	return allow()
}

// ErrorCode is the API error code of a blocking reason.
func ErrorCode(reason string) string {
	switch reason {
	case ReasonOptedOut:
		return "CONTACT_OPTED_OUT"
	case ReasonMetaMarketingStopped:
		return "MARKETING_STOPPED"
	case ReasonUndeliverable:
		return "CONTACT_UNDELIVERABLE"
	case ReasonOptOutRequested:
		return "OPT_OUT_REQUESTED"
	case ReasonConsentUnknown:
		return "CONSENT_UNKNOWN"
	}
	return "BLOCKED_BY_CONSENT"
}

// Explain says why a message is blocked, for API errors and the panel.
func Explain(reason string) string {
	switch reason {
	case ReasonOptedOut:
		return "This contact opted out of these messages; they cannot be sent by anyone, including the API and automations."
	case ReasonMetaMarketingStopped:
		return "The customer stopped marketing messages from this business in WhatsApp; Meta would reject them."
	case ReasonUndeliverable:
		return "Meta could not deliver to this number; nothing is sent until the customer writes again."
	case ReasonOptOutRequested:
		return "The customer asked to stop receiving messages; an agent must confirm or clear the request first."
	case ReasonConsentUnknown:
		return "The consent settings block marketing to contacts whose consent is unknown."
	}
	return "Blocked by the contact's consent."
}

// Err is the API error for a blocking decision (nil when allowed).
func (d Decision) Err() error {
	if d.Allowed() {
		return nil
	}
	return &apperr.Error{Status: http.StatusUnprocessableEntity, Code: ErrorCode(d.Reason), Message: Explain(d.Reason)}
}
