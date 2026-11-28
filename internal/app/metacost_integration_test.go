//go:build integration

package app

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// testRateCard is an invented rate card: test values, not Meta's prices.
const testRateCard = `market,countries,category,price,currency,effective_from,tier_lower,tier_upper,source
Turkey,TR,marketing,0.0100,USD,2020-01-01,,,test fixture
Turkey,,marketing,0.0150,USD,2020-06-01,,,test fixture
Turkey,,utility,0.0040,USD,2020-01-01,1,1,test fixture
Turkey,,utility,0.0030,USD,2020-01-01,2,,test fixture
Turkey,,marketing,0.5000,TRY,2020-01-01,,,test fixture
Other,,marketing,0.0900,USD,2020-01-01,,,test fixture
`

// uploadRates imports a rate card in the platform admin panel (the card is
// shared by every workspace; token, a workspace session, cannot).
func (h *harness) uploadRates(_ string, csv string) resp {
	h.t.Helper()
	status, raw := h.multipartAt(adminHost, h.platformSession(), "/api/v1/platform/meta-rates/import", "rates.csv", []byte(csv))
	out := resp{status: status, raw: raw}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

// sendTemplate queues and sends a template message; it returns the message
// id and its wamid.
func (h *harness) sendTemplate(token, to, name string) (string, string) {
	h.t.Helper()
	r := h.send(token, map[string]any{"to": to, "type": "template", "template": map[string]any{"name": name, "language": "en"}})
	h.expect(r, 202, "")
	id := r.data()["id"].(string)
	if err := h.deliver(id); err != nil {
		h.t.Fatal(err)
	}
	return id, h.message(token, id)["wamid"].(string)
}

func TestMessageCostFromRateCard(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	accounts := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()
	if cur := accounts[0].(map[string]any)["currency"]; cur != "USD" {
		t.Fatalf("account currency not read from Meta: %v", cur)
	}

	// Invalid files are rejected as a whole with every problem listed.
	r := h.uploadRates(token, "market,countries,category,price,currency,effective_from\nTurkey,TR,promo,abc,USD,2020-01-01\n")
	h.expect(r, 422, "VALIDATION_ERROR")
	if h.count("SELECT count(*) FROM meta_rates") != 0 {
		t.Fatal("rejected file partly imported")
	}
	r = h.uploadRates(token, testRateCard)
	h.expect(r, 200, "")
	if r.data()["rates"] != float64(6) || r.data()["markets"] != float64(1) {
		t.Fatalf("import: %s", r.raw)
	}
	// Re-importing is idempotent.
	h.expect(h.uploadRates(token, testRateCard), 200, "")
	list := h.do(call{method: "GET", path: "/api/v1/settings/meta-rates", token: token})
	h.expect(list, 200, "")
	if len(list.list()) != 6 {
		t.Fatalf("rates: %s", list.raw)
	}
	first := list.list()[0].(map[string]any)
	if first["market"] != "Other" || first["price"] != "0.09" {
		t.Fatalf("first rate: %v", first)
	}
	// The older marketing rate ends where the newer one starts.
	if h.count("SELECT count(*) FROM meta_rates WHERE market = 'Turkey' AND category = 'marketing' AND currency = 'USD' AND effective_to = DATE '2020-06-01'") != 1 {
		t.Fatal("superseded rate not closed")
	}

	billableTrue := true
	// Marketing message: pricing arrives on "sent"; "delivered" without it
	// keeps the values.
	id, wamid := h.sendTemplate(token, "+"+testCustomer, "order_update")
	now := time.Now()
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "sent", testCustomer, id, now, 0,
		whatsapp.Pricing{Type: "regular", Category: "marketing", PricingModel: "PMP", Billable: &billableTrue}))
	h.webhook(whatsapptest.StatusUpdate(testPNID, wamid, "delivered", testCustomer, id, now.Add(time.Second), 0))
	m := h.message(token, id)
	if m["pricing_category"] != "marketing" || m["pricing_type"] != "regular" || m["cost_amount"] != "0.015" || m["cost_currency"] != "USD" {
		t.Fatalf("marketing cost: %v", m)
	}
	var country, model, source string
	var billable bool
	_ = h.pool.QueryRow(h.ctx(), "SELECT recipient_country, pricing_model, billable, cost_source FROM messages WHERE id = $1", id).
		Scan(&country, &model, &billable, &source)
	if country != "TR" || model != "PMP" || !billable || source != "rate_card" {
		t.Fatalf("stored pricing: %s %s %v %s", country, model, billable, source)
	}

	// Utility tiers: the month's first message uses tier 1, the next tier 2.
	u1, w1 := h.sendTemplate(token, "+"+testCustomer, "order_update")
	h.webhook(whatsapptest.StatusUpdate(testPNID, w1, "delivered", testCustomer, u1, now.Add(2*time.Second), 0,
		whatsapp.Pricing{Type: "regular", Category: "utility", PricingModel: "PMP"}))
	u2, w2 := h.sendTemplate(token, "+905551112244", "order_update")
	h.webhook(whatsapptest.StatusUpdate(testPNID, w2, "sent", "905551112244", u2, now.Add(3*time.Second), 0,
		whatsapp.Pricing{Type: "regular", Category: "utility", PricingModel: "PMP"}))
	if c1, c2 := h.message(token, u1)["cost_amount"], h.message(token, u2)["cost_amount"]; c1 != "0.004" || c2 != "0.003" {
		t.Fatalf("tiered utility: %v %v", c1, c2)
	}

	// Free messages cost 0; a country without its own market uses "Other";
	// a category without a rate stays unpriced.
	f, wf := h.sendTemplate(token, "+"+testCustomer, "order_update")
	h.webhook(whatsapptest.StatusUpdate(testPNID, wf, "sent", testCustomer, f, now, 0,
		whatsapp.Pricing{Type: "free_customer_service", Category: "service", PricingModel: "PMP", Subtype: "free_tier"}))
	if fm := h.message(token, f); fm["cost_amount"] != "0" || fm["pricing_type"] != "free_customer_service" || fm["cost_currency"] != "USD" {
		t.Fatalf("free message: %v", fm)
	}
	o, wo := h.sendTemplate(token, "+5511987654321", "order_update")
	h.webhook(whatsapptest.StatusUpdate(testPNID, wo, "sent", "5511987654321", o, now, 0, whatsapp.Pricing{Type: "regular", Category: "marketing"}))
	if om := h.message(token, o); om["cost_amount"] != "0.09" {
		t.Fatalf("other market: %v", om)
	}
	a, wa := h.sendTemplate(token, "+"+testCustomer, "order_update")
	h.webhook(whatsapptest.StatusUpdate(testPNID, wa, "sent", testCustomer, a, now, 0, whatsapp.Pricing{Type: "regular", Category: "authentication"}))
	if am := h.message(token, a); am["cost_amount"] != nil || am["pricing_category"] != "authentication" {
		t.Fatalf("no rate: %v", am)
	}
}

