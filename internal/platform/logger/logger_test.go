package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Options{Level: "debug", Format: "json", Service: "messora", Role: "test"})

	log.Info("event",
		slog.String("access_token", "EAAG-secret"),
		slog.String("meta.app_secret", "s3cr3t"),
		slog.String("Authorization", "Bearer x"),
		slog.String("password", "pw"),
		slog.String("body", "hello customer"),
		slog.String("customer_phone", "+905551112233"),
		slog.String("campaign_id", "c-1"),
		slog.Group("meta", slog.String("token", "nested-secret")),
	)

	out := buf.String()
	for _, leak := range []string{"EAAG-secret", "s3cr3t", "Bearer x", `"pw"`, "hello customer", "905551112233", "nested-secret"} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q in %s", leak, out)
		}
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["customer_phone"] != "***2233" {
		t.Errorf("phone mask = %v", m["customer_phone"])
	}
	if m["campaign_id"] != "c-1" || m["service"] != "messora" || m["role"] != "test" {
		t.Errorf("non-sensitive fields must pass through: %v", m)
	}
}

func TestContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Options{Format: "json"})
	ctx := WithAttrs(context.Background(), slog.String("request_id", "r-1"))
	ctx = WithAttrs(ctx, slog.String("user_id", "u-1"))
	log.InfoContext(ctx, "hi")
	if !strings.Contains(buf.String(), `"request_id":"r-1"`) || !strings.Contains(buf.String(), `"user_id":"u-1"`) {
		t.Fatalf("context attrs missing: %s", buf.String())
	}
}

func TestLevelsAndMask(t *testing.T) {
	if ParseLevel("WARN") != slog.LevelWarn || ParseLevel("nope") != slog.LevelInfo {
		t.Fatal("level parsing")
	}
	if MaskPhone("12") != "***" || MaskPhone("+1 (555) 010-9999") != "***9999" {
		t.Fatal("mask")
	}
}
