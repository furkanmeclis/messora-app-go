//go:build integration

package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

func (h *harness) automation(token string, body map[string]any) string {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: body})
	h.expect(r, 201, "")
	return r.data()["id"].(string)
}

// drain pumps until no task is left.
func (h *harness) drain() {
	h.t.Helper()
	for range 50 {
		if h.pump(context.Background(), 4) == 0 {
			return
		}
	}
	h.t.Fatal("tasks keep coming")
}

func (h *harness) runs(automationID string) int {
	return h.count("SELECT count(*) FROM automation_runs WHERE automation_id = $1", automationID)
}

func TestAutomationRunsOncePerWebhookAndBranches(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()

	// Validation happens at save time.
	h.expect(h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "bad", "trigger_type": "message_received", "steps": []any{map[string]any{"type": "teleport"}}}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "bad", "trigger_type": "message_received", "trigger": map[string]any{"match": "contains"},
		"steps": []any{map[string]any{"type": "reply", "text": "x"}}}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "bad", "trigger_type": "message_received",
		"steps": []any{map[string]any{"type": "http_request", "url": "http://insecure.example"}}}}), 422, "VALIDATION_ERROR")

	id := h.automation(token, map[string]any{"name": "Price question", "active": true, "trigger_type": "message_received",
		"trigger": map[string]any{"match": "contains", "keywords": []string{"fiyat", "price"}},
		"steps": []any{
			map[string]any{"type": "add_tag", "tag": "asked-price"},
			map[string]any{"type": "condition", "if": map[string]any{"field": "contact.var1", "operator": "equals", "value": "gold"},
				"then": []any{map[string]any{"type": "reply", "text": "Merhaba {{first_name}}, Gold fiyatınız özel."}},
				"else": []any{map[string]any{"type": "reply", "text": "Fiyat listemiz: {{message}}"}}},
			map[string]any{"type": "set_field", "field": "var2", "value": "contacted"},
		}})

	// The same message delivered twice (different envelopes, Meta retry).
	at := time.Now()
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.auto-1", "Fiyat nedir?", "Ayşe Kaya", at))
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.auto-1", "Fiyat nedir?", "Ayşe Kaya", at.Add(time.Second)))
	h.drain()
	if n := h.runs(id); n != 1 {
		t.Fatalf("runs: %d", n)
	}
	var status string
	_ = h.pool.QueryRow(context.Background(), "SELECT status FROM automation_runs WHERE automation_id = $1", id).Scan(&status)
	if status != "completed" {
		t.Fatalf("run status %s", status)
	}
	if h.count("SELECT count(*) FROM messages WHERE source = 'automation' AND body = 'Fiyat listemiz: Fiyat nedir?'") != 1 {
		t.Fatal("else branch reply not queued exactly once")
	}
	if h.count("SELECT count(*) FROM contacts WHERE phone_e164 = $1 AND var2 = 'contacted'", "+"+testCustomer) != 1 {
		t.Fatal("set_field not applied")
	}
	// A message without the keyword does not start it.
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.auto-2", "teşekkürler", "", time.Now()))
	h.drain()
	if n := h.runs(id); n != 1 {
		t.Fatalf("unmatched message started a run: %d", n)
	}

	// Run log through the API.
	r := h.do(call{method: "GET", path: "/api/v1/automations/" + id + "/runs", token: token})
	h.expect(r, 200, "")
	run := r.list()[0].(map[string]any)["id"].(string)
	r = h.do(call{method: "GET", path: "/api/v1/automation-runs/" + run, token: token})
	steps := r.data()["steps"].([]any)
	if len(steps) != 4 { // add_tag, branch, reply, set_field
		t.Fatalf("logged steps: %s", r.raw)
	}
}

func TestAutomationsCannotTriggerEachOther(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	a := h.automation(token, map[string]any{"name": "Tag VIPs", "active": true, "trigger_type": "message_received",
		"steps": []any{map[string]any{"type": "add_tag", "tag": "vip"}, map[string]any{"type": "reply", "text": "Hoş geldiniz"}}})
	b := h.automation(token, map[string]any{"name": "Greet VIPs", "active": true, "trigger_type": "tag_added",
		"trigger": map[string]any{"tag": "vip"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "greeted"}}})
	c := h.automation(token, map[string]any{"name": "Follow up delivered", "active": true, "trigger_type": "message_status",
		"trigger": map[string]any{"status": "delivered"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "followed"}}})

	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.loop-1", "hi", "", time.Now()))
	h.drain()
	// Deliver the automation's reply: its status must not trigger C.
	var replyID string
	_ = h.pool.QueryRow(context.Background(), "SELECT id FROM messages WHERE source = 'automation'").Scan(&replyID)
	_, _ = h.pool.Exec(context.Background(), "UPDATE messages SET status = 'sent', status_rank = 3, wamid = 'wamid.reply-1' WHERE id = $1", replyID)
	h.webhook(whatsapptest.StatusUpdate(testPNID, "wamid.reply-1", "delivered", testCustomer, replyID, time.Now(), 0))
	h.drain()
	if h.runs(a) != 1 || h.runs(b) != 0 || h.runs(c) != 0 {
		t.Fatalf("runs a=%d b=%d c=%d; automation effects must not trigger automations", h.runs(a), h.runs(b), h.runs(c))
	}
	// The same tag added by a person does trigger B.
	r := h.do(call{method: "GET", path: "/api/v1/contacts?q=" + testCustomer[4:], token: token})
	cid := r.list()[0].(map[string]any)["id"].(string)
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/contacts/" + cid, token: token, body: map[string]any{"tags": []string{"vip", "manual"}}}), 200, "")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/contacts/" + cid, token: token, body: map[string]any{"tags": []string{"VIP"}}}), 200, "")
	h.drain()
	if h.runs(b) != 0 {
		// "vip" was already there (added by A), so it is not newly added.
		t.Fatalf("tag that already existed triggered B: %d", h.runs(b))
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/contacts/" + cid, token: token, body: map[string]any{"tags": []string{}}}), 200, "")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/contacts/" + cid, token: token, body: map[string]any{"tags": []string{"vip"}}}), 200, "")
	h.drain()
	if h.runs(b) != 1 {
		t.Fatalf("person adding a tag did not trigger B: %d", h.runs(b))
	}
}

