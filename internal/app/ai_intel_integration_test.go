//go:build integration

package app

import (
	"context"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
)

// leadProfile is a profile that builds the profile of unknown numbers.
func leadProfile(handoff bool) map[string]any {
	return map[string]any{"data_collection": map[string]any{"enabled": true, "may_create_contact": true, "may_update_fields": true,
		"handoff_when_complete": handoff, "fields": []any{
			map[string]any{"key": "name", "label": "Ad soyad", "required": true},
			map[string]any{"key": "attr:company", "label": "Firma", "required": true},
			map[string]any{"key": "email", "label": "E-posta", "required": false},
			map[string]any{"key": "attr:plan", "label": "Paket", "required": false, "type": "select", "options": []string{"Gold", "Silver"}},
		}}}
}

// byTurn answers the n-th reply turn with replies[n] (the last one repeats).
func byTurn(s *aiScript, replies ...func(ai.Request) (ai.Response, error)) {
	n := 0
	s.answer(func(req ai.Request) (ai.Response, error) {
		i := min(n, len(replies)-1)
		n++
		return replies[i](req)
	})
}

func fields(kv ...string) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return map[string]any{"extracted_fields": m}
}

func (h *harness) contactOf(token, customer string) map[string]any {
	h.t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(context.Background(), "SELECT id FROM contacts WHERE phone_e164 = $1", "+"+customer).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	r := h.do(call{method: "GET", path: "/api/v1/contacts/" + id.String(), token: token})
	h.expect(r, 200, "")
	return r.data()
}

func sourceOf(contact map[string]any, field string) map[string]any {
	for _, s := range contact["field_sources"].([]any) {
		if m := s.(map[string]any); m["field"] == field {
			return m
		}
	}
	return nil
}

func TestAILeadCaptureBuildsProfileOfUnknownNumber(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(leadProfile(true))
	byTurn(env.script,
		aiReply("Merhaba! Size nasıl yardımcı olabilirim? Adınızı öğrenebilir miyim?", 0.9, "pricing"),
		aiReply("Teşekkürler Ayşe Hanım. Hangi firmada çalışıyorsunuz?", 0.9, "pricing",
			fields("name", "Ayşe Yılmaz", "email", "gecersiz-eposta", "attr:plan", "gold")),
		aiReply("Harika, ekibimiz sizinle iletişime geçecek.", 0.9, "pricing", fields("attr:company", "Acme Ltd", "attr:health", "x")),
	)
	h.say("wamid.lc1", "Merhaba, fiyat almak istiyorum")
	c := h.contactOf(env.owner, testCustomer)
	if c["created_by_inbound_at"] == nil || c["profile_status"] != "collecting" {
		t.Fatalf("new contact: %v", c)
	}
	turns := env.script.turns()
	if !strings.Contains(turns[0].System, "## New customer") || !strings.Contains(turns[0].System, "Firma (attr:company)") ||
		!strings.Contains(turns[0].System, "Never ask for health") {
		t.Fatalf("lead capture rules missing: %s", turns[0].System)
	}

	h.say("wamid.lc2", "Ben Ayşe Yılmaz")
	c = h.contactOf(env.owner, testCustomer)
	if c["name"] != "Ayşe Yılmaz" || c["email"] != nil || c["attributes"].(map[string]any)["plan"] != "Gold" {
		t.Fatalf("collected fields (invalid e-mail must be dropped, select normalised): %v", c)
	}
	if s := sourceOf(c, "name"); s == nil || s["source"] != "ai" || s["verified"] != false {
		t.Fatalf("provenance: %v", c["field_sources"])
	}
	if p := c["profile_progress"].(map[string]any); p["required"] != float64(2) || p["filled"] != float64(1) {
		t.Fatalf("progress: %v", p)
	}

	h.say("wamid.lc3", "Acme Ltd'de çalışıyorum")
	id := h.conversationOf(testCustomer)
	c = h.contactOf(env.owner, testCustomer)
	attrs := c["attributes"].(map[string]any)
	if c["profile_status"] != "complete" || attrs["company"] != "Acme Ltd" || attrs["health"] != nil {
		t.Fatalf("complete profile (only configured fields stored): %v", c)
	}
	kinds := h.activityKinds(env.owner, id)
	if !has(kinds, "profile_completed") {
		t.Fatalf("activity: %v", kinds)
	}
	// handoff_when_complete: the team gets the new customer after the AI's last message.
	sent := h.metaTexts()
	if !strings.Contains(sent[len(sent)-1], "ekibimiz sizinle") {
		t.Fatalf("the AI's last message was not sent: %v", sent)
	}
	hs := h.handoffs(env.owner, id)
	if hs[0]["reason"] != "profile_complete" || hs[0]["collected_fields"].(map[string]any)["attr:company"] != "Acme Ltd" {
		t.Fatalf("handoff: %v", hs[0])
	}

	// The contacts list filter "AI tarafından oluşturulan".
	r := h.do(call{method: "GET", path: "/api/v1/contacts?ai_profile=true", token: env.owner})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("filter: %s", r.raw)
	}
	cid := c["id"].(string)

	// A person verifies the name.
	r = h.do(call{method: "POST", path: "/api/v1/contacts/" + cid + "/verify-fields", token: env.owner, body: map[string]any{"fields": []string{"name"}}})
	h.expect(r, 200, "")
	if s := sourceOf(r.data(), "name"); s["verified"] != true || s["verified_at"] == nil {
		t.Fatalf("verify: %s", r.raw)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'contact.verify_fields'") != 1 {
		t.Fatal("verification not audited")
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + cid + "/verify-fields", token: env.owner,
		body: map[string]any{"fields": []string{"phone"}}}), 422, "VALIDATION_ERROR")
}

