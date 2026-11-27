//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/conversations"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// pump plays the worker: it drains due outbox events and runs their task
// handlers, sends in parallel. Deferred tasks (rate limits) become due again
// after their delay. It returns how many tasks ran.
func (h *harness) pump(ctx context.Context, workers int) int {
	h.t.Helper()
	type ev struct {
		id      int64
		topic   string
		payload map[string]any
	}
	rows, err := h.pool.Query(ctx, `UPDATE outbox_events SET published_at = now()
		WHERE id IN (SELECT id FROM outbox_events WHERE published_at IS NULL AND available_at <= now() ORDER BY id LIMIT 5000 FOR UPDATE SKIP LOCKED)
		RETURNING id, topic, payload`)
	if err != nil {
		h.t.Fatal(err)
	}
	var evs []ev
	for rows.Next() {
		var e ev
		if err := rows.Scan(&e.id, &e.topic, &e.payload); err != nil {
			h.t.Fatal(err)
		}
		evs = append(evs, e)
	}
	rows.Close()

	retry := func(e ev, after time.Duration) {
		if _, err := h.pool.Exec(ctx, "UPDATE outbox_events SET published_at = NULL, available_at = now() + make_interval(secs => $2) WHERE id = $1",
			e.id, after.Seconds()); err != nil {
			h.t.Error(err)
		}
	}
	run := func(e ev) {
		var err error
		id := func(k string) uuid.UUID { return uuid.MustParse(e.payload[k].(string)) }
		switch e.topic {
		case "task:" + messaging.TypeSend:
			err = h.svc.messaging.Send(ctx, id("message_id"))
		case "task:" + campaigns.TypeMaterialize:
			err = h.svc.campaigns.Materialize(ctx, id("campaign_id"))
		case "task:" + campaigns.TypeDispatch:
			err = h.svc.campaigns.Dispatch(ctx, id("campaign_id"), int32(e.payload["gen"].(float64)))
		case "task:" + trigger.TypeEvaluate:
			var ev trigger.Event
			raw, _ := json.Marshal(e.payload)
			_ = json.Unmarshal(raw, &ev)
			err = h.svc.automation.Evaluate(ctx, ev)
		case "task:" + automation.TypeRun, "task:" + automation.TypeResume:
			err = h.svc.automation.Run(ctx, id("run_id"))
		case "task:" + conversations.TypeAIReply:
			err = h.svc.inbox.HandleAIReply(ctx, id("conversation_id"), id("message_id"))
		case "task:" + ai.TypeClassifyReply:
			err = h.svc.ai.RunClassification(ctx, id("classification_id"))
		case "task:" + messaging.TypeUserPreference:
			var up messaging.UserPreferenceEvent
			raw, _ := json.Marshal(e.payload)
			_ = json.Unmarshal(raw, &up)
			err = h.svc.messaging.ApplyUserPreference(ctx, up)
		default:
			return // realtime and other events
		}
		var r *messaging.ErrRetry
		if errors.As(err, &r) {
			retry(e, r.After)
			return
		}
		if errors.Is(err, automation.ErrRetriesExhausted) {
			return // the worker archives the task (failed jobs page)
		}
		if err != nil {
			h.t.Errorf("%s: %v", e.topic, err)
		}
	}
	// Control tasks run in order; sends run concurrently.
	var sends []ev
	for _, e := range evs {
		if e.topic == "task:"+messaging.TypeSend {
			sends = append(sends, e)
			continue
		}
		run(e)
	}
	ch := make(chan ev)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range ch {
				run(e)
			}
		}()
	}
	for _, e := range sends {
		ch <- e
	}
	close(ch)
	wg.Wait()
	return len(evs)
}

// runCampaign pumps until the campaign reaches a final state.
func (h *harness) runCampaign(id string, timeout time.Duration) string {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var status string
		if err := h.pool.QueryRow(h.ctx(), "SELECT status FROM campaigns WHERE id = $1", id).Scan(&status); err != nil {
			h.t.Fatal(err)
		}
		if status == "completed" || status == "cancelled" || status == "failed" {
			return status
		}
		if h.pump(h.ctx(), 32) == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	h.t.Fatalf("campaign %s did not finish within %s", id, timeout)
	return ""
}

