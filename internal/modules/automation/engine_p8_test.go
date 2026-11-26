package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
)

func TestWindowCandidate(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	at := func(ago time.Duration) *time.Time { v := now.Add(-ago); return &v }
	def := Trigger{} // 60 minutes, unanswered only
	cases := []struct {
		name      string
		tr        Trigger
		last      *time.Time
		direction string
		want      bool
	}{
		{"closes in 30 min, customer waiting", def, at(23*time.Hour + 30*time.Minute), "inbound", true},
		{"closes in exactly 60 min", def, at(23 * time.Hour), "inbound", true},
		{"closes in 61 min: too early", def, at(22*time.Hour + 59*time.Minute), "inbound", false},
		{"window already closed", def, at(24*time.Hour + time.Minute), "inbound", false},
		{"never wrote", def, nil, "inbound", false},
		{"agent replied last", def, at(23*time.Hour + 30*time.Minute), "outbound", false},
		{"agent replied last, answered included", Trigger{IncludeAnswered: true}, at(23*time.Hour + 30*time.Minute), "outbound", true},
		{"custom 180 minutes", Trigger{Minutes: 180}, at(21*time.Hour + 30*time.Minute), "inbound", true},
		{"custom 15 minutes, 30 left", Trigger{Minutes: 15}, at(23*time.Hour + 30*time.Minute), "inbound", false},
	}
	for _, c := range cases {
		if got := WindowCandidate(c.tr, c.last, c.direction, now); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestScannedTriggersFireOncePerOccasion(t *testing.T) {
	conv, msg, next := uuid.New(), uuid.New(), uuid.New()
	if first, again := WindowKey(conv, msg), WindowKey(conv, msg); first != again {
		t.Fatal("the same window must have the same key")
	}
	if WindowKey(conv, msg) == WindowKey(conv, next) {
		t.Fatal("a new customer message opens a new window with a new key")
	}
	if NoReplyKey("customer", conv, msg) == NoReplyKey("agent", conv, msg) {
		t.Fatal("no-reply keys differ by direction")
	}
}

func TestNoReplyRange(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	after, before := NoReplyRange(Trigger{Minutes: 120}, now)
	if !before.Equal(now.Add(-2*time.Hour)) || !after.Equal(before.Add(-24*time.Hour)) {
		t.Fatalf("range %s – %s", after, before)
	}
}

func TestRetryWaitDoublesUpToAnHour(t *testing.T) {
	p := RetryPolicy{Max: 5, Backoff: 30}
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}
	for i, w := range want {
		if got := retryWait(p, i); got != w {
			t.Fatalf("attempt %d: %s, want %s", i, got, w)
		}
	}
	if got := retryWait(RetryPolicy{Max: 10, Backoff: 1800}, 3); got != time.Hour {
		t.Fatalf("cap: %s", got)
	}
	if (&Step{Type: "add_tag", Retry: &RetryPolicy{Max: 2, Backoff: 1}}).retryPolicy() != nil {
		t.Fatal("database steps are never retried")
	}
	if (&Step{Type: "ai_detect_intent", Retry: &RetryPolicy{Max: 0}}).retryPolicy() != nil {
		t.Fatal("max 0 is no retry")
	}
	if (&Step{Type: "http_request", Retry: &RetryPolicy{Max: 2, Backoff: 5}}).retryPolicy() == nil {
		t.Fatal("http retry lost")
	}
}

func TestCompileNewSteps(t *testing.T) {
	g := uuid.New()
	ok := []Step{
		{Type: "ai_generate_message", Prompt: "Thank {{first_name}} for the order", Output: "send", Retry: &RetryPolicy{Max: 2, Backoff: 10}},
		{Type: "ai_analyze_message"},
		{Type: "ai_detect_intent", Intents: []string{"price", "human_request"}, Variable: "why"},
		{Type: "condition", If: &Condition{Field: "var.why", Operator: "equals", Value: "price"}, Then: []Step{{Type: "reply", Text: "{{var:ai_message}}"}}},
		{Type: "ai_extract_fields", Fields: []ExtractField{{Key: "email"}, {Key: "attr:city", Label: "Şehir"}}},
		{Type: "ai_classify_segment", Tags: []string{"gold", "silver"}},
		{Type: "ai_classify_segment", GroupIDs: []uuid.UUID{g}},
		{Type: "ai_auto_reply"},
		{Type: "ai_route_to_agent", Text: "Needs a person"},
		{Type: "ai_build_profile", Attribute: "profile"},
		{Type: "handoff_to_ai", Mode: "awaiting_approval"},
		{Type: "handoff_to_human"},
		{Type: "add_internal_note", Text: "Check this customer", ForAI: true},
		{Type: "condition", If: &Condition{Field: "event.reaction", Operator: "equals", Value: "negative"}, Then: []Step{{Type: "add_tag", Tag: "unhappy"}}},
	}
	AssignIDs(ok)
	prog, err := Compile(ok)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog) != 16 { // 12 actions, 2 branches and their then-steps
		t.Fatalf("program length %d", len(prog))
	}
	bad := map[string]Step{
		"prompt":        {Type: "ai_generate_message"},
		"output":        {Type: "ai_generate_message", Prompt: "x", Output: "print"},
		"intents empty": {Type: "ai_detect_intent"},
		"intent other":  {Type: "ai_detect_intent", Intents: []string{"other"}},
		"intent case":   {Type: "ai_detect_intent", Intents: []string{"Price"}},
		"fields":        {Type: "ai_extract_fields", Fields: []ExtractField{{Key: "phone"}}},
		"both choices":  {Type: "ai_classify_segment", Tags: []string{"a"}, GroupIDs: []uuid.UUID{g}},
		"no choices":    {Type: "ai_classify_segment"},
		"mode":          {Type: "handoff_to_ai", Mode: "off"},
		"note":          {Type: "add_internal_note"},
		"variable":      {Type: "ai_analyze_message", Variable: "9x"},
		"retry on tag":  {Type: "add_tag", Tag: "x", Retry: &RetryPolicy{Max: 1, Backoff: 1}},
		"retry max":     {Type: "http_request", URL: "https://x.example", Retry: &RetryPolicy{Max: 11, Backoff: 1}},
		"retry backoff": {Type: "send_email", To: "a@b.c", Subject: "s", Body: "b", Retry: &RetryPolicy{Max: 1}},
		"var condition": {Type: "condition", If: &Condition{Field: "var.Bad", Operator: "is_set"}, Then: []Step{{Type: "add_tag", Tag: "x"}}},
	}
	for name, s := range bad {
		if _, err := Compile([]Step{s}); err == nil {
			t.Errorf("%s: compiled", name)
		}
	}
}

