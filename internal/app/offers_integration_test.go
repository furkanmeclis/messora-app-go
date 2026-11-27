//go:build integration

package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// Meta's documented creation examples (limited-time offer and media card
// carousel pages).
const (
	ltoComponents = `[
	  {"type":"header","format":"image","example":{"header_handle":["4::aW..."]}},
	  {"type":"limited_time_offer","limited_time_offer":{"text":"Expiring offer!","has_expiration":true}},
	  {"type":"body","text":"Good news, {{1}}! Use code {{2}} to get 25% off all Caribbean Destination packages!","example":{"body_text":[["Pablo","CARIBE25"]]}},
	  {"type":"buttons","buttons":[{"type":"copy_code","example":"CARIBE25"},
	   {"type":"url","text":"Book now!","url":"https://awesomedestinations.com/offers?code={{1}}","example":["https://awesomedestinations.com/offers?ref=n3mtql"]}]}]`
	carouselCard = `{"components":[{"type":"header","format":"image","example":{"header_handle":["4::an..."]}},
	  {"type":"buttons","buttons":[{"type":"quick_reply","text":"Send me more like this!"},
	   {"type":"url","text":"Shop","url":"https://www.luckyshrub.com/rare-succulents/{{1}}","example":["BLUE_ELF"]}]}]}`
	carouselComponents = `[{"type":"body","text":"Rare succulents for sale! {{1}}, add these unique plants to your collection. Each of these rare succulents are {{2}} if you checkout using code {{3}}.","example":{"body_text":[["Pablo","30%","30OFF"]]}},
	  {"type":"carousel","cards":[` + carouselCard + `,` + carouselCard + `]}]`
)

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatalf("%v: %s", err, a)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	return reflect.DeepEqual(x, y)
}

func (h *harness) accountID(token string) string {
	h.t.Helper()
	return h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()[0].(map[string]any)["id"].(string)
}

func TestCreateLTOAndCarouselTemplates(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	acc := h.accountID(token)
	create := func(name, components string) resp {
		return h.do(call{method: "POST", path: "/api/v1/templates", token: token, body: map[string]any{
			"account_id": acc, "name": name, "language": "en_US", "category": "MARKETING", "components": json.RawMessage(components)}})
	}
	for _, c := range []struct{ name, comps string }{{"lto_caribbean", ltoComponents}, {"carousel_succulents", carouselComponents}} {
		r := create(c.name, c.comps)
		h.expect(r, 201, "")
		if r.data()["status"] != "PENDING" {
			t.Fatalf("%s: status %v", c.name, r.data()["status"])
		}
		got := h.meta.CreatedTemplates()
		var body struct {
			Name, Language, Category string
			ParameterFormat          string          `json:"parameter_format"`
			Components               json.RawMessage `json:"components"`
		}
		if err := json.Unmarshal(got[len(got)-1], &body); err != nil {
			t.Fatal(err)
		}
		if body.Name != c.name || body.Category != "MARKETING" || body.Language != "en_US" || body.ParameterFormat != "POSITIONAL" {
			t.Fatalf("%s: Meta got %s", c.name, got[len(got)-1])
		}
		if !jsonEqual(t, body.Components, []byte(c.comps)) {
			t.Fatalf("%s: components changed on the way to Meta:\n%s", c.name, body.Components)
		}
	}

	// Invalid components never reach Meta and name the field.
	before := len(h.meta.CreatedTemplates())
	r := create("lto_footer", strings.Replace(ltoComponents, `{"type":"buttons"`, `{"type":"footer","text":"T&C apply"},{"type":"buttons"`, 1))
	h.expect(r, 422, "VALIDATION_ERROR")
	if !strings.Contains(string(r.raw), "components.3") {
		t.Fatalf("field not named: %s", r.raw)
	}
	r = create("carousel_one", `[{"type":"body","text":"x"},{"type":"carousel","cards":[`+carouselCard+`]}]`)
	h.expect(r, 422, "VALIDATION_ERROR")
	r = create("catalog", `[{"type":"body","text":"x"},{"type":"buttons","buttons":[{"type":"MPM","text":"View items"}]}]`)
	h.expect(r, 422, "VALIDATION_ERROR")
	if !strings.Contains(string(r.raw), "catalog") {
		t.Fatalf("catalog message: %s", r.raw)
	}
	if n := len(h.meta.CreatedTemplates()); n != before {
		t.Fatalf("%d invalid templates reached Meta", n-before)
	}
}

