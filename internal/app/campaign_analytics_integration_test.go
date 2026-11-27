//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
)

// inboundButton builds a notification with a tapped template quick reply.
func inboundButton(from, wamid, payload, text string, at time.Time) []byte {
	b, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{map[string]any{
		"id": "waba", "changes": []any{map[string]any{"field": "messages", "value": map[string]any{
			"messaging_product": "whatsapp",
			"metadata":          map[string]string{"display_phone_number": "15550000000", "phone_number_id": testPNID},
			"contacts":          []any{map[string]any{"wa_id": from, "profile": map[string]string{"name": ""}}},
			"messages": []any{map[string]any{"from": from, "id": wamid, "timestamp": fmt.Sprint(at.Unix()), "type": "button",
				"button": map[string]string{"payload": payload, "text": text}}}}}}}}})
	return b
}

// download GETs a binary API response.
func (h *harness) downloadAs(token, path string) (int, http.Header, []byte) {
	h.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.srv.URL+path, nil)
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, raw
}

type sentMsg struct{ id, wamid, phone string }

func (h *harness) campaignMessages(id string) []sentMsg {
	h.t.Helper()
	rows, err := h.pool.Query(context.Background(), `SELECT m.id::text, coalesce(m.wamid, ''), c.phone_e164 FROM messages m
		JOIN contacts c ON c.id = m.contact_id WHERE m.campaign_id = $1 ORDER BY c.phone_e164`, id)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []sentMsg
	for rows.Next() {
		var m sentMsg
		_ = rows.Scan(&m.id, &m.wamid, &m.phone)
		out = append(out, m)
	}
	return out
}

// counters reads the incremental counters and checks them against a full
// recompute from the rows (which must find no drift).
func (h *harness) countersMatchRecompute(id string) map[string]int64 {
	h.t.Helper()
	ctx := context.Background()
	read := func() map[string]any {
		var raw []byte
		if err := h.pool.QueryRow(ctx, `SELECT to_jsonb(k) - 'updated_at' - 'reconciled_at' FROM campaign_counters k WHERE campaign_id = $1`, id).Scan(&raw); err != nil {
			h.t.Fatal(err)
		}
		m := map[string]any{}
		_ = json.Unmarshal(raw, &m)
		return m
	}
	before := read()
	var drifted bool
	if err := h.pool.QueryRow(ctx, "SELECT campaign_counters_recompute($1)", id).Scan(&drifted); err != nil {
		h.t.Fatal(err)
	}
	after := read()
	if drifted {
		for k, v := range after {
			if fmt.Sprint(before[k]) != fmt.Sprint(v) {
				h.t.Errorf("counter %s: incremental %v, recomputed %v", k, before[k], v)
			}
		}
		h.t.Fatal("incremental counters drifted from the recompute")
	}
	out := map[string]int64{}
	for k, v := range after {
		if f, ok := v.(float64); ok {
			out[k] = int64(f)
		}
	}
	return out
}

