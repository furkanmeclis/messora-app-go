// Package apperr defines errors that carry their HTTP meaning, so services
// stay free of HTTP code while handlers can render precise responses.
package apperr

import (
	"errors"
	"net/http"
	"time"
)

// Error is a client-facing error. Message must be safe to show to users.
type Error struct {
	Status     int
	Code       string
	Message    string
	Field      string        // optional: the request field at fault
	RetryAfter time.Duration // optional: for 429 responses
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Constructors for the common cases.

// Validation is a 422 for an invalid field.
func Validation(field, message string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: message, Field: field}
}

// Unauthorized is a 401.
func Unauthorized(code, message string) *Error {
	return &Error{Status: http.StatusUnauthorized, Code: code, Message: message}
}

// Forbidden is a 403.
func Forbidden(code, message string) *Error {
	return &Error{Status: http.StatusForbidden, Code: code, Message: message}
}

// NotFound is a 404.
func NotFound(message string) *Error {
	return &Error{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: message}
}

// Conflict is a 409.
func Conflict(code, message string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Message: message}
}

// RateLimited is a 429.
func RateLimited(code, message string, retryAfter time.Duration) *Error {
	return &Error{Status: http.StatusTooManyRequests, Code: code, Message: message, RetryAfter: retryAfter}
}

// Unavailable is a 503 for a disabled or unreachable dependency.
func Unavailable(code, message string) *Error {
	return &Error{Status: http.StatusServiceUnavailable, Code: code, Message: message}
}

// ErrReauthRequired is returned when a sensitive action needs a fresh
// password or TOTP confirmation.
var ErrReauthRequired = Forbidden("REAUTH_REQUIRED", "Confirm your password to continue")