// seedContacts inserts n contacts quickly, tagged "audience".
func (h *harness) seedContacts(n int, prefix string) string {
	h.t.Helper()
	ctx := h.ctx()
	var tag string
	if err := h.pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ('audience') ON CONFLICT (tenant_id, lower(name)) DO UPDATE SET name = tags.name RETURNING id").Scan(&tag); err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `
		WITH c AS (
			INSERT INTO contacts (phone_e164, name, var1, source)
			SELECT $1 || lpad(i::text, 8, '0'), 'Contact ' || i, 'City ' || (i % 7), 'import'
			FROM generate_series(1, $2::int) AS i
			RETURNING id)
		INSERT INTO contact_tags (contact_id, tag_id) SELECT id, $3 FROM c`, prefix, n, tag); err != nil {
		h.t.Fatal(err)
	}
	return tag
}

func (h *harness) createCampaign(token, tag string, window int, params map[string]any) string {
	h.t.Helper()
	body := map[string]any{"name": "Promo " + uuid.NewString()[:8], "template_name": "order_update", "template_language": "en",
		"audience": map[string]any{"tag_id": tag}, "window_size": window}
	if params != nil {
		body["template_params"] = params
	}
	r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: body})
	h.expect(r, 201, "")
	return r.data()["id"].(string)
}

// assertExactlyOnce checks every queued recipient was accepted by Meta
// exactly once and nothing else was sent.
func (h *harness) assertExactlyOnce(campaign string, want int) {
	h.t.Helper()
	ids := map[string]int{}
	for _, s := range h.meta.Sent() {
		if s.WAMID != "" {
			ids[s.Message.BizOpaqueCallbackData]++
		}
	}
	rows, err := h.pool.Query(h.ctx(), "SELECT id::text FROM messages WHERE campaign_id = $1", campaign)
	if err != nil {
		h.t.Fatal(err)
	}
	n := 0
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		if ids[id] != 1 {
			h.t.Fatalf("message %s accepted %d times", id, ids[id])
		}
		n++
	}
	rows.Close()
	if n != want {
		h.t.Fatalf("campaign has %d messages, want %d", n, want)
	}
	if lost := h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND state = 'pending'", campaign); lost != 0 {
		h.t.Fatalf("%d recipients never queued", lost)
	}
}

func TestCampaignLifecycleAndPersonalisation(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(30, "+90551")
	// One opted-out and one without var1 (skipped: no fallback for it).
	_, _ = h.pool.Exec(h.ctx(), "UPDATE contacts SET opted_out_at = now() WHERE phone_e164 = '+9055100000001'")
	_, _ = h.pool.Exec(h.ctx(), "UPDATE contacts SET var1 = NULL WHERE phone_e164 = '+9055100000002'")

	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: map[string]any{"name": "x", "template_name": "order_update",
		"template_language": "en", "audience": map[string]any{"tag_id": tag, "all": true}}}), 422, "VALIDATION_ERROR")
	id := h.createCampaign(token, tag, 10, map[string]any{"body": []string{"{{first_name}}", "{{var1}}"}})

	// Unapproved templates cannot start.
	bad := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: map[string]any{"name": "pending tpl", "template_name": "promo",
		"template_language": "en", "audience": map[string]any{"tag_id": tag}}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + bad.data()["id"].(string) + "/start", token: token}), 422, "VALIDATION_ERROR")

	r := h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token})
	h.expect(r, 200, "")
	if r.data()["status"] != "preparing" {
		t.Fatalf("start: %s", r.raw)
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/campaigns/" + id, token: token, body: map[string]any{"name": "late edit"}}), 409, "CAMPAIGN_STARTED")
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	h.assertExactlyOnce(id, 28)

	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id, token: token})
	counts := r.data()["counts"].(map[string]any)
	if counts["recipients"] != float64(30) || counts["recipients_skipped"] != float64(2) || counts["sent"] != float64(28) {
		t.Fatalf("counts: %v", counts)
	}
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/recipients?state=skipped", token: token})
	reasons := map[string]bool{}
	for _, x := range r.list() {
		reasons[x.(map[string]any)["skip_reason"].(string)] = true
	}
	if !reasons["opted_out"] || !reasons["missing_var1"] {
		t.Fatalf("skip reasons: %v", reasons)
	}
	// Personalisation reached Meta.
	var found bool
	for _, s := range h.meta.Sent() {
		if s.Message.To == "9055100000005" {
			p := s.Message.Template.Components[0].Parameters
			found = p[0].Text == "Contact" && p[1].Text == "City 5"
		}
	}
	if !found {
		t.Fatal("template parameters not rendered from the contact")
	}
	// Webhook statuses flow into campaign counts.
	var wamid string
	_ = h.pool.QueryRow(h.ctx(), "SELECT wamid FROM messages WHERE campaign_id = $1 LIMIT 1", id).Scan(&wamid)
	// Dispatcher ticks share the two-second progress throttle; let it lapse.
	if _, err := h.pool.Exec(h.ctx(), "UPDATE campaigns SET progress_published_at = now() - interval '1 minute' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	before := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'rt:campaign:' || $1::text", id)
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "delivered", "x", "", time.Now(), 0))
	if after := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'rt:campaign:' || $1::text", id); after != before+1 {
		t.Fatalf("delivery status did not publish campaign progress: %d → %d", before, after)
	}
	// A burst within two seconds marks the campaign dirty instead; the
	// reaper publishes it.
	var w2 string
	_ = h.pool.QueryRow(h.ctx(), "SELECT wamid FROM messages WHERE campaign_id = $1 AND wamid <> $2 LIMIT 1", id, wamid).Scan(&w2)
	h.webhook(whatsapptest.StatusUpdate(testPNID, w2, "delivered", "x", "", time.Now(), 0))
	if h.count("SELECT count(*) FROM campaigns WHERE id = $1 AND progress_dirty", id) != 1 {
		t.Fatal("throttled progress not marked dirty")
	}
	if _, err := h.svc.campaigns.Reap(h.ctx(), h.svc.messaging.Send); err != nil {
		t.Fatal(err)
	}
	if h.count("SELECT count(*) FROM campaigns WHERE id = $1 AND progress_dirty", id) != 0 {
		t.Fatal("reaper did not publish dirty progress")
	}
	counts = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id, token: token}).data()["counts"].(map[string]any)
	if counts["delivered"] != float64(2) {
		t.Fatalf("delivered not counted: %v", counts)
	}
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/events", token: token})
	kinds := []string{}
	for _, e := range r.list() {
		kinds = append(kinds, e.(map[string]any)["kind"].(string))
	}
	if strings.Join(kinds, ",") != "completed,materialized,started" {
		t.Fatalf("events: %v", kinds)
	}
	if h.count("SELECT count(*) FROM outbox_events WHERE topic = 'rt:campaign:' || $1::text", id) == 0 {
		t.Fatal("no realtime progress published")
	}
	// Campaign messages show up in the recipients' inbox threads.
	if got := h.count("SELECT count(*) FROM conversations cv JOIN messages m ON m.id = cv.last_message_id WHERE m.campaign_id = $1", id); got != 28 {
		t.Fatalf("conversations with a campaign message: %d", got)
	}
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/campaigns/" + id, token: token}), 204, "")
}

func TestCampaignPauseResumeTenTimes(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	const n = 2000
	tag := h.seedContacts(n, "+90552")
	id := h.createCampaign(token, tag, 50, nil)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	h.pump(h.ctx(), 8) // materialise

	for i := range 10 {
		h.pump(h.ctx(), 8)
		h.pump(h.ctx(), 8)
		r := h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/pause", token: token})
		h.expect(r, 200, "")
		h.pump(h.ctx(), 8) // queued sends become held
		sentBefore := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id)
		h.pump(h.ctx(), 8)
		if after := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id); after != sentBefore {
			t.Fatalf("round %d: %d messages sent while paused", i, after-sentBefore)
		}
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/pause", token: token}), 409, "INVALID_TRANSITION")
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume", token: token}), 200, "")
	}
	if st := h.runCampaign(id, 2*time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	h.assertExactlyOnce(id, n)
	if got := h.count("SELECT count(*) FROM campaign_events WHERE campaign_id = $1 AND kind IN ('paused', 'resumed')", id); got != 20 {
		t.Fatalf("events: %d", got)
	}
}

func TestCampaignCancelAndThroughputBackoff(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	throttle := messaging.NewThrottle(testutil.Redis(t), 80)
	h.svc.messaging.SetThrottle(throttle)
	tag := h.seedContacts(300, "+90553")

	// 130429 halves the per-number rate; 131056 is retried for that recipient.
	id := h.createCampaign(token, tag, 100, nil)
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 130429, HTTPStatus: 429}, whatsapptest.Behavior{ErrorCode: 131056})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	h.pump(h.ctx(), 1)
	h.pump(h.ctx(), 1)
	h.pump(h.ctx(), 1)
	var pn uuid.UUID
	_ = h.pool.QueryRow(h.ctx(), "SELECT phone_number_id FROM campaigns WHERE id = $1", id).Scan(&pn)
	if r := throttle.Rate(h.ctx(), pn); r != 40 {
		t.Fatalf("rate after 130429: %d, want 40", r)
	}
	if h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND error_code IN ('130429', '131056') AND status = 'pending'", id) == 0 {
		t.Fatal("throttled messages were not put back for retry")
	}
	// Cancel stops everything that has not been sent.
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/cancel", token: token}), 200, "")
	h.runCampaign(id, 30*time.Second)
	sent := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id)
	h.pump(h.ctx(), 8)
	if again := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id); again != sent {
		t.Fatal("messages sent after cancel")
	}
	if h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND state = 'pending'", id) != 0 ||
		h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status IN ('pending', 'held')", id) != 0 {
		t.Fatal("unsent work left after cancel")
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume", token: token}), 409, "INVALID_TRANSITION")
}

// Crash recovery without killing a process: a message left "sending" by a
// dead worker becomes unknown (never re-sent), lost pending tasks are
// re-created, and a stalled dispatcher is restarted. The stack-level
// kill -9 variant lives in scripts/chaos.sh.
func TestCampaignReaperRecoversCrashedWork(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(40, "+90554")
	id := h.createCampaign(token, tag, 40, nil)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	h.pump(h.ctx(), 1) // materialise
	h.pump(h.ctx(), 1) // first dispatch tick queues 40 messages
	ctx := h.ctx()
	// Simulate a crash: drop every queued task, claim five messages as if a
	// worker had started sending them, and age everything.
	if _, err := h.pool.Exec(ctx, "DELETE FROM outbox_events WHERE published_at IS NULL"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE messages SET status = 'sending', status_rank = 1
		WHERE id IN (SELECT id FROM messages WHERE campaign_id = $1 ORDER BY id LIMIT 5)`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE messages SET updated_at = now() - interval '1 hour' WHERE campaign_id = $1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE campaigns SET updated_at = now() - interval '1 hour' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	fixed, err := h.svc.campaigns.Reap(ctx, h.svc.messaging.Send)
	if err != nil || fixed < 41 {
		t.Fatalf("reaper fixed %d: %v", fixed, err)
	}
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'unknown'", id); got != 5 {
		t.Fatalf("unknown: %d", got)
	}
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	for _, s := range h.meta.Sent() {
		var st string
		_ = h.pool.QueryRow(ctx, "SELECT status FROM messages WHERE id::text = $1", s.Message.BizOpaqueCallbackData).Scan(&st)
		if st == "unknown" {
			t.Fatal("a message the crashed worker may have sent was sent again")
		}
	}
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id); got != 35 {
		t.Fatalf("sent: %d", got)
	}
}

// Phase 5 load criterion: 100,000 recipients against the mock with no
// duplicate and no loss. MESSORA_LOAD_RECIPIENTS overrides the size.
func TestCampaignLoadHundredThousand(t *testing.T) {
	if testing.Short() {
		t.Skip("load test")
	}
	n := 100_000
	if v, err := strconv.Atoi(os.Getenv("MESSORA_LOAD_RECIPIENTS")); err == nil && v > 0 {
		n = v
	}
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(n, "+90555")
	id := h.createCampaign(token, tag, 2000, nil)
	start := time.Now()
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	if st := h.runCampaign(id, 20*time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	t.Logf("%d recipients in %s", n, time.Since(start).Round(time.Second))
	h.assertExactlyOnce(id, n)
	// Only realtime notices may still be waiting for the relay.
	if left := h.count("SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND topic LIKE 'task:%'"); left != 0 {
		t.Fatalf("%d tasks left", left)
	}
}