func TestCampaignAnalyticsCountersMatchRecompute(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	h.expect(h.uploadRates(token, "market,countries,category,price,currency,effective_from\nTurkey,TR,utility,0.0050,USD,2026-01-01\n"), 200, "")
	tag := h.seedTagged(10, "+90570", "mixed")
	id := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}, "window_size": 20,
		"reaction_map": map[string]string{"Evet": "positive", "Hayır": "negative"}})
	// The first contact is undeliverable, the second is opted out.
	if _, err := h.pool.Exec(ctx, "UPDATE contacts SET opted_out_at = now(), marketing_consent = 'opted_out', service_consent = 'opted_out' WHERE phone_e164 = '+9057000000002'"); err != nil {
		t.Fatal(err)
	}
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 131026})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	for range 6 {
		h.pump(ctx, 1)
	}
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	ms := h.campaignMessages(id)
	byPhone := map[string]sentMsg{}
	for _, m := range ms {
		byPhone[m.phone] = m
	}
	now := time.Now()
	price := whatsapp.Pricing{Type: "regular", Category: "utility", PricingModel: "PMP"}
	// 3..8 delivered (3 with pricing), 3..5 read, 9 withheld by Meta after it was sent.
	phone := func(i int) string { return fmt.Sprintf("+90570%08d", i) }
	for i := 3; i <= 8; i++ {
		m := byPhone[phone(i)]
		var p []whatsapp.Pricing
		if i == 3 {
			p = []whatsapp.Pricing{price}
		}
		h.webhook(whatsapptest.StatusUpdate(testPNID, m.wamid, "delivered", m.phone[1:], m.id, now, 0, p...))
	}
	for i := 3; i <= 5; i++ {
		m := byPhone[phone(i)]
		h.webhook(whatsapptest.StatusUpdate(testPNID, m.wamid, "read", m.phone[1:], m.id, now, 0))
	}
	m9 := byPhone[phone(9)]
	h.webhook(whatsapptest.StatusUpdate(testPNID, m9.wamid, "failed", m9.phone[1:], m9.id, now, 131049))
	// Replies: text (3), quick replies (4 positive, 5 negative), opt-out
	// keyword (6), a reply from the undeliverable contact (1: never reached).
	at := time.Now()
	h.webhook(whatsapptest.InboundText(testPNID, phone(3)[1:], "wamid.in.3", "Teşekkürler", "", at))
	h.webhook(whatsapptest.InboundText(testPNID, phone(3)[1:], "wamid.in.3b", "Bir şey daha", "", at.Add(time.Second)))
	h.webhook(inboundButton(phone(4)[1:], "wamid.in.4", "Evet", "Evet", at))
	h.webhook(inboundButton(phone(5)[1:], "wamid.in.5", "", "Hayır", at))
	h.webhook(whatsapptest.InboundText(testPNID, phone(6)[1:], "wamid.in.6", "STOP", "", at))
	var unreached string
	if err := h.pool.QueryRow(ctx, "SELECT phone_e164 FROM campaign_recipients WHERE campaign_id = $1 AND error_code = '131026'", id).Scan(&unreached); err != nil {
		t.Fatal(err)
	}
	if unreached >= phone(3) && unreached <= phone(9) {
		t.Fatalf("the undeliverable recipient %s overlaps the scripted statuses", unreached)
	}
	h.webhook(whatsapptest.InboundText(testPNID, unreached[1:], "wamid.in.1", "Merhaba", "", at))
	h.pump(ctx, 1)

	k := h.countersMatchRecompute(id)
	want := map[string]int64{"recipients": 10, "o_blocked": 1, "sent": 8, "delivered": 6, "read": 3, "o_failed": 2, "rejected": 2,
		"replied": 4, "positive": 1, "negative": 1, "opted_out": 1, "err_131049": 1, "priced": 1, "m_failed": 2}
	for key, v := range want {
		if k[key] != v {
			t.Errorf("%s = %d, want %d (%v)", key, k[key], v, k)
		}
	}
	// The API reads the same counters.
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/" + id, token: token})
	h.expect(r, 200, "")
	counts := r.data()["counts"].(map[string]any)
	if counts["a_replied"] != float64(4) || counts["a_positive"] != float64(1) || counts["a_cost_micros"] != float64(5000) {
		t.Fatalf("counts %v", counts)
	}
	// A manual label wins over the quick reply and stays after remapping.
	var rid string
	_ = h.pool.QueryRow(ctx, "SELECT id::text FROM campaign_recipients WHERE campaign_id = $1 AND phone_e164 = $2", id, phone(4)).Scan(&rid)
	h.expect(h.do(call{method: "PUT", path: "/api/v1/campaigns/" + id + "/recipients/" + rid + "/reaction", token: token,
		body: map[string]any{"reaction": "neutral"}}), 204, "")
	h.expect(h.do(call{method: "PUT", path: "/api/v1/campaigns/" + id + "/reaction-map", token: token,
		body: map[string]any{"reaction_map": map[string]string{"Evet": "negative", "Hayır": "positive"}}}), 200, "")
	k = h.countersMatchRecompute(id)
	if k["neutral"] != 1 || k["positive"] != 1 || k["negative"] != 0 {
		t.Fatalf("after remap %v", k)
	}

	// Analysis center: sources and hourly series.
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/analytics", token: token})
	h.expect(r, 200, "")
	src := map[string]string{}
	for _, m := range r.data()["metrics"].([]any) {
		mm := m.(map[string]any)
		src[mm["key"].(string)] = mm["source"].(string)
	}
	if src["delivered"] != "meta" || src["replied"] != "system" || src["opted_out"] != "system" || src["audience"] != "system" {
		t.Fatalf("sources %v", src)
	}
	if len(r.data()["hourly"].([]any)) == 0 {
		t.Fatal("no hourly series")
	}

	// Reconciler: nothing to repair; a damaged counter is repaired.
	if _, err := h.pool.Exec(ctx, "UPDATE campaign_counters SET delivered = 999 WHERE campaign_id = $1", id); err != nil {
		t.Fatal(err)
	}
	n, err := h.svc.campaigns.ReconcileCounters(ctx)
	if err != nil || n != 1 {
		t.Fatalf("reconcile %d %v", n, err)
	}
	if got := h.count("SELECT delivered FROM campaign_counters WHERE campaign_id = $1", id); got != 6 {
		t.Fatalf("repaired delivered = %d", got)
	}
}

