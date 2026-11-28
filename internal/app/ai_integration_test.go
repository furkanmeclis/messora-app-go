//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai/aitest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// aiKey is accepted by the emulated providers (a test fixture, not a key).
const aiKey = "sk-messora-test-key-0123456789" // gitleaks:allow (test fixture)

// useAIEmulator points the AI module at local emulated providers and
// enables the fake provider.
func (h *harness) useAIEmulator() *aitest.Server {
	h.t.Helper()
	emu := aitest.New(aiKey)
	h.t.Cleanup(emu.Close)
	bases := map[ai.Kind]string{}
	for k, v := range emu.BaseURLs() {
		bases[ai.Kind(k)] = v
	}
	h.svc.ai = ai.New(h.pool, h.sealer(), ai.Options{AllowFake: true, HTTP: emu.Client(), BaseURLs: bases}, testutil.DiscardLogger())
	h.svc.inbox.SetAI(h.svc.ai)
	wireCampaignAI(h.svc.ai, h.svc.campaigns, h.svc.inbox, h.svc.queries)
	h.svc.automation.SetAI(h.svc.ai)
	h.remount()
	return emu
}

func (h *harness) aiProvider(token, kind, name, key string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/ai/providers", token: token, body: map[string]any{"kind": kind, "name": name, "api_key": key}})
	h.expect(r, 201, "")
	if strings.Contains(string(r.raw), key) {
		h.t.Fatalf("create response contains the key: %s", r.raw)
	}
	return r.data()
}