func TestCompileDraftLeavesSetupStepsOpen(t *testing.T) {
	steps := []Step{{ID: "tpl", Type: "send_template", Template: &TemplateRef{Language: "tr"}}, {ID: "tag", Type: "add_tag", Tag: "x"}}
	if _, err := Compile(steps); err == nil {
		t.Fatal("a template without a name must not compile")
	}
	if _, err := CompileDraft(steps, map[string]bool{"tpl": true}); err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := CompileDraft([]Step{{ID: "tpl", Type: "teleport"}}, map[string]bool{"tpl": true}); err == nil {
		t.Fatal("an unknown step is never lenient")
	}
	items := []SetupItem{{StepID: "tpl", Field: "template", Label: "Choose"}, {StepID: "gone", Field: "template"}}
	if p := PendingSetup(steps, items); len(p) != 1 || p[0].StepID != "tpl" {
		t.Fatalf("pending %v", p)
	}
	steps[0].Template.Name = "follow_up"
	if p := PendingSetup(steps, items); len(p) != 0 {
		t.Fatalf("filled template still pending: %v", p)
	}
	a := []Step{{ID: "as", Type: "assign"}}
	if len(PendingSetup(a, []SetupItem{{StepID: "as", Field: "assignee_id"}})) != 1 {
		t.Fatal("an unassigned assign step is open")
	}
	if setupBlocks(true, PendingSetup(a, []SetupItem{{StepID: "as", Field: "assignee_id", Label: "Agent"}})) == nil {
		t.Fatal("enabling with open setup must fail")
	}
	if setupBlocks(false, []SetupItem{{StepID: "as"}}) != nil {
		t.Fatal("saving disabled with open setup is fine")
	}
}

