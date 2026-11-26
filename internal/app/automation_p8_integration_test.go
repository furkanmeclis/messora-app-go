//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

const (
	customer2 = "905550000002"
	customer3 = "905550000003"
	customer4 = "905550000004"
)

// fastForward makes every pending task and waiting run due now (a step's
// retry wait, a delay).
func (h *harness) fastForward() {
	h.t.Helper()
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, "UPDATE outbox_events SET available_at = now() WHERE published_at IS NULL"); err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE automation_runs SET resume_at = now() WHERE status = 'waiting'"); err != nil {
		h.t.Fatal(err)
	}
}

// scan runs the time-based trigger scanner and every resulting task.
func (h *harness) scan() int {
	h.t.Helper()
	n, err := h.svc.automation.Scan(context.Background())
	if err != nil {
		h.t.Fatal(err)
	}
	h.drain()
	return n
}

// ageThread moves a customer's conversation back in time: its window
// opened `ago` ago and every message is that much older.
func (h *harness) ageThread(customer string, ago time.Duration) {
	h.t.Helper()
	ctx := context.Background()
	secs := ago.Seconds()
	if _, err := h.pool.Exec(ctx, `UPDATE messages m SET created_at = m.created_at - make_interval(secs => $2), occurred_at = m.occurred_at - make_interval(secs => $2)
		FROM contacts c WHERE c.id = m.contact_id AND c.phone_e164 = $1`, "+"+customer, secs); err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE conversations v SET last_inbound_at = v.last_inbound_at - make_interval(secs => $2),
		last_message_at = v.last_message_at - make_interval(secs => $2) FROM contacts c WHERE c.id = v.contact_id AND c.phone_e164 = $1`,
		"+"+customer, secs); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) hasTag(customer, tag string) bool {
	return h.count(`SELECT count(*) FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id JOIN contacts c ON c.id = ct.contact_id
		WHERE c.phone_e164 = $1 AND t.name = $2`, "+"+customer, tag) == 1
}

func (h *harness) lastRun(token, automationID string) map[string]any {
	h.t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(context.Background(), "SELECT id FROM automation_runs WHERE automation_id = $1 ORDER BY id DESC LIMIT 1", automationID).Scan(&id); err != nil {
		h.t.Fatal(err)
	}
	r := h.do(call{method: "GET", path: "/api/v1/automation-runs/" + id.String(), token: token})
	h.expect(r, 200, "")
	return r.data()
}

func TestWindowClosingFiresOnceAtTheRightTime(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	id := h.automation(token, map[string]any{"name": "Window", "active": true, "trigger_type": "window_closing",
		"trigger": map[string]any{"minutes": 60},
		"steps":   []any{map[string]any{"type": "add_tag", "tag": "pencere"}}})

	// A waits for an answer, its window closes in 30 minutes.
	h.inbound(testCustomer, "wamid.wa1", "Merhaba, siparişim?", time.Now())
	// B got an answer from the team.
	h.inbound(customer2, "wamid.wb1", "Fiyat?", time.Now())
	h.expect(h.send(token, map[string]any{"to": "+" + customer2, "type": "text", "text": map[string]any{"body": "100 TL"}}), 202, "")
	// C's window closes in 4 hours; D's closed an hour ago.
	h.inbound(customer3, "wamid.wc1", "Selam", time.Now())
	h.inbound(customer4, "wamid.wd1", "Selam", time.Now())
	h.drain()
	h.ageThread(testCustomer, 23*time.Hour+30*time.Minute)
	h.ageThread(customer2, 23*time.Hour+30*time.Minute)
	h.ageThread(customer3, 20*time.Hour)
	h.ageThread(customer4, 25*time.Hour)

	if n := h.scan(); n != 1 {
		t.Fatalf("first scan started %d runs, want 1", n)
	}
	if !h.hasTag(testCustomer, "pencere") || h.hasTag(customer2, "pencere") || h.hasTag(customer3, "pencere") || h.hasTag(customer4, "pencere") {
		t.Fatal("window_closing fired for the wrong conversations")
	}
	// The next minutes' scans find the same window: nothing new.
	if n := h.scan(); n != 0 || h.runs(id) != 1 {
		t.Fatalf("second scan: %d new, %d runs", n, h.runs(id))
	}
	run := h.lastRun(token, id)
	if run["status"] != "completed" || !strings.HasPrefix(run["event"].(map[string]any)["key"].(string), "window:") {
		t.Fatalf("run: %v", run)
	}

	// Three hours and ten minutes later (fake clock): C's window closes in
	// 50 minutes; A's has closed; B is still answered.
	h.svc.automation.SetClock(func() time.Time { return time.Now().Add(3*time.Hour + 10*time.Minute) })
	if n := h.scan(); n != 1 || !h.hasTag(customer3, "pencere") {
		t.Fatalf("later scan started %d (C tagged %v)", n, h.hasTag(customer3, "pencere"))
	}
	h.svc.automation.SetClock(time.Now)

	// A writes again: a new window, which fires once more when it closes.
	h.inbound(testCustomer, "wamid.wa2", "Hâlâ bekliyorum", time.Now())
	h.drain()
	if n := h.scan(); n != 0 {
		t.Fatalf("a fresh window fired at once: %d", n)
	}
	h.ageThread(testCustomer, 23*time.Hour+45*time.Minute)
	if n := h.scan(); n != 1 || h.count("SELECT count(*) FROM automation_runs WHERE automation_id = $1 AND contact_id = (SELECT id FROM contacts WHERE phone_e164 = $2)",
		id, "+"+testCustomer) != 2 {
		t.Fatalf("new window: %d", n)
	}
}

func TestNoReplyTriggers(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	quiet := h.automation(token, map[string]any{"name": "Quiet customer", "active": true, "trigger_type": "no_reply",
		"trigger": map[string]any{"no_reply_from": "customer", "minutes": 60},
		"steps":   []any{map[string]any{"type": "add_tag", "tag": "sessiz"}}})
	waiting := h.automation(token, map[string]any{"name": "Waiting customer", "active": true, "trigger_type": "no_reply",
		"trigger": map[string]any{"no_reply_from": "agent", "minutes": 30},
		"steps":   []any{map[string]any{"type": "add_tag", "tag": "bekliyor"}}})

	// Customer 1 got our answer and went quiet; customer 2 waits for us.
	h.inbound(testCustomer, "wamid.n1", "Bilgi alabilir miyim?", time.Now())
	h.expect(h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "Tabii, nasıl yardımcı olabilirim?"}}), 202, "")
	h.inbound(customer2, "wamid.n2", "Kargom nerede?", time.Now())
	h.drain()
	if n := h.scan(); n != 0 {
		t.Fatalf("fired before the waiting time: %d", n)
	}
	h.ageThread(testCustomer, 20*time.Minute)
	h.ageThread(customer2, 40*time.Minute)
	if n := h.scan(); n != 1 || !h.hasTag(customer2, "bekliyor") || h.hasTag(testCustomer, "sessiz") {
		t.Fatalf("after 40 minutes: %d", n)
	}
	h.ageThread(testCustomer, time.Hour)
	if n := h.scan(); n != 1 || !h.hasTag(testCustomer, "sessiz") || h.hasTag(customer2, "sessiz") {
		t.Fatalf("after 80 minutes: %d", n)
	}
	if n := h.scan(); n != 0 || h.runs(quiet) != 1 || h.runs(waiting) != 1 {
		t.Fatalf("repeated: %d (runs %d/%d)", n, h.runs(quiet), h.runs(waiting))
	}
}

func TestAIStepWithAIOffFailsTheRun(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	id := h.automation(token, map[string]any{"name": "Intent", "active": true, "trigger_type": "message_received",
		"steps": []any{
			map[string]any{"type": "ai_detect_intent", "intents": []string{"price"}, "retry": map[string]any{"max": 3, "backoff": 60}},
			map[string]any{"type": "add_tag", "tag": "never"},
		}})
	h.say("wamid.off1", "fiyat nedir")
	run := h.lastRun(token, id)
	if run["status"] != "failed" || run["error_code"] != "ai_disabled" || !strings.Contains(run["error"].(string), "AI is turned off") {
		t.Fatalf("run: %v", run)
	}
	steps := run["steps"].([]any)
	if len(steps) != 1 || steps[0].(map[string]any)["outcome"] != "failed" {
		t.Fatalf("a configuration error is not retried: %v", steps)
	}
	if h.hasTag(testCustomer, "never") {
		t.Fatal("the run went on")
	}
}

// setupAIOnly switches the AI on with the fake provider and no profile.
func (h *harness) setupAIOnly(token string) *aiScript {
	h.t.Helper()
	h.useAIEmulator()
	fake := h.aiProvider(token, "fake", "Fake", aiKey)
	h.enableAI(token, fake["id"].(string), "fake-small")
	s := &aiScript{}
	h.svc.ai.SetFakeScript(s.run)
	return s
}

func lastUser(req ai.Request) string {
	if len(req.Messages) == 0 {
		return ""
	}
	return req.Messages[len(req.Messages)-1].Text
}

func TestAIDetectIntentBranchesAndExtractsFields(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	s := h.setupAIOnly(token)
	s.answer(func(req ai.Request) (ai.Response, error) {
		switch req.Output.Name {
		case "messora_intent":
			intent := "other"
			if strings.Contains(lastUser(req), "Latest customer message: fiyat") {
				intent = "price"
			}
			return structured(map[string]any{"intent": intent, "confidence": 0.9}), nil
		case "messora_fields":
			if !strings.Contains(lastUser(req), "Ayşe") {
				return structured(map[string]any{"fields": map[string]any{"f0": nil, "f1": nil, "f2": nil}}), nil
			}
			return structured(map[string]any{"fields": map[string]any{"f0": "Ayşe Kaya", "f1": "ayse@example.com", "f2": "İzmir"}}), nil
		}
		return ai.Response{}, errors.New("unexpected request " + req.Output.Name)
	})
	id := h.automation(token, map[string]any{"name": "Route", "active": true, "trigger_type": "message_received",
		"steps": []any{
			map[string]any{"id": "intent", "type": "ai_detect_intent", "intents": []string{"price", "human_request"}},
			map[string]any{"id": "branch", "type": "condition", "if": map[string]any{"field": "var.intent", "operator": "equals", "value": "price"},
				"then": []any{map[string]any{"id": "price", "type": "add_tag", "tag": "fiyat-soran"}},
				"else": []any{map[string]any{"id": "other", "type": "add_tag", "tag": "diger"}}},
			map[string]any{"id": "extract", "type": "ai_extract_fields", "fields": []any{
				map[string]any{"key": "name", "label": "Ad"}, map[string]any{"key": "email"}, map[string]any{"key": "attr:city", "label": "Şehir"}}},
		}})

	h.say("wamid.i1", "fiyat nedir? Ben Ayşe Kaya, ayse@example.com, İzmir")
	run := h.lastRun(token, id)
	if run["status"] != "completed" || run["vars"].(map[string]any)["intent"] != "price" {
		t.Fatalf("run: %v", run)
	}
	if !h.hasTag(testCustomer, "fiyat-soran") || h.hasTag(testCustomer, "diger") {
		t.Fatal("price branch not taken")
	}
	var name, email, city string
	if err := h.pool.QueryRow(context.Background(), "SELECT name, coalesce(email, ''), coalesce(attributes ->> 'city', '') FROM contacts WHERE phone_e164 = $1",
		"+"+testCustomer).Scan(&name, &email, &city); err != nil {
		t.Fatal(err)
	}
	if name != "Ayşe Kaya" || email != "ayse@example.com" || city != "İzmir" {
		t.Fatalf("fields: %q %q %q", name, email, city)
	}
	if h.count("SELECT count(*) FROM ai_usage WHERE purpose = 'automation' AND automation_run_id = $1", run["id"]) != 2 {
		t.Fatal("AI usage not recorded per step with the run")
	}
	steps := run["steps"].([]any)
	extract := steps[len(steps)-1].(map[string]any)["detail"].(map[string]any)
	if extract["source"] != automation.SourceAIUnverified || len(extract["fields"].([]any)) != 3 {
		t.Fatalf("extract detail: %v", extract)
	}

	h.say("wamid.i2", "merhaba")
	if !h.hasTag(testCustomer, "diger") || h.lastRun(token, id)["vars"].(map[string]any)["intent"] != "other" {
		t.Fatal("other branch not taken")
	}
}

func TestStepRetryWithIdempotentSend(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	s := h.setupAIOnly(token)
	calls := 0
	s.answer(func(ai.Request) (ai.Response, error) {
		calls++
		if calls == 1 {
			return ai.Response{}, &ai.Error{Kind: ai.KindOverloaded, Provider: ai.KindFake, Message: "scripted overload"}
		}
		return ai.Response{Text: "Siparişiniz yarın kargoda.", Usage: ai.Usage{InputTokens: 10, OutputTokens: 5}, StopReason: ai.StopEnd}, nil
	})
	id := h.automation(token, map[string]any{"name": "Generate", "active": true, "trigger_type": "message_received",
		"steps": []any{map[string]any{"id": "gen", "type": "ai_generate_message", "prompt": "Tell {{first_name}} the order ships tomorrow",
			"output": "send", "retry": map[string]any{"max": 2, "backoff": 1}}}})
	h.say("wamid.r1", "siparişim ne zaman gelir?")
	run := h.lastRun(token, id)
	if run["status"] != "waiting" || run["attempt"] != float64(1) {
		t.Fatalf("after the failed attempt: %v", run)
	}
	h.fastForward()
	h.drain()
	run = h.lastRun(token, id)
	if run["status"] != "completed" {
		t.Fatalf("after the retry: %v", run)
	}
	steps := run["steps"].([]any)
	if len(steps) != 2 || steps[0].(map[string]any)["outcome"] != "retrying" || steps[1].(map[string]any)["outcome"] != "ok" {
		t.Fatalf("steps: %v", steps)
	}
	if v := run["visited"].([]any); len(v) != 2 || v[0] != "gen" || v[1] != "gen" {
		t.Fatalf("trace: %v", v)
	}
	sent := "SELECT count(*) FROM messages WHERE source = 'automation' AND body = 'Siparişiniz yarın kargoda.'"
	if h.count(sent) != 1 {
		t.Fatal("message not queued exactly once")
	}

	// The step runs again (a recovered run): the AI answers again, the
	// message is not queued twice.
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, "UPDATE automation_runs SET status = 'running', pc = 0, finished_at = NULL WHERE id = $1", run["id"]); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.automation.Run(ctx, uuid.MustParse(run["id"].(string))); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.count(sent) != 1 || calls != 3 {
		t.Fatalf("rerun sent again (%d messages, %d AI calls)", h.count(sent), calls)
	}
	last := h.lastRun(token, id)["steps"].([]any)
	if d := last[len(last)-1].(map[string]any)["detail"].(map[string]any); d["deduplicated"] != true {
		t.Fatalf("rerun detail: %v", d)
	}
	if len(h.meta.Sent()) != 1 {
		t.Fatalf("meta got %d messages", len(h.meta.Sent()))
	}

	// A step that keeps failing ends the run after its last retry, and the
	// task goes to the dead-letter queue.
	s.answer(func(ai.Request) (ai.Response, error) {
		return ai.Response{}, &ai.Error{Kind: ai.KindUnavailable, Provider: ai.KindFake, Message: "down"}
	})
	h.say("wamid.r2", "tekrar")
	for range 2 {
		h.fastForward()
		h.drain()
	}
	run = h.lastRun(token, id)
	if run["status"] != "failed" || run["error_code"] != "retries_exhausted" || !strings.Contains(run["error"].(string), "after 2 retries") {
		t.Fatalf("exhausted run: %v", run)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE automation_runs SET status = 'running', finished_at = NULL, attempt = 2 WHERE id = $1", run["id"]); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.automation.Run(ctx, uuid.MustParse(run["id"].(string))); !errors.Is(err, automation.ErrRetriesExhausted) {
		t.Fatalf("exhausted Run returned %v", err)
	}
}

func TestEveryPresetInstallsAndCompiles(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	_, agentID := h.inviteAgent(token)
	r := h.do(call{method: "GET", path: "/api/v1/automations/presets", token: token})
	h.expect(r, 200, "")
	if len(r.list()) != 11 {
		t.Fatalf("presets: %d", len(r.list()))
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/automations/presets/nope/install", token: token}), 404, "")
	for _, x := range r.list() {
		p := x.(map[string]any)
		key := p["key"].(string)
		in := h.do(call{method: "POST", path: "/api/v1/automations/presets/" + key + "/install?lang=tr", token: token})
		h.expect(in, 201, "")
		a := in.data()
		if a["active"] != false || a["name"] != p["title"].(map[string]any)["tr"] || a["preset"].(map[string]any)["key"] != key {
			t.Fatalf("%s installed as %v", key, a)
		}
		id := a["id"].(string)
		setup := a["setup"].([]any)
		if len(setup) != len(p["setup"].([]any)) {
			t.Fatalf("%s setup %v", key, setup)
		}
		// Reading it back (the editor) keeps the setup and the preset.
		g := h.do(call{method: "GET", path: "/api/v1/automations/" + id, token: token}).data()
		if len(g["setup"].([]any)) != len(setup) || g["preset"] == nil {
			t.Fatalf("%s read back: %v", key, g)
		}
		enable := map[string]any{"active": true}
		if len(setup) == 0 {
			h.expect(h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: enable}), 200, "")
			continue
		}
		// Open setup: it cannot be enabled, but it can be saved.
		h.expect(h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: enable}), 422, "VALIDATION_ERROR")
		h.expect(h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{"steps": a["steps"]}}), 200, "")
		steps := a["steps"].([]any)
		for _, s := range steps {
			st := s.(map[string]any)
			switch st["type"] {
			case "send_template":
				st["template"] = map[string]any{"name": "order_update", "language": "en"}
			case "assign":
				st["assignee_id"] = agentID
			}
		}
		up := h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{"steps": steps, "active": true}})
		h.expect(up, 200, "")
		if len(up.data()["setup"].([]any)) != 0 || up.data()["active"] != true {
			t.Fatalf("%s after setup: %s", key, up.raw)
		}
	}
	if n := h.count("SELECT count(*) FROM automations WHERE preset_key IS NOT NULL AND active"); n != 11 {
		t.Fatalf("%d active presets", n)
	}
}

func (h *harness) installPreset(token, key string) string {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/automations/presets/" + key + "/install?lang=tr", token: token})
	h.expect(r, 201, "")
	id := r.data()["id"].(string)
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{"active": true}}), 200, "")
	return id
}

func TestSubscriptionPresetEndToEnd(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	id := h.installPreset(token, "unsubscribe_request")
	h.say("wamid.u1", "artık mesaj almak istemiyorum")
	run := h.lastRun(token, id)
	if run["status"] != "completed" {
		t.Fatalf("run: %v", run)
	}
	if h.count("SELECT count(*) FROM messages WHERE source = 'automation' AND body LIKE 'Pazarlama mesajlarımızı almak istemediğinizi anladık%'") != 1 {
		t.Fatal("confirmation question not sent")
	}
	if !h.hasTag(testCustomer, "abonelik-talebi") {
		t.Fatal("not tagged")
	}
	conv := h.conversationOf(testCustomer)
	if h.count("SELECT count(*) FROM conversation_notes WHERE conversation_id = $1 AND author_type = 'system' AND body LIKE 'Müşteri abonelikle ilgili yazdı%'", conv) != 1 {
		t.Fatal("team note missing")
	}
	// The customer confirms with the stop word: opted out and thanked.
	h.say("wamid.u2", "DUR")
	if h.runs(id) != 2 || h.lastRun(token, id)["status"] != "completed" {
		t.Fatalf("stop run: %v", h.lastRun(token, id))
	}
	if h.count("SELECT count(*) FROM messages WHERE source = 'automation' AND body LIKE 'Talebiniz alındı%'") != 1 {
		t.Fatalf("thanks not sent: %v", h.lastRun(token, id)["steps"])
	}
}

func TestHandoffPresetAndEscalationTrigger(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	token := env.owner
	esc := h.automation(token, map[string]any{"name": "Escalations", "active": true, "trigger_type": "ai_escalation",
		"trigger": map[string]any{"reasons": []string{"customer_request"}},
		"steps":   []any{map[string]any{"type": "add_tag", "tag": "eskalasyon"}, map[string]any{"type": "add_internal_note", "text": "AI devretti: {{message}}"}}})

	// The AI owns the conversation; the customer asks for a person: the AI
	// escalates and the automation reacts.
	h.say("wamid.e1", "Merhaba")
	env.script.answer(aiReply("Sizi bir temsilciye aktarıyorum.", 0.9, "human_request"))
	h.say("wamid.e2", "temsilci ile görüşmek istiyorum")
	if h.runs(esc) != 1 || !h.hasTag(testCustomer, "eskalasyon") {
		t.Fatalf("escalation trigger: %d runs", h.runs(esc))
	}
	if ev := h.lastRun(token, esc)["event"].(map[string]any); ev["reason"] != "customer_request" {
		t.Fatalf("event: %v", ev)
	}

	// The handoff preset on a second customer, whose conversation the AI
	// answers until the preset hands it to the team.
	env.script.answer(aiReply("Size yardımcı olayım.", 0.9, "other"))
	id := h.installPreset(token, "handoff_to_agent")
	h.inbound(customer2, "wamid.e3", "yetkili biriyle konuşabilir miyim", time.Now())
	h.drain()
	run := h.lastRun(token, id)
	if run["status"] != "completed" {
		t.Fatalf("handoff run: %v", run)
	}
	conv := h.conversationOf(customer2)
	c := h.conversation(token, conv)
	if c["owner_type"] != "human" || c["pending_handoff_id"] == nil {
		t.Fatalf("conversation after handoff: %v", c)
	}
	hs := h.handoffs(token, conv)
	if len(hs) == 0 || hs[0]["reason"] != "automation" || hs[0]["to_type"] != "pool" {
		t.Fatalf("handoffs: %v", hs)
	}
	if !has(h.activityKinds(token, conv), "handed_to_human") || !has(h.activityKinds(token, conv), "note_added") {
		t.Fatalf("activity: %v", h.activityKinds(token, conv))
	}
}

func TestReactionAndCampaignReplyTriggers(t *testing.T) {
	h := newHarness(t)
	token, phoneID := h.connect()
	neg := h.automation(token, map[string]any{"name": "Unhappy", "active": true, "trigger_type": "reaction",
		"trigger": map[string]any{"reaction": "negative"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "olumsuz"}}})
	anyR := h.automation(token, map[string]any{"name": "Any reaction", "active": true, "trigger_type": "reaction",
		"steps": []any{map[string]any{"type": "condition", "if": map[string]any{"field": "event.reaction", "operator": "equals", "value": "positive"},
			"then": []any{map[string]any{"type": "add_tag", "tag": "olumlu"}}}}})
	reply := h.automation(token, map[string]any{"name": "Campaign reply", "active": true, "trigger_type": "campaign_reply",
		"steps": []any{map[string]any{"type": "add_tag", "tag": "kampanya-cevabi"}}})

	// Our message, then reactions to it.
	h.inbound(testCustomer, "wamid.c0", "Merhaba", time.Now())
	m := h.send(token, map[string]any{"to": "+" + testCustomer, "type": "text", "text": map[string]any{"body": "Yeni ürünlerimiz geldi!"}})
	h.expect(m, 202, "")
	h.drain()
	ctx := context.Background()
	msgID := m.data()["id"].(string)
	if _, err := h.pool.Exec(ctx, "UPDATE messages SET wamid = 'wamid.ours-1' WHERE id = $1", msgID); err != nil {
		t.Fatal(err)
	}
	h.webhook(whatsapptest.InboundReaction(testPNID, testCustomer, "wamid.re1", "wamid.ours-1", "👍", time.Now()))
	h.webhook(whatsapptest.InboundReaction(testPNID, testCustomer, "wamid.re1", "wamid.ours-1", "👍", time.Now())) // redelivered
	h.webhook(whatsapptest.InboundReaction(testPNID, testCustomer, "wamid.re2", "wamid.ours-1", "🤔", time.Now())) // neutral
	h.drain()
	if h.runs(anyR) != 1 || h.runs(neg) != 0 || !h.hasTag(testCustomer, "olumlu") {
		t.Fatalf("positive reaction: any=%d neg=%d", h.runs(anyR), h.runs(neg))
	}
	h.webhook(whatsapptest.InboundReaction(testPNID, testCustomer, "wamid.re3", "wamid.ours-1", "👎", time.Now()))
	h.drain()
	if h.runs(neg) != 1 || !h.hasTag(testCustomer, "olumsuz") {
		t.Fatal("negative reaction")
	}
	if ev := h.lastRun(token, neg)["event"].(map[string]any); ev["source"] != "emoji" || ev["reaction"] != "negative" {
		t.Fatalf("reaction event: %v", ev)
	}
	if h.runs(reply) != 0 {
		t.Fatal("a reaction is not a campaign reply")
	}

	// A campaign reaches the customer: their next message is the campaign
	// reply the analytics count (docs/CAMPAIGN_ANALYTICS.md attribution),
	// and the trigger carries that campaign.
	camp := h.campaignReached(phoneID, "Promo", time.Now())
	h.webhook(whatsapptest.InboundReply(testPNID, testCustomer, "wamid.cr1", "wamid.ours-1", "Fiyatı ne kadar?", time.Now()))
	h.drain()
	if h.runs(reply) != 1 || !h.hasTag(testCustomer, "kampanya-cevabi") {
		t.Fatalf("campaign reply: %d", h.runs(reply))
	}
	var ev map[string]any
	raw := h.lastRun(token, reply)["event"]
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &ev)
	if ev["campaign_id"] != camp {
		t.Fatalf("campaign reply event: %v", ev)
	}
	// A second message is not a second reply to the campaign.
	h.inbound(testCustomer, "wamid.cr2", "Bekliyorum", time.Now())
	h.drain()
	if h.runs(reply) != 1 {
		t.Fatal("follow-up message counted as campaign reply")
	}
}

// installPresetWith installs a preset, fills its setup (assignee, template)
// and enables it.
func (h *harness) installPresetWith(token, key, assigneeID string) string {
	h.t.Helper()
	in := h.do(call{method: "POST", path: "/api/v1/automations/presets/" + key + "/install?lang=tr", token: token})
	h.expect(in, 201, "")
	a := in.data()
	steps := a["steps"].([]any)
	for _, s := range steps {
		st := s.(map[string]any)
		switch st["type"] {
		case "send_template":
			st["template"] = map[string]any{"name": "order_update", "language": "en"}
		case "assign":
			st["assignee_id"] = assigneeID
		}
	}
	id := a["id"].(string)
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{"steps": steps, "active": true}}), 200, "")
	return id
}

// Campaign reactions (P3: quick reply mapping, AI classification, manual
// label) start the reaction presets, once per recipient and polarity.
func TestCampaignReactionsStartReactionPresetsOnce(t *testing.T) {
	h := newHarness(t)
	token, phoneID := h.connect()
	_, agentID := h.inviteAgent(token)
	pos := h.installPresetWith(token, "positive_feedback", agentID)
	neg := h.automation(token, map[string]any{"name": "Unhappy", "active": true, "trigger_type": "reaction",
		"trigger": map[string]any{"reaction": "negative"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "olumsuz"}}})
	ctx := context.Background()

	h.inbound(testCustomer, "wamid.q0", "Merhaba", time.Now())
	h.drain()
	camp := h.campaignReached(phoneID, "Promo", time.Now())
	if _, err := h.pool.Exec(ctx, `UPDATE campaigns SET reaction_map = '{"Evet": "positive", "Hayır": "negative"}' WHERE id = $1`, camp); err != nil {
		t.Fatal(err)
	}
	// The customer taps "Evet" (Meta redelivers the webhook).
	h.webhook(inboundButton(testCustomer, "wamid.q1", "Evet", "Evet", time.Now()))
	h.webhook(inboundButton(testCustomer, "wamid.q1", "Evet", "Evet", time.Now()))
	h.drain()
	if h.runs(pos) != 1 || h.runs(neg) != 0 || !h.hasTag(testCustomer, "olumlu-donus") {
		t.Fatalf("quick reply: positive=%d negative=%d", h.runs(pos), h.runs(neg))
	}
	run := h.lastRun(token, pos)
	if run["status"] != "completed" {
		t.Fatalf("preset run: %v", run)
	}
	ev := run["event"].(map[string]any)
	if ev["source"] != "quick_reply" || ev["reaction"] != "positive" || ev["campaign_id"] != camp {
		t.Fatalf("event: %v", ev)
	}
	conv := h.conversationOf(testCustomer)
	if c := h.conversation(token, conv); c["assignee"] == nil || c["assignee"].(map[string]any)["id"] != agentID {
		t.Fatalf("not assigned to the sales agent: %v", c["assignee"])
	}

	// The same polarity again (an operator confirms it): no second run.
	var rid string
	if err := h.pool.QueryRow(ctx, "SELECT id::text FROM campaign_recipients WHERE campaign_id = $1", camp).Scan(&rid); err != nil {
		t.Fatal(err)
	}
	label := func(r string) {
		t.Helper()
		h.expect(h.do(call{method: "PUT", path: "/api/v1/campaigns/" + camp + "/recipients/" + rid + "/reaction", token: token,
			body: map[string]any{"reaction": r}}), 204, "")
		h.drain()
	}
	label("positive")
	if h.runs(pos) != 1 {
		t.Fatalf("relabelled positive started %d runs", h.runs(pos))
	}
	// The operator corrects it to negative: the negative automation, once.
	label("negative")
	label("neutral")
	label("negative")
	if h.runs(neg) != 1 || h.runs(pos) != 1 || !h.hasTag(testCustomer, "olumsuz") {
		t.Fatalf("manual negative: positive=%d negative=%d", h.runs(pos), h.runs(neg))
	}
	if ev := h.lastRun(token, neg)["event"].(map[string]any); ev["source"] != "manual" {
		t.Fatalf("manual event: %v", ev)
	}

	// An AI label of a second campaign's reply starts it as well.
	camp2 := h.campaignReached(phoneID, "Promo 2", time.Now())
	var rid2 uuid.UUID
	if err := h.pool.QueryRow(ctx, "SELECT id FROM campaign_recipients WHERE campaign_id = $1", camp2).Scan(&rid2); err != nil {
		t.Fatal(err)
	}
	negative := "negative"
	for range 2 {
		if _, err := h.svc.campaigns.SetReaction(ctx, uuid.MustParse(camp2), rid2, &negative, campaigns.ReactionSourceAI); err != nil {
			t.Fatal(err)
		}
	}
	h.drain()
	if h.runs(neg) != 2 {
		t.Fatalf("AI label: %d negative runs", h.runs(neg))
	}
	if ev := h.lastRun(token, neg)["event"].(map[string]any); ev["source"] != "ai" || ev["campaign_id"] != camp2 {
		t.Fatalf("AI event: %v", ev)
	}
}
