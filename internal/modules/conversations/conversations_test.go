package conversations

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

func user(perms ...string) *principal.Principal {
	slices.Sort(perms)
	return &principal.Principal{Kind: principal.KindUser, ID: uuid.Must(uuid.NewV7()), Permissions: perms}
}

func TestVisible(t *testing.T) {
	owner := user(PermRead, PermReadAll, "conversations.assign")
	agent := user(PermRead, "conversations.reply")
	other := user(PermRead)
	stranger := user("contacts.read")
	mine, theirs := &agent.ID, &other.ID

	cases := []struct {
		who      string
		p        *principal.Principal
		assignee *uuid.UUID
		want     bool
	}{
		{"owner/unassigned", owner, nil, true},
		{"owner/agent's", owner, mine, true},
		{"owner/other's", owner, theirs, true},
		{"agent/unassigned", agent, nil, true},
		{"agent/own", agent, mine, true},
		{"agent/other's", agent, theirs, false},
		{"no permission/unassigned", stranger, nil, false},
		{"anonymous", nil, nil, false},
	}
	for _, c := range cases {
		if got := Visible(c.p, c.assignee); got != c.want {
			t.Errorf("%s: visible=%v, want %v", c.who, got, c.want)
		}
	}
	if !SeesAll(owner) || SeesAll(agent) || SeesAll(nil) {
		t.Fatal("SeesAll")
	}
}

func TestFilterParams(t *testing.T) {
	agent := user(PermRead)
	owner := user(PermRead, PermReadAll)
	someone := uuid.Must(uuid.NewV7())

	p, err := Filter{Assignee: "me"}.params(agent)
	if err != nil || p.AssigneeID == nil || *p.AssigneeID != agent.ID || p.SeeAll || p.ViewerID != agent.ID {
		t.Fatalf("me: %+v %v", p, err)
	}
	p, _ = Filter{Assignee: "unassigned"}.params(owner)
	if !p.OnlyUnassigned || p.AssigneeID != nil || !p.SeeAll {
		t.Fatalf("unassigned: %+v", p)
	}
	p, _ = Filter{Assignee: someone.String()}.params(owner)
	if p.AssigneeID == nil || *p.AssigneeID != someone {
		t.Fatalf("by id: %+v", p)
	}
	if _, err := (Filter{Assignee: "nobody"}).params(owner); !isValidation(err, "assignee") {
		t.Fatalf("bad assignee: %v", err)
	}
	// Search is a contains match with LIKE wildcards escaped.
	p, _ = Filter{Query: `  50%_off\ `}.params(owner)
	if p.Q == nil || *p.Q != `%50\%\_off\\%` {
		t.Fatalf("q: %v", p.Q)
	}
	if _, err := (Filter{Query: strings.Repeat("x", 101)}).params(owner); !isValidation(err, "q") {
		t.Fatalf("long q: %v", err)
	}
	p, _ = Filter{}.params(owner)
	if p.Q != nil || p.AssigneeID != nil || p.OnlyUnassigned || p.OnlyUnread {
		t.Fatalf("empty filter: %+v", p)
	}
}

func isValidation(err error, field string) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Status == 422 && ae.Field == field
}

func TestConversationToAPIWindow(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	in := now.Add(-23 * time.Hour)
	name, dir, typ, body, status := "Ali Agent", "inbound", "text", "  çok\n  uzun   bir mesaj ", "received"
	assignee := uuid.Must(uuid.NewV7())
	row := db.ListConversationsRow{ID: uuid.Must(uuid.NewV7()), Status: "open", AssigneeID: &assignee, AssigneeName: &name,
		LastInboundAt: &in, LastDirection: &dir, LastType: &typ, LastBody: &body, LastStatus: &status, UnreadCount: 2}

	c := conversationToAPI(row, now)
	if !c.WindowOpen || c.WindowExpiresAt == nil || !c.WindowExpiresAt.Equal(in.Add(24*time.Hour)) {
		t.Fatalf("window: %v %v", c.WindowOpen, c.WindowExpiresAt)
	}
	if c.Assignee == nil || c.Assignee.Name != name || c.LastMessage == nil || c.LastMessage.Body != "çok uzun bir mesaj" {
		t.Fatalf("conversation: %+v", c)
	}
	if c := conversationToAPI(row, now.Add(2*time.Hour)); c.WindowOpen {
		t.Fatal("window must close 24 hours after the last customer message")
	}
	row.LastInboundAt, row.AssigneeID, row.LastDirection = nil, nil, nil
	if c := conversationToAPI(row, now); c.WindowOpen || c.WindowExpiresAt != nil || c.Assignee != nil || c.LastMessage != nil {
		t.Fatalf("never written: %+v", c)
	}
	if got := preview(strings.Repeat("a", 300)); len([]rune(got)) != 140 || !strings.HasSuffix(got, "…") {
		t.Fatalf("preview %q", got)
	}
}

