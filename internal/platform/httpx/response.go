// Package httpx holds HTTP plumbing shared by every module: the response
// envelope, error format and middleware.
package httpx

import (
	"encoding/json"
	"net/http"
)

// Meta is attached to every response.
type Meta struct {
	RequestID  string `json:"request_id"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Envelope is the success body: {"data": …, "meta": {…}}.
type Envelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

// ErrorDetail describes one invalid field or sub-problem.
type ErrorDetail struct {
	Field string `json:"field,omitempty"`
	Code  string `json:"code"`
	Msg   string `json:"message,omitempty"`
}

// ErrorBody is the error payload.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details"`
}

// ErrorEnvelope is the error body: {"error": {…}, "meta": {…}}.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

// Standard error codes.
const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeNotFound         = "NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeConflict         = "CONFLICT"
	CodeRateLimited      = "RATE_LIMITED"
	CodeInternal         = "INTERNAL_ERROR"
	CodeUnavailable      = "SERVICE_UNAVAILABLE"
)

// JSON writes data inside the success envelope.
func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	write(w, status, Envelope{Data: data, Meta: Meta{RequestID: RequestIDFrom(r.Context())}})
}

// Error writes the error envelope. message must be safe to show to clients.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...ErrorDetail) {
	if details == nil {
		details = []ErrorDetail{}
	}
	write(w, status, ErrorEnvelope{
		Error: ErrorBody{Code: code, Message: message, Details: details},
		Meta:  Meta{RequestID: RequestIDFrom(r.Context())},
	})
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// NotFound is the router's 404 handler.
func NotFound(w http.ResponseWriter, r *http.Request) {
	Error(w, r, http.StatusNotFound, CodeNotFound, "Resource not found")
}

// MethodNotAllowed is the router's 405 handler.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	Error(w, r, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "Method not allowed")
}
