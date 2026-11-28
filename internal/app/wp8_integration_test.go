//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/credits"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/fairness"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Phase 17 WP8: plans and entitlements (docs/ENTITLEMENTS.md). The tests
// share harnesses through subtests to keep the integration run short.

// setPlan gives tenant id a plan with its own limits (JSON object) the way
// the platform panel stores them, and drops the cached plans.
func (h *harness) setPlan(id uuid.UUID, key, overrides string) {
	h.t.Helper()
	if overrides == "" {
		overrides = "{}"
	}
	if _, err := h.owner.Exec(context.Background(), `UPDATE tenants SET plan_key = $2, plan_overrides = $3 WHERE id = $1`, id, key, overrides); err != nil {
		h.t.Fatal(err)
	}
	h.svc.planDB.InvalidateAll()
}

// limitErr checks a 409 PLAN_LIMIT_REACHED for resource with limit and
// current in the details.
func limitErr(t *testing.T, r resp, resource string, limit, current string) {
	t.Helper()
	if r.status != 409 || r.errCode() != entitlements.CodePlanLimitReached {
		t.Fatalf("want 409 PLAN_LIMIT_REACHED for %s, got %d %s", resource, r.status, r.raw)
	}
	details, _ := r.body["error"].(map[string]any)["details"].([]any)
	got := map[string]string{}
	for _, d := range details {
		m := d.(map[string]any)
		if m["code"] == resource {
			got[m["field"].(string)] = m["message"].(string)
		}
	}
	if got["limit"] != limit || (current != "" && got["current"] != current) {
		t.Fatalf("%s details: %v", resource, details)
	}
}

// featureErr checks a 403 FEATURE_NOT_IN_PLAN naming feature.
func featureErr(t *testing.T, r resp, feature string) {
	t.Helper()
	if r.status != 403 || r.errCode() != entitlements.CodeFeatureNotInPlan || !strings.Contains(string(r.raw), `"code":"`+feature+`"`) {
		t.Fatalf("want 403 FEATURE_NOT_IN_PLAN %s, got %d %s", feature, r.status, r.raw)
	}
}

func usageOf(t *testing.T, plan map[string]any, resource string) map[string]any {
	t.Helper()
	for _, u := range plan["usage"].([]any) {
		if m := u.(map[string]any); m["resource"] == resource {
			return m
		}
	}
	t.Fatalf("no usage of %s: %v", resource, plan["usage"])
	return nil
}