func TestAIProfileRefusesSensitiveFields(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	r := h.do(call{method: "PUT", path: "/api/v1/ai/profiles/" + env.profileID, token: env.owner, body: map[string]any{"name": "Destek",
		"data_collection": map[string]any{"enabled": true, "may_create_contact": true, "may_update_fields": true,
			"fields": []any{map[string]any{"key": "attr:saglik", "label": "Sağlık durumu", "required": false}}}}})
	h.expect(r, 422, "VALIDATION_ERROR")
}

func TestAINeverOverwritesValuesPeopleSetOrVerified(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(map[string]any{"data_collection": map[string]any{"enabled": true, "may_create_contact": false, "may_update_fields": true,
		"fields": []any{map[string]any{"key": "name", "label": "Ad", "required": false}, map[string]any{"key": "email", "label": "E-posta", "required": false},
			map[string]any{"key": "attr:company", "label": "Firma", "required": false}}}})
	byTurn(env.script,
		aiReply("Merhaba!", 0.9, "greeting"),
		aiReply("Not aldım.", 0.9, "info", fields("name", "Ahmet", "attr:company", "Beta", "email", "a@example.com")),
		aiReply("Güncelledim.", 0.9, "info", fields("attr:company", "Gamma", "email", "b@example.com")),
	)
	h.say("wamid.ov1", "Merhaba")
	c := h.contactOf(env.owner, testCustomer)
	if c["profile_status"] != "new" { // an unknown number, but the profile may not build contacts
		t.Fatalf("lead capture without may_create_contact: %v", c["profile_status"])
	}
	cid := c["id"].(string)
	// A person sets the name: theirs, verified.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/contacts/" + cid, token: env.owner, body: map[string]any{"name": "Mehmet"}}), 200, "")
	h.say("wamid.ov2", "Adım Ahmet, firmam Beta")
	c = h.contactOf(env.owner, testCustomer)
	if c["name"] != "Mehmet" || c["attributes"].(map[string]any)["company"] != "Beta" || c["email"] != "a@example.com" {
		t.Fatalf("after turn 2: %v", c)
	}
	if s := sourceOf(c, "name"); s["source"] != "user" || s["verified"] != true {
		t.Fatalf("name provenance: %v", s)
	}
	// The company is verified; the e-mail stays the AI's unverified value.
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + cid + "/verify-fields", token: env.owner,
		body: map[string]any{"fields": []string{"attr:company"}}}), 200, "")
	h.say("wamid.ov3", "Firmam artık Gamma, e-postam b@example.com")
	c = h.contactOf(env.owner, testCustomer)
	if c["attributes"].(map[string]any)["company"] != "Beta" {
		t.Fatalf("a verified value was overwritten: %v", c["attributes"])
	}
	if c["email"] != "b@example.com" {
		t.Fatalf("the AI's own unverified value was not updated: %v", c["email"])
	}
	id := h.conversationOf(testCustomer)
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/activity?limit=200", token: env.owner})
	kept := 0
	for _, a := range r.list() {
		if d := a.(map[string]any)["data"].(map[string]any); d["status"] == "kept_existing" {
			kept++
		}
	}
	if kept != 2 { // name in turn 2, company in turn 3
		t.Fatalf("kept_existing items = %d", kept)
	}
}

