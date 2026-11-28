//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/channels"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// Messora's own Meta app (Tech Provider) in the tests.
const (
	tpAppID       = "7700112233"
	tpAppSecret   = "messora-tech-provider-app-secret-0001" // gitleaks:allow (test fixture)
	tpConfigID    = "990011223344"
	tpSystemToken = "EAAG-messora-system-user-token-000001" // gitleaks:allow (test fixture)
	tpCallback    = "https://hooks.messora.test/meta/tp"
)

// enableTechProvider configures and turns on the Tech Provider as the
// platform admin would (the owner role writes platform_settings, as the
// `messora tech-provider` command does).
func (h *harness) enableTechProvider() channels.TechProviderConfig {
	h.t.Helper()
	h.meta.SetApp(tpAppID, tpAppSecret)
	h.meta.SetTokenInfo(tpSystemToken, whatsapptest.TokenInfo{AppID: tpAppID, Scopes: channels.TechProviderScopes})
	on, id, conf, secret, tok := true, tpAppID, tpConfigID, tpAppSecret, tpSystemToken
	cfg, _, err := h.svc.channels.UpdateTechProvider(context.Background(), h.owner, channels.TechProviderPatch{
		Enabled: &on, AppID: &id, ConfigurationID: &conf, AppSecret: &secret, SystemUserToken: &tok})
	if err != nil {
		h.t.Fatal(err)
	}
	return cfg
}

// signup provisions an Embedded Signup at the fake Meta and returns its code.
func (h *harness) signup(waba, phone string) string {
	code, _ := h.meta.AddSignup(whatsapptest.Signup{AppID: tpAppID, WABA: waba, PhoneNumberID: phone, Display: "+90 555 " + phone, VerifiedName: "İşletme " + waba})
	return code
}

func (h *harness) onboard(token string, body map[string]any) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/channels/tech-provider/onboardings", token: token, body: body})
	h.expect(r, 200, "")
	return r.data()
}

func (h *harness) retryOnboarding(token, id string, body map[string]any) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/channels/tech-provider/onboardings/" + id + "/retry", token: token, body: body})
	h.expect(r, 200, "")
	return r.data()
}

func wantOnboarding(t *testing.T, o map[string]any, status, step, code string) {
	t.Helper()
	got := ""
	if c, ok := o["error_code"].(string); ok {
		got = c
	}
	if o["status"] != status || o["step"] != step || got != code {
		t.Fatalf("onboarding: status %v step %v code %q (%v), want %s %s %q", o["status"], o["step"], got, o["error_message"], status, step, code)
	}
}

// tpPost delivers a body to the platform webhook signed with secret.
func (h *harness) tpPost(secret string, body []byte) int {
	h.t.Helper()
	return h.post(h.srv.URL+channels.TechProviderWebhookPath, secret, body)
}

func entries(t *testing.T, bodies ...[]byte) []byte {
	t.Helper()
	var all []json.RawMessage
	for _, b := range bodies {
		var e struct {
			Entry []json.RawMessage `json:"entry"`
		}
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		all = append(all, e.Entry...)
	}
	out, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": all})
	return out
}

func (h *harness) route(waba string) (uuid.UUID, bool) {
	h.t.Helper()
	var t uuid.UUID
	err := h.owner.QueryRow(context.Background(), `SELECT tenant_id FROM meta_routes WHERE waba_id = $1`, waba).Scan(&t)
	if err != nil {
		return uuid.Nil, false
	}
	return t, true
}

