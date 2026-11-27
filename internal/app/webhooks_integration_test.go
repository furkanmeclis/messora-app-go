//go:build integration

package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/safehttp"
)

// hookReceiver is an integrator's endpoint: it verifies every signature and
// answers with the programmed status codes (then 200).
type hookReceiver struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	secret string
	codes  []int
	got    []hookRequest
}

type hookRequest struct {
	header http.Header
	env    map[string]any
	valid  bool
}

func newHookReceiver(t *testing.T) *hookReceiver {
	rc := &hookReceiver{t: t}
	rc.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rc.mu.Lock()
		defer rc.mu.Unlock()
		hr := hookRequest{header: r.Header.Clone(),
			valid: webhooks.Verify(rc.secret, r.Header.Get(webhooks.HeaderSignature), body, 5*time.Minute, time.Now())}
		if err := json.Unmarshal(body, &hr.env); err != nil {
			t.Errorf("receiver got invalid JSON: %s", body)
		}
		rc.got = append(rc.got, hr)
		code := 200
		if len(rc.codes) > 0 {
			code, rc.codes = rc.codes[0], rc.codes[1:]
		}
		w.WriteHeader(code)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(rc.srv.Close)
	return rc
}

func (rc *hookReceiver) program(codes ...int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.codes = codes
}

func (rc *hookReceiver) requests() []hookRequest {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return append([]hookRequest(nil), rc.got...)
}

func (rc *hookReceiver) last() hookRequest {
	rc.t.Helper()
	got := rc.requests()
	if len(got) == 0 {
		rc.t.Fatal("receiver got nothing")
	}
	return got[len(got)-1]
}

// allowLocalReceivers lets the delivery client reach httptest servers on
// 127.0.0.1. safehttp only honours AllowPrivate inside test binaries.
func (h *harness) allowLocalReceivers() {
	h.svc.webhooks.SetClient(safehttp.New(safehttp.Options{AllowPrivate: true, AllowHTTP: true, Timeout: 2 * time.Second}))
}

// createEndpoint registers rc for the given events and teaches it the secret.
func (h *harness) createEndpoint(token string, rc *hookReceiver, events ...string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/webhook-endpoints", token: token, body: map[string]any{
		"url": rc.srv.URL + "/hooks/messora", "description": "CRM", "event_types": events}})
	h.expect(r, 201, "")
	secret := r.data()["secret"].(string)
	ep := r.data()["endpoint"].(map[string]any)
	if !strings.HasPrefix(secret, "whsec_") || ep["secret_last4"] != secret[len(secret)-4:] {
		h.t.Fatalf("secret %q / %v", secret, ep["secret_last4"])
	}
	rc.mu.Lock()
	rc.secret = secret
	rc.mu.Unlock()
	return ep
}

// pumpWebhooks plays the worker for webhook tasks that are due and returns
// how many ran.
func (h *harness) pumpWebhooks() int {
	h.t.Helper()
	ctx := h.ctx()
	total := 0
	for range 20 {
		rows, err := h.pool.Query(ctx, `UPDATE outbox_events SET published_at = now()
			WHERE id IN (SELECT id FROM outbox_events WHERE published_at IS NULL AND available_at <= now()
			             AND topic LIKE 'task:webhook:%' ORDER BY id FOR UPDATE SKIP LOCKED)
			RETURNING topic, payload`)
		if err != nil {
			h.t.Fatal(err)
		}
		type task struct {
			topic   string
			payload map[string]string
		}
		var tasks []task
		for rows.Next() {
			var tk task
			if err := rows.Scan(&tk.topic, &tk.payload); err != nil {
				h.t.Fatal(err)
			}
			tasks = append(tasks, tk)
		}
		rows.Close()
		if len(tasks) == 0 {
			break
		}
		for _, tk := range tasks {
			var err error
			switch tk.topic {
			case "task:" + webhooks.TypeFanOut:
				_, err = h.svc.webhooks.FanOut(ctx, uuid.MustParse(tk.payload["event_id"]))
			case "task:" + webhooks.TypeDeliver:
				err = h.svc.webhooks.Deliver(ctx, uuid.MustParse(tk.payload["delivery_id"]))
			}
			if err != nil {
				h.t.Fatalf("%s: %v", tk.topic, err)
			}
		}
		total += len(tasks)
	}
	return total
}