// ltoTemplate adds (and syncs) an approved limited-time offer template:
// image header, two body variables, copy code (button 0), URL with a
// variable (button 1).
func (h *harness) ltoTemplate(token string) {
	h.t.Helper()
	h.meta.AddTemplate(testWABA, whatsapp.TemplateInfo{ID: "910", Name: "xmas_lto", Language: "tr", Category: "MARKETING", Status: "APPROVED",
		Components: []byte(ltoComponents)})
	h.expect(h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + h.accountID(token) + "/sync", token: token}), 200, "")
}

func (h *harness) createOffer(token string, body map[string]any) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "POST", path: "/api/v1/offers", token: token, body: body})
	h.expect(r, 201, "")
	return r.data()
}

func ltoCampaign(offer string, tag string) map[string]any {
	return map[string]any{"name": "Yılbaşı " + time.Now().Format("150405.000"), "template_name": "xmas_lto", "template_language": "tr",
		"audience": map[string]any{"tag_id": tag}, "offer_id": offer, "window_size": 1,
		"template_params": map[string]any{"body": []string{"{{first_name}}", "{{offer_coupon}}"}, "buttons": []string{"", "{{var1}}"}}}
}

func TestLTOCampaignTakesTheOffer(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.ltoTemplate(token)
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(2, "+90553")
	ends := time.Now().Add(3 * time.Hour).Truncate(time.Millisecond).UTC()
	offer := h.createOffer(token, map[string]any{"title": "Yılbaşı", "discount_text": "%25", "coupon_code": "XMAS25",
		"image_url": "https://cdn.test/xmas.jpg", "starts_at": time.Now().Add(-time.Hour), "ends_at": ends, "status": "scheduled"})
	if offer["status"] != "active" {
		t.Fatalf("a published offer that already started is active, got %v", offer["status"])
	}
	// The countdown needs expiration_time_ms; the offer gives it (and the
	// coupon, and the header image), so the campaign sets none of them.
	r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: ltoCampaign(offer["id"].(string), tag)})
	h.expect(r, 201, "")
	id := r.data()["id"].(string)
	if r.data()["offer_id"] != offer["id"] {
		t.Fatalf("offer_id: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")
	if st := h.runCampaign(id, 30*time.Second); st != "completed" {
		t.Fatalf("campaign %s", st)
	}
	sent := h.meta.Sent()
	if len(sent) != 2 {
		t.Fatalf("%d sends", len(sent))
	}
	var got struct {
		Template struct {
			Components []map[string]any `json:"components"`
		} `json:"template"`
	}
	if err := json.Unmarshal(sent[0].Raw, &got); err != nil {
		t.Fatal(err)
	}
	comps, _ := json.Marshal(got.Template.Components)
	want := `[{"type":"header","parameters":[{"type":"image","image":{"link":"https://cdn.test/xmas.jpg"}}]},
	  {"type":"body","parameters":[{"type":"text","text":"Contact"},{"type":"text","text":"XMAS25"}]},
	  {"type":"limited_time_offer","parameters":[{"type":"limited_time_offer","limited_time_offer":{"expiration_time_ms":` +
		jsonInt(ends.UnixMilli()) + `}}]},
	  {"type":"button","sub_type":"copy_code","index":"0","parameters":[{"type":"coupon_code","coupon_code":"XMAS25"}]},
	  {"type":"button","sub_type":"url","index":"1","parameters":[{"type":"text","text":"CITY"}]}]`
	// The URL suffix is the contact's var1 ("City n").
	want = strings.Replace(want, `"CITY"`, mustJSON(urlSuffix(got.Template.Components)), 1)
	if !jsonEqual(t, comps, []byte(want)) {
		t.Fatalf("Meta got\n%s\nwant\n%s", comps, want)
	}
	if !strings.HasPrefix(urlSuffix(got.Template.Components), "City ") {
		t.Fatalf("url suffix %q", urlSuffix(got.Template.Components))
	}
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }
func mustJSON(v any) string  { b, _ := json.Marshal(v); return string(b) }

