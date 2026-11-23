package automation

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

func cond(then, els []Step) Step {
	return Step{Type: "condition", If: &Condition{Field: "message.body", Operator: "is_set"}, Then: then, Else: els}
}

func ids(steps []Step) []string {
	var out []string
	walkSteps(steps, "steps", func(s *Step, _ string) { out = append(out, s.ID) })
	return out
}

func TestAssignIDsKeepsGivenIDsAndNamesTheRest(t *testing.T) {
	cases := []struct {
		name  string
		steps []Step
		keep  map[int]string // position in walk order → id that must stay
	}{
		{"none", []Step{{Type: "add_tag"}, {Type: "reply"}}, nil},
		{"some", []Step{{ID: "a", Type: "add_tag"}, {Type: "reply"}}, map[int]string{0: "a"}},
		{"nested", []Step{cond([]Step{{ID: "yes-1", Type: "reply"}}, []Step{{Type: "reply"}, {Type: "mute"}}), {Type: "delay"}},
			map[int]string{1: "yes-1"}},
	}
	for _, c := range cases {
		AssignIDs(c.steps)
		got := ids(c.steps)
		seen := map[string]bool{}
		for i, id := range got {
			if !validStepID(id) {
				t.Errorf("%s: step %d has invalid id %q", c.name, i, id)
			}
			if seen[id] {
				t.Errorf("%s: id %q twice", c.name, id)
			}
			seen[id] = true
			if want, ok := c.keep[i]; ok && id != want {
				t.Errorf("%s: step %d id %q, want %q kept", c.name, i, id, want)
			}
		}
		if err := ValidateIDs(c.steps); err != nil {
			t.Errorf("%s: assigned ids rejected: %v", c.name, err)
		}
	}
}

func TestValidateIDs(t *testing.T) {
	long := strings.Repeat("x", 65)
	cases := []struct {
		name  string
		steps []Step
		path  string // "" = valid
	}{
		{"no ids (legacy clients)", []Step{{Type: "reply"}, cond([]Step{{Type: "reply"}}, nil)}, ""},
		{"valid ids", []Step{{ID: "a_1", Type: "reply"}, {ID: "B-2", Type: "reply"}}, ""},
		{"reserved", []Step{{ID: "trigger", Type: "reply"}}, "steps[0].id"},
		{"bad characters", []Step{{ID: "a b", Type: "reply"}}, "steps[0].id"},
		{"dot", []Step{{ID: "a.b", Type: "reply"}}, "steps[0].id"},
		{"too long", []Step{{ID: long, Type: "reply"}}, "steps[0].id"},
		{"duplicate at top", []Step{{ID: "x", Type: "reply"}, {ID: "x", Type: "reply"}}, "steps[1].id"},
		{"duplicate across branches", []Step{{ID: "c", Type: "condition", If: &Condition{Field: "message.body", Operator: "is_set"},
			Then: []Step{{ID: "x", Type: "reply"}}, Else: []Step{{ID: "x", Type: "reply"}}}}, "steps[0].else[0].id"},
		{"condition reuses child id", []Step{{ID: "c", Type: "condition", If: &Condition{Field: "message.body", Operator: "is_set"},
			Then: []Step{{ID: "c", Type: "reply"}}}}, "steps[0].then[0].id"},
	}
	for _, c := range cases {
		err := ValidateIDs(c.steps)
		if c.path == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(errText(err), c.path) {
			t.Errorf("%s: want error at %s, got %v", c.name, c.path, err)
		}
		if _, cerr := Compile(c.steps); cerr == nil {
			t.Errorf("%s: Compile accepted invalid ids", c.name)
		}
	}
}

// errText includes the field path of validation errors.
func errText(err error) string {
	if e, ok := apperr.As(err); ok {
		return e.Field + " " + e.Message
	}
	return err.Error()
}

func TestLegacyIDsArePositionalAndStable(t *testing.T) {
	legacy := func() []Step {
		return []Step{
			{Type: "add_tag", Tag: "a"},
			cond([]Step{{Type: "reply", Text: "yes"}}, []Step{{Type: "reply", Text: "no"}, cond([]Step{{Type: "mute", Seconds: 60}}, nil)}),
			{Type: "delay", Seconds: 5},
		}
	}
	a, b := legacy(), legacy()
	LegacyIDs(a)
	LegacyIDs(b)
	want := []string{"s0", "s1", "s1t0", "s1e0", "s1e1", "s1e1t0", "s2"}
	if got := ids(a); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("legacy ids %v, want %v", got, want)
	}
	if strings.Join(ids(a), ",") != strings.Join(ids(b), ",") {
		t.Fatal("legacy ids are not stable")
	}
	// Ids present win; a derived id that collides gets a suffix.
	mixed := []Step{{Type: "reply"}, {ID: "s0", Type: "reply"}}
	LegacyIDs(mixed)
	if mixed[0].ID != "s0_2" || mixed[1].ID != "s0" {
		t.Fatalf("collision: %v", ids(mixed))
	}
	if err := ValidateIDs(mixed); err != nil {
		t.Fatal(err)
	}
}