func TestUserPreferencesAreRecordedAndAnnounced(t *testing.T) {
	h := newHarness(t)
	h.connect()
	at := time.Now().Add(-time.Minute).Truncate(time.Second)
	body := whatsapptest.UserPreferences(testWABA, testPNID, whatsapp.UserPreference{WaID: testCustomer, UserID: "TR.123",
		Detail: "User requested to stop marketing messages", Category: "marketing_messages", Value: "stop", Timestamp: json.Number(fmtUnix(at))})
	if st := h.webhook(body); st != 200 {
		t.Fatalf("webhook: %d", st)
	}
	var phone, value string
	var occurred time.Time
	var pn *uuid.UUID
	if err := h.pool.QueryRow(h.ctx(), "SELECT phone_e164, value, occurred_at, phone_number_id FROM meta_user_preferences").
		Scan(&phone, &value, &occurred, &pn); err != nil {
		t.Fatal(err)
	}
	if phone != "+"+testCustomer || value != "stop" || !occurred.Equal(at) || pn == nil {
		t.Fatalf("row: %s %s %v %v", phone, value, occurred, pn)
	}
	var payload []byte
	if err := h.pool.QueryRow(h.ctx(), "SELECT payload FROM outbox_events WHERE topic = 'task:"+messaging.TypeUserPreference+"'").Scan(&payload); err != nil {
		t.Fatalf("event not written: %v", err)
	}
	var ev messaging.UserPreferenceEvent
	if err := json.Unmarshal(payload, &ev); err != nil || ev.PhoneE164 != "+"+testCustomer || ev.Value != "stop" || ev.Category != "marketing_messages" {
		t.Fatalf("event: %s %v", payload, err)
	}
	// The same change delivered again (a retry with a different envelope) is recorded once.
	h.webhook(whatsapptest.AccountChange(testWABA, "user_preferences", map[string]any{
		"metadata":         map[string]string{"phone_number_id": testPNID},
		"user_preferences": []any{map[string]any{"wa_id": testCustomer, "user_id": "TR.123", "category": "marketing_messages", "value": "stop", "timestamp": at.Unix(), "detail": "again"}}}))
	if n := h.count("SELECT count(*) FROM meta_user_preferences"); n != 1 {
		t.Fatalf("duplicate preference rows: %d", n)
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:" + messaging.TypeUserPreference + "'"); n != 1 {
		t.Fatalf("duplicate events: %d", n)
	}
}

