package ai

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrorKind classifies a failed provider call so callers know whether and
// when to retry.
type ErrorKind string

// Error kinds (ai_usage.error_kind).
const (
	// KindRateLimited: retry after RetryAfter (or with backoff when 0).
	KindRateLimited ErrorKind = "rate_limited"
	// KindQuotaExceeded: out of credit or over a spend limit; do not retry.
	KindQuotaExceeded ErrorKind = "quota_exceeded"
	// KindAuth: the key is invalid or lacks access; do not retry.
	KindAuth ErrorKind = "auth"
	// KindOverloaded: the provider is temporarily overloaded; retry with backoff.
	KindOverloaded ErrorKind = "overloaded"
	// KindBadRequest: the request was rejected; do not retry unchanged.
	KindBadRequest ErrorKind = "bad_request"
	// KindRefusal: the model declined to answer (safety); do not retry.
	KindRefusal ErrorKind = "refusal"
	// KindUnavailable: network failure or a 5xx; retry with backoff.
	KindUnavailable ErrorKind = "unavailable"
	// KindInvalidOutput: the structured output did not match the schema.
	KindInvalidOutput ErrorKind = "invalid_output"
	// KindDisabled: AI is switched off for the company.
	KindDisabled ErrorKind = "disabled"
	// KindNotConfigured: no usable provider or model.
	KindNotConfigured ErrorKind = "not_configured"
	// KindCreditsExhausted: the company's AI credits are used up (calls
	// with the platform's keys pause until the monthly reset or a top-up).
	KindCreditsExhausted ErrorKind = "credits_exhausted"
	// KindNotInPlan: the company's plan does not include AI (ai_agents).
	KindNotInPlan ErrorKind = "not_in_plan"
)

// Error is a typed provider error. Message never contains the API key.
type Error struct {
	Kind       ErrorKind
	Provider   Kind
	Status     int           // HTTP status, 0 when none
	Code       string        // provider error code, when given
	Message    string        // safe to show to admins
	RetryAfter time.Duration // for KindRateLimited, when the provider says
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("ai")
	if e.Provider != "" {
		b.WriteString(" " + string(e.Provider))
	}
	b.WriteString(": " + string(e.Kind))
	if e.Status != 0 {
		fmt.Fprintf(&b, " (%d)", e.Status)
	}
	if e.Message != "" {
		b.WriteString(": " + e.Message)
	}
	return b.String()
}

// Is matches errors of the same kind, so errors.Is(err, ErrRateLimited) works.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Provider == "" && t.Status == 0 && t.Message == "" && t.Kind == e.Kind
}

// Retryable reports whether the same request may succeed later.
func (e *Error) Retryable() bool {
	switch e.Kind {
	case KindRateLimited, KindOverloaded, KindUnavailable:
		return true
	}
	return false
}

// Sentinels for errors.Is.
var (
	ErrRateLimited   = &Error{Kind: KindRateLimited}
	ErrQuotaExceeded = &Error{Kind: KindQuotaExceeded}
	ErrAuth          = &Error{Kind: KindAuth}
	ErrOverloaded    = &Error{Kind: KindOverloaded}
	ErrBadRequest    = &Error{Kind: KindBadRequest}
	ErrRefusal       = &Error{Kind: KindRefusal}
	ErrUnavailable   = &Error{Kind: KindUnavailable}
	ErrInvalidOutput = &Error{Kind: KindInvalidOutput}
	// ErrAIDisabled is returned by every AI call while ai.enabled is off.
	ErrAIDisabled    = &Error{Kind: KindDisabled}
	ErrNotConfigured = &Error{Kind: KindNotConfigured}
	// ErrCreditsExhausted: no AI credits left for the platform's keys.
	ErrCreditsExhausted = &Error{Kind: KindCreditsExhausted}
	// ErrNotInPlan: the plan has no AI.
	ErrNotInPlan = &Error{Kind: KindNotInPlan}
)

// KindOf returns the error kind of err ("" when err is not an *Error).
func KindOf(err error) ErrorKind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return ""
}

// IsRetryable reports whether err is a typed error worth retrying; other
// errors (e.g. a context cancellation) are not.
func IsRetryable(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Retryable()
}

// RetryAfterOf returns the delay the provider asked for (0 = unknown).
func RetryAfterOf(err error) time.Duration {
	var e *Error
	if errors.As(err, &e) {
		return e.RetryAfter
	}
	return 0
}

// byStatus is the default classification of an HTTP status; providers
// refine it with their error codes.
func byStatus(status int) ErrorKind {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return KindAuth
	case status == http.StatusPaymentRequired:
		return KindQuotaExceeded
	case status == http.StatusTooManyRequests:
		return KindRateLimited
	case status == 529 || status == http.StatusServiceUnavailable:
		return KindOverloaded
	case status >= 500:
		return KindUnavailable
	case status >= 400:
		return KindBadRequest
	}
	return KindUnavailable
}

// retryAfter parses a Retry-After header (seconds or an HTTP date).
func retryAfter(h http.Header, now time.Time) time.Duration {
	if h == nil {
		return 0
	}
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if s, err := strconv.ParseFloat(v, 64); err == nil && s >= 0 {
		return time.Duration(s * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

// shorten bounds a provider message stored or shown to admins.
func shorten(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
