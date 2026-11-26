//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
)

// aiScript plays the model: it records every request and answers with
// next (reply turns) or summary (handoff summaries).
type aiScript struct {
	mu       sync.Mutex
	requests []ai.Request
	next     func(req ai.Request) (ai.Response, error)
	summary  func(req ai.Request) (ai.Response, error)
}

func (s *aiScript) run(_ context.Context, req ai.Request) (ai.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	next, summary := s.next, s.summary
	s.mu.Unlock()
	if req.Output != nil && req.Output.Name == "messora_handoff_summary" {
		if summary == nil {
			return structured(map[string]any{"summary": "AI özeti: müşteri yardım istiyor.", "detected_need": "Kargo bilgisi",
				"suggested_next_action": "Kargo durumunu kontrol edin."}), nil
		}
		return summary(req)
	}
	return next(req)
}

// turns are the reply requests (not summaries) the model received.
func (s *aiScript) turns() []ai.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ai.Request
	for _, r := range s.requests {
		if r.Output != nil && r.Output.Name == ai.ReplySchemaName {
			out = append(out, r)
		}
	}
	return out
}

func (s *aiScript) answer(f func(req ai.Request) (ai.Response, error)) {
	s.mu.Lock()
	s.next = f
	s.mu.Unlock()
}

func structured(v any) ai.Response {
	b, _ := json.Marshal(v)
	return ai.Response{Text: string(b), Structured: b, Usage: ai.Usage{InputTokens: 100, OutputTokens: 20}, StopReason: ai.StopEnd}
}

// aiReply is a structured reply of the model.
func aiReply(text string, confidence float64, intent string, extra ...map[string]any) func(ai.Request) (ai.Response, error) {
	return func(ai.Request) (ai.Response, error) {
		v := map[string]any{"reply": text, "confidence": confidence, "intent": intent, "escalate": false, "escalate_reason": "",
			"extracted_fields": map[string]any{}, "actions": []any{}}
		for _, e := range extra {
			for k, x := range e {
				v[k] = x
			}
		}
		return structured(v), nil
	}
}

type aiEnv struct {
	owner, phoneID, profileID, providerID string
	script                                *aiScript
}

// setupAI connects a number, the fake AI provider (switched on) and an AI
// profile on the number; AI turns run without the debounce delay.
func (h *harness) setupAI(profile map[string]any) aiEnv {
	h.t.Helper()
	owner, phoneID := h.connect()
	h.useAIEmulator()
	h.svc.inbox.SetDebounce(0)
	fake := h.aiProvider(owner, "fake", "Fake", aiKey)
	h.enableAI(owner, fake["id"].(string), "fake-small")
	body := map[string]any{"name": "Destek", "phone_number_id": phoneID, "auto_reply": true, "status": "active",
		"system_prompt": "Sen Acme'nin destek asistanısın.", "disclosure_mode": "always_first_message",
		"handoff": map[string]any{"on_request": true, "on_low_confidence": 0.5, "on_no_answer": true,
			"on_critical_intents": []string{"complaint"}, "on_unauthorized_action": true}}
	for k, v := range profile {
		body[k] = v
	}
	r := h.do(call{method: "POST", path: "/api/v1/ai/profiles", token: owner, body: body})
	h.expect(r, 201, "")
	s := &aiScript{}
	s.answer(aiReply("Kargonuz yarın yola çıkıyor.", 0.9, "order_status"))
	h.svc.ai.SetFakeScript(s.run)
	return aiEnv{owner: owner, phoneID: phoneID, profileID: r.data()["id"].(string), providerID: fake["id"].(string), script: s}
}

// say delivers a customer message and runs every resulting task.
func (h *harness) say(wamid, body string) {
	h.t.Helper()
	h.inbound(testCustomer, wamid, body, time.Now())
	h.drain()
}

func (h *harness) conversation(token, id string) map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id, token: token})
	h.expect(r, 200, "")
	return r.data()
}