func TestCampaignForecastSnapshotAndComparison(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	tag := h.seedTagged(6, "+90571", "forecast")
	aud := map[string]any{"tag_id": tag}
	r := h.do(call{method: "POST", path: "/api/v1/campaigns/forecast", token: token, body: map[string]any{"audience": aud,
		"template_name": "order_update", "template_language": "en"}})
	h.expect(r, 200, "")
	f := r.data()
	rates := f["rates"].(map[string]any)
	if rates["status"] != "insufficient_history" || rates["campaigns_needed"] != float64(3) || rates["recipients_needed"] != float64(500) {
		t.Fatalf("rates %v", rates)
	}
	if f["eligible"] != float64(6) || f["duration_seconds"] == nil || f["expected"].(map[string]any)["delivered"] != nil {
		t.Fatalf("forecast %s", r.raw)
	}
	if f["cost"].(map[string]any)["not_in_rate_card"] != float64(6) {
		t.Fatalf("without a rate card nothing is priced: %v", f["cost"])
	}

	// History: three completed utility campaigns with 600 recipients.
	for i := range 3 {
		var hid string
		if err := h.pool.QueryRow(ctx, `INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language, audience, finished_at)
			SELECT 'old '||$1, 'completed', id, 'order_update', 'en', '{"all":true}', now() - interval '1 day' FROM phone_numbers LIMIT 1 RETURNING id`, fmt.Sprint(i)).Scan(&hid); err != nil {
			t.Fatal(err)
		}
		if _, err := h.pool.Exec(ctx, `UPDATE campaign_counters SET recipients = 200, sent = 200, delivered = 180, read = 90, replied = 18, positive = 9
			WHERE campaign_id = $1`, hid); err != nil {
			t.Fatal(err)
		}
	}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/forecast", token: token, body: map[string]any{"audience": aud,
		"template_name": "order_update", "template_language": "en"}})
	h.expect(r, 200, "")
	exp := r.data()["expected"].(map[string]any)
	if r.data()["rates"].(map[string]any)["status"] != "ok" || exp["sent"] != float64(6) || exp["delivered"] != float64(5) {
		t.Fatalf("with history %s", r.raw)
	}

	id := h.newCampaign(token, map[string]any{"audience": aud})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	if got := h.count("SELECT count(*) FROM campaign_forecasts WHERE campaign_id = $1", id); got != 1 {
		t.Fatalf("forecast snapshots %d", got)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE campaign_forecasts SET forecast = '{}' WHERE campaign_id = $1", id); err == nil {
		t.Fatal("a stored forecast must be immutable")
	}
	for _, m := range h.campaignMessages(id)[:3] {
		h.webhook(whatsapptest.StatusUpdate(testPNID, m.wamid, "delivered", m.phone[1:], m.id, time.Now(), 0))
	}
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/forecast", token: token})
	h.expect(r, 200, "")
	d := r.data()
	if d["final"] != true || d["forecast"] == nil {
		t.Fatalf("forecast vs actual %s", r.raw)
	}
	got := map[string]map[string]any{}
	for _, m := range d["metrics"].([]any) {
		mm := m.(map[string]any)
		got[mm["metric"].(string)] = mm
	}
	if got["delivered"]["forecast"] != float64(5) || got["delivered"]["actual"] != float64(3) || got["delivered"]["delta"] != float64(-2) {
		t.Fatalf("delivered %v", got["delivered"])
	}
	if got["sent"]["actual"] != float64(6) || got["total"]["delta"] != float64(0) {
		t.Fatalf("sent %v total %v", got["sent"], got["total"])
	}
}