func TestAccountSignalWebhooksAndSync(t *testing.T) {
	h := newHarness(t)
	token, phoneID := h.connect()
	numbers := func() map[string]any {
		r := h.do(call{method: "GET", path: "/api/v1/channels/phone-numbers", token: token})
		h.expect(r, 200, "")
		return r.list()[0].(map[string]any)
	}
	if n := numbers(); n["throughput_mps"] != nil || n["signals_checked_at"] == nil {
		t.Fatalf("after connect: %v", n)
	}

	// Throughput upgrade and portfolio limit from phone_number_quality_update.
	h.webhook(whatsapptest.QualityUpdate(testWABA, "15550100000", "THROUGHPUT_UPGRADE", "TIER_UNLIMITED", "TIER_10K"))
	if n := numbers(); n["throughput_mps"] != float64(1000) || n["messaging_limit"] != "TIER_10K" {
		t.Fatalf("after quality update: %v", n)
	}
	if h.count("SELECT count(*) FROM whatsapp_account_events WHERE kind = 'phone_number_quality_update.throughput_upgrade' AND phone_number_id = $1", phoneID) != 1 {
		t.Fatal("quality event not stored")
	}
	if h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:channels:signal_sync'") != 1 {
		t.Fatal("signal re-sync not requested")
	}

	// Portfolio limit from business_capability_update (numeric form too).
	h.webhook(whatsapptest.AccountChange(testWABA, "business_capability_update", map[string]any{"max_daily_conversations_per_business": 100000}))
	if n := numbers(); n["messaging_limit"] != "TIER_100K" {
		t.Fatalf("after capability update: %v", n)
	}

	// Template quality.
	h.webhook(whatsapptest.TemplateQuality(testWABA, "777", "order_update", "en", "GREEN", "YELLOW"))
	r := h.do(call{method: "GET", path: "/api/v1/templates", token: token})
	for _, x := range r.list() {
		if tpl := x.(map[string]any); tpl["name"] == "order_update" && tpl["quality_score"] != "YELLOW" {
			t.Fatalf("template quality: %v", tpl)
		}
	}

	// Account alerts are kept.
	h.webhook(whatsapptest.AccountUpdate(testWABA, "ACCOUNT_RESTRICTION", map[string]any{
		"restriction_info": []any{map[string]any{"restriction_type": "RESTRICTED_BIZ_INITIATED_MESSAGING", "expiration": 1641330498}}}))
	if h.count("SELECT count(*) FROM whatsapp_account_events WHERE kind = 'account_update.account_restriction'") != 1 {
		t.Fatal("account_update not stored")
	}
	// An unknown WABA is ignored, not retried.
	h.webhook(whatsapptest.AccountUpdate("999999", "ACCOUNT_DELETED", nil))
	if h.count("SELECT count(*) FROM webhook_events WHERE status <> 'processed'") != 0 {
		t.Fatal("events left unprocessed")
	}

	// The hourly sync reads Meta's current values.
	h.meta.SetThroughput(testWABA, testPNID, "STANDARD")
	if err := h.svc.channels.SyncSignals(h.ctx(), nil); err != nil {
		t.Fatal(err)
	}
	if n := numbers(); n["throughput_mps"] != float64(80) || n["quality_rating"] != "GREEN" {
		t.Fatalf("after sync: %v", n)
	}
}