func (h *harness) activityKinds(token, id string) []string {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/activity?limit=200", token: token})
	h.expect(r, 200, "")
	var out []string
	for _, a := range r.list() {
		out = append(out, a.(map[string]any)["kind"].(string))
	}
	return out
}

// hasDisclosure reports whether s carries the AI disclosure (in the
// workspace language).
func hasDisclosure(s string) bool {
	return strings.Contains(s, "Yapay zekâ asistanı yanıtlıyor.") || strings.Contains(s, "An AI assistant is answering.")
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (h *harness) aiMessages() int {
	return h.count("SELECT count(*) FROM messages WHERE source = 'ai' AND sent_by = 'ai'")
}

// metaTexts are the texts of the messages that reached Meta.
func (h *harness) metaTexts() []string {
	var out []string
	for _, s := range h.meta.Sent() {
		out = append(out, string(s.Raw))
	}
	return out
}

func (h *harness) handoffs(token, id string) []map[string]any {
	h.t.Helper()
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/handoffs", token: token})
	h.expect(r, 200, "")
	var out []map[string]any
	for _, x := range r.list() {
		out = append(out, x.(map[string]any))
	}
	return out
}

func (h *harness) eventsOfType(channel, typ string) []rtEvent {
	var out []rtEvent
	for _, e := range h.rtEvents() {
		if e.channel == channel && e.body["type"] == typ {
			out = append(out, e)
		}
	}
	return out
}

func TestAIHandoffBothDirections(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	owner := env.owner

	// The first customer message: the profile auto replies, so the AI owns
	// the conversation and answers, with the disclosure in front.
	h.say("wamid.h1", "Siparişim nerede?")
	id := h.conversationOf(testCustomer)
	c := h.conversation(owner, id)
	if c["owner_type"] != "ai" || c["ai_mode"] != "auto_reply" || c["ai_profile_id"] != env.profileID || c["ai_status"] != "idle" {
		t.Fatalf("conversation: %v", c)
	}
	sent := h.metaTexts()
	if len(sent) != 1 || !hasDisclosure(sent[0]) || !strings.Contains(sent[0], "Kargonuz yarın yola çıkıyor.") {
		t.Fatalf("meta: %v", sent)
	}
	if h.aiMessages() != 1 || h.count("SELECT count(*) FROM messages WHERE source = 'ai' AND ai_usage_id IS NOT NULL") != 1 {
		t.Fatal("AI message not marked")
	}
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/messages", token: owner})
	h.expect(r, 200, "")
	if m := r.list()[0].(map[string]any); m["source"] != "ai" || m["sent_by"] != "ai" || m["ai_usage_id"] == nil {
		t.Fatalf("message: %v", m)
	}
	// The second turn has no disclosure again.
	h.say("wamid.h2", "Teşekkürler, peki iade?")
	if sent = h.metaTexts(); len(sent) != 2 || hasDisclosure(sent[1]) {
		t.Fatalf("second reply: %v", sent)
	}
	kinds := h.activityKinds(owner, id)
	if !has(kinds, "handed_to_ai") || !has(kinds, "ai_reply_sent") {
		t.Fatalf("activity %v", kinds)
	}
	ch := realtime.ConversationChannel(uuid.MustParse(id))
	if len(h.eventsOfType(ch, realtime.TypeConversationHandoff)) == 0 || len(h.eventsOfType(ch, realtime.TypeAIStatus)) < 2 ||
		len(h.eventsOfType(ch, realtime.TypeConversationActivity)) < 3 {
		t.Fatalf("realtime events missing")
	}

	// AI → human (the pool): waiting for a person, everybody is told.
	r = h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/handoff", token: owner,
		body: map[string]any{"to": "human", "assignee_id": nil, "note": "Fatura itirazı"}})
	h.expect(r, 200, "")
	if d := r.data(); d["owner_type"] != "human" || d["ai_mode"] != "off" || d["pending_handoff_id"] == nil || d["assignee"] != nil {
		t.Fatalf("handed to human: %v", d)
	}
	hs := h.handoffs(owner, id)
	if hs[0]["from_type"] != "ai" || hs[0]["to_type"] != "pool" || hs[0]["reason"] != "manual" || hs[0]["reason_detail"] != "Fatura itirazı" || hs[0]["accepted_at"] != nil {
		t.Fatalf("handoff: %v", hs[0])
	}
	if len(h.conversations(owner, "?ai=waiting_human")) != 1 || len(h.conversations(owner, "?ai=ai")) != 0 {
		t.Fatal("list filters")
	}
	// A customer message now gets no AI answer.
	h.say("wamid.h3", "Hâlâ bekliyorum")
	if len(h.metaTexts()) != 2 {
		t.Fatal("the AI answered a conversation a person owns")
	}

	// A person takes it over.
	r = h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/takeover", token: owner})
	h.expect(r, 200, "")
	if d := r.data(); d["owner_type"] != "human" || d["pending_handoff_id"] != nil || d["assignee"] == nil {
		t.Fatalf("taken over: %v", d)
	}
	if hs = h.handoffs(owner, id); hs[0]["accepted_at"] == nil || hs[0]["accepted_by"] == nil {
		t.Fatalf("accepted: %v", hs[0])
	}

	// Human → AI: the unanswered customer message is answered at once.
	r = h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/ai-mode", token: owner, body: map[string]any{"mode": "auto_reply"}})
	h.expect(r, 200, "")
	if r.data()["owner_type"] != "ai" {
		t.Fatalf("to ai: %v", r.data())
	}
	h.drain()
	if len(h.metaTexts()) != 3 {
		t.Fatalf("unanswered message: %v\n%v\n%v", h.metaTexts(), h.conversation(owner, id), h.activityKinds(owner, id))
	}

	// An agent's reply takes the conversation back.
	r = h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/messages", token: owner,
		body: map[string]any{"type": "text", "text": map[string]any{"body": "Ben ilgileniyorum."}}})
	h.expect(r, 202, "")
	if c = h.conversation(owner, id); c["owner_type"] != "human" || c["ai_mode"] != "off" {
		t.Fatalf("after reply: %v", c)
	}
	kinds = h.activityKinds(owner, id)
	for _, k := range []string{"handed_to_human", "handoff_accepted", "assignee_changed", "message_sent"} {
		if !has(kinds, k) {
			t.Errorf("activity misses %s: %v", k, kinds)
		}
	}
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action IN ('conversation.handoff', 'conversation.takeover', 'conversation.ai_mode')"); n < 3 {
		t.Fatalf("audit rows = %d", n)
	}
	// Notifications for the pool went to the personal channels.
	notes := 0
	for _, e := range h.rtEvents() {
		if e.body["type"] == realtime.TypeNotificationCreated && e.body["kind"] == "conversation.handed_over" && strings.HasPrefix(e.channel, "personal:#") {
			notes++
		}
	}
	if notes == 0 {
		t.Fatal("no handoff notice")
	}

	// Paging: the latest item last, the cursor reaches older ones.
	r = h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/activity?limit=2", token: owner})
	h.expect(r, 200, "")
	page := r.list()
	cursor, _ := r.body["meta"].(map[string]any)["next_cursor"].(string)
	if len(page) != 2 || cursor == "" || page[0].(map[string]any)["id"].(string) > page[1].(map[string]any)["id"].(string) {
		t.Fatalf("page: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/activity?limit=2&cursor=" + cursor, token: owner})
	h.expect(r, 200, "")
	if older := r.list(); len(older) != 2 || older[1].(map[string]any)["id"].(string) >= page[0].(map[string]any)["id"].(string) {
		t.Fatalf("older page: %s", r.raw)
	}
}

