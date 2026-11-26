package automation

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
)

// presetFS holds the built-in presets (one JSON file each). A preset is
// versioned: changing its definition bumps version; installed copies keep
// the version they came from and are never changed afterwards.
//
//go:embed presets/*.json
var presetFS embed.FS

// Localized is a text in the panel's languages.
type Localized struct {
	TR string `json:"tr"`
	EN string `json:"en"`
}

// For returns the text in lang (tr or en; anything else is English).
func (l Localized) For(lang string) string {
	if strings.HasPrefix(lang, "tr") && l.TR != "" {
		return l.TR
	}
	return l.EN
}

// PresetSetup is a value the user must choose after installing a preset.
type PresetSetup struct {
	StepID string    `json:"step_id"`
	Field  string    `json:"field"`
	Label  Localized `json:"label"`
}

// Preset is a ready-made automation of the library.
type Preset struct {
	Key         string          `json:"key"`
	Version     int             `json:"version"`
	Category    string          `json:"category"`
	Title       Localized       `json:"title"`
	Description Localized       `json:"description"`
	Guidance    *Localized      `json:"guidance,omitempty"`
	RequiresAI  bool            `json:"requires_ai,omitempty"`
	TriggerType string          `json:"trigger_type"`
	Trigger     Trigger         `json:"trigger"`
	Steps       []Step          `json:"steps"`
	Setup       []PresetSetup   `json:"setup"`
	raw         json.RawMessage `json:"-"`
}

// SetupItem is an open setup item of an automation
// (automations.setup).
type SetupItem struct {
	StepID string `json:"step_id"`
	Field  string `json:"field"`
	Label  string `json:"label"`
}

var (
	presetsOnce sync.Once
	presets     []Preset
	presetsErr  error
)

// Presets returns the library, in a fixed order.
func Presets() ([]Preset, error) {
	presetsOnce.Do(func() { presets, presetsErr = loadPresets(presetFS) })
	return presets, presetsErr
}

// PresetByKey returns one preset.
func PresetByKey(key string) (Preset, bool) {
	list, err := Presets()
	if err != nil {
		return Preset{}, false
	}
	for _, p := range list {
		if p.Key == key {
			return p, true
		}
	}
	return Preset{}, false
}

// presetOrder is the gallery order.
var presetOrder = []string{"welcome", "unsubscribe_request", "collect_customer_info", "handoff_to_agent", "ai_auto_reply",
	"window_reminder", "follow_up_unanswered", "campaign_follow_up", "positive_feedback", "negative_feedback", "detect_silent_customers"}

func loadPresets(fsys fs.FS) ([]Preset, error) {
	files, err := fs.Glob(fsys, "presets/*.json")
	if err != nil {
		return nil, err
	}
	var out []Preset
	for _, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var p Preset
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("preset %s: %w", f, err)
		}
		if err := p.check(); err != nil {
			return nil, fmt.Errorf("preset %s: %w", f, err)
		}
		p.raw = raw
		out = append(out, p)
	}
	slices.SortStableFunc(out, func(a, b Preset) int {
		ia, ib := slices.Index(presetOrder, a.Key), slices.Index(presetOrder, b.Key)
		if ia < 0 {
			ia = len(presetOrder)
		}
		if ib < 0 {
			ib = len(presetOrder)
		}
		return ia - ib
	})
	return out, nil
}

// check validates a preset: its trigger and steps compile (with its setup
// items open), every setup item names a step and field that is still
// empty, and its steps have ids.
func (p Preset) check() error {
	if p.Key == "" || p.Version < 1 || p.Title.TR == "" || p.Title.EN == "" || p.Description.TR == "" || p.Description.EN == "" {
		return fmt.Errorf("key, version, title and description are required")
	}
	if err := ValidateTrigger(p.TriggerType, p.Trigger); err != nil {
		return err
	}
	for _, id := range stepIDList(p.Steps) {
		if id == "" {
			return fmt.Errorf("every step needs an id")
		}
	}
	items := p.SetupItems("en")
	if pending := PendingSetup(p.Steps, items); len(pending) != len(items) {
		return fmt.Errorf("setup items must name existing, empty step fields")
	}
	_, err := CompileDraft(p.Steps, SetupStepIDs(items))
	return err
}

// SetupItems are the preset's setup items with labels in lang.
func (p Preset) SetupItems(lang string) []SetupItem {
	out := make([]SetupItem, len(p.Setup))
	for i, s := range p.Setup {
		out[i] = SetupItem{StepID: s.StepID, Field: s.Field, Label: s.Label.For(lang)}
	}
	return out
}

// Clone returns a deep copy of the preset's steps (installing must never
// share the library's values).
func (p Preset) Clone() (Trigger, []Step) {
	var fresh Preset
	_ = json.Unmarshal(p.raw, &fresh)
	return fresh.Trigger, fresh.Steps
}

func stepIDList(steps []Step) []string {
	var ids []string
	walkSteps(steps, "", func(s *Step, _ string) { ids = append(ids, s.ID) })
	return ids
}

// findStep returns the step with id.
func findStep(steps []Step, id string) *Step {
	var found *Step
	walkSteps(steps, "", func(s *Step, _ string) {
		if s.ID == id && found == nil {
			found = s
		}
	})
	return found
}

// filled reports whether a step's field has a value.
func filled(s *Step, field string) bool {
	switch field {
	case "template":
		return s.Template != nil && strings.TrimSpace(s.Template.Name) != "" && strings.TrimSpace(s.Template.Language) != ""
	case "assignee_id":
		return s.AssigneeID != nil
	case "group_id":
		return s.GroupID != nil
	case "group_ids":
		return len(s.GroupIDs) > 0
	case "tags":
		return len(s.Tags) > 0
	}
	raw, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	v, ok := m[field]
	if !ok || v == nil {
		return false
	}
	if str, ok := v.(string); ok {
		return strings.TrimSpace(str) != ""
	}
	return true
}

// PendingSetup returns the setup items still open for steps: the step
// exists and the field is empty. Items of removed steps are dropped.
func PendingSetup(steps []Step, items []SetupItem) []SetupItem {
	out := []SetupItem{}
	for _, it := range items {
		if s := findStep(steps, it.StepID); s != nil && !filled(s, it.Field) {
			out = append(out, it)
		}
	}
	return out
}

// SetupStepIDs are the steps with open setup items (compiled leniently).
func SetupStepIDs(items []SetupItem) map[string]bool {
	ids := map[string]bool{}
	for _, it := range items {
		ids[it.StepID] = true
	}
	return ids
}
