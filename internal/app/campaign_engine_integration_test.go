//go:build integration

package app

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

// seedTagged inserts n contacts with phone prefix and tags them tagName.
func (h *harness) seedTagged(n int, prefix, tagName string) string {
	h.t.Helper()
	ctx := context.Background()
	var tag string
	if err := h.pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ($1) RETURNING id", tagName).Scan(&tag); err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `
		WITH c AS (
			INSERT INTO contacts (phone_e164, name, var1, source)
			SELECT $1 || lpad(i::text, 8, '0'), 'Contact ' || i, 'City', 'import'
			FROM generate_series(1, $2::int) AS i
			RETURNING id)
		INSERT INTO contact_tags (contact_id, tag_id) SELECT id, $3 FROM c`, prefix, n, tag); err != nil {
		h.t.Fatal(err)
	}
	return tag
}

func (h *harness) newCampaign(token string, body map[string]any) string {
	h.t.Helper()
	b := map[string]any{"name": "Promo " + uuid.NewString()[:8], "template_name": "order_update", "template_language": "en"}
	for k, v := range body {
		b[k] = v
	}
	r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: b})
	h.expect(r, 201, "")
	return r.data()["id"].(string)
}

func (h *harness) status(id string) string {
	h.t.Helper()
	var st string
	if err := h.pool.QueryRow(context.Background(), "SELECT status FROM campaigns WHERE id = $1", id).Scan(&st); err != nil {
		h.t.Fatal(err)
	}
	return st
}

// useScheduler paces sends like the worker does, with the number's
// throughput set to mps.
func (h *harness) useScheduler(mps int) *messaging.Throttle {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), "UPDATE phone_numbers SET throughput_mps = $1", mps); err != nil {
		h.t.Fatal(err)
	}
	t := messaging.NewThrottle(h.rdb, messaging.DefaultThroughput).WithCeilings(h.svc.messaging.ThroughputCeiling).SetReservedShare(0.2)
	h.svc.messaging.SetThrottle(t)
	h.svc.campaigns.SetScheduler(t)
	return t
}

// Two campaigns on one number share the campaign lane by priority (4:1),
// never exceed the number's throughput, and an API message sent meanwhile
// goes out at once from the reserved share.
func TestCampaignSchedulerSharesByPriority(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 200 * time.Millisecond
	const mps = 25 // lane = 20: high 16, low 4
	h.useScheduler(mps)
	hiTag := h.seedTagged(400, "+90561", "high")
	loTag := h.seedTagged(400, "+90562", "low")
	hi := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": hiTag}, "priority": "high"})
	lo := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": loTag}, "priority": "low"})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + hi + "/start", token: token}), 200, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + lo + "/start", token: token}), 200, "")

	ctx := context.Background()
	start := time.Now()
	var apiQueued time.Time
	var apiID string
	for time.Since(start) < 7*time.Second {
		if apiID == "" && time.Since(start) > 4*time.Second {
			r := h.send(token, map[string]any{"to": "+905559990001", "type": "template",
				"template": map[string]any{"name": "order_update", "language": "en"}})
			h.expect(r, 202, "")
			apiID, apiQueued = r.data()["id"].(string), time.Now()
		}
		if h.pump(ctx, 32) == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	// The queue view shows both with their shares.
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/queue", token: token})
	h.expect(r, 200, "")
	qn := r.list()[0].(map[string]any)
	if qn["ceiling_mps"] != float64(mps) || qn["reserved_share"] != 0.2 || qn["campaign_capacity_mps"] != float64(20) {
		t.Fatalf("queue number: %v", qn)
	}
	shares := map[string]float64{}
	for _, c := range qn["campaigns"].([]any) {
		m := c.(map[string]any)
		shares[m["id"].(string)] = m["share_mps"].(float64)
		if m["eta_seconds"] == nil {
			t.Fatalf("no ETA: %v", m)
		}
	}
	if shares[hi] != 16 || shares[lo] != 4 {
		t.Fatalf("shares: %v", shares)
	}

	// Sends per campaign, after the first second's bursts.
	byID := map[string]string{}
	rows, _ := h.pool.Query(ctx, "SELECT id::text, coalesce(campaign_id::text, '') FROM messages WHERE direction = 'outbound'")
	for rows.Next() {
		var id, c string
		_ = rows.Scan(&id, &c)
		byID[id] = c
	}
	rows.Close()
	from, to := start.Add(2*time.Second), start.Add(6*time.Second)
	n := map[string]int{}
	perSecond := map[int64]int{}
	var apiSentAt time.Time
	for _, s := range h.meta.Sent() {
		if s.Message.BizOpaqueCallbackData == apiID {
			apiSentAt = s.At
		}
		perSecond[s.At.Unix()]++
		if s.At.After(from) && s.At.Before(to) {
			n[byID[s.Message.BizOpaqueCallbackData]]++
		}
	}
	t.Logf("4 s window: high %d, low %d (want ≈64 and ≈16)", n[hi], n[lo])
	if total := n[hi] + n[lo]; total > 4*20+20 || total < 4*20/2 {
		t.Fatalf("campaign lane: %d sends in 4 s at 20 mps", total)
	}
	if ratio := float64(n[hi]) / float64(max(n[lo], 1)); ratio < 2.5 || ratio > 6 {
		t.Fatalf("priority share high/low = %.2f, want ≈4", ratio)
	}
	if n[lo] == 0 {
		t.Fatal("low priority starved")
	}
	for sec, c := range perSecond {
		// GCRA allows one second of burst on top of the rate.
		if c > 2*mps {
			t.Fatalf("%d sends in second %d, throughput %d", c, sec, mps)
		}
	}
	if apiSentAt.IsZero() || apiSentAt.Sub(apiQueued) > 1500*time.Millisecond {
		t.Fatalf("API message waited %s behind campaigns", apiSentAt.Sub(apiQueued))
	}

	// Priority is editable while running, and audited.
	r = h.do(call{method: "PATCH", path: "/api/v1/campaigns/" + lo + "/settings", token: token, body: map[string]any{"priority": "high"}})
	h.expect(r, 200, "")
	if r.data()["priority"] != "high" {
		t.Fatalf("settings: %s", r.raw)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'campaign.priority_change' AND entity_id = $1", lo) != 1 {
		t.Fatal("priority change not audited")
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/campaigns/" + lo + "/settings", token: token, body: map[string]any{"priority": "urgent"}}), 422, "VALIDATION_ERROR")
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'campaign.started' AND entity_id IN ($1, $2)", hi, lo) != 2 {
		t.Fatal("start not audited")
	}
}

// The portfolio messaging limit counts unique customers reached outside
// their window in 24 hours; the campaign waits in paused_limit and resumes
// by itself when the oldest ones age out.
func TestCampaignPortfolioLimitPausesAndResumes(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, "UPDATE phone_numbers SET messaging_limit = 'TIER_250'"); err != nil {
		t.Fatal(err)
	}
	tag := h.seedTagged(300, "+90563", "limit")
	// Ten customers wrote in the last day: their window is open and they do
	// not count.
	for i := 1; i <= 10; i++ {
		h.webhook(whatsapptest.InboundText(testPNID, fmt.Sprintf("90563%08d", i), fmt.Sprintf("wamid.in%d", i), "hi", "", time.Now()))
	}
	// One customer was reached outside the window an hour ago (another
	// campaign); reaching them again costs nothing.
	if _, err := h.pool.Exec(ctx, `INSERT INTO portfolio_reach (account_id, contact_id, last_at)
		SELECT p.account_id, c.id, now() - interval '1 hour' FROM phone_numbers p, contacts c WHERE c.phone_e164 = '+9056300000015'`); err != nil {
		t.Fatal(err)
	}
	id := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	deadline := time.Now().Add(time.Minute)
	for h.status(id) != "paused_limit" {
		if time.Now().After(deadline) {
			t.Fatalf("not paused by the limit: %s", h.status(id))
		}
		if h.pump(ctx, 16) == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	h.pump(ctx, 16)
	sent := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id)
	if counted := h.count("SELECT count(*) FROM portfolio_reach WHERE last_at > now() - interval '24 hours'"); counted != 250 {
		t.Fatalf("counted %d unique customers, want exactly 250 (sent %d)", counted, sent)
	}
	if sent != 250-1+1+10 {
		t.Fatalf("sent %d, want 260 (249 new, 1 already counted, 10 in their window)", sent)
	}
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/" + id, token: token})
	c := r.data()
	if c["status"] != "paused_limit" || c["paused_reason"] != "messaging_limit" || c["resume_at"] == nil {
		t.Fatalf("campaign: %s", r.raw)
	}
	at, _ := time.Parse(time.RFC3339Nano, c["resume_at"].(string))
	if time.Until(at) < 22*time.Hour {
		t.Fatalf("resume at %s, want when the first counted customer ages out (≈23 h)", at)
	}
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/pauses", token: token})
	p := r.list()[0].(map[string]any)
	if p["reason"] != "messaging_limit" || p["status"] != "paused_limit" || p["snapshot"].(map[string]any)["sent"] != float64(sent) {
		t.Fatalf("pause: %v", p)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE action = 'campaign.paused_limit' AND entity_id = $1", id) != 1 {
		t.Fatal("limit pause not audited")
	}
	// Nothing happens before the resume time.
	if _, err := h.svc.campaigns.StartDue(ctx); err != nil {
		t.Fatal(err)
	}
	if h.status(id) != "paused_limit" {
		t.Fatal("resumed early")
	}
	// A day later the counted customers age out and the job resumes.
	if _, err := h.pool.Exec(ctx, "UPDATE portfolio_reach SET last_at = last_at - interval '25 hours'"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE campaigns SET resume_at = now() - interval '1 second' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.campaigns.StartDue(ctx); err != nil {
		t.Fatal(err)
	}
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	h.assertExactlyOnce(id, 300)
	if h.count("SELECT count(*) FROM campaign_pauses WHERE campaign_id = $1 AND resumed_at IS NOT NULL", id) != 1 {
		t.Fatal("pause not closed on resume")
	}
}

// "Previous campaign's unreached customers": the chosen classes only, one
// recipient per contact, hard blocks never selectable.
func TestCampaignPreviousCampaignAudience(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	tag := h.seedTagged(40, "+90564", "first")
	src := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + src + "/start", token: token}), 200, "")
	// A running campaign cannot be a source yet.
	r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: map[string]any{"name": "again", "template_name": "order_update",
		"template_language": "en", "audience": map[string]any{"previous_campaign": map[string]any{"campaign_id": src, "classes": []string{"temporary"}}}}})
	h.expect(r, 422, "VALIDATION_ERROR")
	if st := h.runCampaign(src, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	// Outcomes: 5 temporary, 4 rejected, 3 stopped marketing, 2 ecosystem
	// limit, 3 never sent (cancelled), the rest reached.
	set := func(n int, offset int, class string) {
		if _, err := h.pool.Exec(ctx, `UPDATE messages SET status = 'failed', status_rank = 6, failure_class = $3
			WHERE id IN (SELECT id FROM messages WHERE campaign_id = $1 ORDER BY id OFFSET $2 LIMIT $4)`, src, offset, class, n); err != nil {
			t.Fatal(err)
		}
	}
	set(5, 0, "temporary")
	set(4, 5, "rejected_by_meta")
	set(3, 9, "user_stopped_marketing")
	set(2, 12, "ecosystem_limit")
	if _, err := h.pool.Exec(ctx, `UPDATE campaign_recipients SET state = 'cancelled', message_id = NULL
		WHERE message_id IN (SELECT id FROM messages WHERE campaign_id = $1 ORDER BY id OFFSET 14 LIMIT 3)`, src); err != nil {
		t.Fatal(err)
	}
	aud := map[string]any{"previous_campaign": map[string]any{"campaign_id": src, "classes": []string{"temporary", "ecosystem_limit", "never_sent"}}}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/preflight", token: token, body: map[string]any{"audience": aud,
		"template_name": "order_update", "template_language": "en"}})
	h.expect(r, 200, "")
	pf := r.data()
	by := pf["by_failure_class"].(map[string]any)
	want := map[string]float64{"temporary": 5, "rejected_by_meta": 4, "user_stopped_marketing": 3, "ecosystem_limit": 2, "never_sent": 3, "reached": 23}
	for k, v := range want {
		if by[k] != v {
			t.Fatalf("by_failure_class %v, want %v", by, want)
		}
	}
	if pf["total"] != float64(10) || pf["eligible"] != float64(10) {
		t.Fatalf("preflight: %v", pf)
	}
	// Hard blocks cannot be chosen.
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: map[string]any{"name": "x", "template_name": "order_update",
		"template_language": "en", "audience": map[string]any{"previous_campaign": map[string]any{"campaign_id": src,
			"classes": []string{"user_stopped_marketing"}}}}}), 422, "VALIDATION_ERROR")

	again := h.newCampaign(token, map[string]any{"audience": aud})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + again + "/start", token: token}), 200, "")
	if st := h.runCampaign(again, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	h.assertExactlyOnce(again, 10)
	if got := h.count(`SELECT count(*) FROM campaign_recipients r WHERE r.campaign_id = $1 AND r.contact_id IN (
		SELECT r2.contact_id FROM campaign_recipients r2 LEFT JOIN messages m ON m.id = r2.message_id WHERE r2.campaign_id = $2
		AND campaign_recipient_class(r2.state, r2.skip_reason, m.status, m.failure_class) IN ('temporary', 'ecosystem_limit', 'never_sent'))`, again, src); got != 10 {
		t.Fatalf("recipients from the chosen classes: %d", got)
	}
	if got := h.count("SELECT count(DISTINCT contact_id) FROM campaign_recipients WHERE campaign_id = $1", again); got != 10 {
		t.Fatalf("distinct contacts %d", got)
	}
	// The recipients table shows why the source missed each one.
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + src + "/recipients?class=temporary", token: token})
	h.expect(r, 200, "")
	if len(r.list()) != 5 || r.list()[0].(map[string]any)["outcome"] != "failed" || r.list()[0].(map[string]any)["unreached_class"] != "temporary" {
		t.Fatalf("recipients: %s", r.raw)
	}
}

// A 131049 in the status webhook (after Meta accepted the message) gives
// the recipient one new message a day later, exactly once, while the
// campaign is still in progress; consent is checked again then.
func TestCampaignWithheldStatusResendsOnce(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	tag := h.seedTagged(4, "+90565", "withheld")
	id := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}, "window_size": 3})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	h.pump(ctx, 1) // materialise
	h.pump(ctx, 1) // first tick queues three
	h.pump(ctx, 4) // they are sent; the next tick waits for them
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id); got != 3 {
		t.Fatalf("sent %d", got)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/pause", token: token}), 200, "")
	type msg struct{ id, wamid, phone string }
	var ms []msg
	rows, _ := h.pool.Query(ctx, `SELECT m.id::text, m.wamid, c.phone_e164 FROM messages m JOIN contacts c ON c.id = m.contact_id
		WHERE m.campaign_id = $1 ORDER BY m.id`, id)
	for rows.Next() {
		var m msg
		_ = rows.Scan(&m.id, &m.wamid, &m.phone)
		ms = append(ms, m)
	}
	rows.Close()
	withheld := whatsapptest.StatusUpdate(testPNID, ms[0].wamid, "failed", ms[0].phone[1:], ms[0].id, time.Now(), 131049)
	h.webhook(withheld)
	h.webhook(whatsapptest.StatusUpdate(testPNID, ms[1].wamid, "failed", ms[1].phone[1:], ms[1].id, time.Now(), 131049))
	if got := h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND state = 'retry' AND next_attempt_at > now() + interval '23 hours'", id); got != 2 {
		t.Fatalf("recipients waiting for a resend: %d", got)
	}
	// The second one opts out meanwhile.
	if _, err := h.pool.Exec(ctx, "UPDATE contacts SET opted_out_at = now() WHERE phone_e164 = $1", ms[1].phone); err != nil {
		t.Fatal(err)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume", token: token}), 200, "")
	for range 6 {
		h.pump(ctx, 4)
	}
	if st := h.status(id); st != "running" {
		t.Fatalf("campaign must wait for the resend, is %s", st)
	}
	if got := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1", id); got != 4 {
		t.Fatalf("resent before the day passed: %d messages", got)
	}
	// A day later.
	if _, err := h.pool.Exec(ctx, "UPDATE campaign_recipients SET next_attempt_at = now() - interval '1 second' WHERE campaign_id = $1 AND state = 'retry'", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE outbox_events SET available_at = now() WHERE published_at IS NULL"); err != nil {
		t.Fatal(err)
	}
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	// The same webhook again changes nothing.
	h.webhook(withheld)
	h.pump(ctx, 4)
	h.assertExactlyOnce(id, 5)
	if got := h.count("SELECT count(*) FROM messages WHERE retry_of = $1", ms[0].id); got != 1 {
		t.Fatalf("resends of the withheld message: %d", got)
	}
	if got := h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND resends = 1 AND state = 'queued'", id); got != 1 {
		t.Fatalf("resent recipients: %d", got)
	}
	if got := h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND state = 'skipped' AND skip_reason = 'opted_out'", id); got != 1 {
		t.Fatalf("opted-out recipient not skipped: %d", got)
	}
}

// Pausing while sends are in flight and resuming (with changes) sends every
// recipient exactly once; the pause is kept with its snapshot.
func TestCampaignPauseDuringInFlightExactlyOnce(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	const n = 600
	tag := h.seedTagged(n, "+90566", "inflight")
	id := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}, "window_size": 120})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")

	for round := range 3 {
		var stop atomic.Bool
		var wg sync.WaitGroup
		wg.Go(func() {
			for !stop.Load() {
				if h.pump(ctx, 16) == 0 {
					time.Sleep(5 * time.Millisecond)
				}
			}
		})
		deadline := time.Now().Add(30 * time.Second)
		for h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status = 'sent'", id) < (round+1)*120 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/pause", token: token}), 200, "")
		time.Sleep(100 * time.Millisecond)
		stop.Store(true)
		wg.Wait()
		h.pump(ctx, 16)
		sent := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status IN ('sent', 'unknown')", id)
		h.pump(ctx, 16)
		if again := h.count("SELECT count(*) FROM messages WHERE campaign_id = $1 AND status IN ('sent', 'unknown')", id); again != sent {
			t.Fatalf("round %d: sent while paused", round)
		}
		r := h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume-preview", token: token, body: map[string]any{"priority": "high"}})
		h.expect(r, 200, "")
		pv := r.data()
		if pv["remaining"] != float64(n-sent) || pv["eligible"] != float64(n-sent) || pv["estimated_seconds"] == nil {
			t.Fatalf("round %d: preview %v (sent %d)", round, pv, sent)
		}
		body := map[string]any{}
		if round == 1 {
			body = map[string]any{"priority": "high", "name": "Renamed on resume"}
		}
		h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume", token: token, body: body}), 200, "")
	}
	if st := h.runCampaign(id, 2*time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	h.assertExactlyOnce(id, n)
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/pauses", token: token})
	ps := r.list()
	if len(ps) != 3 {
		t.Fatalf("pauses: %s", r.raw)
	}
	for _, x := range ps {
		p := x.(map[string]any)
		if p["reason"] != "manual" || p["paused_by"] == nil || p["resumed_at"] == nil || p["snapshot"].(map[string]any)["recipients"] != float64(n) {
			t.Fatalf("pause: %v", p)
		}
	}
	if ch, _ := ps[1].(map[string]any)["changes"].(map[string]any); ch["priority"] == nil || ch["name"] == nil {
		t.Fatalf("resume changes not kept: %v", ps[1])
	}
	c := h.do(call{method: "GET", path: "/api/v1/campaigns/" + id, token: token}).data()
	if c["name"] != "Renamed on resume" || c["priority"] != "high" || c["paused_reason"] != nil {
		t.Fatalf("campaign: %v", c)
	}
	if h.count("SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND action IN ('campaign.paused', 'campaign.resumed')", id) != 6 {
		t.Fatal("pause/resume not audited")
	}
	// A resume can be scheduled.
	id2 := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id2 + "/start", token: token}), 200, "")
	h.pump(ctx, 4)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id2 + "/pause", token: token}), 200, "")
	at := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id2 + "/resume", token: token, body: map[string]any{"resume_at": at}})
	h.expect(r, 200, "")
	if r.data()["status"] != "paused" || r.data()["resume_at"] == nil {
		t.Fatalf("scheduled resume: %s", r.raw)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE campaigns SET resume_at = now() WHERE id = $1", id2); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.campaigns.StartDue(ctx); err != nil {
		t.Fatal(err)
	}
	if h.status(id2) != "running" {
		t.Fatal("scheduled resume did not run")
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id2 + "/cancel", token: token}), 200, "")
}