// campaignReached records campaign name's message to the test customer on
// the number, sent at sentAt; it returns the campaign id.
func (h *harness) campaignReached(phoneID, name string, sentAt time.Time) string {
	h.t.Helper()
	ctx := context.Background()
	var cid, contactID uuid.UUID
	if err := h.pool.QueryRow(ctx, "SELECT id FROM contacts WHERE phone_e164 = $1", "+"+testCustomer).Scan(&contactID); err != nil {
		h.t.Fatal(err)
	}
	if err := h.pool.QueryRow(ctx, `INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language, audience)
		VALUES ($1, 'completed', $2, 'promo', 'en', '{"all": true}') RETURNING id`, name, phoneID).Scan(&cid); err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `INSERT INTO messages (phone_number_id, contact_id, direction, type, body, status, status_rank, source, campaign_id, sent_at, created_at)
		VALUES ($1, $2, 'outbound', 'template', '[template] promo', 'delivered', 4, 'campaign', $3, $4, $4)`, phoneID, contactID, cid, sentAt); err != nil {
		h.t.Fatal(err)
	}
	return cid.String()
}

func (h *harness) usageCampaigns(conversationID string) []string {
	h.t.Helper()
	rows, err := h.pool.Query(context.Background(), "SELECT coalesce(campaign_id::text, '') FROM ai_usage WHERE conversation_id = $1 AND purpose = 'reply' ORDER BY id", conversationID)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

func TestAIUsageAttributedToOneCampaign(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	today := time.Now().UTC().Format(time.DateOnly)
	h.expect(h.do(call{method: "POST", path: "/api/v1/ai/prices", token: env.owner, body: map[string]any{"provider_kind": "fake",
		"model_pattern": "fake-*", "input": "1", "output": "2", "effective_from": today}}), 201, "")
	start := time.Now().Add(-time.Minute)

	h.say("wamid.at1", "Merhaba") // no campaign yet: outside campaigns
	id := h.conversationOf(testCustomer)
	a := h.campaignReached(env.phoneID, "Kampanya A", time.Now())
	h.say("wamid.at2", "Kampanyanız hakkında bir sorum var")
	b := h.campaignReached(env.phoneID, "Kampanya B", time.Now())
	h.say("wamid.at3", "Peki ya yeni kampanya?")
	h.say("wamid.at4", "Bir şey daha")

	got := h.usageCampaigns(id)
	if len(got) != 4 || got[0] != "" || got[1] != a || got[2] != b || got[3] != b {
		t.Fatalf("attribution = %v (A %s, B %s)", got, a, b)
	}

	// A campaign message recorded late (sent before the first request):
	// the reconcile job fills only the unattributed row.
	var first time.Time
	if err := h.pool.QueryRow(context.Background(), "SELECT min(created_at) FROM ai_usage WHERE conversation_id = $1", id).Scan(&first); err != nil {
		t.Fatal(err)
	}
	c := h.campaignReached(env.phoneID, "Kampanya C", first.Add(-time.Minute))
	n, err := h.svc.ai.ReconcileAttribution(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("reconcile = %d, %v", n, err)
	}
	if got = h.usageCampaigns(id); got[0] != c || got[1] != a || got[2] != b {
		t.Fatalf("after reconcile = %v", got)
	}
	if n, _ := h.svc.ai.ReconcileAttribution(context.Background()); n != 0 {
		t.Fatal("reconcile is not idempotent")
	}

	// Per-campaign usage and the operational cost.
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/" + b + "/ai-usage", token: env.owner})
	h.expect(r, 200, "")
	d := r.data()
	// 2 requests × (100 input × 1/1M + 20 output × 2/1M) = 0.00028
	if d["ai_requests"] != float64(2) || d["ai_cost"] != "0.00028" || d["total_cost"] != "0.00028" || d["attribution_days"] != float64(7) {
		t.Fatalf("campaign usage: %s", r.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/campaigns/" + uuid.NewString() + "/ai-usage", token: env.owner}), 404, "")

	// The summary: totals and the campaign breakdown.
	q := url.Values{"from": {start.UTC().Format(time.RFC3339)}, "to": {time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "group_by": {"campaign"}}
	r = h.do(call{method: "GET", path: "/api/v1/ai/usage/summary?" + q.Encode(), token: env.owner})
	h.expect(r, 200, "")
	d = r.data()
	tot := d["totals"].(map[string]any)
	if tot["requests"] != float64(4) || tot["replies"] != float64(4) || tot["failed"] != float64(0) || tot["input_tokens"] != float64(400) ||
		tot["output_tokens"] != float64(80) || tot["cost"] != "0.00056" || d["ai_conversations"] != float64(1) || d["handoff_rate"] != float64(0) {
		t.Fatalf("totals: %s", r.raw)
	}
	byKey := map[string]map[string]any{}
	for _, g := range d["groups"].([]any) {
		m := g.(map[string]any)
		byKey[m["key"].(string)] = m
	}
	if len(byKey) != 3 || byKey[a]["requests"] != float64(1) || byKey[b]["requests"] != float64(2) || byKey[c]["requests"] != float64(1) ||
		byKey[b]["label"] != "Kampanya B" {
		t.Fatalf("groups: %v", byKey)
	}
	var sum float64
	for _, g := range byKey {
		sum += g["requests"].(float64)
	}
	if sum != tot["requests"] {
		t.Fatal("a request was counted for two campaigns")
	}
	for _, g := range []string{"model", "provider", "purpose", "day"} {
		q.Set("group_by", g)
		r = h.do(call{method: "GET", path: "/api/v1/ai/usage/summary?" + q.Encode(), token: env.owner})
		h.expect(r, 200, "")
		if gs := r.data()["groups"].([]any); len(gs) != 1 || gs[0].(map[string]any)["requests"] != float64(4) {
			t.Fatalf("group_by %s: %s", g, r.raw)
		}
	}
	q.Set("group_by", "colour")
	h.expect(h.do(call{method: "GET", path: "/api/v1/ai/usage/summary?" + q.Encode(), token: env.owner}), 422, "VALIDATION_ERROR")
}

func TestAIBudgetPausesAutoReplies(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	today := time.Now().UTC().Format(time.DateOnly)
	h.expect(h.do(call{method: "POST", path: "/api/v1/ai/prices", token: env.owner, body: map[string]any{"provider_kind": "fake",
		"model_pattern": "fake-*", "input": "1", "output": "2", "effective_from": today}}), 201, "")
	r := h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: env.owner, body: map[string]any{
		"enabled": true, "default_provider_id": env.providerID, "default_model": "fake-small", "monthly_budget": "0.0001",
		"disclosure":          map[string]any{"tr": "Yapay zekâ asistanı yanıtlıyor.", "en": "An AI assistant is answering."},
		"data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
	if r.data()["monthly_budget"] != "0.0001" {
		t.Fatalf("settings: %s", r.raw)
	}
	h.say("wamid.b1", "Merhaba") // costs 0.00014: the budget is used up
	if len(h.metaTexts()) != 1 {
		t.Fatal("the first reply was not sent")
	}
	h.say("wamid.b2", "Bir sorum daha var")
	id := h.conversationOf(testCustomer)
	if len(h.metaTexts()) != 1 {
		t.Fatal("the AI answered over budget")
	}
	if hs := h.handoffs(env.owner, id); hs[0]["reason"] != "ai_budget" {
		t.Fatalf("handoff: %v", hs[0])
	}
	h.say("wamid.b3", "Orada mısınız?") // a person owns it now: no new notice
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action = 'ai.budget_exceeded'"); n != 1 {
		t.Fatalf("budget notices = %d", n)
	}
	var ownerID uuid.UUID
	if err := h.pool.QueryRow(context.Background(), "SELECT id FROM users ORDER BY created_at LIMIT 1").Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	told := false
	for _, e := range h.rtEvents() {
		if e.channel == realtime.PersonalChannel(ownerID) && e.body["kind"] == "ai.budget_exceeded" {
			told = true
		}
	}
	if !told {
		t.Fatal("admins not told")
	}
	q := url.Values{"from": {time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}, "to": {time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}
	r = h.do(call{method: "GET", path: "/api/v1/ai/usage/summary?" + q.Encode(), token: env.owner})
	h.expect(r, 200, "")
	if b := r.data()["budget"].(map[string]any); b["exceeded"] != true || b["limit"] != "0.0001" || b["spent"] != "0.00014" {
		t.Fatalf("budget: %v", b)
	}
	if r.data()["escalations"] != float64(1) || r.data()["handoff_rate"] != float64(1) {
		t.Fatalf("handoff rate: %s", r.raw)
	}
	// Removing the budget.
	r = h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: env.owner, body: map[string]any{
		"enabled": true, "default_provider_id": env.providerID, "default_model": "fake-small", "monthly_budget": "",
		"disclosure":          map[string]any{"tr": "Yapay zekâ asistanı yanıtlıyor.", "en": "An AI assistant is answering."},
		"data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
	if r.data()["monthly_budget"] != nil {
		t.Fatalf("budget not removed: %s", r.raw)
	}
}

func TestCampaignReplyClassification(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	var classified atomic.Int32
	env.script.answer(func(req ai.Request) (ai.Response, error) {
		if req.Output != nil && req.Output.Name == ai.ClassifySchemaName {
			classified.Add(1)
			if !strings.Contains(req.Messages[0].Text, "ilgileniyorum") {
				t.Errorf("classified text: %v", req.Messages)
			}
			return structured(map[string]any{"sentiment": "positive", "confidence": 0.95, "reason": "Müşteri ilgileniyor."}), nil
		}
		return aiReply("Harika!", 0.9, "interested")(req)
	})
	// Off by default: nothing is classified.
	h.say("wamid.k0", "Merhaba")
	off := h.campaignReached(env.phoneID, "Kapalıyken", time.Now())
	h.say("wamid.k1", "Evet ilgileniyorum")
	if classified.Load() != 0 || h.count("SELECT count(*) FROM campaign_reply_classifications") != 0 {
		t.Fatal("classified while the setting is off")
	}
	_ = off
	r := h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: env.owner, body: map[string]any{
		"enabled": true, "default_provider_id": env.providerID, "default_model": "fake-small", "classify_campaign_replies": true,
		"disclosure":          map[string]any{"tr": "Yapay zekâ asistanı yanıtlıyor.", "en": "An AI assistant is answering."},
		"data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
	camp := h.campaignReached(env.phoneID, "Kasım", time.Now())
	h.say("wamid.k2", "Evet, ilgileniyorum!")
	h.say("wamid.k3", "Ne zaman başlıyor?") // a second reply is not classified again
	if classified.Load() != 1 {
		t.Fatalf("classifications = %d", classified.Load())
	}
	var status, sentiment string
	var usage *uuid.UUID
	if err := h.pool.QueryRow(context.Background(), "SELECT status, sentiment, ai_usage_id FROM campaign_reply_classifications WHERE campaign_id = $1",
		camp).Scan(&status, &sentiment, &usage); err != nil {
		t.Fatal(err)
	}
	if status != "done" || sentiment != "positive" || usage == nil {
		t.Fatalf("stored: %s %s %v", status, sentiment, usage)
	}
	if h.count("SELECT count(*) FROM ai_usage WHERE purpose = 'classify' AND campaign_id = $1", camp) != 1 {
		t.Fatal("classification usage not attributed to the campaign")
	}
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + camp + "/ai-usage", token: env.owner})
	h.expect(r, 200, "")
	if r.data()["reply_sentiments"].(map[string]any)["positive"] != float64(1) {
		t.Fatalf("sentiments: %s", r.raw)
	}
}