func (h *harness) enableAI(token, providerID, model string) {
	h.t.Helper()
	r := h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: token, body: map[string]any{
		"enabled": true, "default_provider_id": providerID, "default_model": model,
		"disclosure":          map[string]any{"tr": "Yapay zekâ asistanı yanıtlıyor.", "en": "An AI assistant is answering."},
		"data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
}

func TestAIProvidersNeverLeakTheKey(t *testing.T) {
	h := newHarness(t)
	emu := h.useAIEmulator()
	owner, _ := h.setupOwner()
	if err := ai.EnsureSeed(h.ctx(), h.owner); err != nil {
		t.Fatal(err)
	}

	ids := map[string]string{}
	for _, kind := range []string{"openai", "anthropic", "gemini", "deepseek"} {
		p := h.aiProvider(owner, kind, "My "+kind, aiKey)
		if p["status"] != "ok" || p["key_last4"] != aiKey[len(aiKey)-4:] || p["checked_at"] == nil {
			t.Fatalf("%s: %v", kind, p)
		}
		if _, has := p["api_key"]; has {
			t.Fatal("api_key returned")
		}
		ids[kind] = p["id"].(string)
	}
	r := h.do(call{method: "GET", path: "/api/v1/ai/providers", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 4 || strings.Contains(string(r.raw), aiKey) {
		t.Fatalf("list: %s", r.raw)
	}
	// Sealed at rest: the key is not in the row in any form.
	if n := h.count(`SELECT count(*) FROM ai_providers WHERE position($1::bytea IN api_key_enc) > 0 OR name LIKE '%' || $2 || '%'`, []byte(aiKey), aiKey); n != 0 {
		t.Fatal("key stored in clear")
	}

	// Rotating to a wrong key is checked at once; nothing leaks.
	wrong := "sk-wrong-key-abcdefghijklmnop"
	r = h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + ids["openai"], token: owner, body: map[string]any{"api_key": wrong}})
	h.expect(r, 200, "")
	if r.data()["status"] != "invalid" || r.data()["key_last4"] != "mnop" || strings.Contains(string(r.raw), wrong) {
		t.Fatalf("rotate: %s", r.raw)
	}
	r = h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + ids["openai"], token: owner, body: map[string]any{"api_key": aiKey, "name": "OpenAI"}})
	h.expect(r, 200, "")
	if r.data()["status"] != "ok" || r.data()["name"] != "OpenAI" {
		t.Fatalf("rotate back: %s", r.raw)
	}
	// DeepSeek without balance.
	emu.Balance = false
	r = h.do(call{method: "POST", path: "/api/v1/ai/providers/" + ids["deepseek"] + "/check", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "no_balance" {
		t.Fatalf("balance: %s", r.raw)
	}
	emu.Balance = true

	// Custom endpoints: https only and only for DeepSeek / OpenAI-compatible.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + ids["deepseek"], token: owner,
		body: map[string]any{"base_url": "http://api.deepseek.com"}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + ids["anthropic"], token: owner,
		body: map[string]any{"base_url": "https://proxy.example.com"}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/ai/providers", token: owner,
		body: map[string]any{"kind": "openai", "name": "OpenAI", "api_key": aiKey}}), 422, "VALIDATION_ERROR") // duplicate name

	// Model sync from the emulated list endpoints.
	want := map[string]float64{"openai": 2, "anthropic": 2, "gemini": 1, "deepseek": 2}
	for kind, id := range ids {
		r = h.do(call{method: "POST", path: "/api/v1/ai/providers/" + id + "/sync", token: owner})
		h.expect(r, 200, "")
		if r.data()["models"] != want[kind] {
			t.Fatalf("%s sync: %s", kind, r.raw)
		}
	}
	r = h.do(call{method: "GET", path: "/api/v1/ai/models", token: owner})
	h.expect(r, 200, "")
	models := map[string]map[string]any{}
	for _, m := range r.list() {
		mm := m.(map[string]any)
		models[mm["model_id"].(string)] = mm
	}
	if len(models) != 7 {
		t.Fatalf("models: %s", r.raw)
	}
	opus := models["claude-opus-5-5"]
	caps := opus["capabilities"].(map[string]any)
	if opus["context_window"] != float64(1000000) || caps["tools"] != true || caps["vision"] != true {
		t.Fatalf("opus: %v", opus)
	}
	// OpenAI reports nothing: unknown stays null.
	if gpt := models["gpt-6.2-sol"]; gpt["context_window"] != nil || gpt["capabilities"].(map[string]any)["tools"] != nil {
		t.Fatalf("gpt: %v", gpt)
	}
	price := models["gpt-6.2-sol"]["price"].(map[string]any)
	if price["input"] != "2" || price["output"] != "10" || price["confirmed"] != false || price["source"] != "seed" {
		t.Fatalf("seed price: %v", price)
	}
	if models["claude-haiku-5-5"]["price"] == nil || models["deepseek-v4-pro"]["price"] != nil {
		t.Fatal("price matching")
	}

	// The audit trail records every change without the key.
	if n := h.count(`SELECT count(*) FROM audit_logs WHERE action LIKE 'ai.provider.%'`); n != 6 {
		t.Fatalf("audit rows = %d", n)
	}
	if n := h.count(`SELECT count(*) FROM audit_logs WHERE changes::text LIKE '%' || $1 || '%' OR changes::text LIKE '%' || $2 || '%'`, aiKey, wrong); n != 0 {
		t.Fatal("key in the audit log")
	}

	h.expect(h.do(call{method: "DELETE", path: "/api/v1/ai/providers/" + ids["gemini"], token: owner}), 204, "")
	if n := h.count(`SELECT count(*) FROM ai_models WHERE model_id = 'gemini-4-hydrogen'`); n != 0 {
		t.Fatal("models of a deleted provider remain")
	}
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/ai/providers/" + ids["gemini"], token: owner}), 404, "NOT_FOUND")
}