func TestCompileMapsInstructionsToStepIDs(t *testing.T) {
	steps := []Step{
		{ID: "tag", Type: "add_tag", Tag: "a"},
		{ID: "if", Type: "condition", If: &Condition{Field: "contact.var1", Operator: "equals", Value: "x"},
			Then: []Step{{ID: "yes", Type: "reply", Text: "yes"}},
			Else: []Step{{ID: "no", Type: "reply", Text: "no"}, {ID: "no2", Type: "add_tag", Tag: "b"}}},
		{ID: "wait", Type: "delay", Seconds: 5},
	}
	prog, err := Compile(steps)
	if err != nil {
		t.Fatal(err)
	}
	// 0 tag, 1 branch(if), 2 yes, 3 jump, 4 no, 5 no2, 6 wait
	want := []string{"tag", "if", "yes", "", "no", "no2", "wait"}
	for i, in := range prog {
		if in.StepID != want[i] {
			t.Errorf("instruction %d (%s): step id %q, want %q", i, in.Op, in.StepID, want[i])
		}
	}
	if prog[1].Step.ID != "if" || prog[2].Step.ID != "yes" {
		t.Fatal("instruction steps keep their ids")
	}
	// The stored program round-trips the ids.
	b, _ := json.Marshal(prog)
	var back []Instr
	_ = json.Unmarshal(b, &back)
	if back[5].StepID != "no2" {
		t.Fatalf("round trip lost ids: %s", b)
	}
}

func TestLegacyProgramResolvesStepIDs(t *testing.T) {
	// A program stored before ids existed: steps and instructions without
	// ids. The engine names its instructions like the API names its steps.
	steps := []Step{
		{Type: "add_tag", Tag: "a"},
		cond([]Step{{Type: "reply", Text: "yes"}}, []Step{{Type: "reply", Text: "no"}}),
		{Type: "set_field", Field: "var1", Value: "x"},
	}
	old, err := Compile(steps)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range old {
		if in.StepID != "" {
			t.Fatal("legacy program must not have ids")
		}
	}
	raw, _ := json.Marshal(steps)
	want := []string{"s0", "s1", "s1t0", "", "s1e0", "s2"}
	for pc, in := range old {
		if in.Op == "jump" {
			continue
		}
		if got := legacyStepID(raw, pc); got != want[pc] {
			t.Errorf("pc %d: %q, want %q", pc, got, want[pc])
		}
	}
	if legacyStepID(raw, 99) != "" || legacyStepID([]byte("nope"), 0) != "" {
		t.Fatal("out of range or broken steps must give no id")
	}
}

func TestCleanLayout(t *testing.T) {
	known := map[string]bool{"a": true, "b": true}
	out, err := CleanLayout(map[string]Point{"trigger": {0, 0}, "a": {10, 20.5}, "gone": {1, 1}}, known)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out["a"].Y != 20.5 {
		t.Fatalf("layout %v", out)
	}
	for name, l := range map[string]map[string]Point{
		"nan":      {"a": {math.NaN(), 0}},
		"inf":      {"a": {0, math.Inf(1)}},
		"far away": {"a": {2e6, 0}},
	} {
		if _, err := CleanLayout(l, known); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	many := map[string]Point{}
	for i := range MaxLayoutEntries + 1 {
		many[NewStepID()+string(rune('a'+i%26))] = Point{}
	}
	if _, err := CleanLayout(many, known); err == nil {
		t.Error("too many positions accepted")
	}
}

// FuzzStepIDs feeds arbitrary step trees through the id validator, the
// id assignment and the compiler: nothing panics, accepted programs only
// jump forward, and every executed instruction names a step of the tree.
func FuzzStepIDs(f *testing.F) {
	f.Add(`[{"type":"reply","text":"hi"}]`)
	f.Add(`[{"id":"a","type":"add_tag","tag":"x"},{"id":"a","type":"reply","text":"y"}]`)
	f.Add(`[{"id":"trigger","type":"reply","text":"y"}]`)
	f.Add(`[{"type":"condition","if":{"field":"message.body","operator":"is_set"},"then":[{"id":"t","type":"mute","seconds":60}],"else":[{"type":"reply","text":"n"}]},{"type":"delay","seconds":3}]`)
	f.Add(`[{"id":"c","type":"condition","if":{"field":"contact.tag","operator":"equals","value":"v"},"then":[{"id":"c","type":"reply","text":"n"}]}]`)
	f.Add(`[{"id":"a b","type":"reply","text":"x"}]`)
	f.Fuzz(func(t *testing.T, in string) {
		var steps []Step
		if json.Unmarshal([]byte(in), &steps) != nil {
			return
		}
		valid := ValidateIDs(steps) == nil
		if _, err := Compile(steps); err == nil && !valid {
			t.Fatalf("Compile accepted ids ValidateIDs rejects: %s", in)
		}
		if !valid {
			return
		}
		var legacy []Step
		_ = json.Unmarshal([]byte(in), &legacy)
		LegacyIDs(legacy)
		if err := ValidateIDs(legacy); err != nil {
			t.Fatalf("legacy ids invalid: %v (%s)", err, in)
		}
		AssignIDs(steps)
		if err := ValidateIDs(steps); err != nil {
			t.Fatalf("assigned ids invalid: %v (%s)", err, in)
		}
		known := StepIDs(steps)
		prog, err := Compile(steps)
		if err != nil {
			return
		}
		for i, ins := range prog {
			if (ins.Op == "branch" || ins.Op == "jump") && (ins.Target <= i || ins.Target > len(prog)) {
				t.Fatalf("instruction %d jumps to %d", i, ins.Target)
			}
			if ins.Op == "jump" {
				if ins.StepID != "" {
					t.Fatalf("jump %d carries a step id", i)
				}
				continue
			}
			if !known[ins.StepID] {
				t.Fatalf("instruction %d names unknown step %q", i, ins.StepID)
			}
		}
	})
}