func TestDailyCostReconciliation(t *testing.T) {
	h := newHarness(t)
	h.connect()
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	day := func(d int) int64 { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC).Unix() }
	cost := 1.25
	h.meta.SetPricingAnalytics(testWABA, []whatsapp.PricingDataPoint{
		{Start: day(23), End: day(24), PhoneNumber: "15550100000", Country: "TR", PricingCategory: "MARKETING", PricingType: "REGULAR", Volume: 10, Cost: &cost},
		{Start: day(24), End: day(25), PhoneNumber: "15550100000", Country: "TR", PricingCategory: "SERVICE", PricingType: "FREE_CUSTOMER_SERVICE", Volume: 4},
	})
	if err := h.svc.channels.ReconcileCosts(h.ctx(), now); err != nil {
		t.Fatal(err)
	}
	q := h.meta.PricingQueries()
	want := "pricing_analytics.start(" + fmtUnix(time.Unix(day(23), 0)) + ").end(" + fmtUnix(now) + ").granularity(DAILY).dimensions(PHONE,COUNTRY,PRICING_CATEGORY,PRICING_TYPE).metric_types(COST,VOLUME)"
	if len(q) != 1 || q[0] != want {
		t.Fatalf("query %v, want %s", q, want)
	}
	var vol int64
	var c, cur *string
	if err := h.pool.QueryRow(h.ctx(), "SELECT volume, cost::text, currency FROM meta_cost_daily WHERE day = DATE '2026-11-23'").Scan(&vol, &c, &cur); err != nil {
		t.Fatal(err)
	}
	// Meta's figures are in the account's currency.
	if vol != 10 || c == nil || *c != "1.250000" || cur == nil || *cur != "USD" {
		t.Fatalf("day 23: %d %v %v", vol, c, cur)
	}
	// No cost reported (partner credit line): volume only.
	if err := h.pool.QueryRow(h.ctx(), "SELECT volume, cost::text FROM meta_cost_daily WHERE day = DATE '2026-11-24'").Scan(&vol, &c); err != nil {
		t.Fatal(err)
	}
	if vol != 4 || c != nil {
		t.Fatalf("day 24: %d %v", vol, c)
	}
	// Re-running updates in place.
	cost = 2
	h.meta.SetPricingAnalytics(testWABA, []whatsapp.PricingDataPoint{
		{Start: day(23), End: day(24), PhoneNumber: "15550100000", Country: "TR", PricingCategory: "MARKETING", PricingType: "REGULAR", Volume: 12, Cost: &cost}})
	if err := h.svc.channels.ReconcileCosts(h.ctx(), now); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM meta_cost_daily"); n != 2 {
		t.Fatalf("rows: %d", n)
	}
	if n := h.count("SELECT volume FROM meta_cost_daily WHERE day = DATE '2026-11-23'"); n != 12 {
		t.Fatalf("volume not updated: %d", n)
	}
}

func TestThrottleUsesNumberThroughput(t *testing.T) {
	h := newHarness(t)
	_, phoneID := h.connect()
	pn := uuid.MustParse(phoneID)
	ctx := h.ctx()
	other := uuid.Must(uuid.NewV7()) // a number without a known throughput
	throttle := messaging.NewThrottle(testutil.Redis(t), messaging.DefaultThroughput).WithCeilings(h.svc.messaging.ThroughputCeiling)
	if r := throttle.Rate(ctx, other); r != 80 {
		t.Fatalf("unknown throughput: %d", r)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE phone_numbers SET throughput_mps = 1000 WHERE id = $1", pn); err != nil {
		t.Fatal(err)
	}
	if r := throttle.Rate(ctx, pn); r != 1000 {
		t.Fatalf("ceiling not the number's throughput: %d", r)
	}
	if r := throttle.Backoff(ctx, pn); r != 500 {
		t.Fatalf("backoff from 1000: %d", r)
	}
}

func fmtUnix(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }
