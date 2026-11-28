//go:build integration

package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
)

// mediaTemplate adds (and syncs) an approved template with an image
// header, positional body variables and a dynamic URL button in position 1.
func (h *harness) mediaTemplate(token string) {
	h.t.Helper()
	defer func() {
		acc := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: token}).list()[0].(map[string]any)["id"].(string)
		h.expect(h.do(call{method: "POST", path: "/api/v1/channels/accounts/" + acc + "/sync", token: token}), 200, "")
	}()
	h.meta.AddTemplate(testWABA, whatsapp.TemplateInfo{ID: "900", Name: "photo_offer", Language: "tr", Category: "MARKETING", Status: "APPROVED",
		Components: []byte(`[{"type":"HEADER","format":"IMAGE","example":{"header_handle":["https://example.test/h.jpg"]}},
			{"type":"BODY","text":"Merhaba {{1}}, {{2}} için indirim!"},
			{"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Dur"},{"type":"URL","text":"Bak","url":"https://shop.test/{{1}}"}]}]`)})
}

// sentTemplate returns the last template Meta received.
func (h *harness) sentTemplate() whatsapp.Template {
	h.t.Helper()
	sent := h.meta.Sent()
	for i := len(sent) - 1; i >= 0; i-- {
		if sent[i].Message.Template != nil {
			return *sent[i].Message.Template
		}
	}
	h.t.Fatal("no template was sent")
	return whatsapp.Template{}
}

func TestMediaTemplateCampaign(t *testing.T) {
	h := newHarness(t)
	token, _ := h.connect()
	h.mediaTemplate(token)
	h.svc.campaigns.TickInterval = 0
	tag := h.seedContacts(3, "+90552")
	create := func(params map[string]any) string {
		r := h.do(call{method: "POST", path: "/api/v1/campaigns", token: token, body: map[string]any{"name": "Foto " + time.Now().Format("150405.000"),
			"template_name": "photo_offer", "template_language": "tr", "audience": map[string]any{"tag_id": tag}, "template_params": params}})
		h.expect(r, 201, "")
		return r.data()["id"].(string)
	}
	start := func(id string) resp {
		return h.do(call{method: "POST", path: "/api/v1/campaigns/" + id + "/start", token: token})
	}
	body := []string{"{{first_name}}", "{{var1}}"}

	// Missing image, missing button value: refused before anything is sent.
	r := start(create(map[string]any{"body": body, "buttons": []string{"", "kampanya"}}))
	h.expect(r, 422, "VALIDATION_ERROR")
	if !strings.Contains(string(r.raw), "header_media") {
		t.Fatalf("missing image: %s", r.raw)
	}
	r = start(create(map[string]any{"body": body, "header_media": map[string]any{"type": "image", "link": "https://cdn.test/p.jpg"}}))
	h.expect(r, 422, "VALIDATION_ERROR")
	if !strings.Contains(string(r.raw), "buttons") {
		t.Fatalf("missing button: %s", r.raw)
	}

	// Complete: every recipient gets the image, their own values and the
	// button suffix at the template's button position.
	id := create(map[string]any{"body": body, "buttons": []string{"", "{{var1}}"}, "header_media": map[string]any{"type": "image", "link": "https://cdn.test/p.jpg"}})
	h.expect(start(id), 200, "")
	if st := h.runCampaign(id, 30*time.Second); st != "completed" {
		t.Fatalf("campaign %s", st)
	}
	tpl := h.sentTemplate()
	byType := map[string]whatsapp.TemplateComponent{}
	for _, c := range tpl.Components {
		byType[c.Type] = c
	}
	if hdr := byType["header"]; len(hdr.Parameters) != 1 || hdr.Parameters[0].Image == nil || hdr.Parameters[0].Image.Link != "https://cdn.test/p.jpg" {
		t.Fatalf("header: %+v", byType["header"])
	}
	if b := byType["body"]; len(b.Parameters) != 2 || !strings.HasPrefix(b.Parameters[1].Text, "City ") {
		t.Fatalf("body: %+v", b)
	}
	if btn := byType["button"]; btn.Index != "1" || btn.SubType != "url" || !strings.HasPrefix(btn.Parameters[0].Text, "City ") {
		t.Fatalf("button: %+v", btn)
	}
}