func TestAIProfilesSettingsAndTestConsole(t *testing.T) {
	h := newHarness(t)
	emu := h.useAIEmulator()
	owner, phoneID := h.connect()
	if err := ai.EnsureSeed(h.ctx(), h.owner); err != nil {
		t.Fatal(err)
	}
	fake := h.aiProvider(owner, "fake", "Fake", "fake-key-for-tests")
	openai := h.aiProvider(owner, "openai", "OpenAI", aiKey)

	r := h.do(call{method: "GET", path: "/api/v1/ai/settings", token: owner})
	h.expect(r, 200, "")
	if r.data()["enabled"] != false || r.data()["fake_provider_available"] != true {
		t.Fatalf("defaults: %s", r.raw)
	}

	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles", token: owner, body: map[string]any{
		"name": "Satış", "phone_number_id": phoneID, "system_prompt": "Sen Acme'nin satış asistanısın.",
		"handoff":         map[string]any{"on_request": false, "on_low_confidence": 0.6, "on_no_answer": true, "on_critical_intents": []string{"complaint"}, "on_unauthorized_action": true},
		"data_collection": map[string]any{"enabled": true, "fields": []any{map[string]any{"key": "email", "label": "E-posta", "required": true}}, "may_create_contact": true, "may_update_fields": false},
		"allowed_actions": []string{"add_tag"}, "disclosure_mode": "off"}})
	h.expect(r, 201, "")
	prof := r.data()
	pid := prof["id"].(string)
	if prof["handoff"].(map[string]any)["on_request"] != true {
		t.Fatal("handoff on request must be forced on")
	}
	if w := prof["warnings"].([]any); len(w) != 1 || w[0] != "disclosure_off" {
		t.Fatalf("warnings: %v", w)
	}
	// One profile per number.
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles", token: owner, body: map[string]any{"name": "Destek", "phone_number_id": phoneID}})
	h.expect(r, 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/ai/profiles", token: owner, body: map[string]any{"name": "X",
		"allowed_actions": []string{"automation:0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"}}}), 422, "VALIDATION_ERROR")
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles", token: owner, body: map[string]any{"name": "Destek"}})
	h.expect(r, 201, "")
	second := r.data()["id"].(string)
	r = h.do(call{method: "PUT", path: "/api/v1/ai/profiles/" + second, token: owner, body: map[string]any{"name": "Destek", "phone_number_id": phoneID}})
	h.expect(r, 422, "VALIDATION_ERROR")

	msgs := []any{
		map[string]any{"role": "user", "text": "Merhaba, fiyat alabilir miyim?"},
		map[string]any{"role": "operator_note", "text": "INTERNAL: müşteri eski bayi"},
	}
	// AI is off: the console refuses and the refused call is recorded.
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs}})
	h.expect(r, 409, "AI_DISABLED")
	if n := h.count(`SELECT count(*) FROM ai_usage WHERE purpose = 'test' AND status = 'error' AND error_kind = 'disabled'`); n != 1 {
		t.Fatalf("disabled call rows = %d", n)
	}

	h.enableAI(owner, fake["id"].(string), "fake-small")
	var seen ai.Request
	h.svc.ai.SetFakeScript(func(_ context.Context, req ai.Request) (ai.Response, error) {
		seen = req
		return ai.Response{Text: `{"reply":"Tabii!","confidence":0.8,"intent":"pricing","escalate":false,"escalate_reason":"","extracted_fields":{"email":null}}`,
			Structured: json.RawMessage(`{"reply":"Tabii!","confidence":0.8,"intent":"pricing","escalate":false,"escalate_reason":"","extracted_fields":{"email":null}}`),
			Usage:      ai.Usage{InputTokens: 1000, OutputTokens: 500}, StopReason: ai.StopEnd}, nil
	})
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs, "language": "tr"}})
	h.expect(r, 200, "")
	rep := r.data()["reply"].(map[string]any)
	if rep["reply"] != "Tabii!" || rep["intent"] != "pricing" || r.data()["model"] != "fake-small" || r.data()["cost"] != nil {
		t.Fatalf("test console: %s", r.raw)
	}
	if !strings.Contains(r.data()["instructions"].(string), "Sen Acme'nin satış asistanısın.") || !strings.Contains(seen.System, "E-posta") {
		t.Fatalf("instructions: %s", seen.System)
	}
	for _, m := range seen.Messages {
		if m.Role == ai.RoleUser && strings.Contains(m.Text, "INTERNAL") {
			t.Fatal("operator note given as user content")
		}
	}
	if seen.Output == nil || seen.Output.Name != ai.ReplySchemaName {
		t.Fatal("reply schema missing")
	}

	// A price for the fake models makes the cost appear.
	today := time.Now().UTC().Format(time.DateOnly)
	r = h.adminDo(call{method: "POST", path: "/ai-prices", body: map[string]any{"provider_kind": "fake", "model_pattern": "fake-*",
		"input": "1", "output": "2", "effective_from": today}})
	h.expect(r, 201, "")
	if r.data()["confirmed"] != true {
		t.Fatalf("admin-entered price counts as confirmed: %s", r.raw)
	}
	fakePrice := r.data()["id"].(string)
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs}})
	h.expect(r, 200, "")
	if r.data()["cost"] != "0.002" || r.data()["currency"] != "USD" { // 1000×1/1M + 500×2/1M
		t.Fatalf("cost: %s", r.raw)
	}
	// Editing the price never changes recorded costs.
	h.expect(h.adminDo(call{method: "PATCH", path: "/ai-prices/" + fakePrice, body: map[string]any{"input": "100"}}), 200, "")
	if n := h.count(`SELECT count(*) FROM ai_usage WHERE cost = 0.002`); n != 1 {
		t.Fatalf("recorded cost changed (%d)", n)
	}

	// The profile's provider override with an emulated OpenAI: cost from the seeded price.
	r = h.do(call{method: "PUT", path: "/api/v1/ai/profiles/" + pid, token: owner, body: map[string]any{"name": "Satış", "phone_number_id": phoneID,
		"provider_id": openai["id"], "model": "gpt-6.2-sol"}})
	h.expect(r, 200, "")
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs}})
	h.expect(r, 200, "")
	// usage: 1000 input + 200 cached + 100 output → 0.002 + 0.00002 + 0.001
	if r.data()["cost"] != "0.00302" || r.data()["reply"].(map[string]any)["intent"] != "greeting" || r.data()["provider_name"] != "OpenAI" {
		t.Fatalf("openai test: %s", r.raw)
	}
	if req, _ := emu.Last("openai", "/responses"); !strings.Contains(string(req.Body), `"store":false`) || strings.Contains(string(req.Body), aiKey) {
		t.Fatalf("openai request: %s", req.Body)
	}
	// Provider errors are typed: a rate limit is a 429 with Retry-After, recorded.
	emu.Enqueue("openai", aitest.Reply{Status: 429, Header: http.Header{"Retry-After": {"12"}},
		Body: `{"error":{"message":"Rate limit reached","type":"requests","code":"rate_limit_exceeded"}}`})
	r = h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs}})
	h.expect(r, 429, "AI_RATE_LIMITED")
	if r.header.Get("Retry-After") != "12" {
		t.Fatalf("retry-after = %q", r.header.Get("Retry-After"))
	}
	emu.Enqueue("openai", aitest.Reply{Status: 429, Body: `{"error":{"message":"quota","type":"insufficient_quota","code":"insufficient_quota"}}`})
	h.expect(h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs}}), 502, "AI_QUOTA_EXCEEDED")

	// Exactly one usage row per call: 1 disabled + 2 fake + 1 openai ok + 2 openai errors.
	if n := h.count(`SELECT count(*) FROM ai_usage WHERE purpose = 'test' AND profile_id = $1`, pid); n != 6 {
		t.Fatalf("usage rows = %d", n)
	}
	if n := h.count(`SELECT count(*) FROM ai_usage WHERE status = 'error' AND error_kind IN ('rate_limited', 'quota_exceeded')`); n != 2 {
		t.Fatalf("error rows = %d", n)
	}

	// Turning AI off stops everything again.
	r = h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: owner, body: map[string]any{"enabled": false, "default_provider_id": nil,
		"default_model": "", "disclosure": map[string]any{"tr": "x", "en": ""}, "data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/ai/profiles/" + pid + "/test", token: owner, body: map[string]any{"messages": msgs}}), 409, "AI_DISABLED")
	if n := h.count(`SELECT count(*) FROM audit_logs WHERE action IN ('ai.settings.update', 'ai.profile.create', 'ai.profile.update')`); n < 5 {
		t.Fatalf("audit rows = %d", n)
	}
	// Prices are platform data: the platform's audit trail has them.
	var platformRows int
	if err := h.owner.QueryRow(context.Background(), `SELECT count(*) FROM platform_audit_logs WHERE action IN ('ai.price.create', 'ai.price.update')`).Scan(&platformRows); err != nil || platformRows < 2 {
		t.Fatalf("platform audit rows = %d %v", platformRows, err)
	}

	r = h.do(call{method: "GET", path: "/api/v1/ai/profiles", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 2 {
		t.Fatalf("profiles: %s", r.raw)
	}
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/ai/profiles/" + second, token: owner}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/ai/profiles/" + second, token: owner}), 404, "NOT_FOUND")
}

