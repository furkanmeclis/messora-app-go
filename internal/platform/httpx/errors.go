package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// ValidationErrors collects field errors into one 422 response.
type ValidationErrors struct {
	Details []ErrorDetail
}

// Add appends a field error.
func (v *ValidationErrors) Add(field, message string) {
	v.Details = append(v.Details, ErrorDetail{Field: field, Code: "invalid", Msg: message})
}

func (v *ValidationErrors) Error() string { return "validation failed" }

// WriteError renders err. Known application errors keep their status and
// code; anything else is logged and answered with a generic 500 so internal
// details never leak.
func WriteError(w http.ResponseWriter, r *http.Request, err error, log *slog.Logger) {
	var ve *ValidationErrors
	if errors.As(err, &ve) {
		Error(w, r, http.StatusUnprocessableEntity, CodeValidation, "Invalid request", ve.Details...)
		return
	}
	if e, ok := apperr.As(err); ok {
		if e.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(e.RetryAfter.Seconds()))))
		}
		var details []ErrorDetail
		if e.Field != "" {
			details = []ErrorDetail{{Field: e.Field, Code: "invalid", Msg: e.Message}}
		}
		Error(w, r, e.Status, e.Code, e.Message, details...)
		return
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		Error(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body is too large")
		return
	}
	if log != nil {
		log.ErrorContext(r.Context(), "request failed", "err", err)
	}
	Error(w, r, http.StatusInternalServerError, CodeInternal, "Internal server error")
}

// Decode reads a JSON body into v (the schema was already validated by the
// API guard). Unknown fields are rejected as a second line of defence.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return err
		}
		return apperr.Validation("", "Request body is not valid JSON for this operation")
	}
	if dec.More() {
		return apperr.Validation("", "Request body must contain a single JSON object")
	}
	_, _ = io.Copy(io.Discard, r.Body)
	return nil
}

// NoContent writes 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }
