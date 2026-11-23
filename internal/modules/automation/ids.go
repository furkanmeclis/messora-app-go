package automation

import (
	"crypto/rand"
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// TriggerNodeID is the canvas id of the trigger node; no step may use it.
const TriggerNodeID = "trigger"

// MaxLayoutEntries bounds the stored canvas layout (steps plus trigger).
const MaxLayoutEntries = 200

// maxCoordinate bounds a node position on the canvas.
const maxCoordinate = 1e6

var stepIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Point is a node position on the editor's canvas.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// validStepID reports whether id may name a step.
func validStepID(id string) bool {
	return id != TriggerNodeID && stepIDPattern.MatchString(id)
}

// NewStepID returns a fresh random step id ("n" and 12 base-36 digits),
// the same shape the canvas generates in the browser.
func NewStepID() string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var b [12]byte
	_, _ = rand.Read(b[:])
	out := make([]byte, 0, 13)
	out = append(out, 'n')
	for _, c := range b {
		out = append(out, digits[int(c)%len(digits)])
	}
	return string(out)
}

// walkSteps calls fn for every step of the tree, depth first in program
// order, with its error path (steps[1].then[0]).
func walkSteps(steps []Step, path string, fn func(s *Step, path string)) {
	for i := range steps {
		p := fmt.Sprintf("%s[%d]", path, i)
		fn(&steps[i], p)
		walkSteps(steps[i].Then, p+".then", fn)
		walkSteps(steps[i].Else, p+".else", fn)
	}
}

// AssignIDs gives every step without an id a fresh random one, in place.
// Clients that predate step ids keep working: the server names their steps.
func AssignIDs(steps []Step) {
	walkSteps(steps, "steps", func(s *Step, _ string) {
		if s.ID == "" {
			s.ID = NewStepID()
		}
	})
}

// LegacyIDs gives steps without an id one derived from their position
// ("s0", "s1t0" = first then-step of the second step, "s1e0" for else),
// in place. Automations saved before step ids existed therefore read with
// the same ids every time, so their canvas and run traces line up until
// they are saved again (which stores the ids). Ids already present win;
// a derived id that collides with one gets a numeric suffix.
func LegacyIDs(steps []Step) {
	used := map[string]bool{}
	walkSteps(steps, "steps", func(s *Step, _ string) {
		if s.ID != "" {
			used[s.ID] = true
		}
	})
	var fill func(list []Step, prefix string)
	fill = func(list []Step, prefix string) {
		for i := range list {
			s := &list[i]
			base := prefix + strconv.Itoa(i)
			if s.ID == "" {
				id := base
				for n := 2; used[id]; n++ {
					id = base + "_" + strconv.Itoa(n)
				}
				s.ID = id
				used[id] = true
			}
			fill(s.Then, base+"t")
			fill(s.Else, base+"e")
		}
	}
	fill(steps, "s")
}

// ValidateIDs checks that every step id present is well-formed and unique.
// Steps without an id pass (AssignIDs names them).
func ValidateIDs(steps []Step) error {
	seen := map[string]bool{}
	var err error
	walkSteps(steps, "steps", func(s *Step, p string) {
		if err != nil || s.ID == "" {
			return
		}
		switch {
		case !validStepID(s.ID):
			err = bad(p+".id", `Step ids are 1–64 letters, digits, "-" or "_" ("trigger" is reserved).`)
		case seen[s.ID]:
			err = bad(p+".id", fmt.Sprintf("The step id %q is used twice.", s.ID))
		}
		seen[s.ID] = true
	})
	return err
}

// StepIDs returns the ids of every step in the tree.
func StepIDs(steps []Step) map[string]bool {
	ids := map[string]bool{}
	walkSteps(steps, "steps", func(s *Step, _ string) {
		if s.ID != "" {
			ids[s.ID] = true
		}
	})
	return ids
}

// CleanLayout validates canvas positions and keeps only those of the
// trigger and of steps that exist, so the layout never outgrows the steps.
func CleanLayout(layout map[string]Point, ids map[string]bool) (map[string]Point, error) {
	if len(layout) > MaxLayoutEntries {
		return nil, bad("layout", fmt.Sprintf("At most %d node positions.", MaxLayoutEntries))
	}
	out := make(map[string]Point, len(layout))
	for id, p := range layout {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.Abs(p.X) > maxCoordinate || math.Abs(p.Y) > maxCoordinate {
			return nil, bad("layout", "A node position is out of range.")
		}
		if id == TriggerNodeID || ids[id] {
			out[id] = p
		}
	}
	return out, nil
}
