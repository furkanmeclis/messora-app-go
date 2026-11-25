//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

// addMarketingTemplate stores an approved MARKETING template ("spring_sale")
// on the connected account.
func (h *harness) addMarketingTemplate() {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), `INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, components)
		SELECT id, '779', 'spring_sale', 'en', 'MARKETING', 'APPROVED', '[{"type":"BODY","text":"Spring sale!"}]' FROM whatsapp_accounts`); err != nil {
		h.t.Fatal(err)
	}
}

func mustUUID(s string) uuid.UUID { return uuid.MustParse(s) }

func tpl(to, name string) map[string]any {
	return map[string]any{"to": to, "type": "template", "template": map[string]any{"name": name, "language": "en"}}
}

func (h *harness) contactID(phone string) string {
	h.t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(), "SELECT id::text FROM contacts WHERE phone_e164 = $1", phone).Scan(&id); err != nil {
		h.t.Fatalf("contact %s: %v", phone, err)
	}
	return id
}

func (h *harness) contactConsent(token, id string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/contacts/" + id, token: token})
	h.expect(r, 200, "")
	return r.data()["consent"].(map[string]any)
}

// An opted-out contact gets nothing business-initiated from any path:
// campaigns, the API, automations.
func TestConsentOptOutBlocksCampaignAPIAndAutomation(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.addMarketingTemplate()
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(3, "+90561")
	out := h.contactID("+9056100000001")

	// Marketing-only opt-out through the API (status + category).
	r := h.do(call{method: "POST", path: "/api/v1/contacts/" + out + "/consent", token: token,
		body: map[string]any{"status": "opted_out", "category": "marketing", "note": "asked on the phone"}})
	h.expect(r, 200, "")
	c := r.data()["consent"].(map[string]any)
	if c["marketing"].(map[string]any)["status"] != "opted_out" || c["service"].(map[string]any)["status"] != "unknown" || r.data()["opted_out"] != false {
		t.Fatalf("marketing opt-out: %s", r.raw)
	}
	if c["marketing"].(map[string]any)["source"] != "manual" || c["marketing"].(map[string]any)["reason"] != "asked on the phone" {
		t.Fatalf("source/reason: %v", c)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'contact.consent' AND entity_id = $1", out) != 1 {
		t.Fatal("consent change not audited")
	}
	ev := h.do(call{method: "GET", path: "/api/v1/contacts/" + out + "/consent-events", token: token})
	h.expect(ev, 200, "")
	if e := ev.list()[0].(map[string]any); e["kind"] != "opt_out" || e["category"] != "marketing" || e["source"] != "manual" || e["actor_name"] == nil {
		t.Fatalf("consent events: %s", ev.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + out + "/consent", token: token,
		body: map[string]any{"kind": "opt_out", "category": "marketing"}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + out + "/consent", token: token, body: map[string]any{}}), 422, "VALIDATION_ERROR")

	// API: marketing refused, utility (service consent) still allowed.
	h.expect(h.send(token, tpl("+9056100000001", "spring_sale")), 422, "CONTACT_OPTED_OUT")
	h.expect(h.send(token, tpl("+9056100000001", "order_update")), 202, "")

	// Campaign: the marketing opt-out is skipped at materialisation.
	body := map[string]any{"name": "Spring", "template_name": "spring_sale", "template_language": "en", "audience": map[string]any{"tag_id": tag}}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: body})
	h.expect(r, 201, "")
	camp := r.data()["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + camp + "/start", token: token}), 200, "")
	if st := h.runCampaign(camp, time.Minute); st != "completed" {
		t.Fatalf("campaign %s", st)
	}
	h.assertExactlyOnce(camp, 2)
	if h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND state = 'skipped' AND skip_reason = 'opted_out'", camp) != 1 {
		t.Fatal("opted-out recipient not skipped")
	}

	// Full (legacy) opt-out: no automation template either, even when the
	// automation reacts to the customer's own message.
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + out + "/consent", token: token, body: map[string]any{"kind": "opt_out"}}), 200, "")
	auto := h.automation(token, map[string]any{"name": "Send status", "active": true, "trigger_type": "message_received",
		"steps": []any{map[string]any{"type": "send_template", "template": map[string]any{"name": "order_update", "language": "en"}}}})
	before := h.count("SELECT count(*) FROM messages WHERE contact_id = $1 AND direction = 'outbound'", out)
	h.webhook(whatsapptest.InboundText(testPNID, "9056100000001", "wamid.consent-auto", "durum?", "", time.Now()))
	h.drain()
	if h.runs(auto) != 1 {
		t.Fatal("automation did not run")
	}
	if after := h.count("SELECT count(*) FROM messages WHERE contact_id = $1 AND direction = 'outbound'", out); after != before {
		t.Fatal("automation sent a template to an opted-out contact")
	}
	if h.count("SELECT count(*) FROM automation_run_steps WHERE outcome = 'skipped' AND detail->>'reason' = 'CONTACT_OPTED_OUT'") != 1 {
		t.Fatal("automation step not skipped for consent")
	}
	// The customer wrote after opting out: replying in that conversation is allowed.
	h.expect(h.send(token, map[string]any{"to": "+9056100000001", "type": "text", "text": map[string]any{"body": "Siparişiniz yolda."}}), 202, "")
}

