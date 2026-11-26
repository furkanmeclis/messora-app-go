package conversations

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
)

func TestLeadCaptureOnlyForUnknownNumbers(t *testing.T) {
	now := time.Now()
	st := func(s string) *string { return &s }
	p := ai.Profile{DataCollection: ai.DataCollection{Enabled: true, MayCreateContact: true,
		Fields: []ai.DataField{{Key: "name", Label: "Ad", Required: true}, {Key: "attr:company", Label: "Firma", Required: true}}}}
	unknown := db.Contact{CreatedByInboundAt: &now, ProfileStatus: st(ProfileNew), Attributes: []byte(`{}`)}
	cases := []struct {
		name string
		p    ai.Profile
		c    db.Contact
		want bool
	}{
		{"new contact from an unknown number", p, unknown, true},
		{"collecting", p, db.Contact{CreatedByInboundAt: &now, ProfileStatus: st(ProfileCollecting)}, true},
		{"complete", p, db.Contact{CreatedByInboundAt: &now, ProfileStatus: st(ProfileComplete)}, false},
		{"known contact", p, db.Contact{}, false},
		{"profile may not create contacts", ai.Profile{DataCollection: ai.DataCollection{Enabled: true, Fields: p.DataCollection.Fields}}, unknown, false},
		{"no data collection", ai.Profile{DataCollection: ai.DataCollection{MayCreateContact: true, Fields: p.DataCollection.Fields}}, unknown, false},
	}
	for _, c := range cases {
		if got := LeadCapture(c.p, c.c); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	req := BuildTurn(TurnInput{Profile: p, Contact: db.Contact{CreatedByInboundAt: &now, ProfileStatus: st(ProfileCollecting), Name: "Ayşe",
		Attributes: []byte(`{}`)}, Lang: ai.LangTR})
	if !strings.Contains(req.System, "## New customer") || !strings.Contains(req.System, "Firma (attr:company)") ||
		strings.Contains(req.System, "Ad (name)") {
		t.Fatalf("lead capture instructions: %s", req.System)
	}
	if req = BuildTurn(TurnInput{Profile: p, Contact: db.Contact{Attributes: []byte(`{}`)}}); strings.Contains(req.System, "## New customer") {
		t.Fatal("a known contact got the new-customer rules")
	}
}

func TestContactValues(t *testing.T) {
	e := "a@b.co"
	v := ContactValues(db.Contact{Name: "Ayşe", Email: &e, Attributes: []byte(`{"company":"Acme","n":3}`)})
	if v["name"] != "Ayşe" || v["email"] != e || v["attr:company"] != "Acme" || v["attr:n"] != "3" || v["var1"] != "" {
		t.Fatalf("%v", v)
	}
}
