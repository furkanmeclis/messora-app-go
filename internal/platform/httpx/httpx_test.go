package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRequestIDGeneratedOrPropagated(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if len(seen) != 36 || rec.Header().Get(RequestIDHeader) != seen {
		t.Fatalf("generated id %q / header %q", seen, rec.Header().Get(RequestIDHeader))
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "bff-req-12345")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "bff-req-12345" {
		t.Fatalf("propagated id = %q", seen)
	}

	req.Header.Set(RequestIDHeader, "bad id with spaces\n")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if strings.Contains(seen, " ") {
		t.Fatal("invalid inbound id must be replaced")
	}
}

func TestBFFSecret(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef" // gitleaks:allow (test fixture)
	var leaked string
	inner := RequireBFF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get(BFFHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
	h := RequestID(IdentifyBFF(secret)(inner))

	for name, tc := range map[string]struct {
		header string
		want   int
	}{
		"missing": {"", http.StatusForbidden},
		"wrong":   {"nope", http.StatusForbidden},
		"prefix":  {secret[:31], http.StatusForbidden},
		"valid":   {secret, http.StatusNoContent},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				req.Header.Set(BFFHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
	if leaked != "" {
		t.Fatal("BFF secret must be stripped before handlers")
	}

	// An empty configured secret must never authenticate an empty header.
	open := RequestID(IdentifyBFF("")(inner))
	rec := httptest.NewRecorder()
	open.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatal("empty secret must not authorize")
	}
}

func TestEnvelopes(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			JSON(w, r, http.StatusOK, map[string]int{"n": 1})
			return
		}
		Error(w, r, http.StatusUnprocessableEntity, CodeValidation, "Invalid request", ErrorDetail{Field: "phone", Code: "invalid_e164"})
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/ok", nil))
	var ok Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil || ok.Meta.RequestID == "" {
		t.Fatalf("success envelope: %s", rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/err", nil))
	var e ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 422 || e.Error.Code != CodeValidation || e.Error.Details[0].Field != "phone" || e.Meta.RequestID == "" {
		t.Fatalf("error envelope: %s", rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("API responses must not be cached")
	}
}

func TestRecoverAndAccessLogUseRouteTemplate(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	reg := prometheus.NewRegistry()
	m := NewHTTPMetrics(reg)

	r := chi.NewRouter()
	r.Use(RequestID, AccessLog(log, m), Recover(log))
	r.Get("/contacts/{id}", func(http.ResponseWriter, *http.Request) { panic("boom") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/contacts/secret-id-123", nil))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), CodeInternal) {
		t.Fatalf("recover: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(logs.String(), "secret-id-123") {
		t.Fatal("raw path leaked into logs")
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("GET", "/contacts/{id}", "500")); got != 1 {
		t.Fatalf("metric by route template = %v", got)
	}
}
