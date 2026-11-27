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

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Tenant B's own Meta app and WhatsApp Business Account.
const (
	betaWABA      = "203040506070"
	betaPNID      = "209876543210"
	betaAppID     = "5678"
	betaAppSecret = "beta-meta-app-secret-for-tests"    // gitleaks:allow (test fixture)
	betaToken     = "EAAG-beta-access-token-0123456789" // gitleaks:allow (test fixture)
)

// betaTenant opens tenant B with an owner and its own connected account.
func (h *harness) betaTenant() (*harness, string) {
	h.t.Helper()
	hb := h.tenant("beta")
	owner := hb.setupOwnerAs("owner@beta.test")
	h.meta.AddPhoneNumber(betaWABA, whatsapp.PhoneNumber{ID: betaPNID, DisplayPhoneNumber: "+1 555-020-0000", VerifiedName: "Beta", QualityRating: "GREEN"})
	h.meta.SetTokenInfo(betaToken, whatsapptest.TokenInfo{AppID: betaAppID})
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/accounts", token: owner, body: map[string]any{
		"name": "Beta WhatsApp", "waba_id": betaWABA, "app_id": betaAppID, "app_secret": betaAppSecret, "access_token": betaToken}}), 201, "")
	return hb, owner
}

// hookURL is a keyed webhook address on the test server.
func (h *harness) hookURL(key string) string { return h.srv.URL + "/meta/" + key }

// keyOf is the webhook key at the end of a callback URL.
func keyOf(conn map[string]any) string {
	u := conn["callback_url"].(string)
	return u[strings.LastIndex(u, "/")+1:]
}

