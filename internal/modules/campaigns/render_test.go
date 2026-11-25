package campaigns

import (
	"encoding/json"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

func sp(s string) *string { return &s }

func TestRender(t *testing.T) {
	c := db.Contact{PhoneE164: "+905551112233", Name: "Ayşe Yılmaz", Var1: sp("Gold"), Attributes: []byte(`{"city":"Izmir"}`)}

	tpl, miss := Render(Shape{}, Params{Body: json.RawMessage(`["{{first_name}}", "Seviye: {{var1}}", "{{attr:city}}"]`), Buttons: []string{"orders/{{phone}}"}}, c, "promo", "tr")
	if miss != "" {
		t.Fatalf("missing %q", miss)
	}
	if tpl.Name != "promo" || tpl.Language.Code != "tr" || len(tpl.Components) != 2 {
		t.Fatalf("template: %+v", tpl)
	}
	b := tpl.Components[0].Parameters
	if b[0].Text != "Ayşe" || b[1].Text != "Seviye: Gold" || b[2].Text != "Izmir" {
		t.Fatalf("body: %+v", b)
	}
	if btn := tpl.Components[1]; btn.Type != "button" || btn.SubType != "url" || btn.Index != "0" || btn.Parameters[0].Text != "orders/+905551112233" {
		t.Fatalf("button: %+v", btn)
	}

	// Named parameters keep their names, in a stable order.
	tpl, _ = Render(Shape{}, Params{Body: json.RawMessage(`{"tier":"{{var1}}","first_name":"{{first_name}}"}`)}, c, "p", "en")
	if p := tpl.Components[0].Parameters; p[0].ParameterName != "first_name" || p[1].ParameterName != "tier" {
		t.Fatalf("named: %+v", p)
	}

	// Empty values skip the recipient unless a fallback exists.
	if _, miss := Render(Shape{}, Params{Body: json.RawMessage(`["{{var2}}"]`)}, c, "p", "en"); miss != "var2" {
		t.Fatalf("missing var2 not reported: %q", miss)
	}
	tpl, miss = Render(Shape{}, Params{Body: json.RawMessage(`["{{var2}}"]`), Fallback: map[string]string{"var2": "-"}}, c, "p", "en")
	if miss != "" || tpl.Components[0].Parameters[0].Text != "-" {
		t.Fatalf("fallback: %q %+v", miss, tpl)
	}
	// Profile name stands in for a missing name; no parameters → no components.
	tpl, _ = Render(Shape{}, Params{}, db.Contact{ProfileName: "Ali"}, "p", "en")
	if len(tpl.Components) != 0 {
		t.Fatal("unexpected components")
	}
	if v := value("name", db.Contact{ProfileName: "Ali"}); v != "Ali" {
		t.Fatalf("profile fallback: %q", v)
	}
	// Literal text without placeholders is passed through; an empty literal is missing.
	if _, miss := Render(Shape{}, Params{Body: json.RawMessage(`["  "]`)}, c, "p", "en"); miss == "" {
		t.Fatal("empty literal accepted")
	}
}

func TestAudienceValidation(t *testing.T) {
	id := db.Contact{}.ID
	for _, a := range []Audience{{}, {All: true, TagID: &id}, {SegmentID: &id, GroupID: &id}} {
		if a.validate() == nil {
			t.Errorf("accepted %+v", a)
		}
	}
	for _, a := range []Audience{{All: true}, {TagID: &id}, {GroupID: &id}, {SegmentID: &id}} {
		if err := a.validate(); err != nil {
			t.Errorf("rejected %+v: %v", a, err)
		}
	}
}

func TestRenderHeaderMedia(t *testing.T) {
	c := db.Contact{PhoneE164: "+905551112233", Name: "Ayşe"}
	tpl, miss := Render(Shape{}, Params{HeaderMedia: &HeaderMedia{Type: "document", Link: "https://example.com/menu.pdf", Filename: "menu.pdf"},
		Body: json.RawMessage(`["{{first_name}}"]`)}, c, "menu", "tr")
	if miss != "" || len(tpl.Components) != 2 {
		t.Fatalf("components: %+v %q", tpl.Components, miss)
	}
	h := tpl.Components[0]
	if h.Type != "header" || h.Parameters[0].Type != "document" || h.Parameters[0].Document.Link != "https://example.com/menu.pdf" || h.Parameters[0].Document.Filename != "menu.pdf" {
		t.Fatalf("header: %+v", h)
	}
	if tpl.Components[1].Parameters[0].Text != "Ayşe" {
		t.Fatal("body lost")
	}
	// Empty button entries mean "no dynamic part" and are skipped.
	tpl, miss = Render(Shape{}, Params{Buttons: []string{"", "orders/{{phone}}"}}, c, "b", "en")
	if miss != "" || len(tpl.Components) != 1 || tpl.Components[0].Index != "1" {
		t.Fatalf("buttons: %+v %q", tpl.Components, miss)
	}
}