func TestAINoteForAIStaysInternal(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	h.say("wamid.n1", "Merhaba")
	id := h.conversationOf(testCustomer)

	const secret = "GIZLI-NOT-42 müşteri VIP, kargo ücretsiz"
	r := h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/notes", token: env.owner, body: map[string]any{"body": secret, "for_ai": true}})
	h.expect(r, 201, "")
	if r.data()["for_ai"] != true || r.data()["author"] == nil {
		t.Fatalf("note: %v", r.data())
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/notes", token: env.owner,
		body: map[string]any{"body": "PRIVATE-TEAM-ONLY"}}), 201, "")
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/notes", token: env.owner, body: map[string]any{"body": "  "}}), 422, "VALIDATION_ERROR")

	env.script.answer(func(req ai.Request) (ai.Response, error) {
		for _, m := range req.Messages {
			if m.Role == ai.RoleOperatorNote && strings.Contains(m.Text, "GIZLI-NOT-42") {
				return aiReply("Kargo sizin için ücretsiz.", 0.9, "shipping")(req)
			}
		}
		return aiReply("Kargo 50 TL.", 0.9, "shipping")(req)
	})
	h.say("wamid.n2", "Kargo ücreti ne kadar?")
	turns := env.script.turns()
	last := turns[len(turns)-1]
	for _, m := range last.Messages {
		if m.Role != ai.RoleOperatorNote && strings.Contains(m.Text, "GIZLI-NOT-42") {
			t.Fatalf("note given as %s content", m.Role)
		}
		if strings.Contains(m.Text, "PRIVATE-TEAM-ONLY") {
			t.Fatal("a note not for the AI reached the AI")
		}
	}
	if strings.Contains(last.System, "GIZLI-NOT-42") {
		t.Fatal("note in the system prompt")
	}
	sent := h.metaTexts()
	if !strings.Contains(sent[len(sent)-1], "Kargo sizin için ücretsiz.") {
		t.Fatalf("the note did not reach the AI: %v", sent)
	}
	for _, s := range sent {
		if strings.Contains(s, "GIZLI-NOT-42") || strings.Contains(s, "PRIVATE-TEAM-ONLY") {
			t.Fatalf("note text sent to Meta: %s", s)
		}
	}
	r = h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/notes", token: env.owner})
	h.expect(r, 200, "")
	if len(r.list()) != 2 || !has(h.activityKinds(env.owner, id), "note_added") {
		t.Fatalf("notes: %s", r.raw)
	}
}