// handshake runs Meta's GET verification against u.
func handshake(t *testing.T, u, verifyToken, host string) (int, string) {
	t.Helper()
	q := url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {verifyToken}, "hub.challenge": {"8812"}}
	req, _ := http.NewRequest(http.MethodGet, u+"?"+q.Encode(), nil)
	if host != "" {
		req.Host = host
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func (h *harness) post(u, secret string, body []byte) int {
	h.t.Helper()
	code, err := whatsapptest.Deliver(h.ctx(), u, secret, body)
	if err != nil {
		h.t.Fatal(err)
	}
	return code
}

func TestKeyedWebhookPerTenant(t *testing.T) {
	h := newHarness(t)
	ownerA, _ := h.connect()
	hb, ownerB := h.betaTenant()
	connA, connB := h.metaConnection(ownerA), hb.metaConnection(ownerB)
	keyA, keyB := keyOf(connA), keyOf(connB)
	verifyA, verifyB := connA["verify_token"].(string), connB["verify_token"].(string)
	if keyA == keyB || verifyA == verifyB || len(keyA) != 52 || len(keyB) != 52 {
		t.Fatalf("tenants share webhook values: %s %s", keyA, keyB)
	}
	if connB["host_callback_url"] != "https://beta.messora.test/webhooks/whatsapp" {
		t.Fatalf("B's host callback: %v", connB["host_callback_url"])
	}
	// The address is stable across reads.
	if again := hb.metaConnection(ownerB); keyOf(again) != keyB || again["verify_token"] != verifyB {
		t.Fatal("reading the connection changed it")
	}

	// GET verification with each tenant's own verify token only.
	if code, body := handshake(t, h.hookURL(keyA), verifyA, ""); code != 200 || body != "8812" {
		t.Fatalf("A's handshake: %d %q", code, body)
	}
	if code, _ := handshake(t, h.hookURL(keyA), verifyB, ""); code != 403 {
		t.Fatalf("B's verify token accepted at A's address: %d", code)
	}
	if code, _ := handshake(t, h.hookURL(keyB), verifyB, ""); code != 200 {
		t.Fatalf("B's handshake: %d", code)
	}
	// The /webhooks prefix serves the same address (edge proxy route).
	if code, _ := handshake(t, h.srv.URL+"/webhooks/meta/"+keyB, verifyB, ""); code != 200 {
		t.Fatalf("B's handshake under /webhooks: %d", code)
	}
	// Unknown and malformed keys are not found, before any signature check.
	for _, k := range []string{strings.Repeat("a", 52), "short", strings.ToUpper(keyB), keyB + "a"} {
		if code, _ := handshake(t, h.hookURL(k), verifyB, ""); code != 404 {
			t.Errorf("key %q: %d", k, code)
		}
		if code := h.post(h.hookURL(k), betaAppSecret, []byte(`{}`)); code != 404 {
			t.Errorf("POST key %q: %d", k, code)
		}
	}
	// Backwards compatible: B's own address, resolved by host.
	if code, _ := handshake(t, h.srv.URL+"/webhooks/meta", verifyB, hb.host); code != 200 {
		t.Fatalf("B's host-resolved handshake: %d", code)
	}
	if code, _ := handshake(t, h.srv.URL+"/webhooks/whatsapp", verifyA, hb.host); code != 403 {
		t.Fatalf("A's verify token at B's host: %d", code)
	}

	// Signatures: only B's app secret signs for B's key, and the other way
	// round; nothing rejected is stored.
	inB := whatsapptest.ForWABA(betaWABA, whatsapptest.InboundText(betaPNID, testCustomer, "wamid.beta-1", "selam beta", "Beta Müşteri", time.Now()))
	if code := h.post(h.hookURL(keyB), testAppSecret, inB); code != 401 {
		t.Fatalf("A's secret signed for B's key: %d", code)
	}
	inA := whatsapptest.InboundText(testPNID, testCustomer, "wamid.alpha-1", "selam alpha", "Alpha Müşteri", time.Now())
	if code := h.post(h.hookURL(keyA), betaAppSecret, inA); code != 401 {
		t.Fatalf("B's secret signed for A's key: %d", code)
	}
	if n := h.count("SELECT count(*) FROM webhook_events"); n != 0 {
		t.Fatalf("A stored %d rejected webhooks", n)
	}
	if n := hb.count("SELECT count(*) FROM webhook_events"); n != 0 {
		t.Fatalf("B stored %d rejected webhooks", n)
	}
	if hb.metaConnection(ownerB)["health"].(map[string]any)["signature_failures"] != float64(1) {
		t.Fatal("B's signature failure not counted")
	}

	// B's app signing events of A's WhatsApp Business Account: dropped,
	// counted, never stored in either tenant.
	if code := h.post(h.hookURL(keyB), betaAppSecret, inA); code != 200 {
		t.Fatalf("foreign WABA: %d", code)
	}
	if hb.count("SELECT count(*) FROM webhook_events")+h.count("SELECT count(*) FROM webhook_events") != 0 {
		t.Fatal("an event for another tenant's account was stored")
	}
	health := hb.metaConnection(ownerB)["health"].(map[string]any)
	if health["waba_mismatches"] != float64(1) || health["last_mismatch_waba_id"] != testWABA {
		t.Fatalf("mismatch not recorded: %v", health)
	}
	// A batch mixing both accounts keeps only B's own entry.
	var a, b struct {
		Object string            `json:"object"`
		Entry  []json.RawMessage `json:"entry"`
	}
	_ = json.Unmarshal(inA, &a)
	_ = json.Unmarshal(inB, &b)
	mixed, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []json.RawMessage{a.Entry[0], b.Entry[0]}})
	if code := h.post(h.hookURL(keyB), betaAppSecret, mixed); code != 200 {
		t.Fatalf("mixed batch: %d", code)
	}
	var stored []byte
	if err := hb.pool.QueryRow(hb.ctx(), "SELECT body FROM webhook_events").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), testWABA) || !strings.Contains(string(stored), betaWABA) {
		t.Fatalf("stored batch: %s", stored)
	}

	// B's own event lands in B's inbox and B's realtime channels, once
	// however often Meta repeats it.
	if _, err := hb.pool.Exec(hb.ctx(), "DELETE FROM webhook_events"); err != nil {
		t.Fatal(err)
	}
	var fromOutbox int64
	if err := h.sys.QueryRow(tenant.System(context.Background()), "SELECT coalesce(max(id), 0) FROM outbox_events").Scan(&fromOutbox); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if code := h.post(h.hookURL(keyB), betaAppSecret, inB); code != 200 {
			t.Fatalf("B's webhook: %d", code)
		}
	}
	if n := hb.count("SELECT count(*) FROM webhook_events"); n != 1 {
		t.Fatalf("B stored a repeated delivery %d times", n)
	}
	hb.processWebhooks()
	if n := hb.count("SELECT count(*) FROM messages WHERE wamid = 'wamid.beta-1' AND direction = 'inbound'"); n != 1 {
		t.Fatalf("B's inbound message: %d", n)
	}
	if n := h.count("SELECT count(*) FROM messages WHERE direction = 'inbound'"); n != 0 {
		t.Fatalf("A received %d inbound messages", n)
	}
	r := hb.do(call{method: "GET", path: "/api/v1/conversations", token: ownerB})
	hb.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("B's inbox: %s", r.raw)
	}
	// The realtime events of that message go to B's channels.
	rows, err := h.sys.Query(tenant.System(context.Background()), "SELECT id, tenant_id, topic, payload FROM outbox_events WHERE id > $1 AND topic LIKE 'rt:%' ORDER BY id", fromOutbox)
	if err != nil {
		t.Fatal(err)
	}
	var published []string
	pub := realtime.Publisher(captureClient(func(ch string) { published = append(published, ch) }))
	for rows.Next() {
		var e outbox.Event
		var tid *uuid.UUID
		if err := rows.Scan(&e.ID, &tid, &e.Topic, &e.Payload); err != nil {
			t.Fatal(err)
		}
		if tid != nil {
			e.TenantID = *tid
		}
		if err := pub.Publish(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	rows.Close()
	if len(published) == 0 {
		t.Fatal("no realtime event for B's inbound message")
	}
	for _, ch := range published {
		if !strings.HasPrefix(ch, "tenant:"+hb.tenantID.String()+":") {
			t.Errorf("B's inbound message published on %s", ch)
		}
	}
	if !strings.Contains(strings.Join(published, " "), hb.wire(realtime.InboxChannel)) {
		t.Errorf("B's inbox channel missing from %v", published)
	}
	// Health: the last webhook time is recorded.
	if hb.metaConnection(ownerB)["health"].(map[string]any)["last_webhook_at"] == nil {
		t.Fatal("B's last webhook time missing")
	}

	// Duplicate detection is per tenant: the same body may exist in two
	// tenants.
	var sum []byte
	if err := hb.pool.QueryRow(hb.ctx(), "SELECT payload_sha256 FROM webhook_events").Scan(&sum); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx(), "INSERT INTO webhook_events (payload_sha256, body) VALUES ($1, $2)", sum, inB); err != nil {
		t.Fatalf("same payload in another tenant: %v", err)
	}

	// Suspended tenants' addresses answer 403.
	if _, err := h.owner.Exec(context.Background(), "UPDATE tenants SET status = 'suspended' WHERE id = $1", hb.tenantID); err != nil {
		t.Fatal(err)
	}
	if code := h.post(h.hookURL(keyB), betaAppSecret, inB); code != 403 {
		t.Fatalf("suspended tenant's webhook: %d", code)
	}
	if code := h.post(h.hookURL(keyA), testAppSecret, inA); code != 200 {
		t.Fatalf("A after B's suspension: %d", code)
	}
}