// A contact who opts out after the campaign queued their message is not
// sent to: the send path checks consent again right before Meta.
func TestConsentRecheckedAtSendTime(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(5, "+90562")
	id := h.createCampaign(token, tag, 10, nil)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	h.pump(context.Background(), 1) // materialise
	h.pump(context.Background(), 1) // dispatch: five messages queued
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'pending'", id); got != 5 {
		t.Fatalf("queued: %d", got)
	}
	late := h.contactID("+9056200000003")
	h.expect(h.do(call{method: "POST", path: "/api/v1/contacts/" + late + "/consent", token: token, body: map[string]any{"kind": "opt_out"}}), 200, "")
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("campaign %s", st)
	}
	var status, code, class string
	if err := h.pool.QueryRow(context.Background(), "SELECT status, error_code, failure_class FROM messages WHERE campaign_id = $1 AND contact_id = $2", id, late).
		Scan(&status, &code, &class); err != nil {
		t.Fatal(err)
	}
	if status != "skipped" || code != "opted_out" || class != "blocked_by_consent" {
		t.Fatalf("late opt-out: %s %s %s", status, code, class)
	}
	for _, s := range h.meta.Sent() {
		if s.Message.To == "9056200000003" {
			t.Fatal("message sent to a contact who opted out")
		}
	}
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id); got != 4 {
		t.Fatalf("sent: %d", got)
	}
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/recipients", token: token})
	h.expect(r, 200, "")
	found := false
	for _, x := range r.list() {
		m := x.(map[string]any)
		found = found || (m["message_status"] == "skipped" && m["failure_class"] == "blocked_by_consent")
	}
	if !found {
		t.Fatalf("recipients: %s", r.raw)
	}
}

