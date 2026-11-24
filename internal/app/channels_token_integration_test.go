//go:build integration

package app

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

// TestWhatsAppTokenLifecycle: tokens are checked with Meta when connected
// or rotated, a refused account is flagged with a readable reason and the
// admins are told, and a working token brings it back at once.
func TestWhatsAppTokenLifecycle(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	accounts := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: owner}).list()
	acc := accounts[0].(map[string]any)
	id := acc["id"].(string)
	tok := acc["token"].(map[string]any)
	if tok["type"] != "SYSTEM_USER" || tok["expires_at"] != nil || tok["warning"] != nil || tok["checked_at"] == nil {
		t.Fatalf("permanent token: %v", tok)
	}
	path := "/api/v1/channels/accounts/" + id
	patch := func(body map[string]any) resp {
		return h.do(call{method: "PATCH", path: path, token: owner, body: body})
	}

	// Tokens Meta would refuse are rejected before they are stored.
	bad := map[string]whatsapptest.TokenInfo{
		"EAAG-invalid-token-0123456789":  {Invalid: true, Error: "Session has expired"},
		"EAAG-no-messaging-0123456789":   {Scopes: []string{"whatsapp_business_management"}},
		"EAAG-other-business-0123456789": {Targets: map[string][]string{"whatsapp_business_management": {"999"}}},
	}
	for token, info := range bad {
		h.meta.SetTokenInfo(token, info)
		r := patch(map[string]any{"access_token": token})
		h.expect(r, 422, "VALIDATION_ERROR")
		if !strings.Contains(string(r.raw), "access_token") {
			t.Fatalf("%s: %s", token, r.raw)
		}
	}

	// An expiring user token is accepted with a warning.
	exp := time.Now().Add(40 * 24 * time.Hour).Unix()
	h.meta.SetTokenInfo("EAAG-user-token-60-days-0123", whatsapptest.TokenInfo{Type: "USER", ExpiresAt: exp})
	r := patch(map[string]any{"access_token": "EAAG-user-token-60-days-0123"})
	h.expect(r, 200, "")
	tok = r.data()["token"].(map[string]any)
	if tok["type"] != "USER" || tok["expires_at"] == nil || !strings.Contains(tok["warning"].(string), "System User") {
		t.Fatalf("expiring token: %v", tok)
	}

	// A short-lived user token is exchanged for a long-lived one when the
	// app secret is known.
	soon := time.Now().Add(2 * time.Hour).Unix()
	h.meta.SetTokenInfo("EAAG-short-lived-0123456789", whatsapptest.TokenInfo{Type: "USER", ExpiresAt: soon})
	h.meta.SetTokenInfo("EAAG-long-lived-0123456789", whatsapptest.TokenInfo{Type: "USER", ExpiresAt: exp})
	h.meta.AllowExchange("EAAG-short-lived-0123456789", "EAAG-long-lived-0123456789")
	r = patch(map[string]any{"access_token": "EAAG-short-lived-0123456789", "app_id": "1234", "app_secret": "app-secret-0123456789"})
	h.expect(r, 200, "")
	h.meta.RequireToken("EAAG-long-lived-0123456789") // only the exchanged token works now
	r = h.do(call{method: "POST", path: path + "/check", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "active" {
		t.Fatalf("exchanged token not stored: %s", r.raw)
	}

	// Meta stops accepting it: the check flags the account with a readable
	// reason, records it and e-mails the admins (once).
	h.meta.RequireToken("EAAG-replacement-0123456789")
	h.meta.SetTokenInfo("EAAG-long-lived-0123456789", whatsapptest.TokenInfo{Invalid: true, Error: "Session has expired"})
	sentBefore := len(h.mail.sent)
	r = h.do(call{method: "POST", path: path + "/check", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "error" || !strings.Contains(r.data()["last_error"].(string), "not valid") {
		t.Fatalf("flagged: %s", r.raw)
	}
	if len(h.mail.sent) != sentBefore+1 || !strings.Contains(h.mail.last().Subject, "needs attention") {
		t.Fatalf("admins not notified: %d → %d", sentBefore, len(h.mail.sent))
	}
	h.do(call{method: "POST", path: path + "/check", token: owner})
	if len(h.mail.sent) != sentBefore+1 {
		t.Fatal("notice repeated within a day")
	}
	audit := h.do(call{method: "GET", path: "/api/v1/audit-logs?action=channel.attention", token: owner})
	if !strings.Contains(string(audit.raw), "channel.attention") {
		t.Fatalf("audit: %s", audit.raw)
	}

	// Rotating the token brings the account back immediately (it used to
	// stay flagged until a manual sync).
	r = patch(map[string]any{"access_token": "EAAG-replacement-0123456789"})
	h.expect(r, 200, "")
	if r.data()["status"] != "active" || r.data()["last_error"] != nil {
		t.Fatalf("rotation did not reactivate: %s", r.raw)
	}
}