type captureClient func(channel string)

func (c captureClient) Publish(_ context.Context, channel string, _ []byte, _ string) error {
	c(channel)
	return nil
}

func TestWebhookKeyRotation(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	conn := h.metaConnection(owner)
	k1, verify1 := keyOf(conn), conn["verify_token"].(string)
	in := func(wamid string) []byte {
		return whatsapptest.InboundText(testPNID, testCustomer, wamid, "hi", "Ali", time.Now())
	}

	// Rotation with a grace period: both addresses work until it ends.
	r := h.do(call{method: "POST", path: "/api/v1/channels/meta-connection/webhook-key", token: owner, body: map[string]any{"grace_hours": 24}})
	h.expect(r, 200, "")
	k2 := keyOf(r.data())
	if k2 == k1 || r.data()["previous_key_expires_at"] == nil || r.data()["webhook_key_rotated_at"] == nil {
		t.Fatalf("rotation: %s", r.raw)
	}
	if exp, _ := time.Parse(time.RFC3339, r.data()["previous_key_expires_at"].(string)); time.Until(exp) < 23*time.Hour || time.Until(exp) > 25*time.Hour {
		t.Fatalf("grace ends %v", exp)
	}
	for i, k := range []string{k1, k2} {
		if code := h.post(h.hookURL(k), testAppSecret, in("wamid.rot-"+k[:6])); code != 200 {
			t.Fatalf("key %d during the grace period: %d", i+1, code)
		}
	}
	if code, _ := handshake(t, h.hookURL(k1), verify1, ""); code != 200 {
		t.Fatalf("old key handshake during the grace period: %d", code)
	}
	// When the grace period is over the old key is unknown.
	if _, err := h.owner.Exec(context.Background(), "UPDATE meta_webhook_keys SET expires_at = now() - interval '1 second' WHERE expires_at IS NOT NULL"); err != nil {
		t.Fatal(err)
	}
	if code := h.post(h.hookURL(k1), testAppSecret, in("wamid.rot-late")); code != 404 {
		t.Fatalf("expired key: %d", code)
	}
	// Rotation without grace ends the current key at once.
	r = h.do(call{method: "POST", path: "/api/v1/channels/meta-connection/webhook-key", token: owner, body: map[string]any{"grace_hours": 0}})
	h.expect(r, 200, "")
	k3 := keyOf(r.data())
	if r.data()["previous_key_expires_at"] != nil {
		t.Fatalf("no grace, yet a previous key: %s", r.raw)
	}
	if code := h.post(h.hookURL(k2), testAppSecret, in("wamid.rot-k2")); code != 404 {
		t.Fatalf("key ended without grace: %d", code)
	}
	if code := h.post(h.hookURL(k3), testAppSecret, in("wamid.rot-k3")); code != 200 {
		t.Fatalf("new key: %d", code)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/channels/meta-connection/webhook-key", token: owner, body: map[string]any{"grace_hours": 169}}), 422, "")
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action = 'channel.webhook_key_rotate'"); n != 2 {
		t.Fatalf("rotations audited %d times", n)
	}

	// Verify token rotation: the old token stops at once.
	r = h.do(call{method: "POST", path: "/api/v1/channels/meta-connection/verify-token", token: owner})
	h.expect(r, 200, "")
	verify2 := r.data()["verify_token"].(string)
	if verify2 == verify1 || keyOf(r.data()) != k3 {
		t.Fatalf("verify token rotation: %s", r.raw)
	}
	if code, _ := handshake(t, h.hookURL(k3), verify1, ""); code != 403 {
		t.Fatalf("old verify token: %d", code)
	}
	if code, _ := handshake(t, h.hookURL(k3), verify2, ""); code != 200 {
		t.Fatalf("new verify token: %d", code)
	}

	// The routing table is written only through the rotation function,
	// and only for the connection's own tenant.
	if _, err := h.pool.Exec(h.ctx(), "INSERT INTO meta_webhook_keys (key_sha256, tenant_id) VALUES ($1, $2)", make([]byte, 32), tenant.DefaultID); err == nil {
		t.Fatal("the application role wrote the routing table directly")
	}
	hb := h.tenant("gamma")
	if _, err := hb.pool.Exec(hb.ctx(), "SELECT meta_webhook_key_rotate($1, interval '0')", make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM meta_webhook_keys WHERE tenant_id = $1", hb.tenantID); n != 1 {
		t.Fatalf("gamma's routes: %d", n)
	}
	if n := h.count("SELECT count(*) FROM meta_webhook_keys WHERE tenant_id = $1 AND expires_at IS NULL", tenant.DefaultID); n != 1 {
		t.Fatalf("A lost its current key: %d", n)
	}
	if _, err := h.pool.Exec(tenant.System(context.Background()), "SELECT meta_webhook_key_rotate($1, interval '0')", make([]byte, 32)); err == nil {
		t.Fatal("a connection without a tenant routed a key")
	}
}

