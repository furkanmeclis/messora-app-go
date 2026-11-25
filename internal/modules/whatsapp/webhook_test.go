package whatsapp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp/whatsapptest"
)

// The payloads below follow the examples of Meta's webhook reference
// (2026-11): field names must match exactly.

func change(t *testing.T, body string) (whatsapp.Entry, whatsapp.Change) {
	t.Helper()
	var n whatsapp.Notification
	if err := json.Unmarshal([]byte(body), &n); err != nil {
		t.Fatal(err)
	}
	return n.Entry[0], n.Entry[0].Changes[0]
}

func TestStatusPricing(t *testing.T) {
	_, c := change(t, `{"object":"whatsapp_business_account","entry":[{"id":"102290129340398","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550783881","phone_number_id":"106540352242922"},
		"statuses":[{"id":"wamid.X","status":"sent","timestamp":"1750030073","recipient_id":"16505551234",
		"conversation":{"id":"72b14d6bd5407799e66f64d1b338e567","expiration_timestamp":"1750116480","origin":{"type":"marketing"}},
		"pricing":{"billable":true,"pricing_model":"PMP","type":"regular","category":"marketing"}}]}}]}]}`)
	var v whatsapp.MessagesValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		t.Fatal(err)
	}
	p := v.Statuses[0].Pricing
	if p == nil || p.Type != "regular" || p.Category != "marketing" || p.PricingModel != "PMP" || p.Billable == nil || !*p.Billable {
		t.Fatalf("pricing: %+v", p)
	}
	free := `{"statuses":[{"id":"w","status":"delivered","timestamp":"1","pricing":{"pricing_model":"PMP","type":"free_customer_service","category":"service","subtype":"free_tier"}}]}`
	var v2 whatsapp.MessagesValue
	if err := json.Unmarshal([]byte(free), &v2); err != nil {
		t.Fatal(err)
	}
	if p := v2.Statuses[0].Pricing; p.Subtype != "free_tier" || p.Billable != nil {
		t.Fatalf("free pricing: %+v", p)
	}
	// Statuses without pricing (the other of sent/delivered, read) leave it nil.
	var v3 whatsapp.MessagesValue
	if err := json.Unmarshal([]byte(`{"statuses":[{"id":"w","status":"read","timestamp":"1"}]}`), &v3); err != nil || v3.Statuses[0].Pricing != nil {
		t.Fatalf("read status: %+v %v", v3.Statuses[0].Pricing, err)
	}
}

func TestQualityUpdate(t *testing.T) {
	e, c := change(t, `{"entry":[{"id":"102290129340398","time":1748454394,"changes":[{"value":{
		"display_phone_number":"15550783881","event":"THROUGHPUT_UPGRADE","current_limit":"TIER_UNLIMITED"},
		"field":"phone_number_quality_update"}]}],"object":"whatsapp_business_account"}`)
	var v whatsapp.QualityValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		t.Fatal(err)
	}
	if v.Event != "THROUGHPUT_UPGRADE" || v.DisplayPhoneNumber != "15550783881" || v.MessagingLimit() != "" {
		t.Fatalf("throughput upgrade: %+v limit %q", v, v.MessagingLimit())
	}
	if e.ID != "102290129340398" || !e.At().Equal(time.Unix(1748454394, 0)) {
		t.Fatalf("entry: %+v", e)
	}
	_ = json.Unmarshal([]byte(`{"display_phone_number":"1","event":"UPGRADE","current_limit":"TIER_10K","old_limit":"TIER_2K"}`), &v)
	if v.MessagingLimit() != "TIER_10K" || v.OldLimit != "TIER_2K" {
		t.Fatalf("legacy limit: %+v", v)
	}
	_ = json.Unmarshal([]byte(`{"display_phone_number":"1","event":"UPGRADE","current_limit":"TIER_10K","max_daily_conversations_per_business":"TIER_100K"}`), &v)
	if v.MessagingLimit() != "TIER_100K" {
		t.Fatalf("portfolio limit wins: %q", v.MessagingLimit())
	}
	if (whatsapp.Entry{}).At().IsZero() {
		t.Fatal("missing time must default to now")
	}
}

func TestAccountUpdate(t *testing.T) {
	_, c := change(t, `{"entry":[{"id":"1","time":1,"changes":[{"value":{"event":"ACCOUNT_RESTRICTION",
		"restriction_info":[{"restriction_type":"RESTRICTED_BIZ_INITIATED_MESSAGING","expiration":1641330498}]},
		"field":"account_update"}]}],"object":"whatsapp_business_account"}`)
	var v whatsapp.AccountUpdateValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		t.Fatal(err)
	}
	if v.Event != "ACCOUNT_RESTRICTION" || len(v.RestrictionInfo) != 1 || v.RestrictionInfo[0].RestrictionType != "RESTRICTED_BIZ_INITIATED_MESSAGING" {
		t.Fatalf("restriction: %+v", v)
	}
	_ = json.Unmarshal([]byte(`{"event":"ACCOUNT_VIOLATION","phone_number":"15550783881","violation_info":{"violation_type":"SCAM"}}`), &v)
	if v.PhoneNumber != "15550783881" || v.ViolationInfo == nil || v.ViolationInfo.ViolationType != "SCAM" {
		t.Fatalf("violation: %+v", v)
	}
}