func (h *harness) protection(token string, change map[string]any) {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/settings/campaign-protection", token: token})
	h.expect(r, 200, "")
	body := r.data()
	for k, v := range change {
		body[k] = v
	}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/campaign-protection", token: token, body: body}), 200, "")
}

func (h *harness) lastPause(id string) (reason, rule string) {
	h.t.Helper()
	if err := h.pool.QueryRow(context.Background(), `SELECT reason, coalesce(detail->>'rule', '') FROM campaign_pauses
		WHERE campaign_id = $1 ORDER BY id DESC LIMIT 1`, id).Scan(&reason, &rule); err != nil {
		h.t.Fatal(err)
	}
	return reason, rule
}

func TestCampaignSpamProtectionPauses(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	h.protection(token, map[string]any{"window": 20, "min_attempts": 10, "ecosystem_rate_pct": 10})

	// A burst of 131049/131050 after ten good sends.
	tag := h.seedTagged(40, "+90572", "spam")
	id := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}, "window_size": 1})
	script := make([]whatsapptest.Behavior, 10)
	script = append(script, whatsapptest.Behavior{ErrorCode: 131050}, whatsapptest.Behavior{ErrorCode: 131049})
	h.meta.Script(script...)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	for range 60 {
		h.pump(ctx, 1)
		if h.status(id) == "paused" {
			break
		}
	}
	if st := h.status(id); st != "paused" {
		t.Fatalf("status %s", st)
	}
	if reason, rule := h.lastPause(id); reason != campaigns.PauseReasonSpam || rule != campaigns.RuleEcosystemRate {
		t.Fatalf("pause %s/%s", reason, rule)
	}
	r := h.do(call{method: "GET", path: "/api/v1/campaigns/" + id, token: token})
	if r.data()["paused_reason"] != "spam_protection" {
		t.Fatalf("paused reason %v", r.data()["paused_reason"])
	}
	// Resuming counts only new attempts: the old burst does not pause again.
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume", token: token}), 200, "")
	h.meta.Script()
	h.pump(ctx, 1)
	h.pump(ctx, 1)
	if st := h.status(id); st != "running" && st != "completed" {
		t.Fatalf("after resume %s", st)
	}

	// Number quality: Meta flags the number → its running campaign stops.
	tag2 := h.seedTagged(30, "+90573", "quality")
	id2 := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag2}, "window_size": 2})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id2 + "/start", token: token}), 200, "")
	h.pump(ctx, 1)
	h.pump(ctx, 1)
	h.webhook(whatsapptest.QualityUpdate(testWABA, "+1 555-010-0000", "FLAGGED", "", ""))
	if st := h.status(id2); st != "paused" {
		t.Fatalf("quality: status %s", st)
	}
	if reason, rule := h.lastPause(id2); reason != campaigns.PauseReasonSpam || rule != campaigns.RuleNumberQuality {
		t.Fatalf("quality pause %s/%s", reason, rule)
	}

	// 131048 stops at once.
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id2 + "/resume", token: token}), 200, "")
	h.meta.Script(whatsapptest.Behavior{ErrorCode: 131048})
	for range 10 {
		h.pump(ctx, 1)
		if h.status(id2) == "paused" {
			break
		}
	}
	if reason, rule := h.lastPause(id2); reason != campaigns.PauseReasonSpam || rule != campaigns.RuleSpamRateLimit {
		t.Fatalf("131048 pause %s/%s (status %s)", reason, rule, h.status(id2))
	}
}

