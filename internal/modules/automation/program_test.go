package automation

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
)

func TestCompileFlattensBranchesWithForwardJumps(t *testing.T) {
	prog, err := Compile([]Step{
		{Type: "add_tag", Tag: "a"},
		{Type: "condition", If: &Condition{Field: "contact.var1", Operator: "equals", Value: "x"},
			Then: []Step{{Type: "reply", Text: "yes"}},
			Else: []Step{{Type: "reply", Text: "no"}, {Type: "add_tag", Tag: "b"}}},
		{Type: "condition", If: &Condition{Field: "message.body", Operator: "contains", Value: "hi"},
			Then: []Step{{Type: "mute", Seconds: 600}}},
		{Type: "delay", Seconds: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	ops := make([]string, len(prog))
	for i, in := range prog {
		ops[i] = in.Op
	}
	// 0 action, 1 branch→4, 2 then, 3 jump→6, 4 else, 5 else, 6 branch→8, 7 then, 8 delay
	if strings.Join(ops, ",") != "action,branch,action,jump,action,action,branch,action,action" {
		t.Fatalf("ops: %v", ops)
	}
	if prog[1].Target != 4 || prog[3].Target != 6 || prog[6].Target != 8 {
		t.Fatalf("targets: %+v", prog)
	}
	for i, in := range prog {
		if (in.Op == "branch" || in.Op == "jump") && in.Target <= i {
			t.Fatalf("instruction %d jumps backwards", i)
		}
	}
	if prog[2].Step.Then != nil || prog[2].Step.If != nil {
		t.Fatal("actions must not carry nested steps")
	}
}

func TestCompileRejects(t *testing.T) {
	deep := []Step{{Type: "reply", Text: "x"}}
	for range MaxDepth + 1 {
		deep = []Step{{Type: "condition", If: &Condition{Field: "message.body", Operator: "is_set"}, Then: deep}}
	}
	many := make([]Step, MaxSteps+1)
	for i := range many {
		many[i] = Step{Type: "add_tag", Tag: "t"}
	}
	gid := uuid.New()
	cases := map[string][]Step{
		"empty":            nil,
		"unknown type":     {{Type: "teleport"}},
		"empty reply":      {{Type: "reply", Text: " "}},
		"no template":      {{Type: "send_template"}},
		"bad field":        {{Type: "set_field", Field: "password_hash"}},
		"http not https":   {{Type: "http_request", URL: "http://example.com"}},
		"http host header": {{Type: "http_request", URL: "https://example.com", Headers: map[string]string{"Host": "evil"}}},
		"delay zero":       {{Type: "delay"}},
		"delay too long":   {{Type: "delay", Seconds: MaxDelaySecs + 1}},
		"short mute":       {{Type: "mute", Seconds: 5}},
		"cond no field":    {{Type: "condition", If: &Condition{Field: "contact.password", Operator: "equals", Value: "x"}, Then: []Step{{Type: "mute", Seconds: 60}}}},
		"cond no value":    {{Type: "condition", If: &Condition{Field: "message.body", Operator: "contains"}, Then: []Step{{Type: "mute", Seconds: 60}}}},
		"cond empty":       {{Type: "condition", If: &Condition{Field: "message.body", Operator: "is_set"}}},
		"email incomplete": {{Type: "send_email", To: "a@b.co"}},
		"too deep":         deep,
		"too many":         many,
	}
	for name, steps := range cases {
		if _, err := Compile(steps); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := Compile([]Step{{Type: "add_to_group", GroupID: &gid}, {Type: "assign"}, {Type: "close_conversation"}}); err != nil {
		t.Fatalf("valid steps rejected: %v", err)
	}
}

func TestMatchTrigger(t *testing.T) {
	pn, other := uuid.New(), uuid.New()
	msg := func(body string) trigger.Event {
		return trigger.Event{Type: trigger.MessageReceived, Body: body, PhoneNumberID: &pn}
	}
	cases := []struct {
		t    Trigger
		e    trigger.Event
		want bool
	}{
		{Trigger{}, msg("anything"), true},
		{Trigger{Match: "contains", Keywords: []string{"Fiyat"}}, msg("fiyat ne kadar?"), true},
		{Trigger{Match: "contains", Keywords: []string{"fiyat"}}, msg("merhaba"), false},
		{Trigger{Match: "exact", Keywords: []string{"stop"}}, msg("  STOP "), true},
		{Trigger{Match: "exact", Keywords: []string{"stop"}}, msg("stop please"), false},
		{Trigger{Match: "starts_with", Keywords: []string{"#order"}}, msg("#order 123"), true},
		{Trigger{PhoneNumberID: &other}, msg("hi"), false},
		{Trigger{Tag: "VIP"}, trigger.Event{Type: trigger.TagAdded, Tag: "vip"}, true},
		{Trigger{Tag: "vip"}, trigger.Event{Type: trigger.TagAdded, Tag: "gold"}, false},
		{Trigger{Status: "failed"}, trigger.Event{Type: trigger.MessageStatus, Status: "failed"}, true},
		{Trigger{Status: "failed", CampaignID: &other}, trigger.Event{Type: trigger.MessageStatus, Status: "failed", CampaignID: &pn}, false},
		{Trigger{}, trigger.Event{Type: trigger.ConversationClosed}, true},
	}
	for i, c := range cases {
		if got := MatchTrigger(c.t, c.e); got != c.want {
			t.Errorf("case %d: got %v", i, got)
		}
	}
	for _, bad := range []struct {
		typ string
		t   Trigger
	}{
		{trigger.MessageReceived, Trigger{Match: "regex"}},
		{trigger.MessageReceived, Trigger{Match: "contains"}},
		{trigger.TagAdded, Trigger{}},
		{trigger.MessageStatus, Trigger{Status: "sent"}},
		{"teleport", Trigger{}},
		{trigger.NewContact, Trigger{CooldownSeconds: -1}},
	} {
		if ValidateTrigger(bad.typ, bad.t) == nil {
			t.Errorf("trigger %s %+v accepted", bad.typ, bad.t)
		}
	}
}