// WhatsApp's Stop for marketing blocks marketing but not a service reply
// inside the window; Resume clears it.
func TestUserPreferencesStopBlocksMarketingOnly(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.addMarketingTemplate()
	h.inbound(testCustomer, "wamid.up-1", "Merhaba", time.Now()) // opens the window
	id := h.contactID("+" + testCustomer)
	at := time.Now().Add(-time.Minute).Truncate(time.Second)
	h.webhook(whatsapptest.UserPreferences(testWABA, testPNID, whatsapp.UserPreference{WaID: testCustomer, Category: "marketing_messages",
		Value: "stop", Timestamp: json.Number(fmtUnix(at))}))
	h.drain()
	if c := h.contactConsent(token, id); c["meta_marketing_stopped_at"] == nil || c["marketing"].(map[string]any)["status"] != "unknown" {
		t.Fatalf("stop not applied: %v", c)
	}
	if h.count("SELECT count(*) FROM consent_events WHERE contact_id = $1 AND kind = 'meta_marketing_stopped' AND source = 'meta_user_preferences'", id) != 1 {
		t.Fatal("consent event missing")
	}
	h.expect(h.send(token, tpl("+"+testCustomer, "spring_sale")), 422, "MARKETING_STOPPED")
	h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "Tabii, yardımcı olayım."}}), 202, "")
	h.expect(h.send(token, tpl("+"+testCustomer, "order_update")), 202, "")

	// An older event arriving late does not undo the newer choice.
	h.webhook(whatsapptest.UserPreferences(testWABA, testPNID, whatsapp.UserPreference{WaID: testCustomer, Category: "marketing_messages",
		Value: "resume", Timestamp: json.Number(fmtUnix(at.Add(-time.Hour)))}))
	h.drain()
	if c := h.contactConsent(token, id); c["meta_marketing_stopped_at"] == nil {
		t.Fatal("an out-of-order resume cleared the stop")
	}
	// Resume clears the flag (our own marketing consent is untouched).
	h.webhook(whatsapptest.UserPreferences(testWABA, testPNID, whatsapp.UserPreference{WaID: testCustomer, Category: "marketing_messages",
		Value: "resume", Timestamp: json.Number(fmtUnix(at.Add(30 * time.Second)))}))
	h.drain()
	if c := h.contactConsent(token, id); c["meta_marketing_stopped_at"] != nil {
		t.Fatalf("resume not applied: %v", c)
	}
	if h.count("SELECT count(*) FROM consent_events WHERE contact_id = $1 AND kind = 'meta_marketing_resumed'", id) != 1 {
		t.Fatal("resume event missing")
	}
	h.expect(h.send(token, tpl("+"+testCustomer, "spring_sale")), 202, "")
}

// 131050 marks the customer as having stopped marketing; 131026 marks the
// number undeliverable until the customer writes again.
func TestMetaRefusalsFlagContacts(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.addMarketingTemplate()
	ctx := context.Background()

	// Synchronous 131050 on a marketing template.
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 131050})
	r := h.send(token, tpl("+"+testCustomer, "spring_sale"))
	h.expect(r, 202, "")
	h.drain()
	id := h.contactID("+" + testCustomer)
	var status, class string
	_ = h.pool.QueryRow(ctx, "SELECT status, failure_class FROM messages WHERE id = $1", r.data()["id"]).Scan(&status, &class)
	if status != "failed" || class != "user_stopped_marketing" {
		t.Fatalf("131050: %s %s", status, class)
	}
	if c := h.contactConsent(token, id); c["meta_marketing_stopped_at"] == nil {
		t.Fatal("131050 did not flag the contact")
	}
	if h.count("SELECT count(*) FROM consent_events WHERE contact_id = $1 AND kind = 'meta_marketing_stopped' AND source = 'meta_error'", id) != 1 {
		t.Fatal("131050 consent event missing")
	}
	h.expect(h.send(token, tpl("+"+testCustomer, "spring_sale")), 422, "MARKETING_STOPPED")

	// 131026 reported by the status webhook after Meta accepted the request.
	r = h.send(token, tpl("+"+testCustomer, "order_update"))
	h.expect(r, 202, "")
	h.drain()
	mid := r.data()["id"].(string)
	var wamid string
	_ = h.pool.QueryRow(ctx, "SELECT wamid FROM messages WHERE id = $1", mid).Scan(&wamid)
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "failed", testCustomer, mid, time.Now(), 131026))
	_ = h.pool.QueryRow(ctx, "SELECT status, failure_class FROM messages WHERE id = $1", mid).Scan(&status, &class)
	if status != "failed" || class != "undeliverable" {
		t.Fatalf("131026: %s %s", status, class)
	}
	c := h.contactConsent(token, id)
	if c["undeliverable_at"] == nil || c["undeliverable_code"] != "131026" {
		t.Fatalf("131026 did not flag the contact: %v", c)
	}
	h.expect(h.send(token, tpl("+"+testCustomer, "order_update")), 422, "CONTACT_UNDELIVERABLE")
	r = h.do(call{method: "GET", path: "/api/v1/contacts?reachability=undeliverable", token: token})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("reachability filter: %s", r.raw)
	}
	// The customer writes: reachable again.
	h.inbound(testCustomer, "wamid.back", "Merhaba", time.Now())
	if c := h.contactConsent(token, id); c["undeliverable_at"] != nil {
		t.Fatal("inbound did not clear undeliverable")
	}
	h.expect(h.send(token, tpl("+"+testCustomer, "order_update")), 202, "")
}

