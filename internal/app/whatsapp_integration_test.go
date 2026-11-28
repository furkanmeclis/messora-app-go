//go:build integration

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

const (
	testWABA     = whatsapptest.DefaultWABA
	testPNID     = "109876543210"
	testCustomer = "905551112233"
	testToken    = "EAAG-test-access-token-0123456789" // gitleaks:allow (test fixture)
)

// connect sets up an owner and a connected account with one number and an
// approved template.
func (h *harness) connect() (token string, phoneID string) {
	h.t.Helper()
	h.meta.AddPhoneNumber(testWABA, whatsapp.PhoneNumber{ID: testPNID, DisplayPhoneNumber: "+1 555-010-0000", VerifiedName: "Acme", QualityRating: "GREEN"})
	h.meta.AddTemplate(testWABA, whatsapp.TemplateInfo{ID: "777", Name: "order_update", Language: "en", Category: "UTILITY", Status: "APPROVED",
		Components: []byte(`[{"type":"BODY","text":"Your order shipped."}]`)})
	h.meta.AddTemplate(testWABA, whatsapp.TemplateInfo{ID: "778", Name: "promo", Language: "en", Category: "MARKETING", Status: "PENDING"})
	token, _ = h.setupOwner()
	r := h.do(call{method: "POST", path: "/api/v1/channels/accounts", token: token, body: map[string]any{
		"name": "Acme WhatsApp", "waba_id": testWABA, "app_id": testAppID, "app_secret": testAppSecret, "access_token": testToken}})
	h.expect(r, 201, "")
	if r.data()["app_secret_configured"] != true || r.data()["status"] != "active" {
		h.t.Fatalf("account: %s", r.raw)
	}
	if _, ok := r.data()["access_token"]; ok {
		h.t.Fatal("access token must never be returned")
	}
	r = h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + r.data()["id"].(string) + "/sync", token: token})
	h.expect(r, 200, "")
	if r.data()["phone_numbers"] != float64(1) || r.data()["templates"] != float64(2) {
		h.t.Fatalf("sync: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/channels/phone-numbers", token: token})
	h.expect(r, 200, "")
	n := r.list()[0].(map[string]any)
	if n["is_default"] != true || n["verified_name"] != "Acme" {
		h.t.Fatalf("numbers: %s", r.raw)
	}
	return token, n["id"].(string)
}

// metaConnection returns the workspace's Meta connection settings.
func (h *harness) metaConnection(token string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/channels/meta-connection", token: token})
	h.expect(r, 200, "")
	return r.data()
}

// webhook delivers a signed body and processes every pending event.
func (h *harness) webhook(body []byte) int {
	h.t.Helper()
	status, err := whatsapptest.Deliver(h.ctx(), h.srv.URL+"/webhooks/whatsapp", testAppSecret, body)
	if err != nil {
		h.t.Fatal(err)
	}
	h.processWebhooks()
	return status
}