func urlSuffix(comps []map[string]any) string {
	for _, c := range comps {
		if c["sub_type"] == "url" {
			return c["parameters"].([]any)[0].(map[string]any)["text"].(string)
		}
	}
	return ""
}

func TestExpiredOfferBlocksCampaigns(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.ltoTemplate(token)
	tag := h.seedContacts(2, "+90554")
	offer := h.createOffer(token, map[string]any{"title": "Kısa", "coupon_code": "SHORT1",
		"starts_at": time.Now().Add(-time.Hour), "ends_at": time.Now().Add(time.Hour), "status": "scheduled"})
	oid := offer["id"].(string)
	r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: ltoCampaign(oid, tag)})
	h.expect(r, 201, "")
	id := r.data()["id"].(string)

	// Ending the offer by hand is audited and announced.
	r = h.do(call{method: "PATCH", path: "/api/v1/offers/" + oid, token: token, body: map[string]any{"status": "expired"}})
	h.expect(r, 200, "")
	if r.data()["status"] != "expired" {
		t.Fatalf("status %v", r.data()["status"])
	}
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action = 'offer.update' AND entity_id = $1", oid); n != 1 {
		t.Fatalf("%d audit entries", n)
	}
	if n := h.count(`SELECT count(*) FROM outbox_events WHERE topic = 'rt:campaign:all' AND payload->>'type' = 'offer.updated' AND payload->>'offer_id' = $1`, oid); n < 2 {
		t.Fatalf("%d offer.updated events", n)
	}
	r = h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token})
	h.expect(r, 409, "OFFER_EXPIRED")
	if !strings.Contains(string(r.raw), "Kısa") {
		t.Fatalf("message should name the offer: %s", r.raw)
	}
	// New campaigns cannot pick it either, and it is final.
	r = h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: ltoCampaign(oid, tag)})
	h.expect(r, 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/offers/" + oid, token: token, body: map[string]any{"title": "x"}}), 409, "OFFER_EXPIRED")
	if len(h.meta.Sent()) != 0 {
		t.Fatal("nothing may be sent")
	}
}

