package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

func TestAttributeCampaign(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour
	sends := []CampaignSend{{CampaignID: a, SentAt: t0}, {CampaignID: b, SentAt: t0.Add(72 * time.Hour)}}
	cases := []struct {
		name string
		at   time.Time
		want *uuid.UUID
	}{
		{"before any campaign", t0.Add(-time.Second), nil},
		{"at the send instant", t0, &a},
		{"after A, before B", t0.Add(71 * time.Hour), &a},
		{"B reached the contact: B from then on", t0.Add(72 * time.Hour), &b},
		{"after B", t0.Add(100 * time.Hour), &b},
		{"exactly at the end of B's window", t0.Add(72*time.Hour + week), &b},
		{"past B's window: outside campaigns", t0.Add(72*time.Hour + week + time.Second), nil},
	}
	for _, c := range cases {
		got := Attribute(sends, c.at, week)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	// A's window alone: A is not resurrected once B took over and expired.
	if got := Attribute(sends[:1], t0.Add(week+time.Hour), week); got != nil {
		t.Errorf("outside A's window: %v", got)
	}
	// Order of the input does not matter; each instant has one campaign.
	rev := []CampaignSend{sends[1], sends[0]}
	for h := -2; h < 300; h++ {
		at := t0.Add(time.Duration(h) * time.Hour)
		x, y := Attribute(sends, at, week), Attribute(rev, at, week)
		if (x == nil) != (y == nil) || (x != nil && *x != *y) {
			t.Fatalf("order changed the attribution at %v", at)
		}
	}
	if Attribute(nil, t0, week) != nil {
		t.Error("no sends")
	}
}

func TestAttributionWindowSetting(t *testing.T) {
	if (Settings{}).AttributionDays() != 7 || (Settings{CampaignAttributionDays: 3}).AttributionWindow() != 72*time.Hour ||
		(Settings{CampaignAttributionDays: 500}).AttributionDays() != MaxCampaignAttributionDays {
		t.Fatal("attribution window")
	}
}

func TestValidateValue(t *testing.T) {
	sel := DataField{Key: "attr:plan", Type: FieldSelect, Options: []string{"Gold", "Silver"}}
	cases := []struct {
		f      DataField
		in     string
		want   string
		wantOK bool
	}{
		{DataField{Key: "email"}, " Ayse@Example.com ", "ayse@example.com", true},
		{DataField{Key: "email"}, "ayse at example", "", false},
		{DataField{Key: "email"}, "Ayşe <ayse@example.com>", "", false},
		{DataField{Key: "email"}, "ayse@localhost", "", false},
		{DataField{Key: "var1", Type: FieldPhone}, "+90 (555) 111-22-33", "+905551112233", true},
		{DataField{Key: "var1", Type: FieldPhone}, "0090 555 111 22 33", "+905551112233", true},
		{DataField{Key: "var1", Type: FieldPhone}, "0555 111 22 33", "", false},
		{DataField{Key: "var1", Type: FieldPhone}, "call me", "", false},
		{sel, "gold", "Gold", true},
		{sel, "Bronze", "", false},
		{DataField{Key: "var2", Type: FieldNumber}, "12,5", "12.5", true},
		{DataField{Key: "var2", Type: FieldNumber}, "1e9", "", false},
		{DataField{Key: "var3", Type: FieldDate}, "25.11.2026", "2026-11-25", true},
		{DataField{Key: "var3", Type: FieldDate}, "yarın", "", false},
		{DataField{Key: "name"}, "  Ayşe Yılmaz ", "Ayşe Yılmaz", true},
		{DataField{Key: "name"}, "   ", "", false},
	}
	for _, c := range cases {
		got, ok := ValidateValue(c.f, c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s %q: got %q %v, want %q %v", c.f.Key, c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestMayAIWrite(t *testing.T) {
	cases := []struct {
		name    string
		current string
		src     *FieldSource
		want    bool
	}{
		{"empty field", "", nil, true},
		{"empty field with an old record", " ", &FieldSource{Source: SourceUser, Verified: true}, true},
		{"value without provenance (people, import, API)", "Acme", nil, false},
		{"set by a person", "Acme", &FieldSource{Source: SourceUser, Verified: true}, false},
		{"imported", "Acme", &FieldSource{Source: SourceImport}, false},
		{"from the API", "Acme", &FieldSource{Source: SourceAPI}, false},
		{"AI, not verified", "Acme", &FieldSource{Source: SourceAI}, true},
		{"AI, verified by a person", "Acme", &FieldSource{Source: SourceAI, Verified: true}, false},
	}
	for _, c := range cases {
		if got := MayAIWrite(c.current, c.src); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestProgressOf(t *testing.T) {
	fields := []DataField{{Key: "name", Required: true}, {Key: "attr:company", Required: true}, {Key: "email"}}
	p := ProgressOf(fields, map[string]string{"name": "Ayşe"})
	if p.Required != 2 || p.Filled != 1 || len(p.Missing) != 1 || p.Missing[0] != "attr:company" || p.Complete() {
		t.Fatalf("progress: %+v", p)
	}
	if p = ProgressOf(fields, map[string]string{"name": "Ayşe", "attr:company": "Acme"}); !p.Complete() || len(p.Missing) != 0 {
		t.Fatalf("complete: %+v", p)
	}
}

func TestSensitiveFieldsAreRefused(t *testing.T) {
	for _, f := range [][2]string{{"attr:health", "Sağlık durumu"}, {"attr:x", "Dini inanç"}, {"attr:tckn", "TC kimlik no"},
		{"attr:card", "Kredi kartı numarası"}, {"var1", "Password"}, {"attr:blood", "Kronik hastalık"}, {"attr:p", "Political views"}} {
		if !SensitiveField(f[0], f[1]) {
			t.Errorf("%v not refused", f)
		}
	}
	for _, f := range [][2]string{{"name", "Ad soyad"}, {"attr:company", "Firma"}, {"email", "E-posta"}, {"attr:city", "Şehir"},
		{"attr:need", "İhtiyaç"}, {"var1", "Bütçe"}} {
		if SensitiveField(f[0], f[1]) {
			t.Errorf("%v refused", f)
		}
	}
	p := Profile{Name: "x", DisclosureMode: DiscloseFirstMessage, Status: ProfileActive,
		DataCollection: DataCollection{Enabled: true, Fields: []DataField{{Key: "attr:health", Label: "Sağlık"}}}}
	if err := p.normalize(); err == nil {
		t.Fatal("a sensitive field was accepted")
	}
	p.DataCollection.Fields = []DataField{{Key: "attr:plan", Label: "Plan", Type: FieldSelect}}
	if err := p.normalize(); err == nil {
		t.Fatal("a select without options was accepted")
	}
	p.DataCollection.Fields = []DataField{{Key: "attr:plan", Label: "Plan", Type: FieldSelect, Options: []string{"Gold", "gold", " Silver "}}}
	if err := p.normalize(); err != nil || len(p.DataCollection.Fields[0].Options) != 2 {
		t.Fatalf("options: %v %v", err, p.DataCollection.Fields)
	}
}

func TestBudget(t *testing.T) {
	cases := []struct {
		limit, spent string
		want         bool
	}{
		{"", "100", false}, {"50", "49.999999", false}, {"50", "50", true}, {"50", "50.000001", true}, {"12.5", "0", false}, {"x", "9", false},
	}
	for _, c := range cases {
		if got := OverBudget(c.limit, c.spent); got != c.want {
			t.Errorf("OverBudget(%q, %q) = %v", c.limit, c.spent, got)
		}
	}
	if m := MonthStart(time.Date(2026, 11, 30, 23, 59, 0, 0, time.FixedZone("TRT", 3*3600))); !m.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("month start %v", m)
	}
}

func TestTotalCost(t *testing.T) {
	if tot, cur := Total("0.25", []db.CampaignMetaCostRow{{Currency: "USD", Cost: "1.5"}}); tot == nil || *tot != "1.75" || *cur != "USD" {
		t.Fatalf("usd total %v", tot)
	}
	if tot, _ := Total("0.25", nil); tot == nil || *tot != "0.25" {
		t.Fatal("no meta cost")
	}
	if tot, _ := Total("0.25", []db.CampaignMetaCostRow{{Currency: "TRY", Cost: "30"}}); tot != nil {
		t.Fatal("currencies differ: no total")
	}
}

func TestDecodeClassification(t *testing.T) {
	c, err := DecodeClassification([]byte(`{"sentiment":" Positive","confidence":1.4,"reason":"ilgili"}`))
	if err != nil || c.Sentiment != SentimentPositive || c.Confidence != 1 {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := DecodeClassification([]byte(`{"sentiment":"angry","confidence":0.5,"reason":""}`)); err == nil {
		t.Fatal("unknown sentiment accepted")
	}
	req := ClassifyRequest(ClassifyInput{Text: "Evet, ilgileniyorum", CampaignName: "Kasım", Lang: LangTR})
	if req.Output == nil || req.Output.Name != ClassifySchemaName || len(req.Messages) != 1 || req.Messages[0].Role != RoleUser {
		t.Fatalf("request %+v", req)
	}
}

func TestLeadCaptureInstructions(t *testing.T) {
	p := Profile{DataCollection: DataCollection{Enabled: true, Fields: []DataField{{Key: "name", Label: "Ad", Required: true},
		{Key: "attr:company", Label: "Firma", Required: true}}}}
	s := LeadCaptureInstructions(p, ProgressOf(p.DataCollection.Fields, map[string]string{"name": "Ayşe"}))
	for _, want := range []string{"first time", "one or two items", "Firma (attr:company)"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, "Ad (name)") {
		t.Error("a known field is still asked for")
	}
}
