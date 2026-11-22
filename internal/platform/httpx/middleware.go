package httpx

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furkanmeclis/messora-app-go/internal/platform/logger"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	fromBFFKey
)

// RequestIDHeader carries the request ID end to end (BFF → API → jobs).
const RequestIDHeader = "X-Request-ID"

// BFFHeader carries the shared secret proving a request came from the
// Next.js BFF on the internal network.
const BFFHeader = "X-Messora-BFF"

// RequestIDFrom returns the request ID stored in ctx.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// FromBFF reports whether the request carried a valid BFF secret.
func FromBFF(ctx context.Context) bool {
	ok, _ := ctx.Value(fromBFFKey).(bool)
	return ok
}

// RequestID accepts a well-formed inbound X-Request-ID or generates a UUIDv7,
// echoes it on the response and adds it to the logging context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			if u, err := uuid.NewV7(); err == nil {
				id = u.String()
			} else {
				id = uuid.NewString()
			}
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		ctx = logger.WithAttrs(ctx, slog.String("request_id", id))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func validRequestID(s string) bool {
	if len(s) < 8 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// IdentifyBFF marks requests that present the correct BFF secret. It never
// rejects; RequireBFF does.
func IdentifyBFF(secret string) func(http.Handler) http.Handler {
	want := []byte(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get(BFFHeader))
			ok := len(want) > 0 && subtle.ConstantTimeCompare(got, want) == 1
			// Strip the secret so nothing downstream can log or forward it.
			r.Header.Del(BFFHeader)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), fromBFFKey, ok)))
		})
	}
}

// RequireBFF rejects requests that did not come through the BFF.
func RequireBFF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !FromBFF(r.Context()) {
			Error(w, r, http.StatusForbidden, CodeForbidden, "This endpoint is only available to the Messora panel")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures status and size.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Recover turns panics into a 500 envelope and logs the stack.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(p)
					}
					log.ErrorContext(r.Context(), "panic serving request", "panic", p, "stack", string(debug.Stack()))
					Error(w, r, http.StatusInternalServerError, CodeInternal, "Internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// HTTPMetrics holds request instrumentation.
type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewHTTPMetrics registers HTTP metrics.
func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messora_http_requests_total", Help: "HTTP requests by method, route template and status.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "messora_http_request_duration_seconds", Help: "HTTP request latency by route template.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
	reg.MustRegister(m.requests, m.duration)
	return m
}

// AccessLog logs and measures every request using the chi route template
// (never the raw path, which may contain IDs or tokens).
func AccessLog(log *slog.Logger, m *HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			elapsed := time.Since(start)
			if m != nil {
				m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
				m.duration.WithLabelValues(r.Method, route).Observe(elapsed.Seconds())
			}
			if route == "/healthz" || route == "/readyz" {
				return
			}
			level := slog.LevelInfo
			if rec.status >= 500 {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "http request",
				"method", r.Method, "route", route, "status", rec.status,
				"duration_ms", elapsed.Milliseconds(), "bytes", rec.bytes)
		})
	}
}

// MaxBody limits request bodies.
func MaxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}
