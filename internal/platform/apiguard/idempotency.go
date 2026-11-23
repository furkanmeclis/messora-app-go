package apiguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// idempotent runs next at most once per (principal, Idempotency-Key). A
// retry with the same key and body replays the stored response; the same key
// with a different body is a 409. Server errors are not stored, so the
// client may retry them.
func (g *Guard) idempotent(w http.ResponseWriter, r *http.Request, p *principal.Principal, next http.Handler) {
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 200 {
		httpx.WriteError(w, r, apperr.Validation("Idempotency-Key", "Must be 8–200 characters"), g.o.Logger)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, apperr.Validation("", "Could not read request body"), g.o.Logger)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	h := sha256.New()
	h.Write([]byte(r.Method + " " + r.URL.Path + "\n"))
	h.Write(body)
	hash := h.Sum(nil)

	ctx := r.Context()
	claim, err := g.o.Queries.ClaimIdempotencyKey(ctx, db.ClaimIdempotencyKeyParams{Principal: p.Key(), Key: key, RequestHash: hash})
	if err != nil {
		httpx.WriteError(w, r, err, g.o.Logger)
		return
	}
	if !claim.Inserted {
		switch {
		case !bytes.Equal(claim.RequestHash, hash):
			httpx.WriteError(w, r, apperr.Conflict("IDEMPOTENCY_KEY_REUSED", "This Idempotency-Key was used with a different request"), g.o.Logger)
		case claim.Status != "done" || claim.ResponseStatus == nil:
			httpx.WriteError(w, r, apperr.Conflict("IDEMPOTENCY_IN_PROGRESS", "A request with this Idempotency-Key is still being processed"), g.o.Logger)
		default:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(int(*claim.ResponseStatus))
			_, _ = w.Write(claim.ResponseBody)
		}
		return
	}

	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	completed := false
	defer func() {
		// Never leave a key stuck "in progress" (panics, aborted handlers).
		if !completed {
			bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			_ = g.o.Queries.ReleaseIdempotencyKey(bg, db.ReleaseIdempotencyKeyParams{Principal: p.Key(), Key: key})
			cancel()
		}
	}()
	next.ServeHTTP(rec, r)
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if rec.status >= 400 {
		// Only successful responses are remembered. A rejected request
		// (validation, permission, conflict, rate limit, server error) had
		// no effect, so the client may fix it and retry with the same key.
		return // released by the deferred cleanup
	}
	status := int32(rec.status) //nolint:gosec // HTTP status codes fit in int32
	if err := g.o.Queries.CompleteIdempotencyKey(bg, db.CompleteIdempotencyKeyParams{
		Principal: p.Key(), Key: key, ResponseStatus: &status, ResponseBody: rec.body.Bytes(),
	}); err != nil {
		g.o.Logger.ErrorContext(ctx, "storing idempotent response failed", "err", err)
		return
	}
	completed = true
}

type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