func TestMediaHeaders(t *testing.T) {
	id := uuid.MustParse("0193a6c4-7f2e-7b3c-9d1e-4a5b6c7d8e9f")
	cases := []struct {
		mime, name     string
		ctype, disp    string
		dispStartsWith string
	}{
		{"image/jpeg", "", "image/jpeg", `inline; filename=media-0193a6c4-7f2e-7b3c-9d1e-4a5b6c7d8e9f.jpg`, ""},
		{"audio/ogg; codecs=opus", "", "audio/ogg; codecs=opus", "", "inline;"},
		{"application/pdf", "invoice.pdf", "application/pdf", "attachment; filename=invoice.pdf", ""},
		// Active content is never rendered inline.
		{"image/svg+xml", "x.svg", "image/svg+xml", "attachment; filename=x.svg", ""},
		{"text/html", "page.html", "text/html", "attachment; filename=page.html", ""},
		// Garbage from the sender is neutralised.
		{"not a mime", `..\..\etc/passwd`, "application/octet-stream", "attachment; filename=passwd", ""},
		{"image/png; boundary=x; name=evil", "a\"b\r\n.png", "image/png", `inline; filename=ab.png`, ""},
		{"", "rapor şubat.xlsx", "application/octet-stream", "", "attachment; filename*=utf-8''rapor%20%C5%9Fubat.xlsx"},
	}
	for _, c := range cases {
		ct, d := mediaHeaders(c.mime, c.name, id)
		if ct != c.ctype {
			t.Errorf("%q: content type %q, want %q", c.mime, ct, c.ctype)
		}
		if c.disp != "" && d != c.disp {
			t.Errorf("%q %q: disposition %q, want %q", c.mime, c.name, d, c.disp)
		}
		if c.dispStartsWith != "" && !strings.HasPrefix(d, c.dispStartsWith) {
			t.Errorf("%q %q: disposition %q, want prefix %q", c.mime, c.name, d, c.dispStartsWith)
		}
	}
}

func TestReplyInput(t *testing.T) {
	preview := true
	in, _, err := replyInput(api.ReplyRequest{Type: "text", Text: &struct {
		Body       string `json:"body"`
		PreviewUrl *bool  `json:"preview_url,omitempty"`
	}{Body: "Merhaba", PreviewUrl: &preview}})
	if err != nil || in.Type != "text" || in.Text == nil || in.Text.Body != "Merhaba" || !in.Text.PreviewURL {
		t.Fatalf("text: %+v %v", in, err)
	}
	comps := []map[string]any{{"type": "body", "parameters": []any{map[string]any{"type": "text", "text": "42"}}}}
	in, params, err := replyInput(api.ReplyRequest{Type: "template", Template: &struct {
		Components *[]map[string]any   `json:"components,omitempty"`
		Language   string              `json:"language"`
		Name       string              `json:"name"`
		Params     *api.CampaignParams `json:"params,omitempty"`
	}{Name: "order_update", Language: "en", Components: &comps}})
	if err != nil || params != nil || in.Template == nil || in.Template.Name != "order_update" || len(in.Template.Components) != 1 {
		t.Fatalf("template: %+v %v", in, err)
	}
	link := "https://example.com/a.jpg"
	in, _, _ = replyInput(api.ReplyRequest{Type: "image", Media: &api.MediaRef{Link: &link}})
	if in.Media == nil || in.Media.Link != link {
		t.Fatalf("media: %+v", in)
	}
}