func TestAIPricesConfirmImportAndSeedOnce(t *testing.T) {
	h := newHarness(t)
	h.useAIEmulator()
	owner, _ := h.setupOwner()
	ctx := h.ctx()
	if err := ai.EnsureSeed(ctx, h.owner); err != nil {
		t.Fatal(err)
	}
	r := h.do(call{method: "GET", path: "/api/v1/ai/prices", token: owner})
	h.expect(r, 200, "")
	if len(r.list()) != 7 {
		t.Fatalf("seed: %s", r.raw)
	}
	var deepseek map[string]any
	for _, p := range r.list() {
		pm := p.(map[string]any)
		if pm["confirmed"] != false || pm["effective_from"] != "2026-11-25" {
			t.Fatalf("seed rows start unconfirmed: %v", pm)
		}
		if pm["model_pattern"] == "deepseek-flash" {
			deepseek = pm
		}
	}
	if ws := deepseek["rules"].(map[string]any)["windows"].([]any); len(ws) != 2 {
		t.Fatalf("deepseek peak windows: %v", deepseek)
	}
	id := deepseek["id"].(string)
	r = h.adminDo(call{method: "PATCH", path: "/ai-prices/" + id, body: map[string]any{"confirm": true}})
	h.expect(r, 200, "")
	if r.data()["confirmed"] != true || r.data()["cached_input"] != "0.003" || len(r.data()["rules"].(map[string]any)["windows"].([]any)) != 2 {
		t.Fatalf("confirm keeps the figures: %s", r.raw)
	}
	r = h.adminDo(call{method: "PATCH", path: "/ai-prices/" + id, body: map[string]any{"notes": "checked"}})
	h.expect(r, 200, "")
	if r.data()["confirmed"] != true {
		t.Fatal("a note does not clear the confirmation")
	}
	r = h.adminDo(call{method: "PATCH", path: "/ai-prices/" + id, body: map[string]any{"output": "0.70", "cached_input": nil}})
	h.expect(r, 200, "")
	if r.data()["confirmed"] != false || r.data()["output"] != "0.7" || r.data()["cached_input"] != nil {
		t.Fatalf("changed figures need a new confirmation: %s", r.raw)
	}
	h.expect(h.adminDo(call{method: "PATCH", path: "/ai-prices/" + id, body: map[string]any{
		"rules": map[string]any{"windows": []any{map[string]any{"start": "01:00", "end": "01:00", "multiplier": "2"}}}}}), 422, "VALIDATION_ERROR")

	// LiteLLM import (uploaded, never fetched).
	doc := `{"gpt-6.2-sol":{"litellm_provider":"openai","mode":"chat","input_cost_per_token":2.5e-06,"output_cost_per_token":1e-05},
		"claude-new":{"litellm_provider":"anthropic","mode":"chat","input_cost_per_token":3e-06,"output_cost_per_token":1.5e-05},
		"mistral/x":{"litellm_provider":"mistral","input_cost_per_token":1e-06,"output_cost_per_token":1e-06}}`
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("effective_from", "2026-12-01")
	fw, _ := mw.CreateFormFile("file", "model_prices_and_context_window.json")
	_, _ = io.WriteString(fw, doc)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/api/v1/platform/ai-prices/import", &buf)
	req.Host = adminHost
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set(tenant.HostHeader, adminHost)
	req.Header.Set("Authorization", "Bearer "+h.platformSession())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(raw), `"prices":2`) || !strings.Contains(string(raw), `"skipped":1`) {
		t.Fatalf("import: %d %s", res.StatusCode, raw)
	}
	if n := h.count(`SELECT count(*) FROM ai_model_prices WHERE source = 'litellm' AND effective_from = '2026-12-01' AND confirmed_at IS NULL`); n != 2 {
		t.Fatalf("imported rows = %d", n)
	}

	// The seed is stored once: a deleted seed row does not come back.
	h.expect(h.adminDo(call{method: "DELETE", path: "/ai-prices/" + id}), 204, "")
	if err := ai.EnsureSeed(ctx, h.owner); err != nil {
		t.Fatal(err)
	}
	if n := h.count(`SELECT count(*) FROM ai_model_prices WHERE model_pattern = 'deepseek-flash'`); n != 0 {
		t.Fatal("deleted seed row restored")
	}
}