func TestAILowConfidenceEscalatesWithSummary(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(map[string]any{"data_collection": map[string]any{"enabled": true, "may_create_contact": false, "may_update_fields": true,
		"fields": []any{map[string]any{"key": "email", "label": "E-posta", "required": false}}}})
	agent, agentID := h.invite(env.owner, "agent@example.com", "agent")
	env.script.answer(aiReply("Sanırım iade 30 gün.", 0.2, "refund", map[string]any{"extracted_fields": map[string]any{"email": "ayse@example.com"}}))
	h.say("wamid.l1", "İade süresi ne kadar?")
	id := h.conversationOf(testCustomer)

	if len(h.metaTexts()) != 0 {
		t.Fatal("an unsure reply reached the customer")
	}
	c := h.conversation(env.owner, id)
	if c["owner_type"] != "human" || c["pending_handoff_id"] == nil {
		t.Fatalf("not escalated: %v", c)
	}
	hs := h.handoffs(agent, id) // the agent sees the unassigned conversation
	x := hs[0]
	if x["reason"] != "low_confidence" || x["summary_source"] != "ai" || x["summary"] != "AI özeti: müşteri yardım istiyor." ||
		x["detected_need"] != "Kargo bilgisi" || x["suggested_next_action"] == "" ||
		x["collected_fields"].(map[string]any)["email"] != "ayse@example.com" || x["from_type"] != "ai" || x["to_type"] != "pool" {
		t.Fatalf("handoff: %v", x)
	}
	if h.count("SELECT count(*) FROM ai_usage WHERE purpose = 'summarize' AND conversation_id = $1", id) != 1 {
		t.Fatal("summary request not recorded")
	}
	notified := false
	for _, e := range h.rtEvents() {
		if e.channel == realtime.PersonalChannel(uuid.MustParse(agentID)) && e.body["kind"] == "conversation.escalated" && e.body["conversation_id"] == id {
			notified = true
		}
	}
	if !notified || !has(h.activityKinds(env.owner, id), "escalation") {
		t.Fatal("agent not notified / no escalation item")
	}
	if n := h.count("SELECT count(*) FROM audit_logs WHERE action = 'conversation.escalate'"); n != 1 {
		t.Fatalf("audit = %d", n)
	}
	// The agent takes it over by answering.
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/messages", token: agent,
		body: map[string]any{"type": "text", "text": map[string]any{"body": "İade süremiz 14 gün."}}}), 202, "")
	if c = h.conversation(agent, id); c["pending_handoff_id"] != nil || c["assignee"].(map[string]any)["id"] != agentID {
		t.Fatalf("after the agent's reply: %v", c)
	}
}