func TestInboxTemplateParams(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	h.mediaTemplate(owner)
	h.inbound(testCustomer, "wamid.tp1", "Fiyat?", time.Now())
	_, _ = h.pool.Exec(h.ctx(), "UPDATE contacts SET name = 'Ayşe Yılmaz', var1 = 'yaz-indirimi' WHERE phone_e164 = $1", "+"+testCustomer)
	id := h.conversationOf(testCustomer)
	reply := func(params map[string]any) resp {
		return h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/messages", token: owner, body: map[string]any{
			"type": "template", "template": map[string]any{"name": "photo_offer", "language": "tr", "params": params}}})
	}

	// The conversation says which account's templates apply.
	conv := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: owner})
	if conv.data()["phone_number"].(map[string]any)["account_id"] == nil {
		t.Fatalf("account_id missing: %s", conv.raw)
	}

	// Values are checked against the template.
	h.expect(reply(map[string]any{"body": []string{"{{first_name}}", "x"}}), 422, "VALIDATION_ERROR")
	// A placeholder the contact lacks needs a fallback.
	r := reply(map[string]any{"body": []string{"{{first_name}}", "{{var2}}"}, "buttons": []string{"", "a"},
		"header_media": map[string]any{"type": "image", "link": "https://cdn.test/p.jpg"}})
	h.expect(r, 422, "VALIDATION_ERROR")
	if !strings.Contains(string(r.raw), "var2") {
		t.Fatalf("missing placeholder: %s", r.raw)
	}

	r = reply(map[string]any{"body": []string{"{{first_name}}", "Yaz koleksiyonu"}, "buttons": []string{"", "{{var1}}"},
		"header_media": map[string]any{"type": "image", "link": "https://cdn.test/p.jpg"}})
	h.expect(r, 202, "")
	if err := h.deliver(r.data()["id"].(string)); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(h.sentTemplate())
	for _, want := range []string{`"text":"Ayşe"`, `"text":"Yaz koleksiyonu"`, `"text":"yaz-indirimi"`, `"link":"https://cdn.test/p.jpg"`} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("sent template lacks %s: %s", want, got)
		}
	}
}

func TestTemplateSampleUpload(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.connect()
	acc := h.do(call{method: "GET", path: "/api/v1/channels/accounts", token: owner}).list()[0].(map[string]any)["id"].(string)
	png := []byte("\x89PNG\r\n\x1a\nfake image bytes")
	path := "/api/v1/templates/samples?account_id=" + acc + "&filename=ornek.png"

	// The account needs an App ID (resumable uploads are per app); one
	// connected before App IDs were required has none.
	if _, err := h.pool.Exec(h.ctx(), "UPDATE whatsapp_accounts SET app_id = NULL"); err != nil {
		t.Fatal(err)
	}
	h.expect(h.rawUpload("POST", path, owner, "image/png", png), 422, "VALIDATION_ERROR")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/channels/accounts/" + acc, token: owner, body: map[string]any{"app_id": "1234"}}), 200, "")
	h.expect(h.rawUpload("POST", path, owner, "text/plain", []byte("x")), 422, "VALIDATION_ERROR")
	r := h.rawUpload("POST", path, owner, "image/png", png)
	h.expect(r, 201, "")
	if !strings.HasPrefix(r.data()["handle"].(string), "4::") || r.data()["size"] != float64(len(png)) {
		t.Fatalf("sample: %s", r.raw)
	}
}

func TestContactAttributeKeys(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	_, _ = h.pool.Exec(h.ctx(), `INSERT INTO contacts (phone_e164, name, source, attributes) VALUES
		('+905550000001', 'A', 'manual', '{"city":"Ankara","plan":"gold"}'), ('+905550000002', 'B', 'manual', '{"city":"İzmir"}')`)
	r := h.do(call{method: "GET", path: "/api/v1/contacts/attribute-keys", token: owner})
	h.expect(r, 200, "")
	if keys := r.list(); len(keys) != 2 || keys[0] != "city" || keys[1] != "plan" {
		t.Fatalf("keys: %s", r.raw)
	}
}