// timeTravel makes scheduled webhook retries due now.
func (h *harness) timeTravel() {
	h.t.Helper()
	ctx := h.ctx()
	if _, err := h.pool.Exec(ctx, "UPDATE outbox_events SET available_at = now() WHERE published_at IS NULL AND topic LIKE 'task:webhook:%'"); err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE webhook_deliveries SET next_attempt_at = now() WHERE status = 'retrying'"); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) deliveries(token, endpoint string) []map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + endpoint + "/deliveries", token: token})
	h.expect(r, 200, "")
	out := make([]map[string]any, len(r.list()))
	for i, d := range r.list() {
		out[i] = d.(map[string]any)
	}
	return out
}

func TestWebhookEndpointManagement(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()

	r := h.do(call{method: "GET", path: "/api/v1/webhook-event-types", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != len(webhooks.Catalogue) {
		t.Fatalf("catalogue: %s", r.raw)
	}

	// SSRF protection at create time: the default client refuses local,
	// metadata and plain-http destinations.
	for _, u := range []string{"https://127.0.0.1/hook", "https://169.254.169.254/latest/meta-data/", "https://[::1]/hook",
		"https://2130706433/hook", "https://localhost/hook", "http://hooks.example.com/hook"} {
		r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints", token: owner, body: map[string]any{
			"url": u, "event_types": []string{"message.received"}}})
		h.expect(r, 422, "VALIDATION_ERROR")
		if !strings.Contains(string(r.raw), `"url"`) {
			t.Fatalf("%s: error should name the url field: %s", u, r.raw)
		}
	}
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints", token: owner, body: map[string]any{
		"url": "https://hooks.example.com/x", "event_types": []string{"message.deleted"}}})
	h.expect(r, 422, "")

	h.allowLocalReceivers()
	rc := newHookReceiver(t)
	ep := h.createEndpoint(owner, rc, "message.received", "contact.created")
	id := ep["id"].(string)
	if ep["active"] != true || ep["description"] != "CRM" {
		t.Fatalf("endpoint: %v", ep)
	}

	r = h.do(call{method: "GET", path: "/api/v1/webhook-endpoints", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 1 || strings.Contains(string(r.raw), "whsec_") {
		t.Fatalf("list must not reveal the secret: %s", r.raw)
	}
	r = h.do(call{method: "PATCH", path: "/api/v1/webhook-endpoints/" + id, token: owner, body: map[string]any{
		"description": "CRM sync", "event_types": []string{"conversation.closed", "message.status"}, "active": false}})
	h.expect(r, 200, "")
	if r.data()["active"] != false || len(r.data()["event_types"].([]any)) != 2 || r.data()["event_types"].([]any)[0] != "conversation.closed" {
		t.Fatalf("update: %s", r.raw)
	}

	before := rc.secret
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/rotate-secret", token: owner})
	h.expect(r, 200, "")
	if s := r.data()["secret"].(string); s == before || !strings.HasPrefix(s, "whsec_") {
		t.Fatal("rotation must issue a new secret")
	}
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action LIKE 'webhook.%'"); n != 3 {
		t.Fatalf("audit entries: %d", n)
	}
	if n := h.count("SELECT count(*) FROM audit_logs WHERE changes::text LIKE '%whsec_%'"); n != 0 {
		t.Fatal("a secret reached the audit log")
	}
	var sealed []byte
	if err := h.pool.QueryRow(h.ctx(), "SELECT secret_enc FROM webhook_endpoints WHERE id = $1", id).Scan(&sealed); err != nil ||
		strings.Contains(string(sealed), "whsec_") {
		t.Fatal("the secret must be stored encrypted")
	}

	// Only webhooks.manage may manage endpoints.
	agent, _ := h.invite(owner, "agent@example.com", "agent")
	h.expect(h.do(call{method: "GET", path: "/api/v1/webhook-endpoints", token: agent}), 403, "FORBIDDEN")

	h.expect(h.do(call{method: "DELETE", path: "/api/v1/webhook-endpoints/" + id, token: owner}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + id, token: owner}), 404, "NOT_FOUND")
}