func TestOutboundUsesTheTenantsToken(t *testing.T) {
	h := newHarness(t)
	ownerA, phoneA := h.connect()
	hb, ownerB := h.betaTenant()
	phoneB := hb.do(call{method: "GET", path: "/api/v1/channels/phone-numbers", token: ownerB}).list()[0].(map[string]any)["id"].(string)
	// Each customer writes first, so free text may be sent.
	h.inbound(testCustomer, "wamid.out-a", "hi a", time.Now())
	keyB := keyOf(hb.metaConnection(ownerB))
	if code := h.post(h.hookURL(keyB), betaAppSecret, whatsapptest.ForWABA(betaWABA,
		whatsapptest.InboundText(betaPNID, testCustomer, "wamid.out-b", "hi b", "B", time.Now()))); code != 200 {
		t.Fatalf("B's webhook: %d", code)
	}
	hb.processWebhooks()

	send := func(who *harness, token, phone string) string {
		r := who.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "merhaba"}, "phone_number_id": phone})
		who.expect(r, 202, "")
		id := r.data()["id"].(string)
		if err := who.svc.messaging.Send(who.ctx(), uuid.MustParse(id)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	send(h, ownerA, phoneA)
	send(hb, ownerB, phoneB)
	sent := h.meta.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d", len(sent))
	}
	if sent[0].PhoneNumberID != testPNID || sent[0].Token != testToken {
		t.Errorf("A sent from %s with %q", sent[0].PhoneNumberID, sent[0].Token)
	}
	if sent[1].PhoneNumberID != betaPNID || sent[1].Token != betaToken {
		t.Errorf("B sent from %s with %q", sent[1].PhoneNumberID, sent[1].Token)
	}
	// B cannot send from A's number.
	hb.expect(hb.send(ownerB, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "x"}, "phone_number_id": phoneA}), 422, "VALIDATION_ERROR")
}