// 131049 defers the message by at least a day without holding a window
// slot; it is retried once due, exactly once.
func TestEcosystemLimitDefersWithoutHoldingTheWindow(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	tag := h.seedContacts(3, "+90563")
	id := h.createCampaign(token, tag, 1, nil) // one message in flight at a time
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 131049})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	for range 20 {
		h.pump(ctx, 1)
	}
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'deferred'", id); got != 1 {
		t.Fatalf("deferred: %d", got)
	}
	// The deferred message did not block the window: the others went out.
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id); got != 2 {
		t.Fatalf("sent while one was deferred: %d", got)
	}
	var deferredID string
	var wait time.Duration
	if err := h.pool.QueryRow(ctx, `SELECT id::text, extract(epoch FROM next_attempt_at - now())::bigint * interval '1 second'
		FROM messages WHERE campaign_id = $1 AND status = 'deferred'`, id).Scan(&deferredID, &wait); err != nil {
		t.Fatal(err)
	}
	if wait < 24*time.Hour-time.Minute {
		t.Fatalf("retry in %s; Meta asks for at least 24 hours", wait)
	}
	var cs string
	_ = h.pool.QueryRow(ctx, "SELECT status FROM campaigns WHERE id = $1", id).Scan(&cs)
	if cs != "running" {
		t.Fatalf("campaign %s while a message is deferred", cs)
	}
	// An early wake-up does not send.
	sent := len(h.meta.Sent())
	var retry *messaging.ErrRetry
	if err := h.svc.messaging.Send(ctx, mustUUID(deferredID)); !errors.As(err, &retry) || retry.After < 23*time.Hour {
		t.Fatalf("early send: %v", err)
	}
	if len(h.meta.Sent()) != sent {
		t.Fatal("deferred message sent early")
	}
	// A day later (simulated): the retry and the dispatcher are due.
	if _, err := h.pool.Exec(ctx, "UPDATE messages SET next_attempt_at = now() - interval '1 second' WHERE id = $1", deferredID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE outbox_events SET available_at = now() WHERE published_at IS NULL"); err != nil {
		t.Fatal(err)
	}
	h.pump(ctx, 1) // the retry goes out
	if got := h.count("SELECT count(*) FROM messages WHERE id = $1 AND status = 'sent'", deferredID); got != 1 {
		t.Fatal("deferred message not sent once due")
	}
	// While only deferred messages were left the dispatcher ticked slowly
	// (once a minute); its next tick completes the campaign.
	if _, err := h.pool.Exec(ctx, "UPDATE outbox_events SET available_at = now() WHERE published_at IS NULL"); err != nil {
		t.Fatal(err)
	}
	if st := h.runCampaign(id, 30*time.Second); st != "completed" {
		t.Fatalf("campaign %s", st)
	}
	h.assertExactlyOnce(id, 3)
	var attempts int
	_ = h.pool.QueryRow(ctx, "SELECT attempts FROM messages WHERE id = $1", deferredID).Scan(&attempts)
	if attempts != 2 {
		t.Fatalf("attempts: %d", attempts)
	}
}