func (h *harness) processWebhooks() {
	h.t.Helper()
	rows, err := h.pool.Query(h.ctx(), "SELECT id FROM webhook_events WHERE status = 'pending' ORDER BY id")
	if err != nil {
		h.t.Fatal(err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := h.svc.messaging.ProcessWebhook(h.ctx(), id, 0, 10); err != nil {
			h.t.Fatalf("process webhook: %v", err)
		}
	}
}

func (h *harness) send(token string, body map[string]any) resp {
	h.t.Helper()
	return h.do(call{method: "POST", path: "/api/v1/messages", token: token, body: body})
}

// deliver runs the worker's send step for a queued message.
func (h *harness) deliver(id string) error {
	return h.svc.messaging.Send(h.ctx(), uuid.MustParse(id))
}

func (h *harness) message(token, id string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/messages/" + id, token: token})
	h.expect(r, 200, "")
	return r.data()
}

func (h *harness) count(sql string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(h.ctx(), sql, args...).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func TestWebhookVerificationAndSignature(t *testing.T) {
	h := newHarness(t)
	get := func(q url.Values) (int, string) {
		res, err := http.Get(h.srv.URL + "/webhooks/whatsapp?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	body := whatsapptest.InboundText(testPNID, testCustomer, "wamid.in1", "hi", "Ali", time.Now())
	// Before the workspace has an account with an app secret nothing is
	// accepted, and before its connection exists no verify token matches.
	if code, _ := whatsapptest.Deliver(h.ctx(), h.srv.URL+"/webhooks/whatsapp", testAppSecret, body); code != 401 {
		t.Fatalf("webhook without any app secret: %d", code)
	}
	if code, _ := get(url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {""}, "hub.challenge": {"1"}}); code != 403 {
		t.Fatalf("handshake without a connection: %d", code)
	}
	token, _ := h.connect()
	verify := h.metaConnection(token)["verify_token"].(string)
	if code, body := get(url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {verify}, "hub.challenge": {"1158201444"}}); code != 200 || body != "1158201444" {
		t.Fatalf("handshake: %d %q", code, body)
	}
	if code, _ := get(url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {"wrong"}, "hub.challenge": {"1"}}); code != 403 {
		t.Fatalf("wrong verify token accepted: %d", code)
	}

	if code, _ := whatsapptest.Deliver(h.ctx(), h.srv.URL+"/webhooks/whatsapp", "not-the-secret", body); code != 401 {
		t.Fatalf("bad signature: %d", code)
	}
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/webhooks/whatsapp", nil)
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("unsigned webhook: %d", res.StatusCode)
	}
	if n := h.count("SELECT count(*) FROM webhook_events"); n != 0 {
		t.Fatalf("rejected webhooks must not be stored, got %d", n)
	}
	if h := h.metaConnection(token)["health"].(map[string]any); h["signature_failures"] != float64(2) || h["last_signature_failure_at"] == nil {
		t.Fatalf("signature failures not counted: %v", h)
	}
	// Replays of an identical delivery are stored once.
	for range 3 {
		if code, _ := whatsapptest.Deliver(h.ctx(), h.srv.URL+"/webhooks/whatsapp", testAppSecret, body); code != 200 {
			t.Fatalf("valid webhook: %d", code)
		}
	}
	if n := h.count("SELECT count(*) FROM webhook_events"); n != 1 {
		t.Fatalf("replayed webhook stored %d times", n)
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:whatsapp:webhook'"); n != 1 {
		t.Fatalf("processing scheduled %d times", n)
	}
	if h.metaConnection(token)["health"].(map[string]any)["last_webhook_at"] == nil {
		t.Fatal("last webhook time not recorded")
	}
}

func TestInboundMessagesOpenTheWindowAndDeduplicate(t *testing.T) {
	h := newHarness(t)
	token, phoneID := h.connect()

	// Outside the 24-hour window only templates may be sent.
	h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "hello"}}), 422, "OUTSIDE_SERVICE_WINDOW")

	in := whatsapptest.InboundText(testPNID, testCustomer, "wamid.inbound-1", "Merhaba, siparişim nerede?", "Ali Veli", time.Now())
	if code := h.webhook(in); code != 200 {
		t.Fatalf("webhook: %d", code)
	}
	// The same message delivered again inside a different envelope (Meta
	// retries can batch differently) must not duplicate it.
	again := whatsapptest.InboundText(testPNID, testCustomer, "wamid.inbound-1", "Merhaba, siparişim nerede?", "Ali Veli", time.Now().Add(time.Second))
	h.webhook(again)
	if n := h.count("SELECT count(*) FROM messages WHERE direction = 'inbound'"); n != 1 {
		t.Fatalf("inbound messages: %d", n)
	}
	if n := h.count("SELECT count(*) FROM contacts WHERE phone_e164 = $1 AND profile_name = 'Ali Veli' AND last_inbound_at IS NOT NULL", "+"+testCustomer); n != 1 {
		t.Fatal("contact not created from the inbound message")
	}

	r := h.send(token, map[string]any{"to": "+90 555 111 22 33", "type": "text", "text": map[string]any{"body": "Kargoda!"}, "phone_number_id": phoneID, "reply_to": "wamid.inbound-1"})
	h.expect(r, 202, "")
	id := r.data()["id"].(string)
	if r.data()["status"] != "pending" || r.data()["to"] != "+"+testCustomer {
		t.Fatalf("queued: %s", r.raw)
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:whatsapp:send' AND payload->>'message_id' = $1", id); n != 1 {
		t.Fatal("send task not scheduled with the message")
	}
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	m := h.message(token, id)
	if m["status"] != "sent" || m["wamid"] == nil {
		t.Fatalf("after send: %v", m)
	}
	sent := h.meta.Sent()
	last := sent[len(sent)-1].Message
	if last.To != testCustomer || last.Context == nil || last.Context.MessageID != "wamid.inbound-1" || last.BizOpaqueCallbackData != id || sent[len(sent)-1].Token != testToken {
		t.Fatalf("request to Meta: %+v", sent[len(sent)-1])
	}
	// The worker running the same task again (asynq redelivery) is a no-op.
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	if len(h.meta.Sent()) != len(sent) {
		t.Fatal("message sent twice")
	}
}