func TestPlansAndEntitlements(t *testing.T) {
	h := newHarness(t)
	hb := h.tenant("plans")
	ownerB := hb.setupOwnerAs("owner@plans.test")

	t.Run("the catalogue, existing and new companies", func(t *testing.T) {
		r := h.adminDo(call{method: "GET", path: "/plans"})
		h.expect(r, 200, "")
		var keys []string
		for _, p := range r.list() {
			keys = append(keys, p.(map[string]any)["key"].(string))
		}
		if strings.Join(keys, ",") != "starter,growth,pro,enterprise" {
			t.Fatalf("plans: %v", keys)
		}
		// Companies from before plans keep everything (enterprise); a
		// company inserted without a plan gets the default plan.
		if r := h.adminDo(call{method: "GET", path: "/tenants/" + tenant.DefaultID.String() + "/plan"}); r.data()["plan_key"] != "enterprise" {
			t.Fatalf("tenant #1: %s", r.raw)
		}
		if r := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}); r.data()["key"] != entitlements.DefaultKey {
			t.Fatalf("no plan: %s", r.raw)
		}
		// A new company is on trial with the growth plan's limits.
		r = h.adminDo(call{method: "POST", path: "/tenants", body: map[string]any{"name": "Trial Co", "slug": "trialco"}})
		h.expect(r, 201, "")
		id := r.data()["tenant"].(map[string]any)["id"].(string)
		if r.data()["tenant"].(map[string]any)["plan_key"] != entitlements.TrialPlan {
			t.Fatalf("new company: %s", r.raw)
		}
		r = h.adminDo(call{method: "GET", path: "/tenants/" + id + "/plan"})
		h.expect(r, 200, "")
		ov := r.data()["overview"].(map[string]any)
		if ov["key"] != "growth" || ov["subscription_status"] != "trial" || ov["limits"].(map[string]any)["users"] != float64(5) ||
			ov["features"].(map[string]any)["custom_smtp"] != false || ov["ai_credits"].(map[string]any)["included"] != float64(2000) {
			t.Fatalf("trial overview: %v", ov)
		}
	})

	t.Run("limits are enforced where things are created", func(t *testing.T) {
		// Invalid own limits are refused.
		h.expect(h.adminDo(call{method: "PUT", path: "/tenants/" + hb.tenantID.String() + "/plan",
			body: map[string]any{"plan_key": "starter", "overrides": map[string]any{"seats": 3}}}), 422, "VALIDATION_ERROR")
		r := h.adminDo(call{method: "PUT", path: "/tenants/" + hb.tenantID.String() + "/plan", body: map[string]any{"plan_key": "starter",
			"overrides": map[string]any{"users": 2, "contacts": 2, "automation_flows": 1, "ai_agents": 1, "storage_bytes": 64}}})
		h.expect(r, 200, "")
		if r.data()["overview"].(map[string]any)["overridden"] != true {
			t.Fatalf("overrides: %s", r.raw)
		}
		// Seats: the owner and one open invitation.
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/invitations", token: ownerB, body: map[string]any{"email": "a1@plans.test", "role": "agent"}}), 201, "")
		limitErr(t, hb.do(call{method: "POST", path: "/api/v1/invitations", token: ownerB, body: map[string]any{"email": "a2@plans.test", "role": "agent"}}), "users", "2", "2")
		// Contacts (a fair-use hook: no launch plan caps them).
		hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+905550000001"})
		hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+905550000002"})
		limitErr(t, hb.do(call{method: "POST", path: "/api/v1/contacts", token: ownerB, body: map[string]any{"phone": "+905550000003"}}), "contacts", "2", "2")
		// Active automation flows: creating an inactive one is fine,
		// switching it on is not.
		flow := map[string]any{"name": "Flow", "active": true, "trigger_type": "message_received", "steps": []any{map[string]any{"type": "add_tag", "tag": "x"}}}
		hb.automation(ownerB, flow)
		limitErr(t, hb.do(call{method: "POST", path: "/api/v1/automations", token: ownerB, body: flow}), "automation_flows", "1", "1")
		flow["active"] = false
		off := hb.automation(ownerB, flow)
		limitErr(t, hb.do(call{method: "PATCH", path: "/api/v1/automations/" + off, token: ownerB, body: map[string]any{"active": true}}), "automation_flows", "1", "1")
		// AI agents (profiles).
		hb.created(ownerB, "/api/v1/ai/profiles", map[string]any{"name": "Bot 1"})
		limitErr(t, hb.do(call{method: "POST", path: "/api/v1/ai/profiles", token: ownerB, body: map[string]any{"name": "Bot 2"}}), "ai_agents", "1", "1")
		// Storage: an import file past the plan's bytes.
		code, raw := hb.upload("/api/v1/contact-imports?filename=people.csv", ownerB, "text/csv",
			strings.NewReader("phone,name\n+905551110001,Ada Lovelace\n+905551110002,Grace Hopper\n"))
		if code != 409 || !strings.Contains(string(raw), "storage_bytes") {
			t.Fatalf("import over storage: %d %s", code, raw)
		}
		// Features the starter plan lacks.
		featureErr(t, hb.do(call{method: "PUT", path: "/api/v1/settings/smtp", token: ownerB, body: map[string]any{"host": "smtp.x.test",
			"port": 587, "username": "", "from": "a@x.test", "tls": "starttls"}}), "custom_smtp")
		featureErr(t, hb.do(call{method: "POST", path: "/api/v1/settings/smtp/test", token: ownerB, body: map[string]any{"to": "a@x.test"}}), "custom_smtp")
		featureErr(t, hb.do(call{method: "POST", path: "/api/v1/ai/providers", token: ownerB, body: map[string]any{"kind": "openai", "name": "Own", "api_key": aiKey}}), "byo_ai_keys")
		featureErr(t, hb.do(call{method: "POST", path: "/api/v1/api-keys", token: ownerB, body: map[string]any{"name": "k", "scopes": []string{"contacts.read"}}}), "public_api")
		featureErr(t, h.adminDo(call{method: "POST", path: "/tenants/" + hb.tenantID.String() + "/domains", body: map[string]any{"hostname": "panel.plans-co.test"}}), "custom_domain")
		// /me tells the panel what to hide.
		me := hb.do(call{method: "GET", path: "/api/v1/me", token: ownerB}).data()["tenant"].(map[string]any)["plan"].(map[string]any)
		if me["key"] != "starter" || strings.Contains(string(asJSON(me["features"])), "custom_smtp") || !strings.Contains(string(asJSON(me["features"])), "webhooks") {
			t.Fatalf("me.plan: %v", me)
		}
		// The plan page counts what is used.
		p := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()
		if u := usageOf(t, p, "users"); u["used"] != float64(2) || u["limit"] != float64(2) || u["over"] != false {
			t.Fatalf("users usage: %v", u)
		}
		if u := usageOf(t, p, "automation_flows"); u["used"] != float64(1) {
			t.Fatalf("flows usage: %v", u)
		}
	})

	t.Run("a lower plan never deletes: over-limit resources keep working", func(t *testing.T) {
		hb.setPlan(hb.tenantID, "enterprise", "")
		hb.created(ownerB, "/api/v1/contacts", map[string]any{"phone": "+905550000003"})
		second := hb.automation(ownerB, map[string]any{"name": "Second", "active": true, "trigger_type": "message_received",
			"steps": []any{map[string]any{"type": "add_tag", "tag": "y"}}})
		hb.setPlan(hb.tenantID, "starter", `{"contacts": 2, "automation_flows": 1}`)
		p := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()
		if u := usageOf(t, p, "contacts"); u["used"] != float64(3) || u["over"] != true {
			t.Fatalf("contacts after downgrade: %v", u)
		}
		if u := usageOf(t, p, "automation_flows"); u["used"] != float64(2) || u["over"] != true {
			t.Fatalf("flows after downgrade: %v", u)
		}
		// Still on and still editable; only switching more on is refused.
		r := hb.do(call{method: "PATCH", path: "/api/v1/automations/" + second, token: ownerB, body: map[string]any{"name": "Second (renamed)"}})
		hb.expect(r, 200, "")
		if r.data()["active"] != true {
			t.Fatalf("flow switched off by the downgrade: %s", r.raw)
		}
		limitErr(t, hb.do(call{method: "POST", path: "/api/v1/contacts", token: ownerB, body: map[string]any{"phone": "+905550000004"}}), "contacts", "2", "3")
	})

	t.Run("phone numbers", func(t *testing.T) {
		ownerA, _ := h.connect() // one number, on enterprise
		h.setPlan(tenant.DefaultID, "starter", "")
		defer h.setPlan(tenant.DefaultID, "enterprise", "")
		h.meta.AddPhoneNumber("2000000002", whatsapp.PhoneNumber{ID: "209876543210", DisplayPhoneNumber: "+1 555-010-0002", VerifiedName: "Acme 2"})
		limitErr(t, h.do(call{method: "POST", path: "/api/v1/channels/accounts", token: ownerA, body: map[string]any{
			"name": "Second WABA", "waba_id": "2000000002", "app_id": testAppID, "app_secret": testAppSecret, "access_token": testToken}}), "phone_numbers", "1", "1")
	})

	t.Run("campaign rate: the lower of fairness and plan", func(t *testing.T) {
		capOf := campaignCap(fairness.Static(fairness.Settings{TenantShare: 0.5, TenantCampaignMPS: 200}), h.svc.planOf())
		if v := capOf(tenant.WithID(context.Background(), hb.tenantID)); v != 20 {
			t.Fatalf("starter cap = %v", v)
		}
		if v := capOf(tenant.WithID(context.Background(), tenant.DefaultID)); v != 200 {
			t.Fatalf("enterprise cap = %v", v)
		}
	})

	t.Run("plan edits are platform work and audited", func(t *testing.T) {
		r := h.adminDo(call{method: "PATCH", path: "/plans/starter", body: map[string]any{"limits": map[string]any{"webhook_endpoints": 4},
			"price_monthly": "1390"}})
		h.expect(r, 200, "")
		if r.data()["limits"].(map[string]any)["webhook_endpoints"] != float64(4) || r.data()["price_monthly"] != "1390.00" ||
			r.data()["limits"].(map[string]any)["users"] != float64(2) {
			t.Fatalf("patched plan: %s", r.raw)
		}
		// The companies on it see the change at once on this replica.
		if l := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data()["limits"].(map[string]any); l["webhook_endpoints"] != float64(4) {
			t.Fatalf("plan not followed: %v", l)
		}
		if n := h.platformCount(`SELECT count(*) FROM platform_audit_logs WHERE action = 'plan.update' AND changes->>'key' = 'starter'`); n != 1 {
			t.Fatalf("plan audit = %d", n)
		}
		// Companies cannot reach the platform's plan operations.
		if code, _ := hb.rawAt(hb.host, "GET", "/api/v1/platform/plans", ownerB, true); code != 404 {
			t.Fatalf("tenant host /platform/plans: %d", code)
		}
		if code, _ := hb.rawAt(hb.host, "GET", "/api/v1/platform/tenants/"+hb.tenantID.String()+"/plan", ownerB, true); code != 404 {
			t.Fatalf("tenant host /platform/tenants/…/plan: %d", code)
		}
	})

	t.Run("plan change requests", func(t *testing.T) {
		h.platformSession()
		r := hb.do(call{method: "POST", path: "/api/v1/plan/change-requests", token: ownerB, body: map[string]any{"plan_key": "growth",
			"billing_cycle": "yearly", "message": "Daha fazla kullanıcı lazım"}})
		hb.expect(r, 201, "")
		reqID := r.data()["id"].(string)
		hb.expect(hb.do(call{method: "POST", path: "/api/v1/plan/change-requests", token: ownerB, body: map[string]any{"plan_key": "pro"}}), 409, "PLAN_REQUEST_OPEN")
		if h.countMailTo("platform-admin@messora.test", "plans") != 1 {
			t.Fatal("platform admins were not told")
		}
		if p := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data(); p["open_request"].(map[string]any)["id"] != reqID {
			t.Fatalf("open request: %v", p["open_request"])
		}
		r = h.adminDo(call{method: "GET", path: "/plan-requests?status=open"})
		if len(r.list()) != 1 || r.list()[0].(map[string]any)["tenant"].(map[string]any)["slug"] != "plans" {
			t.Fatalf("inbox: %s", r.raw)
		}
		r = h.adminDo(call{method: "POST", path: "/plan-requests/" + reqID + "/resolve", body: map[string]any{"status": "done", "apply": true, "note": "ok"}})
		h.expect(r, 200, "")
		if p := hb.do(call{method: "GET", path: "/api/v1/plan", token: ownerB}).data(); p["key"] != "growth" || p["open_request"] != nil {
			t.Fatalf("after resolve: %v", p["key"])
		}
		h.expect(h.adminDo(call{method: "POST", path: "/plan-requests/" + reqID + "/resolve", body: map[string]any{"status": "declined"}}), 409, "PLAN_REQUEST_CLOSED")
		if n := hb.count(`SELECT count(*) FROM audit_logs WHERE action = 'plan.change'`); n == 0 {
			t.Fatal("the company's audit log misses the plan change")
		}
	})

	t.Run("the Meta rate card is public and read-only", func(t *testing.T) {
		for _, host := range []string{h.host, hb.host, adminHost, "unknown.example"} {
			code, body, hdr := h.publicGet(host, "/api/v1/meta-rates")
			if code != 200 || !strings.HasPrefix(body, `{"data":[`) || !strings.Contains(hdr, "public") {
				t.Fatalf("%s: %d %s (%s)", host, code, body, hdr)
			}
		}
		if code, _ := hb.rawAt(hb.host, "POST", "/api/v1/platform/meta-rates/import", ownerB, true); code != 404 {
			t.Fatalf("tenant import: %d", code)
		}
	})

	t.Run("companies see only their own usage and credits", func(t *testing.T) {
		if _, err := h.svc.credits.Grant(hb.ctx(), db.New(h.pool), credits.Grant{Credits: 7, Note: "test grant"}); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := h.pool.QueryRow(h.ctx(), `SELECT count(*) FROM ai_credit_lots`).Scan(&n); err != nil || n != 0 {
			t.Fatalf("tenant #1 sees %d lots of another company (%v)", n, err)
		}
		if err := hb.pool.QueryRow(hb.ctx(), `SELECT count(*) FROM ai_credit_lots`).Scan(&n); err != nil || n != 1 {
			t.Fatalf("B's own lots: %d (%v)", n, err)
		}
	})
}

