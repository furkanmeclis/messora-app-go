//go:build integration

package app

import (
	"encoding/json"
	"strings"
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
		if h.pump(h.ctx(), 4) == 0 {
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

	// Listing and reading an automation that never ran (regression: NULL
	// last-run time made the list fail).
	r0 := h.do(call{method: "GET", path: "/api/v1/automations", token: token})
	h.expect(r0, 200, "")
	if first := r0.list()[0].(map[string]any); first["run_count"] != float64(0) || first["last_run_at"] != nil {
		t.Fatalf("never-run automation: %s", r0.raw)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/automations/" + id, token: token}), 200, "")

	// The same message delivered twice (different envelopes, Meta retry).
	at := time.Now()
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.auto-1", "Fiyat nedir?", "Ayşe Kaya", at))
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.auto-1", "Fiyat nedir?", "Ayşe Kaya", at.Add(time.Second)))
	h.drain()
	if n := h.runs(id); n != 1 {
		t.Fatalf("runs: %d", n)
	}
	var status string
	_ = h.pool.QueryRow(h.ctx(), "SELECT status FROM automation_runs WHERE automation_id = $1", id).Scan(&status)
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
	if g := h.do(call{method: "GET", path: "/api/v1/automations/" + id, token: token}); g.data()["run_count"] != float64(1) || g.data()["last_run_at"] == nil {
		t.Fatalf("automation stats: %s", g.raw)
	}
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
	_ = h.pool.QueryRow(h.ctx(), "SELECT id FROM messages WHERE source = 'automation'").Scan(&replyID)
	_, _ = h.pool.Exec(h.ctx(), "UPDATE messages SET status = 'sent', status_rank = 3, wamid = 'wamid.reply-1' WHERE id = $1", replyID)
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
	if err := h.pool.QueryRow(h.ctx(), "SELECT id, status FROM automation_runs WHERE automation_id = $1", id).Scan(&run, &status); err != nil || status != "waiting" {
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
	ctx := h.ctx()
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

// visited reads the trace of an automation's only run through the API.
func (h *harness) visited(token, automationID string) (status string, ids []string) {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/automations/" + automationID + "/runs", token: token})
	h.expect(r, 200, "")
	runs := r.list()
	if len(runs) != 1 {
		h.t.Fatalf("runs: %s", r.raw)
	}
	run := runs[0].(map[string]any)
	g := h.do(call{method: "GET", path: "/api/v1/automation-runs/" + run["id"].(string), token: token})
	h.expect(g, 200, "")
	for _, v := range g.data()["visited"].([]any) {
		ids = append(ids, v.(string))
	}
	if listed := run["visited"].([]any); len(listed) != len(ids) {
		h.t.Fatalf("list and detail disagree: %s / %s", r.raw, g.raw)
	}
	return g.data()["status"].(string), ids
}

func TestAutomationCanvasIDsLayoutAndRunTrace(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	ctx := h.ctx()

	// Step ids must be unique and well-formed.
	r := h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "bad", "trigger_type": "message_received",
		"steps": []any{map[string]any{"id": "x", "type": "add_tag", "tag": "a"}, map[string]any{"id": "x", "type": "add_tag", "tag": "b"}}}})
	h.expect(r, 422, "VALIDATION_ERROR")
	if !strings.Contains(string(r.raw), "steps[1].id") {
		t.Fatalf("duplicate id error: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "bad", "trigger_type": "message_received",
		"steps": []any{map[string]any{"id": "trigger", "type": "add_tag", "tag": "a"}}}}), 422, "VALIDATION_ERROR")

	// A branched flow as the canvas saves it: ids plus node positions.
	r = h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "Canvas", "active": true, "trigger_type": "message_received",
		"trigger": map[string]any{"match": "contains", "keywords": []string{"canvas"}},
		"steps": []any{
			map[string]any{"id": "tag", "type": "add_tag", "tag": "canvas"},
			map[string]any{"id": "check", "type": "condition", "if": map[string]any{"field": "contact.var1", "operator": "equals", "value": "gold"},
				"then": []any{map[string]any{"id": "vip", "type": "add_tag", "tag": "vip"}},
				"else": []any{
					map[string]any{"id": "std", "type": "set_field", "field": "var3", "value": "standard"},
					map[string]any{"id": "std-tag", "type": "add_tag", "tag": "standard"},
				}},
			map[string]any{"id": "done", "type": "set_field", "field": "var2", "value": "seen"},
		},
		"layout": map[string]any{
			"trigger": map[string]any{"x": 0, "y": 0}, "tag": map[string]any{"x": 0, "y": 140.5},
			"check": map[string]any{"x": 0, "y": 280}, "vip": map[string]any{"x": -160, "y": 420},
			"ghost": map[string]any{"x": 9, "y": 9}, // no such step: dropped
		},
	}})
	h.expect(r, 201, "")
	id := r.data()["id"].(string)
	layout := r.data()["layout"].(map[string]any)
	if len(layout) != 4 || layout["ghost"] != nil || layout["tag"].(map[string]any)["y"] != 140.5 {
		t.Fatalf("layout: %s", r.raw)
	}
	steps := r.data()["steps"].([]any)
	if steps[1].(map[string]any)["else"].([]any)[1].(map[string]any)["id"] != "std-tag" {
		t.Fatalf("step ids not kept: %s", r.raw)
	}

	// Moving nodes is not a new version.
	r = h.do(call{method: "PATCH", path: "/api/v1/automations/" + id, token: token, body: map[string]any{
		"layout": map[string]any{"trigger": map[string]any{"x": 5, "y": 5}, "done": map[string]any{"x": 0, "y": 700}}}})
	h.expect(r, 200, "")
	if r.data()["version"] != float64(1) || len(r.data()["layout"].(map[string]any)) != 2 {
		t.Fatalf("layout update: %s", r.raw)
	}

	// The contact is not gold: the run takes the "no" branch.
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.canvas-1", "canvas please", "", time.Now()))
	h.drain()
	status, trace := h.visited(token, id)
	if status != "completed" || strings.Join(trace, ",") != "tag,check,std,std-tag,done" {
		t.Fatalf("trace %s %v", status, trace)
	}

	// Older clients send steps without ids: the server names them.
	r = h.do(call{method: "POST", path: "/api/v1/automations", token: token, body: map[string]any{
		"name": "No ids", "trigger_type": "new_contact",
		"steps": []any{map[string]any{"type": "add_tag", "tag": "a"},
			map[string]any{"type": "condition", "if": map[string]any{"field": "message.body", "operator": "is_set"},
				"then": []any{map[string]any{"type": "add_tag", "tag": "b"}}}}}})
	h.expect(r, 201, "")
	named := map[string]bool{}
	for _, s := range r.data()["steps"].([]any) {
		named[s.(map[string]any)["id"].(string)] = true
		for _, c := range asList(s.(map[string]any)["then"]) {
			named[c.(map[string]any)["id"].(string)] = true
		}
	}
	if len(named) != 3 || named[""] {
		t.Fatalf("server-assigned ids: %s", r.raw)
	}
	var stored string
	_ = h.pool.QueryRow(ctx, "SELECT steps->1->'then'->0->>'id' FROM automations WHERE id = $1", r.data()["id"]).Scan(&stored)
	if !named[stored] {
		t.Fatalf("ids are not stored: %q", stored)
	}

	// An automation saved before step ids existed (steps and program
	// without ids, no layout) reads with positional ids, and its runs are
	// traced with the same ids.
	legacySteps := []automation.Step{
		{Type: "add_tag", Tag: "legacy"},
		{Type: "condition", If: &automation.Condition{Field: "message.body", Operator: "contains", Value: "yes"},
			Then: []automation.Step{{Type: "set_field", Field: "var4", Value: "yes"}},
			Else: []automation.Step{{Type: "set_field", Field: "var4", Value: "no"}}},
	}
	prog, err := automation.Compile(legacySteps)
	if err != nil {
		t.Fatal(err)
	}
	sj, _ := json.Marshal(legacySteps)
	pj, _ := json.Marshal(prog)
	var legacy string
	if err := h.pool.QueryRow(ctx, `INSERT INTO automations (name, active, trigger_type, trigger, steps, program, layout)
		VALUES ('Legacy', true, 'message_received', '{"match":"contains","keywords":["legacy"]}', $1, $2, NULL) RETURNING id::text`,
		sj, pj).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	g := h.do(call{method: "GET", path: "/api/v1/automations/" + legacy, token: token})
	h.expect(g, 200, "")
	ls := g.data()["steps"].([]any)
	if ls[0].(map[string]any)["id"] != "s0" || ls[1].(map[string]any)["else"].([]any)[0].(map[string]any)["id"] != "s1e0" ||
		len(g.data()["layout"].(map[string]any)) != 0 {
		t.Fatalf("legacy automation: %s", g.raw)
	}
	h.webhook(whatsapptest.InboundText(testPNID, testCustomer, "wamid.legacy-1", "legacy yes", "", time.Now()))
	h.drain()
	status, trace = h.visited(token, legacy)
	if status != "completed" || strings.Join(trace, ",") != "s0,s1,s1t0" {
		t.Fatalf("legacy trace %s %v", status, trace)
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}
