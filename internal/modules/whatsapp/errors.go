package whatsapp

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// APIError is an error response from the Graph API.
type APIError struct {
	HTTPStatus int
	Code       int    `json:"code"`
	Subcode    int    `json:"error_subcode"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	FBTraceID  string `json:"fbtrace_id"`
	Data       struct {
		Details string `json:"details"`
	} `json:"error_data"`
}

func (e *APIError) Error() string {
	d := e.Data.Details
	if d == "" {
		d = e.Message
	}
	return fmt.Sprintf("meta api error %d (http %d): %s", e.Code, e.HTTPStatus, d)
}

// Outcome classifies the result of a send attempt. It decides whether a
// message may be retried without risking a duplicate.
type Outcome int

// Outcomes.
const (
	// Accepted: Meta returned a message ID.
	Accepted Outcome = iota
	// Retryable: the message was certainly not sent (the request never
	// reached Meta, or Meta rejected it for rate limiting / a transient
	// condition). Safe to retry after RetryAfter.
	Retryable
	// Permanent: Meta rejected the message; retrying will not help.
	Permanent
	// Ambiguous: the request was sent but no definite answer arrived
	// (timeout, connection reset, unparseable 5xx). The message MAY have
	// been delivered; it must not be retried automatically.
	Ambiguous
)

func (o Outcome) String() string {
	return [...]string{"accepted", "retryable", "permanent", "ambiguous"}[o]
}

// SendError wraps a failed send with its classification.
type SendError struct {
	Outcome    Outcome
	RetryAfter time.Duration
	// AccountProblem means the credentials or account are unusable (token
	// expired, permission removed, account restricted); the channel should
	// be flagged and sending paused.
	AccountProblem bool
	API            *APIError // nil for transport errors
	Err            error
}

func (e *SendError) Error() string {
	if e.API != nil {
		return fmt.Sprintf("%s: %s", e.Outcome, e.API.Error())
	}
	return fmt.Sprintf("%s: %v", e.Outcome, e.Err)
}

func (e *SendError) Unwrap() error { return e.Err }

// Code returns the Meta error code ("" for transport errors).
func (e *SendError) Code() string {
	if e.API == nil {
		return ""
	}
	return fmt.Sprint(e.API.Code)
}

// AsSendError extracts a *SendError.
func AsSendError(err error) (*SendError, bool) {
	var se *SendError
	ok := errors.As(err, &se)
	return se, ok
}

// Error codes with special handling (Meta error code reference).
const (
	codeAuth             = 0
	codeTemporary        = 2
	codeAppRateLimit     = 4
	codePermission       = 10
	codeTokenExpired     = 190
	codeAccountRestrict  = 368
	codeThroughput       = 130429
	codeUnknown          = 131000
	codeServiceUnavail   = 131016
	codeRegistration     = 131045
	codeOutsideWindow    = 131047
	codeMarketingHeld    = 131049
	codePairRate         = 131056
	codeServerUnavail    = 133004
	codeWABARateLimit    = 80007
	codeRestrictedNumber = 131031
)

// Classify maps a Graph API error to an outcome. Meta returned a
// definite error, so the message was not sent: nothing here is ambiguous.
func Classify(e *APIError, attempt int) *SendError {
	se := &SendError{Outcome: Permanent, API: e, Err: e}
	switch {
	case e.Code == codeThroughput, e.Code == codeAppRateLimit, e.Code == codeWABARateLimit:
		se.Outcome, se.RetryAfter = Retryable, backoff(attempt, 2*time.Second, 2*time.Minute)
	case e.Code == codePairRate:
		// Meta: "retry after 4^X seconds".
		se.Outcome, se.RetryAfter = Retryable, time.Duration(math.Pow(4, float64(min(attempt, 5))))*time.Second
	case e.Code == codeTemporary, e.Code == codeUnknown, e.Code == codeServiceUnavail, e.Code == codeServerUnavail:
		se.Outcome, se.RetryAfter = Retryable, backoff(attempt, 5*time.Second, 10*time.Minute)
	case e.Code == codeMarketingHeld:
		// Withheld for ecosystem health; Meta allows a later retry.
		se.Outcome, se.RetryAfter = Retryable, 6*time.Hour
	case e.Code == codeAuth, e.Code == codeTokenExpired, e.Code == codePermission,
		e.Code >= 200 && e.Code <= 299, e.Code == codeAccountRestrict, e.Code == codeRestrictedNumber,
		e.Code == codeRegistration:
		se.AccountProblem = true
	case e.HTTPStatus == 429:
		se.Outcome, se.RetryAfter = Retryable, backoff(attempt, 2*time.Second, 2*time.Minute)
	}
	return se
}

// IsOutsideWindow reports whether err is Meta's "24-hour window closed".
func IsOutsideWindow(err error) bool {
	se, ok := AsSendError(err)
	return ok && se.API != nil && se.API.Code == codeOutsideWindow
}

func backoff(attempt int, base, limit time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt && d < limit; i++ {
		d *= 2
	}
	return min(d, limit)
}

// AccountProblemReason explains, for the panel, why Meta refused an
// account's credentials and what fixes it.
func AccountProblemReason(e *APIError) string {
	const fix = " Replace the access token under Channels (a System User token never expires)."
	switch {
	case e.Code == codeTokenExpired && e.Subcode == 463:
		return "The access token has expired." + fix
	case e.Code == codeTokenExpired && (e.Subcode == 460 || e.Subcode == 467 || e.Subcode == 458):
		return "The access token was invalidated (password change, logout, or the app's access was removed)." + fix
	case e.Code == codeTokenExpired:
		return "Meta rejected the access token." + fix
	case e.Code == codePermission || (e.Code >= 200 && e.Code <= 299):
		return "The access token lacks a permission (whatsapp_business_messaging and whatsapp_business_management are required, with access to this WhatsApp Business Account)."
	case e.Code == codeAccountRestrict, e.Code == codeRestrictedNumber:
		return "Meta has restricted this WhatsApp Business Account; check Business Manager."
	case e.Code == codeRegistration:
		return "The phone number is not registered for the Cloud API; register it under Channels."
	}
	return e.Error()
}
