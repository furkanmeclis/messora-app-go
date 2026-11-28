//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/webhooks"
	"github.com/furkanmeclis/messora-app-go/internal/platform/apiguard"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// docAt fetches the public API document as a direct caller at host.
func (h *harness) docAt(host string) map[string]any {
	h.t.Helper()
	req, _ := http.NewRequest("GET", h.srv.URL+"/api/v1/openapi.json", nil)
	req.Host = host
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		h.t.Fatalf("openapi.json at %s: %d %s", host, res.StatusCode, b)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		h.t.Fatal(err)
	}
	return d
}

func serverURLs(d map[string]any) []string {
	var out []string
	for _, s := range d["servers"].([]any) {
		out = append(out, s.(map[string]any)["url"].(string))
	}
	return out
}

// apiKey creates an API key with scopes and returns its secret.
func (h *harness) apiKey(owner string, scopes ...string) string {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: map[string]any{"name": "key " + uuid.NewString()[:8], "scopes": scopes}})
	h.expect(r, 201, "")
	return r.data()["secret"].(string)
}

// planFor gives the tenants in plans their plan, everyone else the default.
func (h *harness) planFor(plans map[uuid.UUID]entitlements.Plan) {
	h.svc.plans = entitlements.ProviderFunc(func(_ context.Context, id uuid.UUID) (entitlements.Plan, error) {
		if p, ok := plans[id]; ok {
			return p, nil
		}
		return entitlements.Default(), nil
	})
}

// Every workspace's address serves the public document with its own API
// address as the server (and in the introduction), a custom domain lists
// itself next to the primary address, and the platform address shows the
// per-workspace pattern. The platform panel's operations are never in it.
func TestPerWorkspaceAPIDocs(t *testing.T) {
	h := newHarness(t)
	hb := h.tenant("beta")
	if _, err := h.owner.Exec(context.Background(), `UPDATE tenants SET name = 'Beta Gıda' WHERE id = $1`, hb.tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.owner.Exec(context.Background(), `INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at)
		VALUES ($1, 'api.beta-gida.test', 'custom', now())`, hb.tenantID); err != nil {
		t.Fatal(err)
	}

	a := h.docAt(h.host)
	if got := serverURLs(a); len(got) != 1 || got[0] != "https://messora.test/api/v1" {
		t.Fatalf("A's servers: %v", got)
	}
	b := h.docAt(hb.host)
	if got := serverURLs(b); len(got) != 1 || got[0] != "https://beta.messora.test/api/v1" {
		t.Fatalf("B's servers: %v", got)
	}
	desc := b["info"].(map[string]any)["description"].(string)
	if !strings.Contains(desc, "**Beta Gıda** workspace") || !strings.Contains(desc, "curl https://beta.messora.test/api/v1/api-keys/whoami") ||
		strings.Contains(desc, "https://messora.test/api/v1") {
		t.Fatalf("B's introduction:\n%s", desc)
	}
	custom := h.docAt("api.beta-gida.test")
	if got := serverURLs(custom); len(got) != 2 || got[0] != "https://api.beta-gida.test/api/v1" || got[1] != "https://beta.messora.test/api/v1" {
		t.Fatalf("custom domain servers: %v", got)
	}
	generic := h.docAt("app." + testBaseDomain)
	if got := serverURLs(generic); len(got) != 1 || got[0] != "https://{workspace}."+testBaseDomain+"/api/v1" {
		t.Fatalf("platform host servers: %v", got)
	}
	if got := serverURLs(h.docAt("unknown.example")); got[0] != "https://{workspace}."+testBaseDomain+"/api/v1" {
		t.Fatalf("unknown host servers: %v", got)
	}
	for _, d := range []map[string]any{a, b, custom, generic} {
		for p := range d["paths"].(map[string]any) {
			if strings.HasPrefix(p, "/platform/") {
				t.Errorf("public document lists %s", p)
			}
		}
	}

	// A workspace's API key answers at its own address only: elsewhere it
	// is unknown (401), and the platform addresses serve no workspace API.
	owner, _ := h.setupOwner()
	// The panel shows the workspace's own API base URL.
	me := h.do(call{method: "GET", path: "/api/v1/me", token: owner}).data()["tenant"].(map[string]any)
	if me["api_base_url"] != "https://messora.test/api/v1" || me["url"] != "https://messora.test" {
		t.Fatalf("me.tenant: %v", me)
	}
	ownerB := hb.setupOwnerAs("owner@beta.test")
	if b := hb.do(call{method: "GET", path: "/api/v1/me", token: ownerB}).data()["tenant"].(map[string]any); b["api_base_url"] != "https://beta.messora.test/api/v1" {
		t.Fatalf("B's me.tenant: %v", b)
	}
	key := h.apiKey(owner, "contacts.read")
	h.expect(h.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: key, noBFF: true}), 200, "")
	if code, e := h.rawAt(hb.host, "GET", "/api/v1/api-keys/whoami", key, false); code != 401 || e != "API_KEY_INVALID" {
		t.Errorf("A's key at B: %d %s", code, e)
	}
	if code, e := h.rawAt("api.beta-gida.test", "GET", "/api/v1/contacts", key, false); code != 401 || e != "API_KEY_INVALID" {
		t.Errorf("A's key at B's custom domain: %d %s", code, e)
	}
	for _, host := range []string{"app." + testBaseDomain, "admin." + testBaseDomain, testBaseDomain} {
		if code, _ := h.rawAt(host, "GET", "/api/v1/contacts", key, false); code != 404 {
			t.Errorf("A's key at %s: %d", host, code)
		}
	}
}

