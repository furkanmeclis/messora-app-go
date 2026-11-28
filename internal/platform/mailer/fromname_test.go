package mailer

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// The platform sender sends a company's mail with the company's name: the
// display name changes, the address stays the sender's (Phase 17 WP8).
func TestFromNameReplacesTheDisplayNameOnly(t *testing.T) {
	addr, got := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	var p int
	_, _ = fmtSscan(port, &p)
	s, err := New(Config{Host: host, Port: p, From: "Messora <no-reply@example.com>", TLS: "none"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Send(ctx, Message{To: "ali@example.com", Subject: "Merhaba", Body: "x", FromName: "Acme Gıda", ReplyTo: "destek@acme.test"}); err != nil {
		t.Fatal(err)
	}
	data := <-got
	if !strings.Contains(data, "<no-reply@example.com>") || !strings.Contains(data, "Acme") || strings.Contains(data, `"Messora"`) {
		t.Fatalf("from:\n%s", data)
	}
	if !strings.Contains(data, "Reply-To: <destek@acme.test>") {
		t.Fatalf("reply-to:\n%s", data)
	}
	if err := s.Send(ctx, Message{To: "ali@example.com", Subject: "x", Body: "x", FromName: "Evil\r\nBcc: x@y.z"}); err == nil {
		t.Fatal("a sender name with a line break was accepted")
	}
}