func TestAutomationDelaySurvivesRestart(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	id := h.automation(token, map[string]any{"name": "Nudge", "active": true, "trigger_type": "new_contact",
		"steps": []any{
			map[string]any{"type": "add_tag", "tag": "new"},
			map[string]any{"type": "delay", "seconds": 3600},
			map[string]any{"type": "add_tag", "tag": "nudged"},
		}})
	h.webhook(whatsapptest.InboundText(testPNID, "905559990011", "wamid.new-1", "hello", "", time.Now()))
	h.drain()
	var run uuid.UUID
	var status string
	if err := h.pool.QueryRow(context.Background(), "SELECT id, status FROM automation_runs WHERE automation_id = $1", id).Scan(&run, &status); err != nil || status != "waiting" {
		t.Fatalf("run %s %v", status, err)
	}
	// A second message from the same contact is not a new contact.
	h.webhook(whatsapptest.InboundText(testPNID, "905559990011", "wamid.new-2", "again", "", time.Now()))
	h.drain()
	if h.runs(id) != 1 {
		t.Fatal("new_contact fired twice")
	}

	// "Restart": the queued resume task is gone, time passes, and a fresh
	// engine (as after a deploy) recovers the run through the reaper.
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, "DELETE FROM outbox_events WHERE published_at IS NULL"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE automation_runs SET resume_at = now() - interval '2 minutes' WHERE id = $1", run); err != nil {
		t.Fatal(err)
	}
	fresh := automation.New(h.pool, h.svc.messaging, nil, nil, testutil.DiscardLogger())
	if n, err := fresh.Reap(ctx); err != nil || n != 1 {
		t.Fatalf("reap: %d %v", n, err)
	}
	h.drain()
	_ = h.pool.QueryRow(ctx, "SELECT status FROM automation_runs WHERE id = $1", run).Scan(&status)
	if status != "completed" {
		t.Fatalf("after restart: %s", status)
	}
	if h.count("SELECT count(*) FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id JOIN contacts c ON c.id = ct.contact_id WHERE c.phone_e164 = '+905559990011' AND t.name IN ('new', 'nudged')") != 2 {
		t.Fatal("steps around the delay not applied")
	}
	// Running the resume again (duplicate task) changes nothing.
	if err := h.svc.automation.Run(ctx, run); err != nil {
		t.Fatal(err)
	}
	if h.count("SELECT count(*) FROM automation_run_steps WHERE run_id = $1", run) != 3 {
		t.Fatal("a finished run executed again")
	}
}

func TestAutomationMuteCooldownAndEditing(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	id := h.automation(token, map[string]any{"name": "Away", "active": true, "trigger_type": "message_received",
		"trigger": map[string]any{"cooldown_seconds": 3600},
		"steps":   []any{map[string]any{"type": "reply", "text": "Şu an kapalıyız."}, map[string]any{"type": "delay", "seconds": 600}, map[string]any{"type": "mute", "seconds": 3600}}})
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.m-1", "selam", "", time.Now()))
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.m-2", "orada mısınız", "", time.Now()))
	h.drain()
	if h.runs(id) != 1 {
		t.Fatalf("cooldown ignored: %d runs", h.runs(id))
	}
	// Editing the steps cancels the waiting run.
	r := h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{
		"steps": []any{map[string]any{"type": "mute", "seconds": 3600}}}})
	h.expect(r, 200, "")
	if r.data()["version"] != float64(2) {
		t.Fatalf("version: %s", r.raw)
	}
	if h.count("SELECT count(*) FROM automation_runs WHERE automation_id = $1 AND status = 'cancelled'", id) != 1 {
		t.Fatal("run in progress not cancelled after edit")
	}
	// Disabling stops new runs; deleting removes history.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{"active": false}}), 200, "")
	h.webhook(whatsapptest.InboundText(testPNID, "905558887766", "wamid.m-3", "selam", "", time.Now()))
	h.drain()
	if h.runs(id) != 1 {
		t.Fatal("inactive automation ran")
	}
	r = h.do(call{method: "GET", path: "/api/v1/automations", token: token})
	if r.list()[0].(map[string]any)["run_count"] != float64(1) {
		t.Fatalf("list: %s", r.raw)
	}
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/automations/" + id, token: token}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/automations/" + id, token: token}), 404, "NOT_FOUND")
}