func TestAICustomerAsksForHuman(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	env.script.summary = func(ai.Request) (ai.Response, error) {
		return ai.Response{}, &ai.Error{Kind: ai.KindUnavailable, Provider: ai.KindFake, Message: "down"}
	}
	// The model is confident, but the customer asked for a person.
	h.say("wamid.c1", "Gerçek biriyle görüşmek istiyorum, TEMSİLCİ lütfen")
	id := h.conversationOf(testCustomer)
	if len(h.metaTexts()) != 0 {
		t.Fatal("the AI answered a request for a person")
	}
	x := h.handoffs(env.owner, id)[0]
	if x["reason"] != "customer_request" || x["summary_source"] != "fallback" || !strings.Contains(x["summary"].(string), "TEMSİLCİ") {
		t.Fatalf("handoff: %v", x)
	}
}

func TestAIOutsideTheWindowEscalates(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	h.inbound(testCustomer, "wamid.w1", "Merhaba", time.Now().Add(-25*time.Hour))
	h.drain()
	id := h.conversationOf(testCustomer)
	if len(h.metaTexts()) != 0 || len(env.script.turns()) != 0 {
		t.Fatal("the AI tried to answer outside the window")
	}
	if x := h.handoffs(env.owner, id)[0]; x["reason"] != "window_closed" || x["summary_source"] != "fallback" {
		t.Fatalf("handoff: %v", x)
	}
}

func TestAIDisabledCompanyWideHandsOverNotSilently(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	h.say("wamid.d1", "Merhaba")
	id := h.conversationOf(testCustomer)
	r := h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: env.owner, body: map[string]any{
		"enabled": false, "default_provider_id": env.providerID, "default_model": "fake-small",
		"disclosure":          map[string]any{"tr": "Yapay zekâ asistanı yanıtlıyor.", "en": "An AI assistant is answering."},
		"data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
	h.say("wamid.d2", "Orada mısınız?")
	c := h.conversation(env.owner, id)
	if c["owner_type"] != "human" || c["pending_handoff_id"] == nil || c["ai_status"] != "error" || c["ai_status_detail"] != "disabled" {
		t.Fatalf("conversation: %v", c)
	}
	if x := h.handoffs(env.owner, id)[0]; x["reason"] != "ai_error" || x["reason_detail"] != "disabled" {
		t.Fatalf("handoff: %v", x)
	}
	// Giving it back to the AI is refused while AI is off.
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/ai-mode", token: env.owner, body: map[string]any{"mode": "auto_reply"}}),
		409, "AI_UNAVAILABLE")
}

func TestAIProviderErrorHandsOver(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	env.script.answer(func(ai.Request) (ai.Response, error) {
		return ai.Response{}, &ai.Error{Kind: ai.KindQuotaExceeded, Provider: ai.KindFake, Message: "no credit"}
	})
	h.say("wamid.e1", "Merhaba")
	id := h.conversationOf(testCustomer)
	if x := h.handoffs(env.owner, id)[0]; x["reason"] != "ai_error" || x["reason_detail"] != "quota_exceeded" {
		t.Fatalf("handoff: %v", x)
	}
}

