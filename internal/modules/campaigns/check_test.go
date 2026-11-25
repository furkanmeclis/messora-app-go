package campaigns

import (
	"encoding/json"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

func TestCheckParams(t *testing.T) {
	image := json.RawMessage(`[{"type":"HEADER","format":"IMAGE","example":{"header_handle":["https://x"]}},
		{"type":"BODY","text":"Hi {{1}}, your order {{2}} shipped."},
		{"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Stop"},{"type":"URL","text":"Track","url":"https://t.example/{{1}}"}]}]`)
	named := json.RawMessage(`[{"type":"HEADER","format":"TEXT","text":"Hello {{first_name}}"},{"type":"BODY","text":"Code {{code}}"}]`)
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	ok := Params{Body: raw([]string{"{{first_name}}", "A-1"}), Buttons: []string{"", "{{var1}}"},
		HeaderMedia: &HeaderMedia{Type: "image", Link: "https://cdn.example/p.jpg"}}

	sh := ShapeOf(image, "POSITIONAL", "MARKETING")
	if sh.HeaderMedia != "image" || len(sh.Body) != 2 || len(sh.URLButtons) != 1 || sh.URLButtons[0] != 1 {
		t.Fatalf("shape: %+v", sh)
	}
	cases := []struct {
		name  string
		sh    Shape
		p     Params
		field string
	}{
		{"complete", sh, ok, ""},
		{"no media", sh, Params{Body: ok.Body, Buttons: ok.Buttons}, "template_params.header_media"},
		{"wrong media", sh, Params{Body: ok.Body, Buttons: ok.Buttons, HeaderMedia: &HeaderMedia{Type: "video", Link: "https://x"}}, "template_params.header_media.type"},
		{"missing body value", sh, Params{Body: raw([]string{"x"}), Buttons: ok.Buttons, HeaderMedia: ok.HeaderMedia}, "template_params.body.2"},
		{"blank body value", sh, Params{Body: raw([]string{"x", " "}), Buttons: ok.Buttons, HeaderMedia: ok.HeaderMedia}, "template_params.body.2"},
		{"missing button", sh, Params{Body: ok.Body, HeaderMedia: ok.HeaderMedia}, "template_params.buttons.1"},
		{"named ok", ShapeOf(named, "NAMED", "UTILITY"), Params{Header: raw(map[string]string{"first_name": "{{first_name}}"}), Body: raw(map[string]string{"code": "42"})}, ""},
		{"named missing", ShapeOf(named, "NAMED", "UTILITY"), Params{Header: raw(map[string]string{"first_name": "x"})}, "template_params.body.code"},
		{"media on text template", ShapeOf(named, "NAMED", "UTILITY"), Params{HeaderMedia: ok.HeaderMedia}, "template_params.header_media"},
		{"location missing", Shape{HeaderMedia: "location"}, Params{}, "template_params.location"},
	}
	for _, c := range cases {
		err := CheckParams(c.sh, c.p)
		if c.field == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		e, isApp := apperr.As(err)
		if !isApp || e.Field != c.field {
			t.Errorf("%s: want error on %s, got %v", c.name, c.field, err)
		}
	}
}