func TestTechProviderIsOffByDefault(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	if h.metaConnection(owner)["tech_provider_enabled"] != false {
		t.Fatal("the Tech Provider is offered while off")
	}
	id := uuid.NewString()
	for _, c := range []call{
		{method: "GET", path: "/api/v1/channels/tech-provider"},
		{method: "POST", path: "/api/v1/channels/tech-provider/onboardings", body: map[string]any{"code": "x", "waba_id": "1", "phone_number_id": "2"}},
		{method: "POST", path: "/api/v1/channels/tech-provider/onboardings/" + id + "/retry", body: map[string]any{}},
		{method: "DELETE", path: "/api/v1/channels/tech-provider/onboardings/" + id},
		{method: "POST", path: "/api/v1/channels/accounts/" + id + "/tech-provider/disconnect"},
	} {
		c.token = owner
		h.expect(h.do(c), 404, "TECH_PROVIDER_DISABLED")
	}
	// The platform address does not exist while the method is off.
	for _, p := range []string{channels.TechProviderWebhookPath, "/webhooks" + channels.TechProviderWebhookPath} {
		if code, _ := handshake(t, h.srv.URL+p, "anything", ""); code != 404 {
			t.Errorf("GET %s while off: %d", p, code)
		}
		if code := h.post(h.srv.URL+p, tpAppSecret, []byte(`{"object":"whatsapp_business_account","entry":[]}`)); code != 404 {
			t.Errorf("POST %s while off: %d", p, code)
		}
	}
	// Configured but switched off: still nothing.
	h.enableTechProvider()
	off := false
	if _, _, err := h.svc.channels.UpdateTechProvider(context.Background(), h.owner, channels.TechProviderPatch{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/channels/tech-provider", token: owner}), 404, "TECH_PROVIDER_DISABLED")
	if code, _ := handshake(t, h.srv.URL+channels.TechProviderWebhookPath, "x", ""); code != 404 {
		t.Fatalf("webhook while switched off: %d", code)
	}
	// Turning it on needs a complete configuration.
	on, empty := true, ""
	if _, _, err := h.svc.channels.UpdateTechProvider(context.Background(), h.owner, channels.TechProviderPatch{Enabled: &on, ConfigurationID: &empty}); err == nil {
		t.Fatal("turned on without a configuration ID")
	}

	// The platform API is not reachable with a workspace session, and the
	// public API document never lists it.
	for _, c := range []call{
		{method: "GET", path: "/api/v1/platform/meta/tech-provider"},
		{method: "PATCH", path: "/api/v1/platform/meta/tech-provider", body: map[string]any{"enabled": true}},
		{method: "POST", path: "/api/v1/platform/meta/tech-provider/test"},
	} {
		c.token = owner
		if r := h.do(c); r.status != 401 && r.status != 403 {
			t.Errorf("%s %s with a workspace session: %d %s", c.method, c.path, r.status, r.raw)
		}
	}
	res, err := http.Get(h.srv.URL + "/api/v1/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if bytes.Contains(doc, []byte("/platform/")) || bytes.Contains(doc, []byte("tech-provider")) {
		t.Fatal("the public API document lists Tech Provider operations")
	}
}

func TestTechProviderPlatformSettings(t *testing.T) {
	h := newHarness(t)
	h.enableTechProvider()
	// Sealed: no secret is stored in plain text.
	var raw string
	if err := h.owner.QueryRow(context.Background(), `SELECT value::text FROM platform_settings WHERE key = 'tech_provider'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	cfg, err := h.svc.channels.TechProvider(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{tpAppSecret, tpSystemToken, cfg.VerifyToken} {
		if strings.Contains(raw, s) {
			t.Fatalf("platform_settings holds a secret in plain text: %s", raw)
		}
	}
	if !cfg.Ready() || len(cfg.VerifyToken) != channels.WebhookKeyLength {
		t.Fatalf("config: %+v", cfg)
	}
	// The application role cannot change it.
	writable := channels.NewHandler(h.svc.channels, channels.HandlerOptions{WebhookBaseURL: "https://hooks.messora.test", PlatformDB: h.owner}, testutil.DiscardLogger())
	readOnly := channels.NewHandler(h.svc.channels, channels.HandlerOptions{WebhookBaseURL: "https://hooks.messora.test"}, testutil.DiscardLogger())
	serve := func(hd *channels.Handler, method string, body string) (int, map[string]any) {
		req := httptest.NewRequest(method, "/api/v1/platform/meta/tech-provider", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(tenant.System(context.Background()))
		rec := httptest.NewRecorder()
		switch method {
		case http.MethodGet:
			hd.GetPlatformTechProvider(rec, req)
		case http.MethodPatch:
			hd.UpdatePlatformTechProvider(rec, req)
		default:
			hd.TestPlatformTechProvider(rec, req)
		}
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if s := rec.Body.String(); strings.Contains(s, tpAppSecret) || strings.Contains(s, tpSystemToken) {
			t.Fatalf("the platform API shows a secret: %s", s)
		}
		return rec.Code, out
	}
	code, out := serve(readOnly, http.MethodPatch, `{"configuration_id": "123"}`)
	if code != 403 || out["error"].(map[string]any)["code"] != "PLATFORM_WRITE_DENIED" {
		t.Fatalf("app role wrote platform settings: %d %v", code, out)
	}
	code, out = serve(writable, http.MethodGet, "")
	d, _ := out["data"].(map[string]any)
	if code != 200 || d["enabled"] != true || d["app_secret_configured"] != true || d["system_user_token_configured"] != true ||
		d["verify_token"] != cfg.VerifyToken || d["callback_url"] != tpCallback {
		t.Fatalf("GET: %d %v", code, out)
	}
	code, out = serve(writable, http.MethodPatch, `{"graph_api_version": "v99"}`)
	if code != 422 {
		t.Fatalf("bad version accepted: %d %v", code, out)
	}
	code, out = serve(writable, http.MethodPatch, `{"graph_api_version": "v27.0", "rotate_verify_token": true}`)
	d, _ = out["data"].(map[string]any)
	if code != 200 || d["graph_api_version"] != "v27.0" || d["verify_token"] == cfg.VerifyToken {
		t.Fatalf("PATCH: %d %v", code, out)
	}

	// Test configuration: the webhook subscription is missing, then right.
	code, out = serve(writable, http.MethodPost, "")
	d, _ = out["data"].(map[string]any)
	if code != 200 || d["ok"] != false || !strings.Contains((mustJSON(d)), "tp_subscription_missing") {
		t.Fatalf("test without subscription: %d %v", code, out)
	}
	h.meta.SetAppSubscription(tpAppID, whatsapp.AppSubscription{Object: "whatsapp_business_account", CallbackURL: "https://elsewhere.test/hook", Active: true, Fields: []string{"messages"}})
	if _, out = serve(writable, http.MethodPost, ""); !strings.Contains((mustJSON(out)), "tp_subscription_url") {
		t.Fatalf("test with a wrong callback: %v", out)
	}
	h.meta.SetAppSubscription(tpAppID, whatsapp.AppSubscription{Object: "whatsapp_business_account", CallbackURL: tpCallback, Active: true, Fields: []string{"messages"}})
	code, out = serve(writable, http.MethodPost, "")
	d, _ = out["data"].(map[string]any)
	if code != 200 || d["ok"] != true {
		t.Fatalf("test of a complete set-up: %d %v", code, out)
	}
	// A wrong app secret is caught by debug_token.
	wrong := "a-wrong-app-secret-of-the-app"
	if _, _, err := h.svc.channels.UpdateTechProvider(context.Background(), h.owner, channels.TechProviderPatch{AppSecret: &wrong}); err != nil {
		t.Fatal(err)
	}
	if _, out = serve(writable, http.MethodPost, ""); !strings.Contains((mustJSON(out)), "tp_app_invalid") {
		t.Fatalf("test with a wrong app secret: %v", out)
	}
}

const (
	tpWABA1, tpPhone1 = "301000001", "301000011"
	tpWABA2, tpPhone2 = "302000002", "302000022"
	tpWABA3, tpPhone3 = "303000003", "303000033"
)

func TestTechProviderOnboardingAndRouting(t *testing.T) {
	h := newHarness(t)
	ownerA, _ := h.setupOwner()
	h.enableTechProvider()
	cfg, _ := h.svc.channels.TechProvider(context.Background())

	r := h.do(call{method: "GET", path: "/api/v1/channels/tech-provider", token: ownerA})
	h.expect(r, 200, "")
	if r.data()["app_id"] != tpAppID || r.data()["configuration_id"] != tpConfigID || r.data()["graph_api_version"] != "v26.0" {
		t.Fatalf("tech provider: %s", r.raw)
	}
	if h.metaConnection(ownerA)["tech_provider_enabled"] != true {
		t.Fatal("the Tech Provider is not offered while on")
	}

	// ── A: a failure at every Meta step, each resumed where it stopped ──
	code := h.signup(tpWABA1, tpPhone1)
	h.meta.FailOnce("GET", "oauth", 100, "temporarily unavailable")
	o := h.onboard(ownerA, map[string]any{"code": code, "waba_id": tpWABA1, "phone_number_id": tpPhone1})
	wantOnboarding(t, o, "failed", "exchange", "code_invalid")
	id := o["id"].(string)
	if _, ok := h.route(tpWABA1); ok {
		t.Fatal("routed before the exchange succeeded")
	}
	h.meta.FailOnce("POST", "subscribed_apps", 100, "temporarily unavailable")
	o = h.retryOnboarding(ownerA, id, map[string]any{"code": code})
	wantOnboarding(t, o, "failed", "subscribe", "subscribe_failed")
	if tid, ok := h.route(tpWABA1); !ok || tid != h.tenantID {
		t.Fatalf("route after claim: %v %v", tid, ok)
	}
	if n := h.count("SELECT count(*) FROM whatsapp_accounts WHERE waba_id = $1 AND status = 'disabled' AND connection_method = 'tech_provider'", tpWABA1); n != 1 {
		t.Fatalf("pending account: %d", n)
	}
	h.meta.FailOnce("POST", "register", 133005, "Two step verification PIN Mismatch")
	o = h.retryOnboarding(ownerA, id, map[string]any{}) // the code was used; the token is kept
	wantOnboarding(t, o, "failed", "register", "register_failed")
	if !strings.Contains(o["error_message"].(string), "133005") {
		t.Fatalf("register error: %v", o["error_message"])
	}
	h.meta.FailOnce("GET", "phone_numbers", 2, "Service temporarily unavailable")
	o = h.retryOnboarding(ownerA, id, map[string]any{"pin": "123456"})
	wantOnboarding(t, o, "failed", "finish", "finish_failed")
	if h.meta.Registered(tpPhone1) != "123456" {
		t.Fatalf("registered with %q", h.meta.Registered(tpPhone1))
	}
	o = h.retryOnboarding(ownerA, id, map[string]any{})
	wantOnboarding(t, o, "completed", "done", "")
	if o["attempts"] != float64(5) || o["pin"] != nil || o["pin_generated"] != false {
		t.Fatalf("completed onboarding: %v", o)
	}
	// Finished: no more retries.
	h.expect(h.do(call{method: "POST", path: "/api/v1/channels/tech-provider/onboardings/" + id + "/retry", token: ownerA, body: map[string]any{}}), 409, "ONBOARDING_FINISHED")

	accounts := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: ownerA}).list()
	if len(accounts) != 1 {
		t.Fatalf("accounts: %v", accounts)
	}
	acc := accounts[0].(map[string]any)
	accA := acc["id"].(string)
	if acc["connection_method"] != "tech_provider" || acc["status"] != "active" || acc["app_id"] != tpAppID || acc["app_secret_configured"] != false {
		t.Fatalf("account: %v", acc)
	}
	if h.metaConnection(ownerA)["method"] != "tech_provider" {
		t.Fatal("the workspace's method is not tech_provider")
	}
	if subs := h.meta.Subscribed(tpWABA1); len(subs) != 1 || subs[0] != tpAppID {
		t.Fatalf("subscribed apps: %v", subs)
	}
	if n := h.count("SELECT count(*) FROM phone_numbers WHERE phone_number_id = $1", tpPhone1); n != 1 {
		t.Fatal("the number was not stored")
	}
	for _, action := range []string{"channel.tech_provider.start", "channel.tech_provider.fail", "channel.connect"} {
		if n := h.count("SELECT count(*) FROM audit_logs WHERE action = $1", action); n == 0 {
			t.Errorf("no %s audit entry", action)
		}
	}
	// Its credentials are Messora's to manage.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/channels/accounts/" + accA, token: ownerA, body: map[string]any{"access_token": "EAAG-some-other-token-0123456789"}}), 409, "MANAGED_BY_MESSORA")
	// The token check uses Messora's app.
	h.expect(h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + accA + "/check", token: ownerA}), 200, "")

	// ── The platform webhook ─────────────────────────────────────────────
	if code, body := handshake(t, h.srv.URL+channels.TechProviderWebhookPath, cfg.VerifyToken, ""); code != 200 || body != "8812" {
		t.Fatalf("platform handshake: %d %q", code, body)
	}
	connA := h.metaConnection(ownerA)
	if code, _ := handshake(t, h.srv.URL+channels.TechProviderWebhookPath, connA["verify_token"].(string), ""); code != 403 {
		t.Fatalf("a workspace verify token at the platform address: %d", code)
	}
	inA := whatsapptest.ForWABA(tpWABA1, whatsapptest.InboundText(tpPhone1, testCustomer, "wamid.tp-a-1", "merhaba tp", "TP Müşteri", time.Now()))
	if code := h.tpPost("not-the-platform-secret-at-all", inA); code != 401 {
		t.Fatalf("wrong secret: %d", code)
	}
	if code := h.tpPost(tpAppSecret, inA); code != 200 {
		t.Fatalf("platform webhook: %d", code)
	}
	h.processWebhooks()
	if n := h.count("SELECT count(*) FROM messages WHERE wamid = 'wamid.tp-a-1' AND direction = 'inbound'"); n != 1 {
		t.Fatalf("A's inbound message: %d", n)
	}
	// A WABA of the Tech Provider is not accepted at the workspace's own
	// address, even signed with Messora's secret.
	if code := h.post(h.hookURL(keyOf(connA)), tpAppSecret, inA); code != 401 {
		t.Fatalf("TP event at the own-app address: %d", code)
	}

	// ── B: own-app account plus a Tech Provider WABA ─────────────────────
	hb, ownerB := h.betaTenant()
	code2 := hb.signup(tpWABA2, tpPhone2)
	o = hb.onboard(ownerB, map[string]any{"code": code2, "waba_id": tpWABA2, "phone_number_id": tpPhone2, "name": "Beta TP"})
	wantOnboarding(t, o, "completed", "done", "")
	if pin, _ := o["pin"].(string); len(pin) != 6 || o["pin_generated"] != true || h.meta.Registered(tpPhone2) != pin {
		t.Fatalf("generated PIN: %v (registered %q)", o["pin"], h.meta.Registered(tpPhone2))
	}
	if tid, _ := h.route(tpWABA2); tid != hb.tenantID {
		t.Fatal("B's WABA is not routed to B")
	}
	// A mixed batch: A's entry, B's entry, an unknown WABA.
	inB := whatsapptest.ForWABA(tpWABA2, whatsapptest.InboundText(tpPhone2, testCustomer, "wamid.tp-b-1", "merhaba beta", "Beta Müşteri", time.Now()))
	unknown := whatsapptest.ForWABA("999000999", whatsapptest.InboundText("999000991", testCustomer, "wamid.tp-x-1", "kimse", "X", time.Now()))
	inA2 := whatsapptest.ForWABA(tpWABA1, whatsapptest.InboundText(tpPhone1, testCustomer, "wamid.tp-a-2", "ikinci", "TP Müşteri", time.Now()))
	before, _ := channels.Unrouted()
	if code := h.tpPost(tpAppSecret, entries(t, inA2, inB, unknown)); code != 200 {
		t.Fatalf("mixed batch: %d", code)
	}
	if after, last := channels.Unrouted(); after != before+1 || last != "999000999" {
		t.Fatalf("unrouted: %d → %d (%s)", before, after, last)
	}
	h.processWebhooks()
	hb.processWebhooks()
	if n := h.count("SELECT count(*) FROM messages WHERE wamid IN ('wamid.tp-a-2', 'wamid.tp-b-1', 'wamid.tp-x-1')"); n != 1 ||
		h.count("SELECT count(*) FROM messages WHERE wamid = 'wamid.tp-a-2'") != 1 {
		t.Fatalf("A's part of the batch: %d", n)
	}
	if n := hb.count("SELECT count(*) FROM messages WHERE wamid IN ('wamid.tp-a-2', 'wamid.tp-b-1', 'wamid.tp-x-1')"); n != 1 ||
		hb.count("SELECT count(*) FROM messages WHERE wamid = 'wamid.tp-b-1'") != 1 {
		t.Fatalf("B's part of the batch: %d", n)
	}
	var storedB []byte
	if err := hb.pool.QueryRow(hb.ctx(), "SELECT body FROM webhook_events ORDER BY received_at DESC LIMIT 1").Scan(&storedB); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(storedB), tpWABA1) || strings.Contains(string(storedB), "999000999") {
		t.Fatalf("B stored another tenant's entry: %s", storedB)
	}
	// B's own app secret does not sign for the platform address.
	if code := h.tpPost(betaAppSecret, inB); code != 401 {
		t.Fatalf("B's secret at the platform address: %d", code)
	}

	// ── One WABA, one workspace, one method ──────────────────────────────
	// B claims A's Tech Provider WABA.
	o = hb.onboard(ownerB, map[string]any{"code": hb.signup(tpWABA1, tpPhone1), "waba_id": tpWABA1, "phone_number_id": tpPhone1})
	wantOnboarding(t, o, "failed", "claim", "waba_in_use")
	if tid, _ := h.route(tpWABA1); tid != h.tenantID {
		t.Fatal("B took A's route")
	}
	// A claims B's own-app WABA.
	o = h.onboard(ownerA, map[string]any{"code": h.signup(betaWABA, betaPNID), "waba_id": betaWABA, "phone_number_id": betaPNID})
	wantOnboarding(t, o, "failed", "claim", "waba_in_use")
	if _, ok := h.route(betaWABA); ok {
		t.Fatal("a route was left behind by a refused claim")
	}
	// B onboards its own own-app WABA: refused until that account is disabled.
	o = hb.onboard(ownerB, map[string]any{"code": hb.signup(betaWABA, betaPNID), "waba_id": betaWABA, "phone_number_id": betaPNID})
	wantOnboarding(t, o, "failed", "claim", "waba_connected_own_app")
	hb.expect(hb.do(call{method: "DELETE", path: "/api/v1/channels/tech-provider/onboardings/" + o["id"].(string), token: ownerB}), 200, "")
	// B connects its Tech Provider WABA with its own app: refused.
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/accounts", token: ownerB, body: map[string]any{
		"name": "Beta own", "waba_id": tpWABA2, "app_id": betaAppID, "app_secret": betaAppSecret, "access_token": betaToken}}), 409, "WABA_CONNECTED_TECH_PROVIDER")
	// The token must grant the WABA the browser claims.
	o = hb.onboard(ownerB, map[string]any{"code": hb.signup(tpWABA3, tpPhone3), "waba_id": tpWABA2, "phone_number_id": tpPhone2})
	wantOnboarding(t, o, "failed", "exchange", "waba_not_granted")
	hb.expect(hb.do(call{method: "DELETE", path: "/api/v1/channels/tech-provider/onboardings/" + o["id"].(string), token: ownerB}), 200, "")

	// ── Isolation ────────────────────────────────────────────────────────
	r = hb.do(call{method: "GET", path: "/api/v1/channels/tech-provider", token: ownerB})
	h.expect(r, 200, "")
	if strings.Contains(string(r.raw), id) {
		t.Fatalf("B sees A's onboarding: %s", r.raw)
	}
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/tech-provider/onboardings/" + id + "/retry", token: ownerB, body: map[string]any{}}), 404, "")
	hb.expect(hb.do(call{method: "DELETE", path: "/api/v1/channels/tech-provider/onboardings/" + id, token: ownerB}), 404, "")
	hb.expect(hb.do(call{method: "POST", path: "/api/v1/channels/accounts/" + accA + "/tech-provider/disconnect", token: ownerB}), 404, "")
	var released bool
	if err := hb.pool.QueryRow(hb.ctx(), `SELECT meta_route_release($1)`, tpWABA1).Scan(&released); err != nil || released {
		t.Fatalf("B released A's route: %v %v", released, err)
	}
	if _, err := hb.pool.Exec(hb.ctx(), `INSERT INTO meta_routes (waba_id, tenant_id) VALUES ('555', $1)`, hb.tenantID); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("the app role wrote meta_routes: %v", err)
	}
	if _, err := hb.pool.Exec(hb.ctx(), `SELECT meta_route_claim($1)`, tpWABA1); err == nil {
		t.Fatal("B claimed A's routed WABA")
	}
	if tid, _ := h.route(tpWABA1); tid != h.tenantID {
		t.Fatal("A's route changed")
	}

	// ── Disconnect and switch methods ────────────────────────────────────
	r = h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + accA + "/tech-provider/disconnect", token: ownerA})
	h.expect(r, 200, "")
	if r.data()["account"].(map[string]any)["status"] != "disabled" || r.data()["warning"] != nil {
		t.Fatalf("disconnect: %s", r.raw)
	}
	if _, ok := h.route(tpWABA1); ok {
		t.Fatal("the route survived the disconnect")
	}
	if len(h.meta.Subscribed(tpWABA1)) != 0 {
		t.Fatal("Messora's app is still subscribed")
	}
	if h.metaConnection(ownerA)["method"] != "own_app" {
		t.Fatal("the method did not fall back to own_app")
	}
	if n := h.count("SELECT count(*) FROM messages WHERE wamid LIKE 'wamid.tp-a-%'"); n != 2 {
		t.Fatalf("history lost: %d", n)
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/channels/accounts/" + accA, token: ownerA, body: map[string]any{"status": "active"}}), 409, "TECH_PROVIDER_DISCONNECTED")
	// Events of the disconnected WABA are dropped at the platform address.
	inA3 := whatsapptest.ForWABA(tpWABA1, whatsapptest.InboundText(tpPhone1, testCustomer, "wamid.tp-a-3", "after", "TP Müşteri", time.Now()))
	if code := h.tpPost(tpAppSecret, inA3); code != 200 {
		t.Fatalf("after disconnect: %d", code)
	}
	if n := h.count("SELECT count(*) FROM webhook_events WHERE body::text LIKE '%wamid.tp-a-3%'"); n != 0 {
		t.Fatal("an event of a disconnected WABA was stored")
	}
	// Switch to the company's own app: the same account, with its history.
	ownToken := "EAAG-alpha-own-app-token-0123456789"
	h.meta.SetTokenInfo(ownToken, whatsapptest.TokenInfo{AppID: testAppID})
	r = h.do(call{method: "POST", path: "/api/v1/channels/accounts", token: ownerA, body: map[string]any{
		"name": "Alpha own", "waba_id": tpWABA1, "app_id": testAppID, "app_secret": testAppSecret, "access_token": ownToken}})
	h.expect(r, 201, "")
	if r.data()["id"] != accA || r.data()["connection_method"] != "own_app" || r.data()["status"] != "active" {
		t.Fatalf("switch to own app: %s", r.raw)
	}
	if code := h.tpPost(tpAppSecret, inA3); code != 200 || h.count("SELECT count(*) FROM webhook_events WHERE body::text LIKE '%wamid.tp-a-3%'") != 0 {
		t.Fatal("the platform address accepted an own-app WABA")
	}
	if code := h.post(h.hookURL(keyOf(connA)), testAppSecret, inA3); code != 200 {
		t.Fatalf("own-app address: %d", code)
	}
	if n := h.count("SELECT count(*) FROM webhook_events WHERE body::text LIKE '%wamid.tp-a-3%'"); n != 1 {
		t.Fatalf("own-app event stored %d times", n)
	}
	// And back to the Tech Provider once the own-app account is disabled.
	o = h.onboard(ownerA, map[string]any{"code": h.signup(tpWABA1, tpPhone1), "waba_id": tpWABA1, "phone_number_id": tpPhone1})
	wantOnboarding(t, o, "failed", "claim", "waba_connected_own_app")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/channels/accounts/" + accA, token: ownerA, body: map[string]any{"status": "disabled"}}), 200, "")
	o = h.retryOnboarding(ownerA, o["id"].(string), map[string]any{})
	wantOnboarding(t, o, "completed", "done", "")
	if o["account_id"] != accA {
		t.Fatalf("switch back created another account: %v", o["account_id"])
	}
	if tid, _ := h.route(tpWABA1); tid != h.tenantID {
		t.Fatal("not routed after switching back")
	}

	// ── Cancel an unfinished onboarding ──────────────────────────────────
	h.meta.FailOnce("POST", "register", 100, "temporarily unavailable")
	o = h.onboard(ownerA, map[string]any{"code": h.signup(tpWABA3, tpPhone3), "waba_id": tpWABA3, "phone_number_id": tpPhone3})
	wantOnboarding(t, o, "failed", "register", "register_failed")
	if _, ok := h.route(tpWABA3); !ok {
		t.Fatal("no route before cancel")
	}
	r = h.do(call{method: "DELETE", path: "/api/v1/channels/tech-provider/onboardings/" + o["id"].(string), token: ownerA})
	h.expect(r, 200, "")
	if r.data()["status"] != "cancelled" {
		t.Fatalf("cancel: %s", r.raw)
	}
	if _, ok := h.route(tpWABA3); ok || h.count("SELECT count(*) FROM whatsapp_accounts WHERE waba_id = $1", tpWABA3) != 0 || len(h.meta.Subscribed(tpWABA3)) != 0 {
		t.Fatal("cancel left a route, an account or a subscription")
	}

	// Deleting a Tech Provider account (no history) removes its route.
	o = hb.onboard(ownerB, map[string]any{"code": hb.signup(tpWABA3, tpPhone3), "waba_id": tpWABA3, "phone_number_id": tpPhone3})
	wantOnboarding(t, o, "completed", "done", "")
	if tid, _ := h.route(tpWABA3); tid != hb.tenantID {
		t.Fatal("B's new WABA is not routed to B")
	}
	hb.expect(hb.do(call{method: "DELETE", path: "/api/v1/channels/accounts/" + o["account_id"].(string), token: ownerB}), 204, "")
	if _, ok := h.route(tpWABA3); ok {
		t.Fatal("a deleted account kept its route")
	}
	if len(h.meta.Subscribed(tpWABA3)) != 0 {
		t.Fatal("Messora's app stayed subscribed to a deleted account")
	}
}