func TestAISuggestModeDrafts(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(map[string]any{"auto_reply": false})
	h.say("wamid.s1", "Fiyat alabilir miyim?")
	id := h.conversationOf(testCustomer)
	c := h.conversation(env.owner, id)
	if c["owner_type"] != "none" || c["ai_mode"] != "suggest" || len(h.metaTexts()) != 0 {
		t.Fatalf("suggest mode: %v", c)
	}
	drafts := func() []any {
		r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/ai-drafts?pending=true", token: env.owner})
		h.expect(r, 200, "")
		return r.list()
	}
	d := drafts()
	if len(d) != 1 || d[0].(map[string]any)["kind"] != "suggestion" || d[0].(map[string]any)["body"] != "Kargonuz yarın yola çıkıyor." {
		t.Fatalf("drafts: %v", d)
	}
	draft := d[0].(map[string]any)["id"].(string)
	r := h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/ai-drafts/" + draft + "/accept", token: env.owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "accepted" || r.data()["sent_message_id"] == nil {
		t.Fatalf("accept: %v", r.data())
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/ai-drafts/" + draft + "/accept", token: env.owner}), 409, "DRAFT_DECIDED")
	h.drain()
	if sent := h.metaTexts(); len(sent) != 1 || !strings.Contains(sent[0], "Kargonuz yarın yola çıkıyor.") || hasDisclosure(sent[0]) {
		t.Fatalf("meta: %v", sent)
	}
	if h.count("SELECT count(*) FROM messages WHERE source = 'panel' AND sent_by = 'ai'") != 1 {
		t.Fatal("accepted suggestion not marked as AI-written")
	}

	// Edited, then rejected.
	h.say("wamid.s2", "Peki indirim?")
	draft = drafts()[0].(map[string]any)["id"].(string)
	r = h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/ai-drafts/" + draft + "/accept", token: env.owner,
		body: map[string]any{"body": "Bu hafta %10 indirim var."}})
	h.expect(r, 200, "")
	if r.data()["status"] != "edited" {
		t.Fatalf("edit: %v", r.data())
	}
	h.say("wamid.s3", "Teşekkürler")
	draft = drafts()[0].(map[string]any)["id"].(string)
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/ai-drafts/" + draft + "/reject", token: env.owner}), 200, "")
	if len(drafts()) != 0 || h.count("SELECT count(*) FROM messages WHERE sent_by = 'user' AND body = 'Bu hafta %10 indirim var.'") != 1 {
		t.Fatal("edit / reject")
	}
	if n := h.count("SELECT count(*) FROM ai_usage WHERE purpose = 'suggest'"); n != 3 {
		t.Fatalf("suggest requests = %d", n)
	}
}

func TestAIAwaitingApproval(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(map[string]any{"auto_reply": false})
	h.inbound(testCustomer, "wamid.a0", "Merhaba", time.Now())
	id := h.conversationOf(testCustomer)
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/ai-mode", token: env.owner, body: map[string]any{"mode": "awaiting_approval"}}), 200, "")
	h.drain()
	if len(h.metaTexts()) != 0 {
		t.Fatal("a reply waiting for approval was sent")
	}
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/ai-drafts?pending=true", token: env.owner})
	h.expect(r, 200, "")
	var approval map[string]any
	for _, d := range r.list() {
		if d.(map[string]any)["kind"] == "approval" {
			approval = d.(map[string]any)
		}
	}
	if approval == nil || !hasDisclosure(approval["body"].(string)) {
		t.Fatalf("drafts: %s", r.raw)
	}
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/ai-drafts/" + approval["id"].(string) + "/accept", token: env.owner}), 200, "")
	h.drain()
	if len(h.metaTexts()) != 1 || h.aiMessages() != 1 {
		t.Fatalf("approved reply: %v", h.metaTexts())
	}
	if c := h.conversation(env.owner, id); c["owner_type"] != "ai" || c["ai_mode"] != "awaiting_approval" {
		t.Fatalf("approval keeps the AI owner: %v", c)
	}
}