// Plan hooks (fake provider): request budgets per minute, the monthly API
// quota, feature flags and counts, each for one tenant only.
func TestPlanEnforcement(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	hb := h.tenant("beta")
	ownerB := hb.setupOwnerAs("owner@beta.test")
	keyB := hb.apiKey(ownerB, "contacts.read")

	// Request budget per key: 2 per minute.
	tight := entitlements.Default()
	tight.APIKeyRequestsPerMinute = 2
	key := h.apiKey(owner, "contacts.read", "webhooks.manage")
	h.planFor(map[uuid.UUID]entitlements.Plan{h.tenantID: tight})
	for range 2 {
		h.expect(h.do(call{method: "GET", path: "/api/v1/contacts", token: key, noBFF: true}), 200, "")
	}
	r := h.do(call{method: "GET", path: "/api/v1/contacts", token: key, noBFF: true})
	h.expect(r, 429, "RATE_LIMITED")
	if r.header.Get("RateLimit-Limit") != "2" || r.header.Get("RateLimit-Policy") != "2;w=60" || r.header.Get("Retry-After") == "" {
		t.Fatalf("429 headers: %v", r.header)
	}
	// B's key is untouched by A's plan.
	for range 3 {
		hb.expect(hb.do(call{method: "GET", path: "/api/v1/contacts", token: keyB, noBFF: true}), 200, "")
	}

	// Monthly quota of all keys: 3 calls. Every key call is counted, even on
	// a plan without a quota, so start the month's counter afresh.
	if err := h.rdb.Del(h.ctx(), apiguard.QuotaKey(h.tenantID, time.Now())).Err(); err != nil {
		t.Fatal(err)
	}
	quota := entitlements.Default()
	quota.APICallsPerMonth = 3
	h.planFor(map[uuid.UUID]entitlements.Plan{h.tenantID: quota})
	k1, k2 := h.apiKey(owner, "contacts.read"), h.apiKey(owner, "contacts.read")
	for i, k := range []string{k1, k2, k1} {
		r := h.do(call{method: "GET", path: "/api/v1/contacts", token: k, noBFF: true})
		h.expect(r, 200, "")
		if r.header.Get("X-Messora-Quota-Limit") != "3" || r.header.Get("X-Messora-Quota-Remaining") != strconv.Itoa(2-i) {
			t.Fatalf("call %d quota headers: %v", i, r.header)
		}
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts", token: k2, noBFF: true})
	h.expect(r, 429, entitlements.CodeQuotaExceeded)
	ra, _ := strconv.Atoi(r.header.Get("Retry-After"))
	if ra <= 0 || ra > 31*24*3600 || r.header.Get("RateLimit-Remaining") != "0" || !strings.Contains(string(r.raw), "api_calls_per_month") {
		t.Fatalf("quota 429: %v %s", r.header, r.raw)
	}
	// Panel sessions are not metered by the monthly quota.
	h.expect(h.do(call{method: "GET", path: "/api/v1/contacts", token: owner}), 200, "")

	// public_api off: every key call and creating keys is refused, with the
	// feature in the details; the panel keeps working.
	noAPI := entitlements.Default()
	noAPI.Features = map[entitlements.Feature]bool{entitlements.Webhooks: true}
	h.planFor(map[uuid.UUID]entitlements.Plan{h.tenantID: noAPI})
	r = h.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: k1, noBFF: true})
	h.expect(r, 403, entitlements.CodeFeatureNotInPlan)
	if !strings.Contains(string(r.raw), `"field":"feature"`) || !strings.Contains(string(r.raw), `"code":"public_api"`) {
		t.Fatalf("feature details: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/api-keys", token: owner, body: map[string]any{"name": "x", "scopes": []string{"contacts.read"}}}),
		403, entitlements.CodeFeatureNotInPlan)
	h.expect(h.do(call{method: "GET", path: "/api/v1/contacts", token: owner}), 200, "")
	hb.expect(hb.do(call{method: "GET", path: "/api/v1/api-keys/whoami", token: keyB, noBFF: true}), 200, "")

	// webhooks off: registering, testing and rotating are refused; listing
	// and deleting still work.
	h.allowLocalReceivers()
	rc := newHookReceiver(t)
	ep := h.createEndpoint(owner, rc, "contact.created")
	noHooks := entitlements.Default()
	noHooks.Features = map[entitlements.Feature]bool{entitlements.PublicAPI: true}
	h.planFor(map[uuid.UUID]entitlements.Plan{h.tenantID: noHooks})
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints", token: owner, body: map[string]any{
		"url": rc.srv.URL + "/x", "event_types": []string{"contact.created"}}})
	h.expect(r, 403, entitlements.CodeFeatureNotInPlan)
	if !strings.Contains(string(r.raw), `"code":"webhooks"`) {
		t.Fatalf("webhooks details: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + ep["id"].(string) + "/test", token: owner}), 403, entitlements.CodeFeatureNotInPlan)
	h.expect(h.do(call{method: "GET", path: "/api/v1/webhook-endpoints", token: owner}), 200, "")
	// No deliveries are created while the plan lacks webhooks.
	h.created(owner, "/api/v1/contacts", map[string]any{"phone": "+905550001122", "name": "No hook"})
	h.pumpWebhooks()
	if n := h.count("SELECT count(*) FROM webhook_deliveries"); n != 0 {
		t.Fatalf("%d deliveries without the webhooks feature", n)
	}

	// Endpoint count: 1 allowed, the second is 409 PLAN_LIMIT_REACHED.
	one := entitlements.Default()
	one.WebhookEndpoints = 1
	h.planFor(map[uuid.UUID]entitlements.Plan{h.tenantID: one})
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints", token: owner, body: map[string]any{
		"url": rc.srv.URL + "/second", "event_types": []string{"contact.created"}}})
	h.expect(r, 409, entitlements.CodePlanLimitReached)
	if !strings.Contains(string(r.raw), `"code":"webhook_endpoints"`) || !strings.Contains(string(r.raw), `"message":"1"`) {
		t.Fatalf("limit details: %s", r.raw)
	}
	// B, on the default plan, registers as many as it likes.
	hb.allowLocalReceivers()
	for range 2 {
		hb.createEndpoint(ownerB, newHookReceiver(t), "contact.created")
	}
}