func TestAIPermissionMatrix(t *testing.T) {
	h := newHarness(t)
	h.useAIEmulator()
	owner, _ := h.setupOwner()
	agent, _ := h.invite(owner, "agent@example.com", "agent")
	manager, _ := h.invite(owner, "manager@example.com", "manager")
	admin, _ := h.invite(owner, "admin@example.com", "admin")
	p := h.aiProvider(admin, "fake", "Fake", "fake-key-for-tests")
	r := h.do(call{method: "POST", path: "/api/v1/ai/profiles", token: admin, body: map[string]any{"name": "P"}})
	h.expect(r, 201, "")
	prof := r.data()["id"].(string)

	reads := []call{
		{method: "GET", path: "/api/v1/ai/settings"}, {method: "GET", path: "/api/v1/ai/providers"}, {method: "GET", path: "/api/v1/ai/models"},
		{method: "GET", path: "/api/v1/ai/prices"}, {method: "GET", path: "/api/v1/ai/profiles"}, {method: "GET", path: "/api/v1/ai/profiles/" + prof},
	}
	writes := []call{
		{method: "PUT", path: "/api/v1/ai/settings", body: map[string]any{"enabled": true, "default_provider_id": nil, "default_model": "",
			"disclosure": map[string]any{"tr": "x", "en": "y"}, "data_sharing_notice": map[string]any{"tr": "", "en": ""}}},
		{method: "POST", path: "/api/v1/ai/providers", body: map[string]any{"kind": "fake", "name": "Other", "api_key": "fake-key-2-for-tests"}},
		{method: "PATCH", path: "/api/v1/ai/providers/" + p["id"].(string), body: map[string]any{"name": "Renamed"}},
		{method: "POST", path: "/api/v1/ai/providers/" + p["id"].(string) + "/check"},
		{method: "POST", path: "/api/v1/ai/profiles", body: map[string]any{"name": "Q"}},
		{method: "POST", path: "/api/v1/ai/profiles/" + prof + "/test", body: map[string]any{"messages": []any{map[string]any{"role": "user", "text": "hi"}}}},
	}
	for _, c := range append(append([]call{}, reads...), writes...) {
		c.token = agent
		h.expect(h.do(c), 403, "")
	}
	for _, c := range reads {
		c.token = manager
		h.expect(h.do(c), 200, "")
	}
	for _, c := range writes {
		c.token = manager
		h.expect(h.do(c), 403, "")
	}
	// Admins and owners manage.
	r = h.do(call{method: "PATCH", path: "/api/v1/ai/providers/" + p["id"].(string), token: owner, body: map[string]any{"name": "Renamed"}})
	h.expect(r, 200, "")
}
