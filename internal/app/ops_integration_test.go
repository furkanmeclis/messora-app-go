//go:build integration

package app

import (
	"context"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
)

// sqlOne runs SQL against the test database and returns the first column of
// the first row (or "").
func (h *harness) sqlOne(query string, args ...any) string {
	h.t.Helper()
	var out *string
	rows, err := h.pool.Query(context.Background(), query, args...)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&out); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		h.t.Fatal(err)
	}
	if out == nil {
		return ""
	}
	return *out
}

func (h *harness) overview(token, query string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/ops/overview" + query, token: token})
	h.expect(r, 200, "")
	return r.data()
}

func section(t *testing.T, o map[string]any, name string) map[string]any {
	t.Helper()
	s, ok := o[name].(map[string]any)
	if !ok {
		t.Fatalf("section %s missing: %v", name, o)
	}
	return s
}

func opsIDs(list any, key string) []string {
	var out []string
	for _, x := range list.([]any) {
		out = append(out, x.(map[string]any)[key].(string))
	}
	return out
}

func opsHints(h *harness, section string) int {
	n := 0
	for _, e := range h.eventsOfType(realtime.OpsChannel, realtime.TypeOpsUpdated) {
		if e.body["section"] == section {
			n++
		}
	}
	return n
}

func TestOpsOverview(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	agent, _ := h.invite(owner, "agent@example.com", "agent")
	ownerID := h.do(call{method: "GET", path: "/api/v1/me", token: owner}).data()["user"].(map[string]any)["id"].(string)

	// A number, a running and a limit-paused campaign, and a draft (not listed).
	pn := h.sqlOne(`WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, currency)
	                           VALUES ('Ops', '5550001', '\x00', 'TRY') RETURNING id)
	                INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, quality_rating)
	                SELECT id, '5550002', '+90 555 000 00 02', 'YELLOW' FROM a RETURNING id::text`)
	campaign := func(name, status string, extra string) string {
		return h.sqlOne(`INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language, template_params,
		                                       audience, total_recipients, started_at, paused_reason, send_rate)
		                 VALUES ($1, $2, $3, 'promo', 'tr', '{}', '{"all":true}', 100, now(), nullif($4, ''), 10)
		                 RETURNING id::text`, name, status, pn, extra)
	}
	running := campaign("Running sale", "running", "")
	limited := campaign("Waiting for the limit", "paused_limit", "messaging_limit")
	campaign("Draft", "draft", "")

	// Conversations: an unassigned escalation (everyone with
	// conversations.read sees it), an escalation assigned to the owner (the
	// agent does not), and an unassigned customer waiting.
	conv := func(phone, name string, assignee any, escalate bool) string {
		id := h.sqlOne(`WITH c AS (INSERT INTO contacts (phone_e164, name) VALUES ($1, $2) RETURNING id),
		                     m AS (INSERT INTO messages (phone_number_id, contact_id, direction, type, body, status, source)
		                           SELECT $3, id, 'inbound', 'text', 'Merhaba, yardım lazım', 'received', 'webhook' FROM c
		                           RETURNING id, contact_id, created_at)
		               INSERT INTO conversations (contact_id, phone_number_id, assignee_id, last_message_id, last_message_at,
		                                          last_inbound_at, unread_count, owner_type)
		               SELECT contact_id, $3, $4::uuid, id, created_at, created_at, 1, CASE WHEN $5 THEN 'human' ELSE 'none' END FROM m
		               RETURNING id::text`, phone, name, pn, assignee, escalate)
		if escalate {
			h.sqlOne(`WITH x AS (INSERT INTO conversation_handoffs (conversation_id, from_type, to_type, reason, reason_detail, actor_type)
			                     VALUES ($1, 'ai', 'pool', 'customer_request', 'İnsan istedi', 'ai') RETURNING id)
			          UPDATE conversations SET pending_handoff_id = x.id, ai_mode = 'off' FROM x WHERE conversations.id = $1
			          RETURNING x.id::text`, id)
		}
		return id
	}
	pooled := conv("+905550000011", "Ayşe Havuz", nil, true)
	mine := conv("+905550000012", "Burak Sahip", ownerID, true)
	waiting := conv("+905550000013", "Cem Bekleyen", nil, false)
	withAI := conv("+905550000014", "Deniz AI", nil, false)
	h.sqlOne(`UPDATE conversations SET owner_type = 'ai', ai_mode = 'auto_reply' WHERE id = $1`, withAI)

	// A failed and a running automation run.
	auto := h.sqlOne(`INSERT INTO automations (name, trigger_type, steps, program) VALUES ('Karşılama', 'message_received', '[]', '[]')
	                  RETURNING id::text`)
	hintsBefore := opsHints(h, "automations")
	h.sqlOne(`INSERT INTO automation_runs (automation_id, automation_version, event_key, event, status, error, finished_at, conversation_id)
	          VALUES ($1, 1, 'e1', '{}', 'failed', 'HTTP step: 500', now(), $2),
	                 ($1, 1, 'e2', '{}', 'running', NULL, NULL, NULL)`, auto, mine)
	if got := opsHints(h, "automations"); got != hintsBefore+2 {
		t.Fatalf("automation hints = %d, want %d (one per started run)", got, hintsBefore+2)
	}
	h.sqlOne(`UPDATE automation_runs SET status = 'waiting' WHERE event_key = 'e2'`)
	if got := opsHints(h, "automations"); got != hintsBefore+2 {
		t.Fatalf("waiting is still in progress, no hint expected: %d", got)
	}

	// A job in the dead letter announces itself (worker error handler).
	alertHints := opsHints(h, "alerts")
	h.failJob("whatsapp:send", `{"message_id":"0193a5a0-0000-7000-8000-000000000001"}`, "ops-job")
	if got := opsHints(h, "alerts"); got != alertHints+1 {
		t.Fatalf("failed-job hints = %d, want %d", got, alertHints+1)
	}

	start := time.Now()
	o := h.overview(owner, "?tz=Europe/Istanbul")
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("overview took %v", d)
	}
	for _, s := range []string{"campaigns", "conversations", "activity", "automations", "alerts"} {
		section(t, o, s)
	}
	c := section(t, o, "campaigns")
	counts := c["counts"].(map[string]any)
	if counts["running"] != float64(1) || counts["paused"] != float64(1) || counts["paused_limit"] != float64(1) {
		t.Fatalf("campaign counts: %v", counts)
	}
	if got := opsIDs(c["active"], "id"); len(got) != 2 || got[0] != running || got[1] != limited {
		t.Fatalf("active campaigns %v, want running then limited", got)
	}
	first := c["active"].([]any)[0].(map[string]any)
	if first["currency"] != "TRY" || first["eta_seconds"] == nil || first["counts"] == nil {
		t.Fatalf("running campaign: %v", first)
	}
	if q := c["queue"].([]any); len(q) != 1 {
		t.Fatalf("queue: %v", q)
	}

	cv := section(t, o, "conversations")
	if got := opsIDs(cv["awaiting_human"], "conversation_id"); len(got) != 2 || cv["awaiting_human_total"] != float64(2) {
		t.Fatalf("owner awaiting human: %v", got)
	}
	if got := opsIDs(cv["unassigned"], "conversation_id"); len(got) != 1 || got[0] != waiting {
		t.Fatalf("unassigned waiting: %v", got)
	}
	if cv["ai_active"] != float64(1) || len(cv["recent_inbound"].([]any)) != 4 {
		t.Fatalf("ai_active %v, recent inbound %v", cv["ai_active"], cv["recent_inbound"])
	}

	au := section(t, o, "automations")
	if au["failed_total"] != float64(1) || au["running_total"] != float64(1) {
		t.Fatalf("automations: %v", au)
	}
	failed := au["failed"].([]any)[0].(map[string]any)
	if failed["automation_name"] != "Karşılama" || failed["error"] != "HTTP step: 500" || failed["conversation_id"] != mine {
		t.Fatalf("failed run: %v", failed)
	}

	al := section(t, o, "alerts")
	if al["failed_jobs"] != float64(1) || len(al["numbers"].([]any)) != 1 {
		t.Fatalf("alerts: %v", al)
	}

	// The agent: conversations only, and only what the agent may open.
	o = h.overview(agent, "")
	for _, s := range []string{"campaigns", "activity", "automations", "alerts"} {
		if _, ok := o[s]; ok {
			t.Fatalf("agent got section %s", s)
		}
	}
	cv = section(t, o, "conversations")
	if got := opsIDs(cv["awaiting_human"], "conversation_id"); len(got) != 1 || got[0] != pooled {
		t.Fatalf("agent awaiting human: %v", got)
	}
	for _, m := range cv["recent_inbound"].([]any) {
		if m.(map[string]any)["conversation_id"] == mine {
			t.Fatal("agent sees a message of a conversation assigned to someone else")
		}
	}
	if len(cv["recent_inbound"].([]any)) != 3 || cv["ai_active"] != float64(1) {
		t.Fatalf("agent recent inbound: %v", cv["recent_inbound"])
	}
	// Asking for a section one may not read returns nothing for it.
	o = h.overview(agent, "?sections=campaigns,alerts")
	if _, ok := o["campaigns"]; ok || o["conversations"] != nil {
		t.Fatalf("agent asking for campaigns: %v", o)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/ops/overview?tz=Mars/Base", token: owner}), 422, "VALIDATION_FAILED")

	// Quick actions: the agent takes the pooled escalation over; it leaves
	// the queue and the change reaches the overview of the agent over the
	// unassigned channel (and the owner's over inbox:all).
	before := len(h.eventsOfType(realtime.UnassignedChannel, realtime.TypeConversationUpdated))
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + pooled + "/takeover", token: agent}), 200, "")
	if n := len(h.eventsOfType(realtime.UnassignedChannel, realtime.TypeConversationUpdated)); n <= before {
		t.Fatal("takeover did not announce the handoff to the unassigned pool")
	}
	cv = section(t, h.overview(owner, "?sections=conversations"), "conversations")
	if got := opsIDs(cv["awaiting_human"], "conversation_id"); len(got) != 1 || got[0] != mine {
		t.Fatalf("after takeover: %v", got)
	}

	// Pausing the running campaign announces it on campaign:all, and the
	// counts follow.
	before = len(h.eventsOfType(realtime.CampaignsChannel, realtime.TypeCampaignUpdated))
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + running + "/pause", token: owner}), 200, "")
	if n := len(h.eventsOfType(realtime.CampaignsChannel, realtime.TypeCampaignUpdated)); n <= before {
		t.Fatal("pause was not announced on campaign:all")
	}
	counts = section(t, h.overview(owner, "?sections=campaigns"), "campaigns")["counts"].(map[string]any)
	if counts["running"] != float64(0) || counts["paused"] != float64(2) {
		t.Fatalf("counts after pause: %v", counts)
	}

	// Meta alerts and number quality changes hint the alerts section.
	alertHints = opsHints(h, "alerts")
	h.sqlOne(`UPDATE phone_numbers SET quality_rating = 'RED' WHERE id = $1`, pn)
	h.sqlOne(`UPDATE phone_numbers SET quality_rating = 'RED' WHERE id = $1`, pn) // unchanged: no hint
	if got := opsHints(h, "alerts"); got != alertHints+1 {
		t.Fatalf("quality hints = %d, want %d", got, alertHints+1)
	}
}
