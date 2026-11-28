//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// setupOwnerAs makes email the owner of the harness's tenant (with TOTP)
// and returns the session.
func (h *harness) setupOwnerAs(email string) string {
	h.t.Helper()
	token := h.ownerSession(email, "Owner "+h.host)
	h.enrolTOTP(token)
	return token
}

// rawAt sends a request to host without validating the response against
// the spec (tenant errors may answer any operation).
func (h *harness) rawAt(host, method, path, token string, bff bool) (int, string) {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, nil)
	req.Host = host
	if bff {
		req.Header.Set(httpx.BFFHeader, bffSecret)
		req.Header.Set(tenant.HostHeader, host)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(b, &body)
	return res.StatusCode, body.Error.Code
}

func (h *harness) created(token, path string, body map[string]any) string {
	h.t.Helper()
	r := h.do(call{method: "POST", path: path, token: token, body: body})
	h.expect(r, 201, "")
	if id, ok := r.data()["id"].(string); ok {
		return id
	}
	if k, ok := r.data()["api_key"].(map[string]any); ok {
		return k["id"].(string)
	}
	if e, ok := r.data()["endpoint"].(map[string]any); ok {
		return e["id"].(string)
	}
	if e, ok := r.data()["invitation"].(map[string]any); ok {
		return e["id"].(string)
	}
	h.t.Fatalf("%s: no id in %s", path, r.raw)
	return ""
}

