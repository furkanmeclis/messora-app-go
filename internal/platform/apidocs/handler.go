package apidocs

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
)

// Path is where the server publishes the public document (no
// authentication: it describes only what an API key can already call).
const Path = "/api/v1/openapi.json"

// Handler serves a prepared public document. Any origin may read it, so code
// generators and API clients can import it by URL.
func Handler(doc []byte) http.Handler {
	return PerRequest(func(*http.Request) ([]byte, error) { return doc, nil }, nil)
}

// PerRequest serves the document pick returns for the request: each
// workspace's address serves the document with its own API address
// (Published.For). A failing pick answers 503.
func PerRequest(pick func(*http.Request) ([]byte, error), log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		doc, err := pick(r)
		if err != nil {
			if log != nil {
				log.ErrorContext(r.Context(), "public API document", "err", err)
			}
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		sum := sha256.Sum256(doc)
		etag := `"` + hex.EncodeToString(sum[:12]) + `"`
		h := w.Header()
		h.Set("Content-Type", "application/json; charset=utf-8")
		h.Set("Cache-Control", "public, max-age=300")
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		h.Set("Content-Length", strconv.Itoa(len(doc)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(doc)
	})
}
