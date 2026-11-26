package conversations

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
)

func TestWantsHumanTurkishAndEnglish(t *testing.T) {
	yes := []string{
		"Bir temsilciyle görüşmek istiyorum",
		"TEMSİLCİ",
		"temsilciye bağlar mısınız?",
		"Müşteri temsilcinizle konuşabilir miyim",
		"insanla konuşmak istiyorum",
		"İNSAN",
		"gerçek biri var mı",
		"GERÇEK BİRİYLE görüşmek istiyorum",
		"gercek bir kisi lutfen",
		"canlı destek",
		"yetkiliyle görüşeyim",
		"I want to talk to a human",
		"AGENT please",
		"can I speak to a real person?",
		"Live agent!!",
	}
	for _, s := range yes {
		if !WantsHuman(s) {
			t.Errorf("%q: not recognised", s)
		}
	}
	no := []string{
		"", "Merhaba", "Siparişim nerede?", "insanlık hali", "humanitarian aid", "temsil etmek", "Gerçekten mi?",
		"agenda for tomorrow", strings.Repeat("temsilci ", 100),
	}
	for _, s := range no {
		if WantsHuman(s) {
			t.Errorf("%q: false positive", s)
		}
	}
}

func f64(v float64) *float64 { return &v }

func TestEscalationDecisionTable(t *testing.T) {
	prof := ai.Profile{AllowedActions: []string{"automation:" + uuid.NewString(), "add_tag"},
		Handoff: ai.Handoff{OnRequest: true, OnLowConfidence: f64(0.6), OnCriticalIntents: []string{"complaint", "legal"}, OnUnauthorizedAction: true}}
	allowed := prof.AllowedActions[0]
	ok := ai.Reply{Reply: "Kargonuz yarın yola çıkıyor.", Confidence: 0.9, Intent: "order_status"}
	with := func(f func(*ai.Reply)) ai.Reply { r := ok; f(&r); return r }

	cases := []struct {
		name     string
		reply    ai.Reply
		profile  ai.Profile
		customer string
		reason   Reason // "" = no escalation
	}{
		{"confident answer", ok, prof, "Siparişim nerede?", ""},
		{"customer keyword wins over a confident answer", ok, prof, "temsilciye bağlayın", ReasonCustomerRequest},
		{"model intent human_request", with(func(r *ai.Reply) { r.Intent = "human_request" }), prof, "biriyle konuşabilir miyim", ReasonCustomerRequest},
		{"critical intent", with(func(r *ai.Reply) { r.Intent = "Complaint" }), prof, "berbat hizmet", ReasonCriticalIntent},
		{"model escalates", with(func(r *ai.Reply) { r.Escalate, r.EscalateReason = true, "bilmiyorum" }), prof, "iade?", ReasonNoAnswer},
		{"unauthorized action", with(func(r *ai.Reply) { r.Actions = []string{"create_contact"} }), prof, "kaydet", ReasonUnauthorizedAction},
		{"unauthorized action without that rule", with(func(r *ai.Reply) { r.Actions = []string{"create_contact"} }),
			ai.Profile{Handoff: ai.Handoff{OnRequest: true}}, "kaydet", ""},
		{"low confidence", with(func(r *ai.Reply) { r.Confidence = 0.4 }), prof, "fiyat?", ReasonLowConfidence},
		{"no threshold", with(func(r *ai.Reply) { r.Confidence = 0.1 }), ai.Profile{}, "fiyat?", ""},
		{"empty reply", with(func(r *ai.Reply) { r.Reply = "  " }), prof, "fiyat?", ReasonNoAnswer},
		{"request beats critical", with(func(r *ai.Reply) { r.Intent = "complaint" }), prof, "insanla konuşmak istiyorum", ReasonCustomerRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := Decide(c.reply, c.profile, c.customer)
			if d.Escalate != (c.reason != "") || d.Reason != c.reason {
				t.Fatalf("decision %+v, want reason %q", d, c.reason)
			}
			if d.Escalate && len(d.Actions) > 0 {
				t.Fatal("actions must not run when handing over")
			}
		})
	}

	d := Decide(with(func(r *ai.Reply) { r.Actions = []string{allowed, "create_contact", allowed, " "} }), ai.Profile{AllowedActions: prof.AllowedActions}, "ok")
	if d.Escalate || !slices.Equal(d.Actions, []string{allowed}) || !slices.Equal(d.Refused, []string{"create_contact"}) {
		t.Fatalf("actions split: %+v", d)
	}
}