func TestAIDebounceAnswersABurstOnce(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	h.svc.inbox.SetDebounce(3 * time.Second)
	now := time.Now()
	for i, s := range []string{"Merhaba", "siparişim", "nerede?"} {
		h.inbound(testCustomer, fmt.Sprintf("wamid.b%d", i), s, now.Add(time.Duration(i)*time.Second))
	}
	if n := h.count("SELECT count(*) FROM outbox_events WHERE topic = 'task:ai:reply' AND available_at > now()"); n != 3 {
		t.Fatalf("delayed turns = %d", n)
	}
	h.drain() // nothing is due yet
	if len(env.script.turns()) != 0 {
		t.Fatal("answered before the debounce delay")
	}
	if _, err := h.pool.Exec(context.Background(), "UPDATE outbox_events SET available_at = now() WHERE topic = 'task:ai:reply'"); err != nil {
		t.Fatal(err)
	}
	h.drain()
	turns := env.script.turns()
	if len(turns) != 1 || len(h.metaTexts()) != 1 || h.aiMessages() != 1 {
		t.Fatalf("turns %d, sent %d", len(turns), len(h.metaTexts()))
	}
	var users []string
	for _, m := range turns[0].Messages {
		if m.Role == ai.RoleUser {
			users = append(users, m.Text)
		}
	}
	if strings.Join(users, "|") != "Merhaba|siparişim|nerede?" {
		t.Fatalf("context: %v", users)
	}
	// A redelivered task does nothing.
	id := h.conversationOf(testCustomer)
	var last uuid.UUID
	if err := h.pool.QueryRow(context.Background(), "SELECT ai_handled_message_id FROM conversations WHERE id = $1", id).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.inbox.HandleAIReply(context.Background(), uuid.MustParse(id), last); err != nil || len(h.metaTexts()) != 1 {
		t.Fatalf("redelivery: %v", err)
	}
}

func TestAIRateGuardHandsOver(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	r := h.do(call{method: "PUT", path: "/api/v1/ai/settings", token: env.owner, body: map[string]any{
		"enabled": true, "default_provider_id": env.providerID, "default_model": "fake-small", "max_replies_per_hour": 1,
		"disclosure":          map[string]any{"tr": "Yapay zekâ asistanı yanıtlıyor.", "en": "An AI assistant is answering."},
		"data_sharing_notice": map[string]any{"tr": "", "en": ""}}})
	h.expect(r, 200, "")
	if r.data()["max_replies_per_hour"] != float64(1) {
		t.Fatalf("settings: %s", r.raw)
	}
	h.say("wamid.r1", "Merhaba")
	h.say("wamid.r2", "Bir sorum daha var")
	id := h.conversationOf(testCustomer)
	if x := h.handoffs(env.owner, id)[0]; x["reason"] != "ai_error" || x["reason_detail"] != "rate_limited" || len(h.metaTexts()) != 1 {
		t.Fatalf("handoff: %v", x)
	}
}