func TestWebhookDeliveryLifecycle(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	h.allowLocalReceivers()
	rc := newHookReceiver(t)
	ep := h.createEndpoint(owner, rc, "message.received", "conversation.closed", "contact.created", "contact.opted_out")
	id := ep["id"].(string)

	// An inbound message emits message.received in the transaction that
	// stores it; the worker signs and delivers it.
	h.inbound(testCustomer, "wamid.hook1", "Where is my order?", time.Now())
	if n := h.count("SELECT count(*) FROM outgoing_webhook_events WHERE type = 'message.received'"); n != 1 {
		t.Fatalf("events: %d", n)
	}
	if h.pumpWebhooks() == 0 {
		t.Fatal("nothing to deliver")
	}
	got := rc.last()
	if !got.valid {
		t.Fatal("signature does not verify")
	}
	data := got.env["data"].(map[string]any)
	if got.env["type"] != "message.received" || got.env["version"] != float64(1) || data["text"] != "Where is my order?" ||
		data["contact"].(map[string]any)["phone"] != "+"+testCustomer || data["conversation_id"] == nil {
		t.Fatalf("payload: %v", got.env)
	}
	if got.header.Get(webhooks.HeaderEventID) != got.env["id"] || got.header.Get(webhooks.HeaderEventType) != "message.received" ||
		got.header.Get(webhooks.HeaderAttempt) != "1" || !strings.HasPrefix(got.header.Get("User-Agent"), "Messora-Webhooks") {
		t.Fatalf("headers: %v", got.header)
	}
	ds := h.deliveries(owner, id)
	if len(ds) != 1 || ds[0]["status"] != "succeeded" || ds[0]["last_status_code"] != float64(200) || ds[0]["attempts"] != float64(1) ||
		ds[0]["last_response_body"] != `{"ok":true}` || ds[0]["next_attempt_at"] != nil {
		t.Fatalf("delivery log: %v", ds)
	}

	// 500 is retried later with the same event ID and a fresh signature.
	rc.program(500)
	h.inbound(testCustomer, "wamid.hook2", "Hello?", time.Now())
	h.pumpWebhooks()
	ds = h.deliveries(owner, id)
	if ds[0]["status"] != "retrying" || ds[0]["last_status_code"] != float64(500) || ds[0]["next_attempt_at"] == nil {
		t.Fatalf("after 500: %v", ds[0])
	}
	if h.pumpWebhooks() != 0 {
		t.Fatal("the retry must wait for its backoff")
	}
	h.timeTravel()
	h.pumpWebhooks()
	ds = h.deliveries(owner, id)
	if ds[0]["status"] != "succeeded" || ds[0]["attempts"] != float64(2) {
		t.Fatalf("after retry: %v", ds[0])
	}
	reqs := rc.requests()
	a, b := reqs[len(reqs)-2], reqs[len(reqs)-1]
	if a.header.Get(webhooks.HeaderEventID) != b.header.Get(webhooks.HeaderEventID) || !b.valid || b.header.Get(webhooks.HeaderAttempt) != "2" {
		t.Fatal("a retry carries the same event ID, attempt 2 and a valid signature")
	}

	// Manual redelivery of a succeeded delivery.
	r := h.do(call{method: "POST", path: "/api/v1/webhook-deliveries/" + ds[0]["id"].(string) + "/redeliver", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "pending" || r.data()["attempts"] != float64(0) {
		t.Fatalf("redeliver: %s", r.raw)
	}
	n := len(rc.requests())
	h.pumpWebhooks()
	if len(rc.requests()) != n+1 || rc.last().header.Get(webhooks.HeaderEventID) != b.header.Get(webhooks.HeaderEventID) {
		t.Fatal("redelivery must resend the same event")
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-deliveries/" + uuid.NewString() + "/redeliver", token: owner}), 404, "NOT_FOUND")

	// Conversation, contact and test events.
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + h.conversationOf(testCustomer) + "/close", token: owner}), 200, "")
	r = h.do(call{method: "POST", path: "/api/v1/contacts", token: owner, body: map[string]any{"phone": "+905550001122", "name": "Zeynep"}})
	h.expect(r, 201, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + r.data()["id"].(string) + "/consent", token: owner,
		body: map[string]any{"kind": "opt_out", "note": "asked by phone"}}), 200, "")
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/test", token: owner})
	h.expect(r, 201, "")
	if r.data()["event_type"] != "webhook.test" || r.data()["status"] != "pending" {
		t.Fatalf("test event: %s", r.raw)
	}
	h.pumpWebhooks()
	types := map[string]map[string]any{}
	for _, q := range rc.requests() {
		if !q.valid {
			t.Fatalf("invalid signature on %v", q.env["type"])
		}
		types[q.env["type"].(string)] = q.env["data"].(map[string]any)
	}
	if types["conversation.closed"]["status"] != "closed" || types["conversation.closed"]["actor"].(map[string]any)["type"] != "user" {
		t.Fatalf("conversation.closed: %v", types["conversation.closed"])
	}
	if types["contact.created"]["name"] != "Zeynep" || types["contact.opted_out"]["opted_out"] != true || types["webhook.test"] == nil {
		t.Fatalf("events: %v", types)
	}

	// 410 Gone switches the endpoint off; afterwards nothing is emitted.
	rc.program(410)
	h.inbound(testCustomer, "wamid.hook3", "Bye", time.Now())
	h.pumpWebhooks()
	r = h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + id, token: owner})
	h.expect(r, 200, "")
	if r.data()["active"] != false || r.data()["disabled_reason"] == nil {
		t.Fatalf("410 must disable the endpoint: %s", r.raw)
	}
	if ds = h.deliveries(owner, id); ds[0]["status"] != "failed" || ds[0]["last_status_code"] != float64(410) {
		t.Fatalf("410 delivery: %v", ds[0])
	}
	events := h.count("SELECT count(*) FROM outgoing_webhook_events")
	h.inbound(testCustomer, "wamid.hook4", "Anyone?", time.Now())
	if h.count("SELECT count(*) FROM outgoing_webhook_events") != events {
		t.Fatal("no event may be recorded without an active subscriber")
	}
	// Re-activating clears the reason.
	r = h.do(call{method: "PATCH", path: "/api/v1/webhook-endpoints/" + id, token: owner, body: map[string]any{"active": true}})
	h.expect(r, 200, "")
	if r.data()["disabled_reason"] != nil {
		t.Fatalf("reactivate: %s", r.raw)
	}
	// Filter by status.
	r = h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + id + "/deliveries?status=failed&limit=1", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("status filter: %s", r.raw)
	}
}