func TestBuildTurnKeepsNotesOutOfCustomerContent(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 14, 0, 0, 0, time.UTC)
	secret := "INTERNAL-NOTE-7781: müşteri VIP, kargo ücretsiz"
	attrs, _ := json.Marshal(map[string]any{"plan": "gold"})
	email := "ayse@example.com"
	in := TurnInput{
		Profile:  ai.Profile{SystemPrompt: "Sen Acme asistanısın.", DisclosureMode: ai.DiscloseFirstMessage},
		Settings: ai.DefaultSettings(),
		Lang:     ai.LangTR,
		Contact:  db.Contact{Name: "Ayşe", PhoneE164: "+905551112233", Email: &email, Attributes: attrs},
		Messages: []db.Message{
			{Direction: "inbound", Type: "text", Body: "Merhaba", OccurredAt: t0},
			{Direction: "outbound", Type: "text", Body: "Merhaba, nasıl yardımcı olabilirim?", CreatedAt: t0.Add(time.Minute)},
			{Direction: "inbound", Type: "image", Body: "fatura", OccurredAt: t0.Add(3 * time.Minute)},
			{Direction: "inbound", Type: "reaction", Body: "👍", OccurredAt: t0.Add(4 * time.Minute)},
			{Direction: "inbound", Type: "text", Body: "Kargo ücreti ne kadar?", OccurredAt: t0.Add(5 * time.Minute)},
		},
		Notes: []db.ConversationNote{
			{Body: secret, ForAi: true, CreatedAt: t0.Add(2 * time.Minute)},
			{Body: "late note", ForAi: true, CreatedAt: t0.Add(10 * time.Minute)},
		},
	}
	req := BuildTurn(in)
	roles := make([]string, len(req.Messages))
	for i, m := range req.Messages {
		roles[i] = string(m.Role)
		if m.Role != ai.RoleOperatorNote && (strings.Contains(m.Text, "INTERNAL-NOTE") || strings.Contains(m.Text, "late note")) {
			t.Fatalf("note text in %s content: %q", m.Role, m.Text)
		}
	}
	want := []string{"user", "assistant", "operator_note", "user", "user", "operator_note"}
	if !slices.Equal(roles, want) {
		t.Fatalf("roles %v, want %v", roles, want)
	}
	if req.Messages[2].Text != secret || req.Messages[3].Text != "[image] fatura" {
		t.Fatalf("messages %+v", req.Messages)
	}
	if strings.Contains(req.System, "INTERNAL-NOTE") {
		t.Fatal("note text in the system prompt")
	}
	for _, s := range []string{"Sen Acme asistanısın.", "- name: Ayşe", "- email: ayse@example.com", "- attr:plan: gold", "never reveal or quote them"} {
		if !strings.Contains(req.System, s) {
			t.Errorf("system prompt misses %q", s)
		}
	}
	if req.Output == nil || req.Output.Name != ai.ReplySchemaName {
		t.Fatal("structured reply schema")
	}
}

func TestFallbackSummary(t *testing.T) {
	msgs := []db.Message{
		{Direction: "inbound", Type: "text", Body: "Siparişim gelmedi"},
		{Direction: "outbound", Type: "text", Body: "Bakıyorum"},
		{Direction: "inbound", Type: "text", Body: "Bir temsilci istiyorum"},
	}
	s := FallbackSummary(msgs, ReasonCustomerRequest, "order_status", ai.LangTR)
	if s.Source != "fallback" || !strings.Contains(s.Text, "Siparişim gelmedi") || !strings.Contains(s.Text, "Bir temsilci istiyorum") ||
		strings.Contains(s.Text, "Bakıyorum") || s.Need != "order status" || !strings.Contains(s.NextAction, "temsilci") {
		t.Fatalf("summary %+v", s)
	}
	if e := FallbackSummary(nil, ReasonAIError, "", ai.LangEN); e.Text != "No customer message." || e.NextAction == "" {
		t.Fatalf("empty summary %+v", e)
	}
}