// upload posts a raw body to the harness's tenant host.
func (h *harness) upload(path, token, contentType string, body io.Reader) (int, []byte) {
	h.t.Helper()
	req, _ := http.NewRequest("POST", h.srv.URL+path, body)
	req.Host = h.host
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set(tenant.HostHeader, h.host)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// publicGet fetches path at host without the panel or a session.
func (h *harness) publicGet(host, path string) (int, string, string) {
	h.t.Helper()
	r := h.do(call{host: host, method: "GET", path: path, noBFF: true})
	return r.status, string(r.raw), r.header.Get("Cache-Control")
}

func asJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// platformCount counts rows of a platform table (the owner sees them all).
func (h *harness) platformCount(sql string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.owner.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func TestAICreditsAndKeys(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	today := time.Now().UTC().Format(time.DateOnly)
	// Messora's own (fake) key.
	r := h.adminDo(call{method: "POST", path: "/ai-providers", body: map[string]any{"kind": "fake", "name": "Messora fake", "api_key": aiKey}})
	h.expect(r, 201, "")
	if r.data()["status"] != "ok" || r.data()["model_count"] != float64(2) || strings.Contains(string(r.raw), aiKey) {
		t.Fatalf("platform key: %s", r.raw)
	}
	platformID := r.data()["id"].(string)
	h.expect(h.adminDo(call{method: "POST", path: "/ai-providers", body: map[string]any{"kind": "fake", "name": "Again", "api_key": aiKey}}), 409, "PROVIDER_EXISTS")
	test := func() resp {
		return h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + env.profileID + "/test", token: env.owner,
			body: map[string]any{"messages": []any{map[string]any{"role": "user", "text": "Kargom nerede?"}}}})
	}
	lastUsage := func() (keySource *string, credits *int64) {
		t.Helper()
		if err := h.pool.QueryRow(h.ctx(), `SELECT key_source, credits FROM ai_usage ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&keySource, &credits); err != nil {
			t.Fatal(err)
		}
		return
	}
	used := func() int64 {
		t.Helper()
		var n int64
		if err := h.pool.QueryRow(h.ctx(), `SELECT coalesce(sum(used), 0) FROM ai_credit_lots`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("enterprise with its own key consumes no credits", func(t *testing.T) {
		h.expect(test(), 200, "")
		if ks, c := lastUsage(); ks == nil || *ks != "tenant" || c != nil {
			t.Fatalf("own key usage: %v %v", ks, c)
		}
		if used() != 0 {
			t.Fatal("credits consumed with the company's own key")
		}
	})

	t.Run("enterprise without its own key falls back to Messora's key and credits", func(t *testing.T) {
		h.expect(h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + env.providerID, token: env.owner, body: map[string]any{"enabled": false}}), 200, "")
		h.expect(test(), 200, "")
		// No price for the model yet: an unpriced call costs 5 credits.
		if ks, c := lastUsage(); ks == nil || *ks != "platform" || c == nil || *c != 5000 {
			t.Fatalf("fallback usage: %v %v", ks, c)
		}
		if used() != 5000 {
			t.Fatalf("lots used = %d", used())
		}
	})

	t.Run("credits follow the platform price and conversion", func(t *testing.T) {
		h.expect(h.adminDo(call{method: "POST", path: "/ai-prices", body: map[string]any{"provider_kind": "fake",
			"model_pattern": "fake-*", "input": "1", "output": "2", "effective_from": today}}), 201, "")
		h.expect(test(), 200, "") // 0.00014 USD ÷ 0.001 = 0.14 credits
		if _, c := lastUsage(); c == nil || *c != 140 {
			t.Fatalf("priced credits: %v", c)
		}
		r := h.adminDo(call{method: "PATCH", path: "/ai-settings", body: map[string]any{"markup": "2"}})
		h.expect(r, 200, "")
		if r.data()["markup"] != "2" || len(r.data()["models"].([]any)) != 2 {
			t.Fatalf("ai settings: %s", r.raw)
		}
		h.expect(test(), 200, "")
		if _, c := lastUsage(); c == nil || *c != 280 {
			t.Fatalf("credits with markup 2: %v", c)
		}
		h.expect(h.adminDo(call{method: "PATCH", path: "/ai-settings", body: map[string]any{"markup": "1"}}), 200, "")
		p := h.do(call{method: "GET", path: "/api/v1/plan", token: env.owner}).data()["ai_credits"].(map[string]any)
		if p["used_this_month"] != 5.42 || p["included"] != float64(20000) || p["exhausted"] != false {
			t.Fatalf("plan credits: %v", p)
		}
	})

	t.Run("other plans use Messora's keys and cannot add their own", func(t *testing.T) {
		h.setPlan(tenant.DefaultID, "growth", "")
		featureErr(t, h.do(call{method: "POST", path: "/api/v1/ai/providers", token: env.owner, body: map[string]any{"kind": "fake", "name": "Own 2", "api_key": aiKey}}), "byo_ai_keys")
		featureErr(t, h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + env.providerID, token: env.owner, body: map[string]any{"enabled": true}}), "byo_ai_keys")
		// The stored own key stays (listed, deletable) but unused.
		if r := h.do(call{method: "GET", path: "/api/v1/ai/providers", token: env.owner}); len(r.list()) != 1 {
			t.Fatalf("own keys: %s", r.raw)
		}
		for _, m := range h.do(call{method: "GET", path: "/api/v1/ai/models", token: env.owner}).list() {
			if m.(map[string]any)["key_source"] != "platform" {
				t.Fatalf("model of an own key listed: %v", m)
			}
		}
		h.expect(test(), 200, "")
		if ks, _ := lastUsage(); *ks != "platform" {
			t.Fatal("growth did not use Messora's key")
		}
		// A platform key can be the company default.
		h.enableAI(env.owner, platformID, "fake-small")
		h.expect(test(), 200, "")
	})

	t.Run("used-up credits pause AI replies and steps", func(t *testing.T) {
		h.setPlan(tenant.DefaultID, "growth", `{"ai_credits_per_month": 1}`)
		if me := h.do(call{method: "GET", path: "/api/v1/me", token: env.owner}).data()["tenant"].(map[string]any)["plan"].(map[string]any); me["ai_credits_exhausted"] != true {
			t.Fatalf("me: %v", me)
		}
		h.expect(test(), 409, "AI_CREDITS_EXHAUSTED")
		h.say("wamid.credits1", "Merhaba")
		if len(h.metaTexts()) != 0 {
			t.Fatal("the AI answered without credits")
		}
		id := h.conversationOf(testCustomer)
		if hs := h.handoffs(env.owner, id); len(hs) == 0 || hs[0]["reason"] != "ai_credits" {
			t.Fatalf("handoffs: %v", hs)
		}
		if n := h.count("SELECT count(*) FROM audit_logs WHERE action = 'ai.credits_exhausted'"); n != 1 {
			t.Fatalf("credit notices = %d", n)
		}
	})

	t.Run("a grant restores, audited", func(t *testing.T) {
		r := h.adminDo(call{method: "POST", path: "/tenants/" + tenant.DefaultID.String() + "/ai-credits", body: map[string]any{"credits": 50, "note": "goodwill"}})
		h.expect(r, 201, "")
		if r.data()["granted"] != float64(50) || r.data()["source"] != "grant" {
			t.Fatalf("grant: %s", r.raw)
		}
		h.expect(test(), 200, "")
		if n := h.platformCount(`SELECT count(*) FROM platform_audit_logs WHERE action = 'tenant.ai_credits_grant'`); n != 1 {
			t.Fatalf("grant audit = %d", n)
		}
		r = h.adminDo(call{method: "GET", path: "/tenants/" + tenant.DefaultID.String() + "/plan"})
		if lots := r.data()["lots"].([]any); len(lots) != 2 {
			t.Fatalf("lots: %s", r.raw)
		}
	})

	t.Run("the plan's credits reset every month", func(t *testing.T) {
		next := time.Now().UTC().AddDate(0, 1, 0)
		h.svc.credits.SetClock(func() time.Time { return next })
		defer h.svc.credits.SetClock(time.Now)
		b, err := h.svc.credits.Balance(h.ctx())
		if err != nil {
			t.Fatal(err)
		}
		// The grant (no expiry) carries over; the month starts fresh.
		if b.Included != 1000 || b.UsedThisMonth != 0 || b.Exhausted() || !b.MonthStart.Equal(time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("next month: %+v", b)
		}
	})

	t.Run("concurrent consumption loses nothing", func(t *testing.T) {
		before := used()
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tx, err := h.pool.Begin(h.ctx())
				if err != nil {
					errs <- err
					return
				}
				defer func() { _ = tx.Rollback(h.ctx()) }()
				if err := h.svc.credits.Consume(h.ctx(), db.New(tx), 700); err != nil {
					errs <- err
					return
				}
				errs <- tx.Commit(h.ctx())
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if got := used() - before; got != 16*700 {
			t.Fatalf("booked %d of %d millicredits", got, 16*700)
		}
	})
}

// panelPassword is the platform sender's password in the test.
const panelPassword = "platform-smtp-pass-for-tests" // gitleaks:allow (test fixture)

func TestEmailSendersByPlan(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"organization_name": "Acme Gıda"}}), 200, "")
	platformAddr, platformGot := fakeSMTPServer(t)
	host, port := splitHostPort(t, platformAddr)
	mail, err := settings.NewMail(h.svc.queries, h.sealer(), mailer.Config{Host: host, Port: port, From: "Messora <no-reply@messora.test>", TLS: "none"})
	if err != nil {
		t.Fatal(err)
	}
	mail.SetPlans(h.svc.planOf())
	h.svc.mail = mail
	h.remount()
	msg := mailer.Message{To: "customer@example.com", Subject: "Hello", Body: "Hi\n"}

	t.Run("platform sender: the company's name and reply-to", func(t *testing.T) {
		h.setPlan(tenant.DefaultID, "growth", "")
		h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings/email", token: owner, body: map[string]any{"reply_to": "destek@acme.test"}}), 200, "")
		if err := mail.Send(h.ctx(), msg); err != nil {
			t.Fatal(err)
		}
		got := <-platformGot
		if !strings.Contains(got, "From: \"Acme =?utf-8?q?G=C4=B1da?=\" <no-reply@messora.test>") && !strings.Contains(got, "From: =?utf-8?q?Acme_G=C4=B1da?= <no-reply@messora.test>") {
			t.Fatalf("from: %s", got)
		}
		if !strings.Contains(got, "Reply-To: <destek@acme.test>") {
			t.Fatalf("reply-to: %s", got)
		}
		r := h.do(call{method: "GET", path: "/api/v1/settings/email", token: owner})
		if r.data()["mode"] != "platform" || r.data()["reply_to"] != "destek@acme.test" || r.data()["sender"].(map[string]any)["from"] != "Messora <no-reply@messora.test>" {
			t.Fatalf("email settings: %s", r.raw)
		}
	})

	t.Run("enterprise: its own server, falling back to the platform sender", func(t *testing.T) {
		h.setPlan(tenant.DefaultID, "enterprise", "")
		// A server nobody listens on.
		h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/smtp", token: owner, body: map[string]any{"host": "127.0.0.1", "port": 1,
			"username": "", "from": "Acme <mail@acme.test>", "tls": "none"}}), 200, "")
		mail.Invalidate()
		if err := mail.Send(h.ctx(), msg); err != nil {
			t.Fatalf("fallback send: %v", err)
		}
		if got := <-platformGot; !strings.Contains(got, "no-reply@messora.test") {
			t.Fatalf("fallback went elsewhere: %s", got)
		}
		r := h.do(call{method: "GET", path: "/api/v1/settings/email", token: owner})
		if r.data()["mode"] != "custom" || r.data()["last_failure"] == nil {
			t.Fatalf("after a failure: %s", r.raw)
		}
	})

	t.Run("the platform panel's sender wins over the environment", func(t *testing.T) {
		panelAddr, panelGot := fakeSMTPServer(t)
		ph, pp := splitHostPort(t, panelAddr)
		r := h.adminDo(call{method: "PUT", path: "/email", body: map[string]any{"host": ph, "port": pp, "username": "resend",
			"password": panelPassword, "from": "Messora <noreply@mail.messora.test>", "tls": "none"}})
		h.expect(r, 200, "")
		if r.data()["source"] != "panel" || r.data()["password_set"] != true || r.data()["env"].(map[string]any)["configured"] != true {
			t.Fatalf("platform email: %s", r.raw)
		}
		var raw []byte
		if err := h.owner.QueryRow(context.Background(), `SELECT value::text::bytea FROM platform_settings WHERE key = 'email.smtp'`).Scan(&raw); err != nil ||
			strings.Contains(string(raw), panelPassword) {
			t.Fatalf("sealed password: %v %s", err, raw)
		}
		h.expect(h.adminDo(call{method: "POST", path: "/email/test", body: map[string]any{"to": "ops@messora.test"}}), 204, "")
		if got := <-panelGot; !strings.Contains(got, "noreply@mail.messora.test") {
			t.Fatalf("test e-mail: %s", got)
		}
		h.setPlan(tenant.DefaultID, "growth", "")
		if err := mail.Send(h.ctx(), msg); err != nil {
			t.Fatal(err)
		}
		if got := <-panelGot; !strings.Contains(got, "Reply-To: <destek@acme.test>") {
			t.Fatalf("platform mail through the panel's sender: %s", got)
		}
		if n := h.platformCount(`SELECT count(*) FROM platform_audit_logs WHERE action = 'platform.email_update'`); n != 1 {
			t.Fatalf("email audit = %d", n)
		}
	})
}