func TestCampaignRecipientsTableExportsAndBreakdowns(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.svc.campaigns.TickInterval = 0
	ctx := context.Background()
	tag := h.seedTagged(8, "+90574", "table")
	var vip string
	if err := h.pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ('vip') RETURNING id").Scan(&vip); err != nil {
		t.Fatal(err)
	}
	// Contacts 1–3 are also VIP and corporate.
	if _, err := h.pool.Exec(ctx, `INSERT INTO contact_tags (contact_id, tag_id) SELECT id, $1 FROM contacts WHERE phone_e164 <= '+9057400000003' AND phone_e164 LIKE '+90574%'`, vip); err != nil {
		t.Fatal(err)
	}
	var c1 string
	_ = h.pool.QueryRow(ctx, "SELECT id::text FROM contacts WHERE phone_e164 = '+9057400000001'").Scan(&c1)
	r := h.do(call{method: "PATCH", path: "/api/v1/contacts/" + c1, token: token, body: map[string]any{"contact_type": "kurumsal"}})
	h.expect(r, 200, "")
	if r.data()["contact_type"] != "kurumsal" {
		t.Fatalf("contact type %s", r.raw)
	}
	id := h.newCampaign(token, map[string]any{"audience": map[string]any{"tag_id": tag}})
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	if st := h.runCampaign(id, time.Minute); st != "completed" {
		t.Fatalf("status %s", st)
	}
	ms := h.campaignMessages(id)
	for _, m := range ms[:4] { // contacts 1–4 delivered
		h.webhook(whatsapptest.StatusUpdate(testPNID, m.wamid, "delivered", m.phone[1:], m.id, time.Now(), 0))
	}

	// Search: status filter, total, sort, page.
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/recipients/search", token: token,
		body: map[string]any{"filter": map[string]any{"status": []string{"delivered"}}, "sort": "phone", "limit": 3}})
	h.expect(r, 200, "")
	if r.body["meta"].(map[string]any)["total"] != float64(4) || len(r.list()) != 3 || r.list()[0].(map[string]any)["phone"] != "+9057400000001" {
		t.Fatalf("search %s", r.raw)
	}
	// A select-all with one row unticked.
	first := r.list()[0].(map[string]any)["id"]
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/recipients/search", token: token,
		body: map[string]any{"filter": map[string]any{"status": []string{"delivered"}, "exclude_ids": []any{first}}}})
	h.expect(r, 200, "")
	if r.body["meta"].(map[string]any)["total"] != float64(3) {
		t.Fatalf("exclude_ids %s", r.raw)
	}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/recipients/search", token: token,
		body: map[string]any{"filter": map[string]any{"tag_id": vip, "q": "Contact 2"}}})
	h.expect(r, 200, "")
	if r.body["meta"].(map[string]any)["total"] != float64(1) {
		t.Fatalf("tag + search %s", r.raw)
	}

	// CSV export of the filtered set.
	code, hdr, raw := h.downloadAs(token, "/api/v1/campaigns/"+id+"/recipients/export?format=csv&status=delivered")
	if code != 200 || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") {
		t.Fatalf("csv %d %s", code, raw)
	}
	recs, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil || len(recs) != 5 || recs[0][0] != "phone" || recs[1][0] != "+9057400000001" {
		t.Fatalf("csv rows %v %v", recs, err)
	}
	// XLSX export of the same set.
	code, _, raw = h.downloadAs(token, "/api/v1/campaigns/"+id+"/recipients/export?format=xlsx&status=delivered")
	if code != 200 {
		t.Fatalf("xlsx %d %s", code, raw)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := book.GetRows("Alıcılar")
	if len(rows) != 5 || rows[2][0] != "+9057400000002" {
		t.Fatalf("xlsx rows %v", rows)
	}
	if got := h.count("SELECT count(*) FROM audit_logs WHERE action = 'campaign.export' AND entity_id = $1", id); got != 2 {
		t.Fatalf("export audit %d", got)
	}

	// Background exports: recipients and the report (XLSX + PDF).
	exportFile := func(kind, format string, filter map[string]any) []byte {
		t.Helper()
		body := map[string]any{"kind": kind, "format": format}
		if filter != nil {
			body["filter"] = filter
		}
		r := h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/exports", token: token, body: body})
		h.expect(r, 202, "")
		eid := r.data()["id"].(string)
		if err := h.svc.campaigns.RunExport(ctx, uuid.MustParse(eid)); err != nil {
			t.Fatal(err)
		}
		r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/exports", token: token})
		h.expect(r, 200, "")
		if st := r.list()[0].(map[string]any)["status"]; st != "done" {
			t.Fatalf("export %s %s: %s", kind, format, r.raw)
		}
		code, _, raw := h.downloadAs(token, "/api/v1/campaigns/"+id+"/exports/"+eid+"/download")
		if code != 200 {
			t.Fatalf("download %d %s", code, raw)
		}
		return raw
	}
	raw = exportFile("recipients", "csv", map[string]any{"status": []string{"delivered"}, "tag_id": vip})
	if recs, _ := csv.NewReader(bytes.NewReader(raw)).ReadAll(); len(recs) != 4 {
		t.Fatalf("background csv %d rows", len(recs))
	}
	raw = exportFile("report", "xlsx", nil)
	book, err = excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := book.GetSheetList(); !slices.Equal(got, campaigns.ReportSheets) {
		t.Fatalf("report sheets %v", got)
	}
	if rows, _ := book.GetRows("Alıcılar"); len(rows) != 9 {
		t.Fatalf("report recipients %d", len(rows))
	}
	if pdf := exportFile("report", "pdf", nil); !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	if got := h.count("SELECT count(*) FROM audit_logs WHERE action IN ('campaign.export_requested', 'campaign.export_download') AND entity_id = $1", id); got != 6 {
		t.Fatalf("background export audit %d", got)
	}

	// Breakdowns.
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/breakdown?dimension=tag", token: token})
	h.expect(r, 200, "")
	buckets := map[string]map[string]any{}
	for _, b := range r.list() {
		bb := b.(map[string]any)
		buckets[bb["label"].(string)] = bb
	}
	if buckets["vip"]["recipients"] != float64(3) || buckets["vip"]["delivered"] != float64(3) || buckets["table"]["delivered"] != float64(4) {
		t.Fatalf("tag breakdown %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/campaigns/" + id + "/breakdown?dimension=contact_type", token: token})
	h.expect(r, 200, "")
	for _, b := range r.list() {
		bb := b.(map[string]any)
		if bb["key"] == "kurumsal" && (bb["recipients"] != float64(1) || bb["delivered"] != float64(1)) {
			t.Fatalf("contact type breakdown %s", r.raw)
		}
	}

	// Bulk: tag the delivered ones, then a follow-up draft from them.
	var follow string
	_ = h.pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ('takip') RETURNING id").Scan(&follow)
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/recipients/bulk", token: token,
		body: map[string]any{"action": "add_tag", "target_id": follow, "filter": map[string]any{"status": []string{"delivered"}}}})
	h.expect(r, 200, "")
	if r.data()["affected"] != float64(4) || h.count("SELECT count(*) FROM contact_tags WHERE tag_id = $1", follow) != 4 {
		t.Fatalf("bulk tag %s", r.raw)
	}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/recipients/bulk", token: token,
		body: map[string]any{"action": "follow_up", "filter": map[string]any{"status": []string{"delivered"}}}})
	h.expect(r, 200, "")
	fc := r.data()["campaign"].(map[string]any)
	if fc["status"] != "draft" || len(fc["audience"].(map[string]any)["contact_ids"].([]any)) != 4 {
		t.Fatalf("follow-up %s", r.raw)
	}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/preflight", token: token, body: map[string]any{
		"audience": fc["audience"], "template_name": "order_update", "template_language": "en"}})
	h.expect(r, 200, "")
	if r.data()["total"] != float64(4) {
		t.Fatalf("contact list audience %s", r.raw)
	}
}