func TestTemplateQualityUpdate(t *testing.T) {
	_, c := change(t, `{"entry":[{"id":"102290129340398","time":1674864290,"changes":[{"value":{
		"previous_quality_score":"GREEN","new_quality_score":"YELLOW","message_template_id":806312974732579,
		"message_template_name":"welcome_template","message_template_language":"en-US"},
		"field":"message_template_quality_update"}]}],"object":"whatsapp_business_account"}`)
	var v whatsapp.TemplateQualityValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		t.Fatal(err)
	}
	if v.NewQualityScore != "YELLOW" || v.PreviousQualityScore != "GREEN" || v.MessageTemplateID.String() != "806312974732579" ||
		v.MessageTemplateName != "welcome_template" || v.MessageTemplateLanguage != "en-US" {
		t.Fatalf("template quality: %+v", v)
	}
}

func TestCapabilityUpdate(t *testing.T) {
	cases := map[string]string{
		`{"max_daily_conversations_per_business":"TIER_2K","max_phone_numbers_per_waba":25}`: "TIER_2K",
		`{"max_daily_conversations_per_business":2000,"max_phone_numbers_per_waba":25}`:      "TIER_2K",
		`{"max_daily_conversation_per_phone":-1}`:                                            "TIER_UNLIMITED",
		`{"max_daily_conversation_per_phone":250,"max_phone_numbers_per_business":2}`:        "TIER_250",
		`{"max_phone_numbers_per_waba":25}`:                                                  "",
	}
	for body, want := range cases {
		_, c := change(t, `{"entry":[{"id":"1","changes":[{"field":"business_capability_update","value":`+body+`}]}]}`)
		var v whatsapp.CapabilityValue
		if err := json.Unmarshal(c.Value, &v); err != nil {
			t.Fatal(err)
		}
		if got := v.MessagingLimit(); got != want {
			t.Errorf("%s: %q, want %q", body, got, want)
		}
	}
}

func TestUserPreferences(t *testing.T) {
	_, c := change(t, `{"object":"whatsapp_business_account","entry":[{"id":"102290129340398","changes":[{"value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550783881","phone_number_id":"106540352242922"},
		"contacts":[{"profile":{"name":"Sheena Nelson"},"wa_id":"16505551234","user_id":"US.13491208655302741918"}],
		"user_preferences":[{"wa_id":"16505551234","user_id":"US.13491208655302741918","detail":"User requested to stop marketing messages",
		"category":"marketing_messages","value":"stop","timestamp":1731705721}]},"field":"user_preferences"}]}]}`)
	var v whatsapp.UserPreferencesValue
	if err := json.Unmarshal(c.Value, &v); err != nil {
		t.Fatal(err)
	}
	p := v.UserPreferences[0]
	if v.Metadata.PhoneNumberID != "106540352242922" || p.WaID != "16505551234" || p.UserID != "US.13491208655302741918" ||
		p.Category != "marketing_messages" || p.Value != "stop" || !p.Time().Equal(time.Unix(1731705721, 0)) || p.Detail == "" {
		t.Fatalf("user preference: %+v", v)
	}
}

func TestThroughputMPS(t *testing.T) {
	for level, want := range map[string]int{"STANDARD": 80, "HIGH": 1000, "high": 1000, "NOT_APPLICABLE": 0, "": 0} {
		var n whatsapp.PhoneNumber
		n.Throughput.Level = level
		if got := n.ThroughputMPS(); got != want {
			t.Errorf("%q: %d, want %d", level, got, want)
		}
	}
}

func TestPricingAnalyticsAndCurrency(t *testing.T) {
	m := whatsapptest.New()
	defer m.Close()
	c := newClient(t, m, 5*time.Second)
	m.SetCurrency("555", "EUR")
	cost := 0.25
	m.SetPricingAnalytics("555", []whatsapp.PricingDataPoint{
		{Start: 1748847600, End: 1748934000, Country: "DE", PhoneNumber: "15550783881", PricingType: "REGULAR", PricingCategory: "MARKETING", Volume: 5, Cost: &cost},
		{Start: 1748847600, End: 1748934000, Country: "DE", PricingType: "FREE_ENTRY_POINT", PricingCategory: "SERVICE", Volume: 2},
	})
	ctx := context.Background()
	cur, err := c.GetAccountCurrency(ctx, "555", "tok")
	if err != nil || cur != "EUR" {
		t.Fatalf("currency %q %v", cur, err)
	}
	start := time.Unix(1748761200, 0)
	points, err := c.PricingAnalytics(ctx, "555", "tok", whatsapp.PricingQuery{Start: start, End: start.Add(72 * time.Hour),
		Granularity: "DAILY", Dimensions: []string{"PHONE", "COUNTRY"}, MetricTypes: []string{"COST", "VOLUME"}})
	if err != nil || len(points) != 2 || *points[0].Cost != 0.25 || points[1].Cost != nil || points[0].PhoneNumber != "15550783881" {
		t.Fatalf("points %+v %v", points, err)
	}
	q := m.PricingQueries()
	if len(q) != 1 || q[0] != "pricing_analytics.start(1748761200).end(1749020400).granularity(DAILY).dimensions(PHONE,COUNTRY).metric_types(COST,VOLUME)" {
		t.Fatalf("query: %v", q)
	}
	if _, err := c.GetAccountCurrency(ctx, "unknown", "tok"); err == nil || !strings.Contains(err.Error(), "Unsupported") {
		t.Fatalf("unknown waba: %v", err)
	}
}
