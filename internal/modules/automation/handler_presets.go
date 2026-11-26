package automation

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/consent"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

func presetToAPI(p Preset) api.AutomationPreset {
	out := api.AutomationPreset{Key: p.Key, Version: p.Version, Category: p.Category, RequiresAi: p.RequiresAI,
		Title: api.LocalizedText{Tr: p.Title.TR, En: p.Title.EN}, Description: api.LocalizedText{Tr: p.Description.TR, En: p.Description.EN},
		TriggerType: api.AutomationTriggerType(p.TriggerType), Steps: []api.AutomationStep{}, Setup: []api.AutomationPresetSetup{}}
	if p.Guidance != nil {
		out.Guidance = &api.LocalizedText{Tr: p.Guidance.TR, En: p.Guidance.EN}
	}
	t, steps := p.Clone()
	_ = recode(t, &out.Trigger)
	_ = recode(steps, &out.Steps)
	for _, s := range p.Setup {
		out.Setup = append(out.Setup, api.AutomationPresetSetup{StepId: s.StepID, Field: s.Field,
			Label: api.LocalizedText{Tr: s.Label.TR, En: s.Label.EN}})
	}
	return out
}

// ListAutomationPresets implements api.ServerInterface.
func (h *Handler) ListAutomationPresets(w http.ResponseWriter, r *http.Request) {
	list, err := Presets()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.AutomationPreset, len(list))
	for i, p := range list {
		out[i] = presetToAPI(p)
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// InstallAutomationPreset implements api.ServerInterface: the preset
// becomes a normal automation, inactive, with its setup items open.
func (h *Handler) InstallAutomationPreset(w http.ResponseWriter, r *http.Request, key string, params api.InstallAutomationPresetParams) {
	p, ok := PresetByKey(key)
	if !ok {
		h.fail(w, r, apperr.NotFound("Preset not found."))
		return
	}
	lang := consent.WorkspaceLocale(r.Context(), h.svc.q)
	if params.Lang != nil {
		lang = string(*params.Lang)
	}
	trig, steps := p.Clone()
	def, err := definition(p.TriggerType, trig, steps, p.SetupItems(lang))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	pr := principal.From(r.Context())
	version := int32(p.Version) //nolint:gosec // small
	var a db.Automation
	err = h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
		a, err = q.InsertAutomation(r.Context(), db.InsertAutomationParams{Name: p.Title.For(lang), Description: truncateRunes(p.Description.For(lang), 500),
			TriggerType: p.TriggerType, Trigger: def.trigger, Steps: def.steps, Program: def.program, Layout: []byte("{}"),
			CreatedBy: &pr.ID, Setup: def.setup, PresetKey: &p.Key, PresetVersion: &version})
		if err != nil {
			return err
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: "automation.create", EntityType: "automation", EntityID: &a.ID,
			Changes: map[string]any{"name": a.Name, "active": false, "preset": p.Key, "preset_version": p.Version}})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, toAPI(a, 0, nil))
}