func TestOfferExpiryPausesRunningCampaign(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.ltoTemplate(token)
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(4, "+90556")
	offer := h.createOffer(token, map[string]any{"title": "Flash", "coupon_code": "FLASH",
		"image_url": "https://cdn.test/f.jpg", "starts_at": time.Now().Add(-time.Hour), "ends_at": time.Now().Add(time.Hour), "status": "scheduled"})
	oid := offer["id"].(string)
	r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: ltoCampaign(oid, tag)})
	h.expect(r, 201, "")
	id := r.data()["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token}), 200, "")

	ctx := h.ctx()
	deadline := time.Now().Add(20 * time.Second)
	for len(h.meta.Sent()) == 0 && time.Now().Before(deadline) {
		if h.pump(ctx, 4) == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if len(h.meta.Sent()) == 0 {
		t.Fatal("nothing was sent before the offer ended")
	}
	// The offer ends while recipients are still waiting (window 1).
	if _, err := h.pool.Exec(ctx, "UPDATE offers SET ends_at = now() - interval '1 second' WHERE id = $1", oid); err != nil {
		t.Fatal(err)
	}
	status := ""
	for time.Now().Before(deadline) {
		_ = h.pool.QueryRow(ctx, "SELECT status FROM campaigns WHERE id = $1", id).Scan(&status)
		if status != "running" {
			break
		}
		if h.pump(ctx, 4) == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if status != "paused" {
		t.Fatalf("campaign %s, want paused", status)
	}
	var reason string
	if err := h.pool.QueryRow(ctx, `SELECT detail->>'reason' FROM campaign_events WHERE campaign_id = $1 AND kind = 'paused' ORDER BY id DESC LIMIT 1`, id).Scan(&reason); err != nil || reason != "offer_expired" {
		t.Fatalf("pause reason %q (%v)", reason, err)
	}
	if pending := h.count("SELECT count(*) FROM campaign_recipients WHERE campaign_id = $1 AND state = 'pending'", id); pending == 0 {
		t.Fatal("the pause should leave recipients waiting")
	}
	// The scheduler marks the offer expired; resuming is refused.
	if n, err := h.svc.offers.Tick(ctx); err != nil || n != 1 {
		t.Fatalf("tick: %d %v", n, err)
	}
	if st := h.do(call{method: "GET", path: "/api/v1/offers/" + oid, token: token}).data()["status"]; st != "expired" {
		t.Fatalf("offer %v", st)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/resume", token: token}), 409, "OFFER_EXPIRED")
	// Unlinking is not possible while paused (only drafts are edited), but
	// cancelling is.
	h.expect(h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/cancel", token: token}), 200, "")
}

func TestOfferSchedulerActivatesAndExpires(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	soon := h.createOffer(token, map[string]any{"title": "Soon", "starts_at": time.Now().Add(time.Hour), "ends_at": time.Now().Add(2 * time.Hour), "status": "scheduled"})
	draft := h.createOffer(token, map[string]any{"title": "Draft", "starts_at": time.Now().Add(-time.Hour), "ends_at": time.Now().Add(time.Hour)})
	if soon["status"] != "scheduled" || draft["status"] != "draft" {
		t.Fatalf("statuses %v %v", soon["status"], draft["status"])
	}
	ctx := h.ctx()
	if _, err := h.pool.Exec(ctx, "UPDATE offers SET starts_at = now() - interval '1 minute' WHERE id = $1", soon["id"]); err != nil {
		t.Fatal(err)
	}
	if n, err := h.svc.offers.Tick(ctx); err != nil || n != 1 {
		t.Fatalf("tick: %d %v", n, err)
	}
	r := h.do(call{method: "GET", path: "/api/v1/offers?status=active", token: token})
	if l := r.list(); len(l) != 1 || l[0].(map[string]any)["id"] != soon["id"] {
		t.Fatalf("active: %s", r.raw)
	}
	if _, err := h.pool.Exec(ctx, "UPDATE offers SET ends_at = now() - interval '1 second', starts_at = now() - interval '2 hours'"); err != nil {
		t.Fatal(err)
	}
	if n, err := h.svc.offers.Tick(ctx); err != nil || n != 1 {
		t.Fatalf("tick: %d %v (drafts stay drafts)", n, err)
	}
	if st := h.do(call{method: "GET", path: "/api/v1/offers/" + draft["id"].(string), token: token}).data()["status"]; st != "draft" {
		t.Fatalf("draft became %v", st)
	}

	// Validation and permissions.
	h.expect(h.do(call{method: "POST", path: "/api/v1/offers", token: token, body: map[string]any{"title": "Bad",
		"starts_at": time.Now().Add(time.Hour), "ends_at": time.Now()}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "POST", path: "/api/v1/offers", token: token, body: map[string]any{"title": "Bad",
		"starts_at": time.Now(), "ends_at": time.Now().Add(time.Hour), "image_url": "http://insecure.test/a.jpg"}}), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "DELETE", path: "/api/v1/offers/" + draft["id"].(string), token: token}), 204, "")
	h.expect(h.do(call{method: "GET", path: "/api/v1/offers/" + draft["id"].(string), token: token}), 404, "NOT_FOUND")
}