func TestTemplatesAndOutOfOrderStatuses(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()

	h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "promo", "language": "en"}}), 422, "VALIDATION_ERROR")
	h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "missing", "language": "en"}}), 422, "VALIDATION_ERROR")
	h.expect(h.send(token, map[string]any{"to": "12", "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}}), 422, "VALIDATION_ERROR")

	r := h.send(token, map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}})
	h.expect(r, 202, "")
	id := r.data()["id"].(string)
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	wamid := h.message(token, id)["wamid"].(string)

	// Meta does not guarantee order: read arrives before delivered, and a
	// late "sent" must not move the message backwards.
	now := time.Now()
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "read", testCustomer, id, now.Add(2*time.Second), 0))
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "delivered", testCustomer, id, now.Add(time.Second), 0))
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "sent", testCustomer, id, now, 0))
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "failed", testCustomer, id, now.Add(3*time.Second), 131026))
	m := h.message(token, id)
	if m["status"] != "read" || m["delivered_at"] == nil || m["read_at"] == nil || m["failed_at"] != nil || m["error_code"] != nil {
		t.Fatalf("monotonic status broken: %v", m)
	}

	// A failure before delivery is recorded with Meta's error.
	r = h.send(token, map[string]any{"to": "+905559998877", "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}})
	id2 := r.data()["id"].(string)
	_ = h.deliver(id2)
	w2 := h.message(token, id2)["wamid"].(string)
	h.webhook(whatsapptest.StatusUpdate(testPNID, w2, "failed", "905559998877", id2, time.Now(), 131026))
	m = h.message(token, id2)
	if m["status"] != "failed" || m["error_code"] != "131026" {
		t.Fatalf("failed status: %v", m)
	}

	// Template review results arrive by webhook.
	h.webhook(whatsapptest.TemplateStatus(testWABA, "778", "promo", "en", "REJECTED", "INVALID_FORMAT"))
	r = h.do(call{method: "GET", path: "/api/v1/templates?status=REJECTED", token: token})
	h.expect(r, 200, "")
	if len(r.list()) != 1 || r.list()[0].(map[string]any)["rejected_reason"] != "INVALID_FORMAT" {
		t.Fatalf("template status: %s", r.raw)
	}

	// Create and delete through Meta.
	r = h.do(call{method: "POST", path: "/api/v1/templates", token: token, body: map[string]any{
		"account_id": h.message(token, id)["phone_number_id"], "name": "x", "language": "en", "category": "UTILITY", "components": []any{map[string]any{"type": "BODY", "text": "hi"}}}})
	h.expect(r, 404, "NOT_FOUND") // a phone number id is not an account id
	acc := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()[0].(map[string]any)["id"]
	r = h.do(call{method: "POST", path: "/api/v1/templates", token: token, body: map[string]any{
		"account_id": acc, "name": "shipping_notice", "language": "tr", "category": "UTILITY", "components": []any{map[string]any{"type": "BODY", "text": "Kargonuz yolda"}}}})
	h.expect(r, 201, "")
	if r.data()["status"] != "PENDING" {
		t.Fatalf("created template: %s", r.raw)
	}
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/templates/" + r.data()["id"].(string), token: token}), 204, "")
	if n := h.count("SELECT count(*) FROM message_templates WHERE name = 'shipping_notice'"); n != 0 {
		t.Fatal("template not deleted")
	}
}

func TestSendFailureModesNeverDuplicate(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	queue := func(to string) string {
		r := h.send(token, map[string]any{"to": to, "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}})
		h.expect(r, 202, "")
		return r.data()["id"].(string)
	}
	accepted := func(id string) int {
		n := 0
		for _, s := range h.meta.Sent() {
			if s.Message.BizOpaqueCallbackData == id && s.WAMID != "" {
				n++
			}
		}
		return n
	}

	// Throughput limit: retried later, then sent once.
	id := queue("+905550000001")
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 130429, HTTPStatus: 429})
	err := h.deliver(id)
	var retry *messaging.ErrRetry
	if !asRetry(err, &retry) || retry.After <= 0 {
		t.Fatalf("want retry, got %v", err)
	}
	if m := h.message(token, id); m["status"] != "pending" || m["error_code"] != "130429" {
		t.Fatalf("after rate limit: %v", m)
	}
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	if h.message(token, id)["status"] != "sent" || accepted(id) != 1 {
		t.Fatal("rate-limited message not sent exactly once")
	}

	// Connection dropped after Meta processed the request: unknown, then
	// reconciled from the status webhook through biz_opaque_callback_data.
	id = queue("+905550000002")
	h.meta.Script(whatsapptest.Behavior{DropConnection: true})
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	if m := h.message(token, id); m["status"] != "unknown" || m["wamid"] != nil {
		t.Fatalf("after dropped connection: %v", m)
	}
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	if accepted(id) != 1 {
		t.Fatalf("ambiguous message re-sent: %d", accepted(id))
	}
	var wamid string
	for _, s := range h.meta.Sent() {
		if s.Message.BizOpaqueCallbackData == id {
			wamid = s.WAMID
		}
	}
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "delivered", "905550000002", id, time.Now(), 0))
	if m := h.message(token, id); m["status"] != "delivered" || m["wamid"] != wamid {
		t.Fatalf("not reconciled: %v", m)
	}

	// Worker died between claiming and sending: never sent again.
	id = queue("+905550000003")
	if _, err := h.pool.Exec(h.ctx(), "UPDATE messages SET status = 'sending', status_rank = 1 WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	if m := h.message(token, id); m["status"] != "unknown" || accepted(id) != 0 {
		t.Fatalf("interrupted send: %v", m)
	}

	// Permanent errors fail without retry; an expired token also flags the account.
	id = queue("+905550000004")
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 190, HTTPStatus: 401})
	if err := h.deliver(id); err != nil {
		t.Fatal(err)
	}
	if m := h.message(token, id); m["status"] != "failed" || m["error_code"] != "190" {
		t.Fatalf("expired token: %v", m)
	}
	accs := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()
	if accs[0].(map[string]any)["status"] != "error" {
		t.Fatal("account not flagged after an authentication error")
	}
	// Further messages fail fast instead of hammering Meta.
	before := len(h.meta.Sent())
	id = queue("+905550000005")
	_ = h.deliver(id)
	if h.message(token, id)["status"] != "failed" || len(h.meta.Sent()) != before {
		t.Fatal("message sent through a flagged account")
	}
	// A successful sync clears the flag.
	h.expect(h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + accs[0].(map[string]any)["id"].(string) + "/sync", token: token}), 200, "")
	if h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()[0].(map[string]any)["status"] != "active" {
		t.Fatal("sync did not clear the error")
	}
}

func asRetry(err error, target **messaging.ErrRetry) bool {
	r, ok := err.(*messaging.ErrRetry) //nolint:errorlint // returned unwrapped
	*target = r
	return ok
}

func TestInboundMediaIsDownloaded(t *testing.T) {
	h := newHarness(t)
	h.connect()
	h.meta.AddMedia("media-abc", "image/jpeg", []byte("\xff\xd8\xff fake jpeg"))
	h.webhook(whatsapptest.InboundMedia(testPNID, testCustomer, "wamid.img1", "image", "media-abc", "image/jpeg", "fatura", time.Now()))

	var id uuid.UUID
	if err := h.pool.QueryRow(h.ctx(), "SELECT id FROM media_files WHERE meta_media_id = 'media-abc'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:whatsapp:media_download'"); n != 1 {
		t.Fatal("download not scheduled")
	}
	if err := h.svc.messaging.DownloadMedia(h.ctx(), id); err != nil {
		t.Fatal(err)
	}
	var key string
	var size int64
	if err := h.pool.QueryRow(h.ctx(), "SELECT storage_key, size_bytes FROM media_files WHERE id = $1 AND status = 'stored'", id).Scan(&key, &size); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(h.svc.cfg.Storage.LocalDir, key))
	if err != nil || string(b) != "\xff\xd8\xff fake jpeg" || size != int64(len(b)) {
		t.Fatalf("stored media: %q %v", b, err)
	}
	if h.count("SELECT count(*) FROM messages WHERE wamid = 'wamid.img1' AND body = 'fatura' AND type = 'image'") != 1 {
		t.Fatal("caption not stored as body")
	}
}

func TestChannelPermissionsAndAPIKeySend(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()

	// Developer API: a key with messages.send can send and read status.
	r := h.do(call{method: "POST", path: "/api/v1/auth/reauth", token: token, body: map[string]any{"password": ownerPassword}})
	h.expect(r, 200, "")
	r = h.do(call{method: "POST", path: "/api/v1/api-keys", token: token, body: map[string]any{"name": "shop", "scopes": []string{"messages.send"}}})
	h.expect(r, 201, "")
	key := r.data()["secret"].(string)

	r = h.do(call{method: "POST", path: "/api/v1/messages", token: key, noBFF: true, headers: map[string]string{"Idempotency-Key": "order-1001-shipped"},
		body: map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}}})
	h.expect(r, 202, "")
	r2 := h.do(call{method: "POST", path: "/api/v1/messages", token: key, noBFF: true, headers: map[string]string{"Idempotency-Key": "order-1001-shipped"},
		body: map[string]any{"to": "+" + testCustomer, "type": "template", "template": map[string]any{"name": "order_update", "language": "en"}}})
	if r2.data()["id"] != r.data()["id"] || h.count("SELECT count(*) FROM messages WHERE direction = 'outbound'") != 1 {
		t.Fatal("idempotent retry queued a second message")
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/messages/" + r.data()["id"].(string), token: key, noBFF: true}), 200, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/channels/phone-numbers", token: key, noBFF: true}), 200, "")
	// …but cannot manage channels or templates.
	h.expect(h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: key, noBFF: true}), 403, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/templates", token: key, noBFF: true}), 403, "")

	c := h.metaConnection(token)
	if !strings.HasPrefix(c["callback_url"].(string), "https://hooks.messora.test/meta/") || c["host_callback_url"] != "https://messora.test/webhooks/whatsapp" ||
		len(c["verify_token"].(string)) != 52 || c["signature_verification"] != true || c["method"] != "own_app" || c["tech_provider_enabled"] != false {
		t.Fatalf("meta connection: %v", c)
	}
	// Accounts with history cannot be removed; disabling works.
	acc := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()[0].(map[string]any)["id"].(string)
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/channels/accounts/" + acc, token: token}), 409, "ACCOUNT_HAS_MESSAGES")
	r = h.do(call{method: "PATCH", path: "/api/v1/channels/accounts/" + acc, token: token, body: map[string]any{"status": "disabled"}})
	h.expect(r, 200, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/channels/accounts", token: token, body: map[string]any{"name": "dup", "waba_id": testWABA, "app_id": testAppID, "app_secret": testAppSecret, "access_token": testToken}}), 409, "ACCOUNT_EXISTS")
	if h.count("SELECT count(*) FROM audit_logs WHERE action LIKE 'channel.%'") < 2 {
		t.Fatal("channel changes not audited")
	}
}

func TestOutboundMediaUpload(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	upload := func(name, ctype string, body []byte) (int, map[string]any) {
		status, raw, _ := h.raw("POST", "/api/v1/media?filename="+name, token, ctype, bytes.NewReader(body))
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return status, out
	}
	status, out := upload("price-list.pdf", "application/pdf", []byte("%PDF-1.7 fake"))
	if status != 201 {
		t.Fatalf("upload: %d %v", status, out)
	}
	id := out["data"].(map[string]any)["id"].(string)
	if !strings.HasPrefix(id, "media") || out["data"].(map[string]any)["size"] != float64(13) {
		t.Fatalf("upload result: %v", out)
	}
	if status, _ := upload("x.exe", "application/x-msdownload", []byte("MZ")); status != 422 {
		t.Fatalf("executable accepted: %d", status)
	}
	if status, _ := upload("big.png", "image/png", bytes.Repeat([]byte{1}, 5<<20+1)); status != 422 {
		t.Fatalf("oversized image accepted: %d", status)
	}
	// The uploaded file is sent by id inside the customer service window.
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.media-1", "send the list", "", time.Now()))
	r := h.send(token, map[string]any{"to": "+" + testCustomer, "type": "document", "media": map[string]any{"id": id, "filename": "price-list.pdf"}})
	h.expect(r, 202, "")
	if err := h.deliver(r.data()["id"].(string)); err != nil {
		t.Fatal(err)
	}
	sent := h.meta.Sent()
	if d := sent[len(sent)-1].Message.Document; d == nil || d.ID != id || d.Filename != "price-list.pdf" {
		t.Fatalf("document message: %+v", sent[len(sent)-1].Message)
	}
}