func TestAutomationRunsAreAnnouncedInTheConversation(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	tagger := h.automation(env.owner, map[string]any{"name": "Etiketle", "active": true, "trigger_type": "message_received",
		"steps": []any{map[string]any{"type": "add_tag", "tag": "yazdı"}}})
	manual := h.automation(env.owner, map[string]any{"name": "Sipariş durumu", "active": true, "trigger_type": "tag_added",
		"trigger": map[string]any{"tag": "never-used"}, "steps": []any{map[string]any{"type": "add_tag", "tag": "siparis"}}})
	// The profile may start the order automation; the model asks for it.
	pr := h.do(call{method: "GET", path: "/api/v1/ai/profiles/" + env.profileID, token: env.owner}).data()
	pr["allowed_actions"] = []string{"automation:" + manual}
	for _, k := range []string{"id", "phone_display", "warnings", "created_at", "updated_at"} {
		delete(pr, k)
	}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/ai/profiles/" + env.profileID, token: env.owner, body: pr}), 200, "")
	env.script.answer(aiReply("Siparişinizi kontrol ediyorum.", 0.9, "order_status", map[string]any{"actions": []any{"automation:" + manual}}))

	h.say("wamid.au1", "Siparişim nerede?")
	id := h.conversationOf(testCustomer)
	r := h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/automation-runs", token: env.owner})
	h.expect(r, 200, "")
	byName := map[string]map[string]any{}
	for _, x := range r.list() {
		byName[x.(map[string]any)["automation_name"].(string)] = x.(map[string]any)
	}
	if run := byName["Etiketle"]; run == nil || run["status"] != "completed" || run["started_by"] != "system" || run["steps_done"] != float64(1) ||
		run["last_action"] != "add_tag" || run["finished_at"] == nil {
		t.Fatalf("trigger run: %s", r.raw)
	}
	if run := byName["Sipariş durumu"]; run == nil || run["status"] != "completed" || run["started_by"] != "ai" {
		t.Fatalf("ai run: %s", r.raw)
	}
	ch := realtime.ConversationChannel(uuid.MustParse(id))
	statuses := map[string]bool{}
	for _, e := range h.eventsOfType(ch, realtime.TypeAutomationRunUpdated) {
		statuses[e.body["status"].(string)] = true
	}
	if !statuses["queued"] || !statuses["completed"] {
		t.Fatalf("run events: %v", statuses)
	}
	kinds := h.activityKinds(env.owner, id)
	for _, k := range []string{"automation_started", "automation_completed", "ai_action"} {
		if !has(kinds, k) {
			t.Errorf("activity misses %s: %v", k, kinds)
		}
	}
	_ = tagger
}

func TestContactAIDisabledAndVisibility(t *testing.T) {
	h := newHarness(t)
	env := h.setupAI(nil)
	agent, _ := h.invite(env.owner, "agent@example.com", "agent")
	_, otherID := h.invite(env.owner, "other@example.com", "agent")
	h.say("wamid.v1", "Merhaba")
	id := h.conversationOf(testCustomer)
	contact := h.contactID("+" + testCustomer)

	// An agent may not give a conversation to someone else.
	h.expect(h.do(call{method: "POST", path: "/api/v1/conversations/" + id + "/handoff", token: agent,
		body: map[string]any{"to": "human", "assignee_id": otherID}}), 403, "FORBIDDEN")
	// Assigned to someone else, it is invisible to the agent.
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/assignment", token: env.owner, body: map[string]any{"assignee_id": otherID}}), 200, "")
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/ai-mode", token: agent, body: map[string]any{"mode": "off"}}), 404, "NOT_FOUND")
	h.expect(h.do(call{method: "GET", path: "/api/v1/conversations/" + id + "/activity", token: agent}), 404, "NOT_FOUND")

	r := h.do(call{method: "PUT", path: "/api/v1/contacts/" + contact + "/ai", token: env.owner, body: map[string]any{"disabled": true}})
	h.expect(r, 200, "")
	if r.data()["ai_disabled"] != true {
		t.Fatalf("contact: %s", r.raw)
	}
	c := h.conversation(env.owner, id)
	if c["owner_type"] != "human" || c["contact"].(map[string]any)["ai_disabled"] != true {
		t.Fatalf("conversation: %v", c)
	}
	before := len(env.script.turns())
	h.say("wamid.v2", "Yine ben")
	if len(env.script.turns()) != before {
		t.Fatal("the AI worked for a contact it is disabled for")
	}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/conversations/" + id + "/ai-mode", token: env.owner, body: map[string]any{"mode": "suggest"}}),
		409, "AI_UNAVAILABLE")
	if !has(h.activityKinds(env.owner, id), "contact_ai_disabled") || h.count("SELECT count(*) FROM audit_logs WHERE action = 'contact.ai'") != 1 {
		t.Fatal("contact AI change not recorded")
	}
}
