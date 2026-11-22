// Package logger builds the structured slog logger and enforces redaction of
// sensitive attributes before anything reaches the output.
package logger

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// Options configures New.
type Options struct {
	Level   string // debug | info | warn | error
	Format  string // json | text
	Service string
	Role    string
}

// New returns a logger writing to w with redaction enabled.
func New(w io.Writer, o Options) *slog.Logger {
	h := &slog.HandlerOptions{
		Level:       ParseLevel(o.Level),
		ReplaceAttr: Redact,
	}
	var handler slog.Handler
	if o.Format == "text" {
		handler = slog.NewTextHandler(w, h)
	} else {
		handler = slog.NewJSONHandler(w, h)
	}
	return slog.New(&ctxHandler{Handler: handler}).With(
		slog.String("service", o.Service),
		slog.String("role", o.Role),
	)
}

// ParseLevel maps a level name to slog.Level; unknown names mean info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

const redacted = "[redacted]"

// sensitiveKeys are matched case-insensitively against the full key or as a
// suffix after '_' or '.', e.g. "access_token", "meta.app_secret".
var sensitiveKeys = []string{
	"password", "passwd", "secret", "token", "authorization", "cookie",
	"api_key", "apikey", "access_token", "app_secret", "hmac", "dsn",
	"database_url", "redis_url", "encryption_key", "private_key",
}

// contentKeys hold customer content or personal data; their values are dropped.
var contentKeys = []string{"body", "message_body", "text", "caption", "email"}

// phoneKeys are masked to the last four digits.
var phoneKeys = []string{"phone", "phone_e164", "to", "from", "customer_phone", "wa_id"}

// Redact is a slog ReplaceAttr function. It is exported for tests and for
// wrapping third-party loggers.
func Redact(_ []string, a slog.Attr) slog.Attr {
	k := strings.ToLower(a.Key)
	switch {
	case matches(k, sensitiveKeys), matches(k, contentKeys):
		return slog.String(a.Key, redacted)
	case matches(k, phoneKeys):
		return slog.String(a.Key, MaskPhone(a.Value.String()))
	}
	return a
}

func matches(key string, list []string) bool {
	for _, s := range list {
		if key == s || strings.HasSuffix(key, "_"+s) || strings.HasSuffix(key, "."+s) {
			return true
		}
	}
	return false
}

// MaskPhone keeps the last four digits: "+905551112233" -> "***2233".
func MaskPhone(p string) string {
	digits := make([]rune, 0, len(p))
	for _, r := range p {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) <= 4 {
		return "***"
	}
	return "***" + string(digits[len(digits)-4:])
}

type ctxKey struct{}

// WithAttrs returns a context carrying attributes (request_id, user_id,
// job_id …) that every log call made with that context will include.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	prev, _ := ctx.Value(ctxKey{}).([]slog.Attr)
	merged := make([]slog.Attr, 0, len(prev)+len(attrs))
	merged = append(merged, prev...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, ctxKey{}, merged)
}

type ctxHandler struct{ slog.Handler }

func (h *ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(ctxKey{}).([]slog.Attr); ok {
		r.AddAttrs(attrs...)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ctxHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ctxHandler) WithGroup(name string) slog.Handler {
	return &ctxHandler{Handler: h.Handler.WithGroup(name)}
}