// The outbox delivers at least once: fan-out and delivery may run twice for
// the same event without duplicating anything.
func TestWebhookFanOutAndDeliveryAreIdempotent(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.allowLocalReceivers()
	rc1, rc2 := newHookReceiver(t), newHookReceiver(t)
	h.createEndpoint(owner, rc1, "contact.created")
	h.createEndpoint(owner, rc2, "contact.created", "contact.updated")
	rc3 := newHookReceiver(t)
	h.createEndpoint(owner, rc3, "contact.updated") // not subscribed

	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts", token: owner, body: map[string]any{"phone": "+905550001133"}}), 201, "")
	var eventID uuid.UUID
	if err := h.pool.QueryRow(h.ctx(), "SELECT id FROM outgoing_webhook_events WHERE type = 'contact.created'").Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	ctx := h.ctx()
	for i, want := range []int{2, 0} {
		n, err := h.svc.webhooks.FanOut(ctx, eventID)
		if err != nil || n != want {
			t.Fatalf("fan-out run %d: %d %v", i+1, n, err)
		}
	}
	if n := h.count("SELECT count(*) FROM webhook_deliveries WHERE event_id = $1", eventID); n != 2 {
		t.Fatalf("deliveries: %d", n)
	}
	h.pumpWebhooks() // also runs the relayed fan-out task: still two deliveries
	if n := h.count("SELECT count(*) FROM webhook_deliveries"); n != 2 {
		t.Fatalf("deliveries after pump: %d", n)
	}
	if len(rc1.requests()) != 1 || len(rc2.requests()) != 1 || len(rc3.requests()) != 0 {
		t.Fatalf("requests: %d %d %d", len(rc1.requests()), len(rc2.requests()), len(rc3.requests()))
	}
	// A duplicate delivery task is a no-op.
	var deliveryID uuid.UUID
	if err := h.pool.QueryRow(ctx, "SELECT id FROM webhook_deliveries LIMIT 1").Scan(&deliveryID); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.webhooks.Deliver(ctx, deliveryID); err != nil {
		t.Fatal(err)
	}
	if len(rc1.requests())+len(rc2.requests()) != 2 {
		t.Fatal("a finished delivery was sent again")
	}
	// A delivery held by a live worker is deferred, then taken over once
	// the lease expires.
	if _, err := h.pool.Exec(ctx, "UPDATE webhook_deliveries SET status = 'delivering', locked_until = now() + interval '30 seconds' WHERE id = $1", deliveryID); err != nil {
		t.Fatal(err)
	}
	var ra *queue.RetryAfterError
	if err := h.svc.webhooks.Deliver(ctx, deliveryID); !errors.As(err, &ra) {
		t.Fatalf("leased delivery: %v", err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE webhook_deliveries SET locked_until = now() - interval '1 second' WHERE id = $1", deliveryID); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.webhooks.Deliver(ctx, deliveryID); err != nil {
		t.Fatal(err)
	}
	if len(rc1.requests())+len(rc2.requests()) != 3 {
		t.Fatal("an abandoned delivery must be taken over")
	}
}

// Emitted events commit and roll back with the change that caused them.
func TestWebhookEventsAreTransactional(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.allowLocalReceivers()
	h.createEndpoint(owner, newHookReceiver(t), "campaign.completed")
	ctx := h.ctx()
	c := db.Campaign{ID: uuid.New(), Name: "Launch", Status: "completed", TotalRecipients: 3}

	emitIn := func(commit bool) {
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := webhooks.CampaignFinished(ctx, tx, c); err != nil {
			t.Fatal(err)
		}
		if commit {
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	emitIn(false)
	if h.count("SELECT count(*) FROM outgoing_webhook_events")+h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:webhook:fanout'") != 0 {
		t.Fatal("a rolled back change must not emit")
	}
	emitIn(true)
	if h.count("SELECT count(*) FROM outgoing_webhook_events WHERE type = 'campaign.completed'") != 1 ||
		h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:webhook:fanout'") != 1 {
		t.Fatal("a committed change emits exactly once")
	}
	// Non-terminal states emit nothing.
	tx, _ := h.pool.Begin(ctx)
	running := c
	running.Status = "running"
	if err := webhooks.CampaignFinished(ctx, tx, running); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit(ctx)
	if h.count("SELECT count(*) FROM outgoing_webhook_events") != 1 {
		t.Fatal("running is not a webhook event")
	}
}

// SSRF protection at delivery time: an endpoint that resolves to a private
// address now (DNS changed, or it was stored before the rule) is refused
// without a request and without retries.
func TestWebhookDeliveryRefusesPrivateDestinations(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.allowLocalReceivers()
	rc := newHookReceiver(t)
	ep := h.createEndpoint(owner, rc, "contact.created")

	h.svc.webhooks.SetClient(webhooks.NewClient(true)) // production rules (http allowed)
	r := h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + ep["id"].(string) + "/test", token: owner})
	h.expect(r, 201, "")
	h.pumpWebhooks()
	if len(rc.requests()) != 0 {
		t.Fatal("a loopback receiver was reached")
	}
	ds := h.deliveries(owner, ep["id"].(string))
	if ds[0]["status"] != "failed" || !strings.Contains(ds[0]["last_error"].(string), "loopback") {
		t.Fatalf("delivery: %v", ds[0])
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND topic LIKE 'task:webhook:%'"); n != 0 {
		t.Fatal("a refused destination must not be retried")
	}
	// Retention removes old events with their deliveries.
	if _, err := h.pool.Exec(h.ctx(), "UPDATE outgoing_webhook_events SET created_at = now() - interval '40 days'"); err != nil {
		t.Fatal(err)
	}
	if n, err := h.svc.webhooks.Cleanup(h.ctx(), webhooks.Retention); err != nil || n != 1 {
		t.Fatalf("cleanup: %d %v", n, err)
	}
	if h.count("SELECT count(*) FROM webhook_deliveries") != 0 {
		t.Fatal("deliveries go with their event")
	}
}