// Keywords from the customer change consent, are confirmed inside the
// window and raise the subscription_request trigger.
func TestKeywordOptOutFromInbound(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	ctx := context.Background()
	auto := h.automation(token, map[string]any{"name": "On stop", "active": true, "trigger_type": "subscription_request",
		"trigger": map[string]any{"subscription": "stop"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "unsubscribed"}}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{"name": "bad", "trigger_type": "subscription_request",
		"trigger": map[string]any{"subscription": "maybe"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "x"}}}}), 422, "VALIDATION_ERROR")

	h.inbound(testCustomer, "wamid.kw-1", " İptal! ", time.Now())
	h.drain()
	id := h.contactID("+" + testCustomer)
	c := h.contactConsent(token, id)
	if c["marketing"].(map[string]any)["status"] != "opted_out" || c["marketing"].(map[string]any)["source"] != "keyword" ||
		c["service"].(map[string]any)["status"] != "unknown" {
		t.Fatalf("stop keyword: %v", c)
	}
	if h.count("SELECT count(*) FROM consent_events WHERE contact_id = $1 AND kind = 'opt_out' AND source = 'keyword' AND category = 'marketing'", id) != 1 {
		t.Fatal("keyword consent event missing")
	}
	if h.count("SELECT count(*) FROM conversation_events WHERE kind = 'opted_out'") != 1 {
		t.Fatal("conversation event missing")
	}
	if h.runs(auto) != 1 || h.count("SELECT count(*) FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id WHERE t.name = 'unsubscribed'") != 1 {
		t.Fatal("subscription_request trigger did not run the automation")
	}
	// The confirmation went out inside the window the keyword opened.
	if h.count("SELECT count(*) FROM messages WHERE contact_id = $1 AND direction = 'outbound' AND type = 'text' AND body LIKE 'You will no longer receive marketing%'", id) != 1 {
		t.Fatal("confirmation not queued")
	}

	// Resubscribe.
	h.inbound(testCustomer, "wamid.kw-2", "BAŞLAT", time.Now())
	h.drain()
	if c := h.contactConsent(token, id); c["marketing"].(map[string]any)["status"] != "resubscribed" {
		t.Fatalf("resume keyword: %v", c)
	}
	if h.runs(auto) != 1 {
		t.Fatal("stop-only automation ran on resume")
	}

	// An opt-out phrase marks a request and notifies the owner (nobody is
	// assigned); marketing is then blocked until an agent decides.
	h.inbound("905559998877", "wamid.kw-3", "Artık mesaj almak istemiyorum lütfen", time.Now())
	h.drain()
	other := h.contactID("+905559998877")
	if c := h.contactConsent(token, other); c["marketing"].(map[string]any)["status"] != "opt_out_requested" {
		t.Fatalf("request phrase: %v", c)
	}
	var owner string
	_ = h.pool.QueryRow(ctx, "SELECT id::text FROM users LIMIT 1").Scan(&owner)
	if h.count("SELECT count(*) FROM outbox_events WHERE topic = 'rt:personal:#' || $1::text AND payload->>'kind' = 'consent.opt_out_requested'", owner) != 1 {
		t.Fatal("owner not notified")
	}
	if h.count("SELECT count(*) FROM messages WHERE contact_id = $1 AND direction = 'outbound'", other) != 0 {
		t.Fatal("a request phrase must not be auto-confirmed")
	}
	// Disabled confirmations send nothing.
	r := h.do(call{method: "GET", path: "/api/v1/settings/consent", token: token})
	h.expect(r, 200, "")
	set := r.data()
	set["confirm"] = false
	h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/consent", token: token, body: set}), 200, "")
	h.inbound("905559998866", "wamid.kw-4", "STOP", time.Now())
	h.drain()
	if h.count("SELECT count(*) FROM messages WHERE contact_id = $1 AND direction = 'outbound'", h.contactID("+905559998866")) != 0 {
		t.Fatal("confirmation sent although turned off")
	}
}