// TestTenantIsolationMatrix is the acceptance test of docs/PHASE-17.md §6.1
// for the API: tenant A has data in every family, tenant B (its own host)
// has its own; neither sees, reads or changes the other's rows through any
// list or get endpoint, and no session or API key works at the other
// tenant's address.
func TestTenantIsolationMatrix(t *testing.T) {
	h := newHarness(t)
	h.useAIEmulator()
	if err := ai.EnsureSeed(h.ctx(), h.owner); err != nil {
		t.Fatal(err)
	}

	// ── Tenant A: data in every family ──────────────────────────────────
	ownerA, phoneA := h.connect()
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: ownerA, body: map[string]any{"organization_name": "Acme Alpha"}}), 200, "")
	h.inbound(testCustomer, "wamid.tenant-a-1", "merhaba from alpha", time.Now())
	convA := h.conversationOf(testCustomer)
	contactA := h.contactOf(ownerA, testCustomer)["id"].(string)
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + convA + "/messages", token: ownerA})
	h.expect(r, 200, "")
	msgA := r.list()[0].(map[string]any)["id"].(string)
	tagA := h.created(ownerA, "/api/v1/tags", map[string]any{"name": "VIP"})
	groupA := h.created(ownerA, "/api/v1/groups", map[string]any{"name": "Gold"})
	segA := h.created(ownerA, "/api/v1/segments", map[string]any{"name": "Alpha segment",
		"definition": map[string]any{"field": "phone", "operator": "starts_with", "value": "+90"}})
	campA := h.created(ownerA, "/api/v1/campaigns", map[string]any{"name": "Alpha campaign", "template_name": "order_update",
		"template_language": "en", "audience": map[string]any{"tag_id": tagA}})
	autoA := h.automation(ownerA, map[string]any{"name": "Alpha automation", "active": false, "trigger_type": "message_received",
		"steps": []any{map[string]any{"type": "add_tag", "tag": "alpha"}}})
	h.allowLocalReceivers()
	hookA := h.createEndpoint(ownerA, newHookReceiver(t), "message.received")["id"].(string)
	keyRes := h.do(call{method: "POST", path: "/api/v1/api-keys", token: ownerA, body: map[string]any{"name": "Alpha key", "scopes": []string{"contacts.read", "contacts.create"}}})
	h.expect(keyRes, 201, "")
	keyA, keyAID := keyRes.data()["secret"].(string), keyRes.data()["api_key"].(map[string]any)["id"].(string)
	provA := h.aiProvider(ownerA, "fake", "Alpha AI", "fake-key-for-tests")["id"].(string)
	offerA := h.createOffer(ownerA, map[string]any{"title": "Alpha offer", "coupon_code": "ALPHA1",
		"starts_at": time.Now().Add(-time.Hour), "ends_at": time.Now().Add(24 * time.Hour), "status": "scheduled"})["id"].(string)
	inviteA := h.created(ownerA, "/api/v1/invitations", map[string]any{"email": "agent@alpha.test", "role": "agent"})
	r = h.do(call{method: "GET", path: "/api/v1/templates", token: ownerA})
	h.expect(r, 200, "")
	templateA := r.list()[0].(map[string]any)["id"].(string)
	accountA := h.accountID(ownerA)
	r = h.do(call{method: "GET", path: "/api/v1/me", token: ownerA})
	userA := r.data()["user"].(map[string]any)["id"].(string)
	connA := h.metaConnection(ownerA)
	hookKeyA, verifyA := keyOf(connA), connA["verify_token"].(string)

	secretsA := map[string]string{
		"contact": contactA, "conversation": convA, "message": msgA, "tag": tagA, "group": groupA, "segment": segA,
		"campaign": campA, "automation": autoA, "webhook endpoint": hookA, "api key": keyAID, "ai provider": provA,
		"offer": offerA, "invitation": inviteA, "template": templateA, "account": accountA, "phone number": phoneA,
		"user": userA, "organisation name": "Acme Alpha", "message body": "merhaba from alpha", "invitee": "agent@alpha.test",
		"webhook key": hookKeyA, "verify token": verifyA, "waba": testWABA,
	}

	// ── Tenant B on its own host, with overlapping names ────────────────
	hb := h.tenant("beta")
	ownerB := hb.setupOwnerAs("owner@beta.test")
	// Phone numbers and names are unique per tenant, not across tenants.
	contactB := hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+" + testCustomer, "name": "Beta customer"})
	tagB := hb.created(ownerB, "/api/v1/tags", map[string]any{"name": "VIP"})
	r = hb.do(call{method: "POST", path: "/api/v1/api-keys", token: ownerB, body: map[string]any{"name": "Beta key", "scopes": []string{"contacts.read"}}})
	hb.expect(r, 201, "")
	keyB := r.data()["secret"].(string)

	leaks := func(who *harness, token, path string, secrets map[string]string) {
		t.Helper()
		r := who.do(call{method: "GET", path: path, token: token})
		if r.status != 200 {
			t.Errorf("%s %s: %d %s", who.host, path, r.status, r.raw)
			return
		}
		for what, s := range secrets {
			if strings.Contains(string(r.raw), s) {
				t.Errorf("%s GET %s shows the other tenant's %s (%s)", who.host, path, what, s)
			}
		}
	}

	// Control: the same lists do show each tenant its own rows.
	for path, id := range map[string]string{"/api/v1/contacts": contactA, "/api/v1/campaigns": campA, "/api/v1/automations": autoA,
		"/api/v1/webhook-endpoints": hookA, "/api/v1/api-keys": keyAID, "/api/v1/offers": offerA, "/api/v1/ai/providers": provA,
		"/api/v1/conversations": convA, "/api/v1/tags": tagA} {
		if r := h.do(call{method: "GET", path: path, token: ownerA}); !strings.Contains(string(r.raw), id) {
			t.Fatalf("A's own %s misses %s: %s", path, id, r.raw)
		}
	}
	if r := hb.do(call{method: "GET", path: "/api/v1/tags", token: ownerB}); !strings.Contains(string(r.raw), tagB) {
		t.Fatalf("B's tags miss its own: %s", r.raw)
	}

	// Every list (and singleton) endpoint of B shows nothing of A.
	lists := []string{
		"/api/v1/contacts", "/api/v1/contacts?q=" + testCustomer, "/api/v1/tags", "/api/v1/groups", "/api/v1/segments",
		"/api/v1/conversations", "/api/v1/campaigns", "/api/v1/campaigns/queue", "/api/v1/templates", "/api/v1/automations",
		"/api/v1/webhook-endpoints", "/api/v1/api-keys", "/api/v1/audit-logs", "/api/v1/settings", "/api/v1/settings/consent",
		"/api/v1/settings/campaign-protection", "/api/v1/appearance", "/api/v1/ai/settings",
		"/api/v1/ai/providers", "/api/v1/ai/profiles", "/api/v1/ai/usage/summary?from=" + time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339) + "&to=" + time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "/api/v1/offers", "/api/v1/users",
		"/api/v1/invitations", "/api/v1/channels/accounts", "/api/v1/channels/phone-numbers", "/api/v1/contact-imports",
		"/api/v1/ops/overview", "/api/v1/me", "/api/v1/me/sessions", "/api/v1/contacts/attribute-keys",
		"/api/v1/system/failed-jobs", "/api/v1/system/status", "/api/v1/channels/meta-connection",
		// WP2: memberships, support access, the shared catalogue.
		"/api/v1/me/memberships", "/api/v1/support-access", "/api/v1/settings/meta-rates", "/api/v1/ai/prices",
	}
	for _, p := range lists {
		leaks(hb, ownerB, p, secretsA)
	}
	// And A shows nothing of B.
	secretsB := map[string]string{"contact": contactB, "tag": tagB, "name": "Beta customer", "owner": "owner@beta.test"}
	for _, p := range lists {
		leaks(h, ownerA, p, secretsB)
	}

	// Every get endpoint refuses A's IDs at B, as if they did not exist.
	gets := []string{
		"/contacts/" + contactA, "/contacts/" + contactA + "/consent-events", "/contacts/" + contactA + "/notes",
		"/conversations/" + convA, "/conversations/" + convA + "/messages", "/conversations/" + convA + "/events",
		"/conversations/" + convA + "/activity", "/conversations/" + convA + "/notes", "/conversations/" + convA + "/handoffs",
		"/messages/" + msgA, "/tags/" + tagA + "/insights", "/groups/" + groupA + "/insights", "/groups/" + groupA + "/members",
		"/segments/" + segA, "/segments/" + segA + "/insights", "/campaigns/" + campA, "/campaigns/" + campA + "/recipients",
		"/campaigns/" + campA + "/analytics", "/campaigns/" + campA + "/events", "/automations/" + autoA,
		"/automations/" + autoA + "/runs", "/webhook-endpoints/" + hookA, "/webhook-endpoints/" + hookA + "/deliveries",
		"/offers/" + offerA, "/ai/providers/" + provA, "/templates/" + templateA, "/channels/accounts/" + accountA,
		"/users/" + userA, "/api-keys/" + keyAID,
	}
	for _, p := range gets {
		r := hb.do(call{method: "GET", path: "/api/v1" + p, token: ownerB})
		if r.status == 200 {
			for what, s := range secretsA {
				if strings.Contains(string(r.raw), s) {
					t.Errorf("B GET %s → 200 with A's %s", p, what)
				}
			}
			continue // e.g. an empty sub-list is fine as long as nothing leaks
		}
		if r.status != 404 && r.status != 405 {
			t.Errorf("B GET %s → %d %s, want 404", p, r.status, r.raw)
		}
	}
	// Changes to A's rows from B find nothing.
	for _, c := range []call{
		{method: "PATCH", path: "/api/v1/contacts/" + contactA, body: map[string]any{"name": "pwned"}},
		{method: "DELETE", path: "/api/v1/tags/" + tagA},
		{method: "DELETE", path: "/api/v1/campaigns/" + campA},
		{method: "DELETE", path: "/api/v1/automations/" + autoA},
		{method: "DELETE", path: "/api/v1/webhook-endpoints/" + hookA},
		// WP4: B cannot rotate, test, re-enable or redeliver A's endpoint.
		{method: "POST", path: "/api/v1/webhook-endpoints/" + hookA + "/rotate-secret", body: map[string]any{"grace_hours": 0}},
		{method: "POST", path: "/api/v1/webhook-endpoints/" + hookA + "/test"},
		{method: "PATCH", path: "/api/v1/webhook-endpoints/" + hookA, body: map[string]any{"active": true}},
		{method: "DELETE", path: "/api/v1/api-keys/" + keyAID},
		{method: "DELETE", path: "/api/v1/offers/" + offerA},
		{method: "DELETE", path: "/api/v1/invitations/" + inviteA},
		{method: "PATCH", path: "/api/v1/users/" + userA, body: map[string]any{"status": "disabled"}},
		{method: "POST", path: "/api/v1/conversations/" + convA + "/close"},
		{method: "POST", path: "/api/v1/contacts/" + contactA + "/notes", body: map[string]any{"body": "from beta"}},
		{method: "POST", path: "/api/v1/channels/accounts/" + accountA + "/test"},
		{method: "POST", path: "/api/v1/channels/accounts/" + accountA + "/check"},
		{method: "PATCH", path: "/api/v1/channels/accounts/" + accountA, body: map[string]any{"app_secret": "pwned-secret"}},
		// Tech Provider (WP6; its own two-tenant matrix is
		// TestTechProviderOnboardingAndRouting).
		{method: "POST", path: "/api/v1/channels/accounts/" + accountA + "/tech-provider/disconnect"},
		{method: "POST", path: "/api/v1/channels/tech-provider/onboardings", body: map[string]any{"code": "x", "waba_id": testWABA, "phone_number_id": testPNID}},
	} {
		c.token = ownerB
		if r := hb.do(c); r.status < 400 || r.status == 500 {
			t.Errorf("B %s %s → %d %s", c.method, c.path, r.status, r.raw)
		}
	}
	// A's rows are untouched.
	if got := h.contactOf(ownerA, testCustomer); got["name"] == "pwned" {
		t.Fatal("B renamed A's contact")
	}
	// B rotating its own webhook values leaves A's alone, and A's webhook
	// address keeps answering only to A's verify token.
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/meta-connection/webhook-key", token: ownerB, body: map[string]any{"grace_hours": 0}}), 200, "")
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/meta-connection/verify-token", token: ownerB}), 200, "")
	if again := h.metaConnection(ownerA); keyOf(again) != hookKeyA || again["verify_token"] != verifyA {
		t.Fatal("B's rotation changed A's webhook values")
	}
	if code, _ := handshake(t, h.hookURL(hookKeyA), verifyA, ""); code != 200 {
		t.Fatalf("A's webhook address after B's rotation: %d", code)
	}
	if code, _ := handshake(t, h.hookURL(hookKeyA), hb.metaConnection(ownerB)["verify_token"].(string), ""); code != 403 {
		t.Fatalf("B's verify token at A's address: %d", code)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: keyA, noBFF: true}), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/me", token: ownerA}), 200, "")

	// WP2: no hand-over into a workspace one does not belong to, no
	// platform API at a workspace address, and a support grant of A is
	// invisible at B.
	h.expect(hb.do(call{method: "POST", path: "/api/v1/auth/switch-workspace", token: ownerB, body: map[string]any{"tenant_id": h.tenantID.String()}}), 403, "FORBIDDEN")
	r = hb.do(call{method: "GET", path: "/api/v1/me/memberships", token: ownerB})
	hb.expect(r, 200, "")
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["tenant"].(map[string]any)["id"] != hb.tenantID.String() {
		t.Errorf("B's memberships: %s", r.raw)
	}
	r = h.adminDo(call{method: "POST", path: "/tenants/" + h.tenantID.String() + "/support-sessions", body: map[string]any{"reason": "Isolation check"}})
	h.expect(r, 201, "")
	supportLink := r.data()["redirect_url"].(string)
	supportTok := supportLink[strings.Index(supportLink, "token=")+6:]
	h.expect(hb.do(call{method: "POST", path: "/api/v1/auth/handoff", body: map[string]any{"token": supportTok}}), 401, "HANDOFF_INVALID")
	if r := hb.do(call{method: "GET", path: "/api/v1/support-access", token: ownerB}); len(r.list()) != 0 {
		t.Errorf("B sees A's support grants: %s", r.raw)
	}
	for _, p := range []string{"/api/v1/platform/tenants", "/api/v1/platform/users", "/api/v1/platform/settings"} {
		if code, _ := h.rawAt(hb.host, "GET", p, ownerB, true); code != 404 {
			t.Errorf("platform API %s at B's address: %d", p, code)
		}
	}

	// WP4: each address's API document names only its own workspace.
	if docB := h.docAt(hb.host); strings.Contains(strings.Join(serverURLs(docB), " "), "//"+h.host+"/") {
		t.Errorf("B's API document names A's address: %v", serverURLs(docB))
	}
	// A's webhook delivery budget and API quota counters live under A's
	// tenant prefix in Redis, never under B's.
	if keys, err := h.rdb.Keys(context.Background(), tenant.RedisPrefix(hb.tenantID)+"*webhook*").Result(); err != nil || len(keys) != 0 {
		t.Errorf("B has webhook budget keys: %v %v", keys, err)
	}

	// Credentials work only at their own tenant's address.
	if code, errCode := h.rawAt(hb.host, "GET", "/api/v1/me", ownerA, true); code != 401 {
		t.Errorf("A's session at B: %d %s", code, errCode)
	}
	if code, errCode := h.rawAt(hb.host, "GET", "/api/v1/contacts", keyA, false); code != 401 || errCode != "API_KEY_INVALID" {
		t.Errorf("A's API key at B: %d %s", code, errCode)
	}
	if code, errCode := h.rawAt(h.host, "GET", "/api/v1/contacts", keyB, false); code != 401 || errCode != "API_KEY_INVALID" {
		t.Errorf("B's API key at A: %d %s", code, errCode)
	}
	if code, _ := h.rawAt(h.host, "GET", "/api/v1/me", ownerB, true); code != 401 {
		t.Errorf("B's session at A: %d", code)
	}
	// B's key at B sees B's contact only.
	r = hb.do(call{method: "GET", path: "/api/v1/contacts", token: keyB, noBFF: true})
	hb.expect(r, 200, "")
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["id"] != contactB {
		t.Fatalf("B's key lists %s", r.raw)
	}
	// Signing in at B with A's credentials fails like a wrong password.
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"email": "owner@example.com", "password": ownerPassword}}), 401, "INVALID_CREDENTIALS")

	// The panel's host header counts only with the panel secret: a direct
	// caller at B's address naming A's host still gets B.
	req, _ := http.NewRequest("GET", h.srv.URL+"/api/v1/contacts", nil)
	req.Host = hb.host
	req.Header.Set(tenant.HostHeader, h.host)
	req.Header.Set("Authorization", "Bearer "+keyA)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("forged tenant host header: %d", res.StatusCode)
	}

	// Realtime: B's session gets no token for, and no re-sync of, any of
	// A's channels; its own channels carry its own prefix.
	since := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	for _, ch := range []string{"inbox:all", "inbox:unassigned", "campaign:all", "campaign:" + campA, "conversation:" + convA,
		"personal:#" + userA, "system:ops", "system:notifications"} {
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: ownerB,
			body: map[string]any{"channel": h.wire(ch)}}), 403, "CHANNEL_FORBIDDEN")
		hb.expect(hb.do(call{method: "GET", path: "/api/v1/realtime/sync?channel=" + url.QueryEscape(h.wire(ch)) + "&since=" + url.QueryEscape(since),
			token: ownerB}), 403, "CHANNEL_FORBIDDEN")
	}
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/realtime/subscription-token", token: ownerB,
		body: map[string]any{"channel": hb.wire("inbox:all")}}), 200, "")
	r = hb.do(call{method: "POST", path: "/api/v1/realtime/connection-token", token: ownerB})
	hb.expect(r, 200, "")
	if r.data()["channel_prefix"] != "tenant:"+hb.tenantID.String()+":" {
		t.Errorf("B's channel prefix %v", r.data()["channel_prefix"])
	}

	// Unknown hosts and suspended tenants.
	if code, errCode := h.rawAt("nobody.messora.test", "GET", "/api/v1/me", "", true); code != 404 || errCode != "TENANT_NOT_FOUND" {
		t.Errorf("unknown host: %d %s", code, errCode)
	}
	if _, err := h.owner.Exec(context.Background(), "UPDATE tenants SET status = 'suspended' WHERE id = $1", hb.tenantID); err != nil {
		t.Fatal(err)
	}
	h.svc.tenants.Invalidate(hb.host)
	if code, errCode := h.rawAt(hb.host, "GET", "/api/v1/me", ownerB, true); code != 403 || errCode != "TENANT_SUSPENDED" {
		t.Errorf("suspended tenant: %d %s", code, errCode)
	}
	if code, _ := h.rawAt(h.host, "GET", "/api/v1/me", ownerA, true); code != 200 {
		t.Errorf("A after B's suspension: %d", code)
	}
}