// Outgoing webhook health: failed attempts in a row switch an endpoint off
// (rule shortened for the test), the owners and admins get the branded
// e-mail once, a manual re-enable starts over, and a success resets the
// count.
func TestWebhookAutoDisableAndReEnable(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.allowLocalReceivers()
	h.svc.webhooks.SetDisableRule(3, -1)
	rc := newHookReceiver(t)
	id := h.createEndpoint(owner, rc, "contact.created")["id"].(string)

	// A success, then a failure, then a success: the count resets.
	rc.program(200, 500, 200)
	h.created(owner, "/api/v1/contacts", map[string]any{"phone": "+905550000001", "name": "One"})
	h.pumpWebhooks()
	h.created(owner, "/api/v1/contacts", map[string]any{"phone": "+905550000002", "name": "Two"})
	h.pumpWebhooks()
	ep := h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + id, token: owner}).data()
	if ep["failure_streak"] != float64(1) || ep["failing_since"] == nil || ep["last_success_at"] == nil {
		t.Fatalf("after a failure: %v", ep)
	}
	h.timeTravel()
	h.pumpWebhooks()
	ep = h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + id, token: owner}).data()
	if ep["failure_streak"] != float64(0) || ep["failing_since"] != nil {
		t.Fatalf("a success must reset the count: %v", ep)
	}

	// Three failures in a row switch it off.
	rc.program(503, 503, 503, 503)
	h.created(owner, "/api/v1/contacts", map[string]any{"phone": "+905550000003", "name": "Three"})
	h.pumpWebhooks()
	for range 2 {
		h.timeTravel()
		h.pumpWebhooks()
	}
	ep = h.do(call{method: "GET", path: "/api/v1/webhook-endpoints/" + id, token: owner}).data()
	if ep["active"] != false || ep["disabled_at"] == nil || !strings.Contains(ep["disabled_reason"].(string), "3 delivery attempts in a row failed") {
		t.Fatalf("not switched off: %v", ep)
	}
	if n := h.countMailTo("owner@example.com", "webhook"); n != 1 {
		t.Fatalf("%d e-mails to the owner", n)
	}
	m, _ := h.mailTo("owner@example.com")
	if !strings.Contains(m.Body, rc.srv.URL) || !strings.Contains(m.Body, "/settings/webhooks") || m.HTML == "" {
		t.Fatalf("e-mail: %s", m.Body)
	}
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action = 'webhook.disable'"); n != 1 {
		t.Fatalf("audit: %d", n)
	}
	// Switched off: the retry fails without a request, no second e-mail.
	before := len(rc.requests())
	h.timeTravel()
	h.pumpWebhooks()
	if len(rc.requests()) != before || h.countMailTo("owner@example.com", "webhook") != 1 {
		t.Fatal("a disabled endpoint was called or reported twice")
	}
	// The test event still reaches it (check the receiver before turning it on).
	rc.program()
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/test", token: owner}), 201, "")
	h.pumpWebhooks()
	if last := rc.last(); last.env["type"] != webhooks.TypeTest || !last.valid {
		t.Fatalf("test delivery: %+v", last)
	}

	// Manual re-enable starts over.
	r := h.do(call{method: "PATCH", path: "/api/v1/webhook-endpoints/" + id, token: owner, body: map[string]any{"active": true}})
	h.expect(r, 200, "")
	if d := r.data(); d["active"] != true || d["disabled_reason"] != nil || d["disabled_at"] != nil || d["failure_streak"] != float64(0) {
		t.Fatalf("re-enabled: %v", d)
	}
	h.created(owner, "/api/v1/contacts", map[string]any{"phone": "+905550000004", "name": "Four"})
	h.pumpWebhooks()
	if last := rc.last(); last.env["type"] != webhooks.TypeContactCreated || !last.valid {
		t.Fatalf("after re-enable: %+v", last)
	}
}

