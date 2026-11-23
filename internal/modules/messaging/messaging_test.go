package messaging

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"+905551112233":       "+905551112233",
		"905551112233":        "+905551112233",
		"+90 (555) 111-22-33": "+905551112233",
		" +1 415.555.0100 ":   "+14155550100",
	}
	for in, want := range ok {
		if got, valid := NormalizePhone(in); !valid || got != want {
			t.Errorf("%q → %q %v, want %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "12", "+0123456789", "90555x112233", "++905551112233", "+9055511122334455", "5551112233+"} {
		if got, valid := NormalizePhone(in); valid {
			t.Errorf("%q accepted as %q", in, got)
		}
	}
}

func TestBuildValidatesEveryType(t *testing.T) {
	long := strings.Repeat("a", 4097)
	bad := []SendInput{
		{Type: "text"},
		{Type: "text", Text: &whatsapp.Text{Body: "  "}},
		{Type: "text", Text: &whatsapp.Text{Body: long}},
		{Type: "template", Template: &whatsapp.Template{Name: "x"}},
		{Type: "image"},
		{Type: "image", Media: &whatsapp.Media{ID: "1", Link: "https://x/y.png"}},
		{Type: "image", Media: &whatsapp.Media{Link: "http://insecure/y.png"}},
		{Type: "audio", Media: &whatsapp.Media{ID: "1", Caption: "no"}},
		{Type: "image", Media: &whatsapp.Media{ID: "1", Filename: "a.png"}},
		{Type: "location", Location: &whatsapp.Location{Latitude: 91}},
		{Type: "reaction", Reaction: &whatsapp.Reaction{Emoji: "👍"}},
		{Type: "interactive", Interactive: json.RawMessage(`{"type":"carousel","body":{"text":"x"}}`)},
		{Type: "interactive", Interactive: json.RawMessage(`{"type":"button"}`)},
		{Type: "contacts"},
	}
	for i, in := range bad {
		if _, _, err := in.build(); err == nil {
			t.Errorf("case %d (%s) accepted", i, in.Type)
		} else if ae, ok := apperr.As(err); !ok || ae.Status != 422 {
			t.Errorf("case %d: want a validation error, got %v", i, err)
		}
	}

	good := []struct {
		in   SendInput
		body string
	}{
		{SendInput{Type: "text", Text: &whatsapp.Text{Body: "hi"}, ReplyTo: "wamid.1"}, "hi"},
		{SendInput{Type: "template", Template: &whatsapp.Template{Name: "t", Language: whatsapp.TemplateLanguage{Code: "en"}}}, "[template] t"},
		{SendInput{Type: "document", Media: &whatsapp.Media{Link: "https://x/a.pdf", Filename: "a.pdf", Caption: "invoice"}}, "invoice"},
		{SendInput{Type: "sticker", Media: &whatsapp.Media{ID: "123"}}, ""},
		{SendInput{Type: "location", Location: &whatsapp.Location{Latitude: 41.01, Longitude: 28.97, Name: "Ofis"}}, "Ofis"},
		{SendInput{Type: "reaction", Reaction: &whatsapp.Reaction{MessageID: "wamid.2", Emoji: "👍"}}, "👍"},
		{SendInput{Type: "interactive", Interactive: json.RawMessage(`{"type":"button","body":{"text":"Pick"},"action":{"buttons":[]}}`)}, "Pick"},
	}
	for _, g := range good {
		m, body, err := g.in.build()
		if err != nil {
			t.Fatalf("%s rejected: %v", g.in.Type, err)
		}
		if body != g.body || m.Type != g.in.Type {
			t.Errorf("%s: body %q type %q", g.in.Type, body, m.Type)
		}
	}
	m, _, _ := good[0].in.build()
	if m.Context == nil || m.Context.MessageID != "wamid.1" {
		t.Error("reply context missing")
	}
	m, _, _ = good[2].in.build()
	if m.Document == nil || m.Image != nil {
		t.Error("media placed in the wrong field")
	}
}

func TestMediaStoreRejectsEscapes(t *testing.T) {
	s := NewMediaStore(t.TempDir())
	for _, k := range []string{"", "../etc/passwd", "/etc/passwd", "a/../../b"} {
		if _, err := s.Open(k); err == nil {
			t.Errorf("key %q opened", k)
		}
	}
}