func TestValidateNewTriggers(t *testing.T) {
	good := map[string]Trigger{
		trigger.WindowClosing:       {},
		trigger.NoReply:             {NoReplyFrom: "agent", Minutes: 30},
		trigger.CampaignReply:       {CampaignID: ptr(uuid.New())},
		trigger.Reaction:            {Reaction: "negative"},
		trigger.AIEscalation:        {Reasons: []string{"customer_request", "ai_error"}},
		trigger.SubscriptionRequest: {Subscription: "request"},
	}
	for typ, tr := range good {
		if err := ValidateTrigger(typ, tr); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	if (Trigger{}).WindowMinutes() != DefaultWindowMinutes {
		t.Fatal("default window minutes")
	}
	bad := []struct {
		typ string
		tr  Trigger
	}{
		{trigger.WindowClosing, Trigger{Minutes: 2}},
		{trigger.WindowClosing, Trigger{Minutes: 24 * 60}},
		{trigger.NoReply, Trigger{Minutes: 60}},
		{trigger.NoReply, Trigger{NoReplyFrom: "customer"}},
		{trigger.NoReply, Trigger{NoReplyFrom: "bot", Minutes: 60}},
		{trigger.Reaction, Trigger{Reaction: "meh"}},
		{trigger.AIEscalation, Trigger{Reasons: []string{"manual"}}},
	}
	for _, c := range bad {
		if ValidateTrigger(c.typ, c.tr) == nil {
			t.Errorf("%s %+v accepted", c.typ, c.tr)
		}
	}
}

func TestMatchNewTriggers(t *testing.T) {
	camp, other := uuid.New(), uuid.New()
	pos := trigger.Event{Type: trigger.Reaction, Reaction: trigger.Positive, CampaignID: &camp}
	if !MatchTrigger(Trigger{}, pos) || !MatchTrigger(Trigger{Reaction: "positive"}, pos) || MatchTrigger(Trigger{Reaction: "negative"}, pos) {
		t.Fatal("reaction polarity filter")
	}
	if MatchTrigger(Trigger{CampaignID: &other}, pos) || !MatchTrigger(Trigger{CampaignID: &camp}, pos) {
		t.Fatal("reaction campaign filter")
	}
	reply := trigger.Event{Type: trigger.CampaignReply, CampaignID: &camp}
	if !MatchTrigger(Trigger{}, reply) || MatchTrigger(Trigger{CampaignID: &other}, reply) {
		t.Fatal("campaign reply filter")
	}
	esc := trigger.Event{Type: trigger.AIEscalation, Reason: "low_confidence"}
	if !MatchTrigger(Trigger{}, esc) || !MatchTrigger(Trigger{Reasons: []string{"low_confidence"}}, esc) || MatchTrigger(Trigger{Reasons: []string{"ai_error"}}, esc) {
		t.Fatal("escalation reason filter")
	}
}

func TestPolarity(t *testing.T) {
	for e, want := range map[string]string{"👍": "positive", "👍🏽": "positive", "❤️": "positive", "🙏": "positive", "👎": "negative", "😡": "negative",
		"🤔": "", "": "", "😮": ""} {
		if got := trigger.Polarity(e); got != want {
			t.Errorf("%q: %q, want %q", e, got, want)
		}
	}
}

func TestPresetLibrary(t *testing.T) {
	list, err := Presets()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 11 {
		t.Fatalf("%d presets, want 11", len(list))
	}
	seen := map[string]bool{}
	for i, p := range list {
		if seen[p.Key] {
			t.Fatalf("duplicate %s", p.Key)
		}
		seen[p.Key] = true
		if p.Key != presetOrder[i] {
			t.Fatalf("order: %s at %d", p.Key, i)
		}
		tr, steps := p.Clone()
		if err := ValidateTrigger(p.TriggerType, tr); err != nil {
			t.Fatalf("%s trigger: %v", p.Key, err)
		}
		pending := PendingSetup(steps, p.SetupItems("tr"))
		if len(pending) != len(p.Setup) {
			t.Fatalf("%s setup: %v", p.Key, pending)
		}
		if _, err := CompileDraft(steps, SetupStepIDs(pending)); err != nil {
			t.Fatalf("%s: %v", p.Key, err)
		}
		if len(p.Setup) == 0 {
			if _, err := Compile(steps); err != nil {
				t.Fatalf("%s compiles only as a draft: %v", p.Key, err)
			}
		}
		ai := false
		for _, id := range stepIDList(steps) {
			if IsAIStep(findStep(steps, id).Type) {
				ai = true
			}
		}
		if ai != p.RequiresAI {
			t.Fatalf("%s: requires_ai %v but AI steps %v", p.Key, p.RequiresAI, ai)
		}
		// Meta's window rule: no preset sends filler to keep the window open.
		if p.TriggerType == trigger.WindowClosing {
			for _, id := range stepIDList(steps) {
				if s := findStep(steps, id); s.Type == "reply" || s.Type == "ai_generate_message" {
					t.Fatalf("%s sends a message on window_closing", p.Key)
				}
			}
			if p.Guidance == nil || !strings.Contains(p.Guidance.TR, "müşterinin mesajı") {
				t.Fatalf("%s needs the window guidance", p.Key)
			}
		}
	}
	// Clones never share the library's values.
	p, _ := PresetByKey("follow_up_unanswered")
	_, a := p.Clone()
	a[0].Template.Name = "changed"
	_, b := p.Clone()
	if b[0].Template.Name != "" {
		t.Fatal("clone shares the preset")
	}
	if _, ok := PresetByKey("nope"); ok {
		t.Fatal("unknown preset found")
	}
}