// Secret rotation with overlap: during the grace period deliveries carry a
// signature with the old and the new secret, so a receiver on either
// accepts them; without grace the old secret stops at once.
func TestWebhookSecretRotationOverlap(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.allowLocalReceivers()
	rc := newHookReceiver(t)
	id := h.createEndpoint(owner, rc, "contact.created")["id"].(string)
	old := rc.secret

	r := h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/rotate-secret", token: owner, body: map[string]any{"grace_hours": 24}})
	h.expect(r, 200, "")
	fresh := r.data()["secret"].(string)
	until, _ := time.Parse(time.RFC3339, r.data()["endpoint"].(map[string]any)["previous_secret_expires_at"].(string))
	if fresh == old || time.Until(until) < 23*time.Hour {
		t.Fatalf("rotation: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/test", token: owner}), 201, "")
	h.pumpWebhooks()
	last := rc.last()
	sig := last.header.Get(webhooks.HeaderSignature)
	if strings.Count(sig, "v1=") != 2 || !last.valid { // the receiver still holds the old secret
		t.Fatalf("overlap signature %q valid=%v", sig, last.valid)
	}
	rc.mu.Lock()
	rc.secret = fresh
	rc.mu.Unlock()
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/test", token: owner}), 201, "")
	h.pumpWebhooks()
	if !rc.last().valid {
		t.Fatal("the new secret must verify during the overlap")
	}

	// Out of range is a validation error; grace 0 retires the old one.
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/rotate-secret", token: owner, body: map[string]any{"grace_hours": 500}}), 422, "")
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/rotate-secret", token: owner, body: map[string]any{"grace_hours": 0}})
	h.expect(r, 200, "")
	if r.data()["endpoint"].(map[string]any)["previous_secret_expires_at"] != nil {
		t.Fatalf("no overlap expected: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/test", token: owner}), 201, "")
	h.pumpWebhooks()
	last = rc.last()
	if strings.Count(last.header.Get(webhooks.HeaderSignature), "v1=") != 1 || last.valid { // receiver holds the previous secret
		t.Fatal("the retired secret still verifies")
	}
	// Without a body the overlap is 24 hours.
	r = h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/rotate-secret", token: owner})
	h.expect(r, 200, "")
	if r.data()["endpoint"].(map[string]any)["previous_secret_expires_at"] == nil {
		t.Fatal("default overlap missing")
	}
}