// Preflight counts an audience by consent class for the template's category
// and follows the company's unknown-consent policy.
func TestCampaignPreflightCounts(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.addMarketingTemplate()
	ctx := context.Background()
	tag := h.seedContacts(7, "+90564")
	for _, q := range []string{
		"UPDATE contacts SET opted_out_at = now() WHERE phone_e164 = '+9056400000001'", // legacy full opt-out
		"UPDATE contacts SET meta_marketing_stopped_at = now() WHERE phone_e164 = '+9056400000002'",
		"UPDATE contacts SET undeliverable_at = now(), undeliverable_code = '131026' WHERE phone_e164 = '+9056400000003'",
		"UPDATE contacts SET marketing_consent = 'opt_out_requested' WHERE phone_e164 = '+9056400000004'",
		"UPDATE contacts SET marketing_consent = 'opted_in', service_consent = 'opted_in' WHERE phone_e164 = '+9056400000005'",
	} {
		if _, err := h.pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	preflight := func(template string) map[string]any {
		r := h.do(call{method: "POST", path: "/api/v1/campaigns/preflight", token: token, body: map[string]any{
			"audience": map[string]any{"tag_id": tag}, "template_name": template, "template_language": "en"}})
		h.expect(r, 200, "")
		return r.data()
	}
	reasons := func(list any) map[string]float64 {
		out := map[string]float64{}
		for _, x := range list.([]any) {
			m := x.(map[string]any)
			out[m["reason"].(string)] = m["count"].(float64)
		}
		return out
	}
	p := preflight("spring_sale")
	if p["category"] != "marketing" || p["unknown_marketing"] != "allow" || p["total"] != float64(7) || p["eligible"] != float64(3) ||
		p["warned"] != float64(2) || p["blocked"] != float64(4) || p["hard_blocked"] != float64(3) {
		t.Fatalf("marketing preflight: %v", p)
	}
	b := reasons(p["blocked_reasons"])
	if b["opted_out"] != 1 || b["meta_marketing_stopped"] != 1 || b["undeliverable"] != 1 || b["opt_out_requested"] != 1 {
		t.Fatalf("blocked reasons: %v", b)
	}
	if w := reasons(p["warned_reasons"]); w["consent_unknown"] != 2 {
		t.Fatalf("warned reasons: %v", w)
	}
	// Utility needs service consent only.
	p = preflight("order_update")
	if p["category"] != "utility" || p["eligible"] != float64(5) || p["blocked"] != float64(2) {
		t.Fatalf("utility preflight: %v", p)
	}
	// The company blocks unknown consent for marketing.
	r := h.do(call{method: "GET", path: "/api/v1/settings/consent", token: token})
	set := r.data()
	set["unknown_marketing"] = "block"
	h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/consent", token: token, body: set}), 200, "")
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'settings.consent_update'") != 1 {
		t.Fatal("settings change not audited")
	}
	p = preflight("spring_sale")
	if p["unknown_marketing"] != "block" || p["eligible"] != float64(1) || p["blocked"] != float64(6) || p["hard_blocked"] != float64(3) {
		t.Fatalf("blocking preflight: %v", p)
	}
	// The policy applies to sends too.
	h.expect(h.send(token, tpl("+9056400000006", "spring_sale")), 422, "CONSENT_UNKNOWN")
	h.expect(h.send(token, tpl("+9056400000005", "spring_sale")), 202, "")
	// Bad settings and requests are refused.
	set["keywords"] = map[string]any{"stop": []string{"STOP"}, "request": []string{}, "resume": []string{"stop"}}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/consent", token: token, body: set}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/preflight", token: token, body: map[string]any{
		"audience": map[string]any{"tag_id": tag}, "template_name": "nope", "template_language": "en"}}), 422, "VALIDATION_ERROR")
	// Contact list filters by consent.
	r = h.do(call{method: "GET", path: "/api/v1/contacts?marketing_consent=opted_out", token: token})
	h.expect(r, 200, "")
	if len(r.list()) != 1 {
		t.Fatalf("marketing_consent filter: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/contacts?marketing_consent=unknown&page=1", token: token})
	if len(r.list()) != 4 { // 2, 3, 6, 7
		t.Fatalf("unknown filter: %s", r.raw)
	}
}