func TestConnectionTestEndpoint(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	acc := h.accountID(owner)
	test := func() map[string]any {
		t.Helper()
		r := h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + acc + "/test", token: owner})
		h.expect(r, 200, "")
		return r.data()
	}
	codes := func(d map[string]any) map[string]string {
		out := map[string]string{}
		for _, c := range d["checks"].([]any) {
			m := c.(map[string]any)
			out[m["key"].(string)] = m["status"].(string) + ":" + m["code"].(string)
		}
		return out
	}

	// Not subscribed yet: the test subscribes the app; no webhook yet.
	d := test()
	got := codes(d)
	if d["ok"] != true || d["subscribed"] != true || d["subscribed_now"] != true || d["phone_numbers"] != float64(1) ||
		got["app_secret"] != "ok:app_ok" || got["token"] != "ok:token_ok" || got["phone_numbers"] != "ok:numbers_ok" ||
		got["subscription"] != "ok:subscribed_now" || got["webhook"] != "warning:webhook_none" {
		t.Fatalf("first test: %v %v", got, d)
	}
	if subs := h.meta.Subscribed(testWABA); len(subs) != 1 || subs[0] != testAppID {
		t.Fatalf("subscriptions on Meta: %v", subs)
	}
	// Subscribed now, and a webhook arrived.
	h.inbound(testCustomer, "wamid.test-1", "hi", time.Now())
	d = test()
	if got := codes(d); d["subscribed_now"] != false || got["subscription"] != "ok:subscribed" || got["webhook"] != "ok:webhook_ok" {
		t.Fatalf("second test: %v", got)
	}

	// A token of another app is pointed out; an invalid one fails the test
	// and flags the account.
	h.meta.SetTokenInfo(testToken, whatsapptest.TokenInfo{AppID: "999"})
	if got := codes(test()); got["token"] != "warning:token_other_app" {
		t.Fatalf("other app's token: %v", got)
	}
	h.meta.SetTokenInfo(testToken, whatsapptest.TokenInfo{Invalid: true, Error: "Session has expired"})
	d = test()
	if got := codes(d); d["ok"] != false || got["token"] != "error:token_invalid" || d["account"].(map[string]any)["status"] != "error" {
		t.Fatalf("invalid token: %v %v", got, d["account"])
	}

	// Another tenant cannot test this account.
	hb := h.tenant("delta")
	ownerB := hb.setupOwnerAs("owner@delta.test")
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/accounts/" + acc + "/test", token: ownerB}), 404, "")
}