// Delivery budgets: one endpoint has at most a few requests in flight per
// worker (one while it is failing), the tenant's deliveries per minute come
// from its plan, and a deferral costs no attempt. Another tenant's
// deliveries are not held back by either.
func TestWebhookDeliveryBudgets(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.allowLocalReceivers()
	hb := h.tenant("beta")
	ownerB := hb.setupOwnerAs("owner@beta.test")

	// A receiver that holds requests until released.
	release := make(chan struct{})
	var once sync.Once
	rc := newHookReceiver(t)
	arrived := make(chan struct{}, 4)
	rc.mu.Lock()
	rc.hold = func() {
		arrived <- struct{}{}
		<-release
	}
	rc.mu.Unlock()
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	id := h.createEndpoint(owner, rc, "contact.created")["id"].(string)
	// The endpoint is failing: one request at a time.
	if _, err := h.pool.Exec(h.ctx(), `UPDATE webhook_endpoints SET failure_streak = 5, failing_since = now() WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"+905550000011", "+905550000012"} {
		h.created(owner, "/api/v1/contacts", map[string]any{"phone": p, "name": "Budget"})
	}
	// Fan out without delivering.
	var events []uuid.UUID
	rows, _ := h.pool.Query(h.ctx(), `SELECT id FROM outgoing_webhook_events ORDER BY id`)
	for rows.Next() {
		var e uuid.UUID
		_ = rows.Scan(&e)
		events = append(events, e)
	}
	rows.Close()
	for _, e := range events {
		if _, err := h.svc.webhooks.FanOut(h.ctx(), e); err != nil {
			t.Fatal(err)
		}
	}
	var ds []uuid.UUID
	rows, _ = h.pool.Query(h.ctx(), `SELECT id FROM webhook_deliveries ORDER BY id`)
	for rows.Next() {
		var d uuid.UUID
		_ = rows.Scan(&d)
		ds = append(ds, d)
	}
	rows.Close()
	if len(ds) != 2 {
		t.Fatalf("deliveries: %v", ds)
	}
	done := make(chan error, 1)
	go func() { done <- h.svc.webhooks.Deliver(h.ctx(), ds[0]) }()
	<-arrived
	if n := h.svc.webhooks.InFlight(uuid.MustParse(id)); n != 1 {
		t.Fatalf("in flight: %d", n)
	}
	err := h.svc.webhooks.Deliver(h.ctx(), ds[1])
	var ra *queue.RetryAfterError
	if !errors.As(err, &ra) || queue.IsFailure(err) {
		t.Fatalf("second delivery to a failing endpoint: %v", err)
	}
	if n := h.count("SELECT attempts FROM webhook_deliveries WHERE id = $1", ds[1]); n != 0 {
		t.Fatalf("a deferral consumed an attempt: %d", n)
	}

	// Meanwhile tenant B's endpoint is delivered at once.
	hb.allowLocalReceivers()
	rcB := newHookReceiver(t)
	hb.createEndpoint(ownerB, rcB, "contact.created")
	hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+905550000013", "name": "Beta"})
	hb.pumpWebhooks()
	if len(rcB.requests()) != 1 {
		t.Fatal("B's delivery waited for A's endpoint")
	}
	once.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := h.svc.webhooks.Deliver(h.ctx(), ds[1]); err != nil {
		t.Fatalf("after the first finished: %v", err)
	}

	// Plan: one delivery per minute for A; B keeps its default.
	perMinute := entitlements.Default()
	perMinute.WebhookDeliveriesPerMinute = 1
	h.planFor(map[uuid.UUID]entitlements.Plan{h.tenantID: perMinute})
	rc.mu.Lock()
	rc.hold = nil
	rc.mu.Unlock()
	// Start the tenant's delivery budget from scratch at the new rate.
	if err := h.rdb.Del(context.Background(), tenant.RedisKey(h.ctx(), "rl:webhook:deliveries")).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx(), `UPDATE webhook_endpoints SET failure_streak = 0, failing_since = NULL`); err != nil {
		t.Fatal(err)
	}
	var tests []uuid.UUID
	for range 2 {
		r := h.do(call{method: "POST", path: "/api/v1/webhook-endpoints/" + id + "/test", token: owner})
		h.expect(r, 201, "")
		tests = append(tests, uuid.MustParse(r.data()["id"].(string)))
	}
	if err := h.svc.webhooks.Deliver(h.ctx(), tests[0]); err != nil {
		t.Fatal(err)
	}
	err = h.svc.webhooks.Deliver(h.ctx(), tests[1])
	if !errors.As(err, &ra) || ra.After <= 0 {
		t.Fatalf("over the plan's deliveries per minute: %v", err)
	}
	for i := range 2 {
		hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+90555000002" + strconv.Itoa(i), "name": "Beta"})
	}
	hb.pumpWebhooks()
	if len(rcB.requests()) != 3 {
		t.Fatalf("B's deliveries held back by A's plan: %d", len(rcB.requests()))
	}
}
