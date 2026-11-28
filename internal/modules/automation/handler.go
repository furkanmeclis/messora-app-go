package automation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Handler serves the automation operations.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err, h.log)
}

// recode converts between API and engine types through JSON, so step
// fields the API schema leaves open are kept exactly.
func recode(from, to any) error {
	b, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, to)
}

func toAPI(a db.Automation, runs int64, last *time.Time) api.Automation {
	var out api.Automation
	// Automations saved before step ids existed read with ids derived from
	// each step's position (stable until the next save stores real ones).
	var st []Step
	_ = json.Unmarshal(a.Steps, &st)
	LegacyIDs(st)
	steps := []api.AutomationStep{}
	_ = recode(st, &steps)
	var t api.AutomationTrigger
	_ = json.Unmarshal(a.Trigger, &t)
	layout := api.AutomationLayout{}
	if len(a.Layout) > 0 {
		_ = json.Unmarshal(a.Layout, &layout)
	}
	out = api.Automation{Id: a.ID, Name: a.Name, Description: a.Description, Active: a.Active,
		TriggerType: api.AutomationTriggerType(a.TriggerType), Trigger: t, Steps: steps, Layout: layout, Version: int(a.Version),
		RunCount: runs, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Setup: []api.AutomationSetupItem{}}
	var setup []SetupItem
	_ = json.Unmarshal(a.Setup, &setup)
	for _, s := range setup {
		out.Setup = append(out.Setup, api.AutomationSetupItem{StepId: s.StepID, Field: s.Field, Label: s.Label})
	}
	if a.PresetKey != nil {
		v := 0
		if a.PresetVersion != nil {
			v = int(*a.PresetVersion)
		}
		out.Preset = &api.AutomationPresetRef{Key: *a.PresetKey, Version: v}
	}
	out.LastRunAt = last
	return out
}

// layoutJSON validates canvas positions against the steps they belong to.
func layoutJSON(layout *api.AutomationLayout, steps []byte) ([]byte, error) {
	var in map[string]Point
	if err := recode(layout, &in); err != nil {
		return nil, apperr.Validation("layout", "Invalid layout.")
	}
	var st []Step
	_ = json.Unmarshal(steps, &st)
	LegacyIDs(st) // positions of a legacy automation use its derived ids
	clean, err := CleanLayout(in, StepIDs(st))
	if err != nil {
		return nil, err
	}
	return json.Marshal(clean)
}

// compiled is a validated automation definition.
type compiled struct {
	trigger, steps, program, setup []byte
	pending                        []SetupItem
}

// definition validates and compiles a trigger and steps. setup is the
// automation's open setup items: those still unfilled stay open, and
// their steps may lack the value (CompileDraft).
func definition(typ string, trig any, steps any, setup []SetupItem) (compiled, error) {
	var t Trigger
	if trig != nil {
		if err := recode(trig, &t); err != nil {
			return compiled{}, apperr.Validation("trigger", "Invalid trigger.")
		}
	}
	if err := ValidateTrigger(typ, t); err != nil {
		return compiled{}, err
	}
	var st []Step
	if err := recode(steps, &st); err != nil {
		return compiled{}, apperr.Validation("steps", "Invalid steps.")
	}
	if err := ValidateIDs(st); err != nil {
		return compiled{}, err
	}
	// Steps sent without ids (older clients) are named here.
	AssignIDs(st)
	pending := PendingSetup(st, setup)
	prog, err := CompileDraft(st, SetupStepIDs(pending))
	if err != nil {
		return compiled{}, err
	}
	var c compiled
	c.trigger, _ = json.Marshal(t)
	c.steps, _ = json.Marshal(st)
	c.program, _ = json.Marshal(prog)
	c.setup, _ = json.Marshal(pending)
	c.pending = pending
	return c, nil
}

// setupBlocks refuses enabling an automation with open setup items.
func setupBlocks(active bool, pending []SetupItem) error {
	if !active || len(pending) == 0 {
		return nil
	}
	labels := make([]string, len(pending))
	for i, p := range pending {
		labels[i] = p.Label
	}
	return apperr.Validation("active", "Finish the setup before enabling: "+strings.Join(labels, "; ")+".")
}

func (h *Handler) references(ctx context.Context, steps []byte) error {
	var st []Step
	_ = json.Unmarshal(steps, &st)
	var walk func([]Step) error
	walk = func(list []Step) error {
		for _, s := range list {
			if s.GroupID != nil {
				if _, err := h.svc.q.GetContactGroup(ctx, *s.GroupID); errors.Is(err, pgx.ErrNoRows) {
					return apperr.Validation("steps", "A step refers to a group that does not exist.")
				}
			}
			if err := walk(s.Then); err != nil {
				return err
			}
			if err := walk(s.Else); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(st)
}

// ListAutomations implements api.ServerInterface.
func (h *Handler) ListAutomations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.q.ListAutomations(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]api.Automation, len(rows))
	for i, a := range rows {
		out[i] = toAPI(db.Automation{ID: a.ID, Name: a.Name, Description: a.Description, Active: a.Active, TriggerType: a.TriggerType,
			Trigger: a.Trigger, Steps: a.Steps, Program: a.Program, Version: a.Version, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Layout: a.Layout,
			Setup: a.Setup, PresetKey: a.PresetKey, PresetVersion: a.PresetVersion},
			a.RunCount, lastRun(a.HasRun, a.LastRunAt))
	}
	httpx.JSON(w, r, http.StatusOK, out)
}

// CreateAutomation implements api.ServerInterface.
func (h *Handler) CreateAutomation(w http.ResponseWriter, r *http.Request) {
	var req api.CreateAutomationRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	var trig any
	if req.Trigger != nil {
		trig = req.Trigger
	}
	def, err := definition(string(req.TriggerType), trig, req.Steps, nil)
	if err == nil {
		err = h.references(r.Context(), def.steps)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	lj := []byte("{}")
	if req.Layout != nil {
		if lj, err = layoutJSON(req.Layout, def.steps); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	p := principal.From(r.Context())
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	var a db.Automation
	err = h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
		if req.Active != nil && *req.Active {
			if err := h.svc.checkActive(r.Context(), q, nil); err != nil {
				return err
			}
		}
		a, err = q.InsertAutomation(r.Context(), db.InsertAutomationParams{Name: strings.TrimSpace(req.Name), Description: desc,
			Active: req.Active != nil && *req.Active, TriggerType: string(req.TriggerType), Trigger: def.trigger, Steps: def.steps,
			Program: def.program, Layout: lj, CreatedBy: &p.ID, Setup: def.setup})
		if err != nil {
			return err
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: "automation.create", EntityType: "automation", EntityID: &a.ID,
			Changes: map[string]any{"name": a.Name, "active": a.Active}})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusCreated, toAPI(a, 0, nil))
}

func (h *Handler) load(w http.ResponseWriter, r *http.Request, id uuid.UUID) (db.Automation, bool) {
	a, err := h.svc.q.GetAutomation(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Automation not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return a, false
	}
	return a, true
}

// GetAutomation implements api.ServerInterface.
func (h *Handler) GetAutomation(w http.ResponseWriter, r *http.Request, id api.AutomationId) {
	a, err := h.svc.q.GetAutomationWithStats(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Automation not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, toAPI(db.Automation{ID: a.ID, Name: a.Name, Description: a.Description, Active: a.Active,
		TriggerType: a.TriggerType, Trigger: a.Trigger, Steps: a.Steps, Program: a.Program, Version: a.Version,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Layout: a.Layout,
		Setup: a.Setup, PresetKey: a.PresetKey, PresetVersion: a.PresetVersion}, a.RunCount, lastRun(a.HasRun, a.LastRunAt)))
}

func lastRun(has bool, at time.Time) *time.Time {
	if !has {
		return nil
	}
	return &at
}

// UpdateAutomation implements api.ServerInterface.
func (h *Handler) UpdateAutomation(w http.ResponseWriter, r *http.Request, id api.AutomationId) {
	var req api.UpdateAutomationRequest
	if err := httpx.Decode(r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	cur, ok := h.load(w, r, id)
	if !ok {
		return
	}
	p := db.UpdateAutomationParams{ID: id, Name: req.Name, Description: req.Description, Active: req.Active}
	redefined := req.TriggerType != nil || req.Trigger != nil || req.Steps != nil
	var pending []SetupItem
	_ = json.Unmarshal(cur.Setup, &pending)
	if redefined {
		typ := cur.TriggerType
		if req.TriggerType != nil {
			typ = string(*req.TriggerType)
		}
		var trig any = json.RawMessage(cur.Trigger)
		if req.Trigger != nil {
			trig = req.Trigger
		}
		var steps any = json.RawMessage(cur.Steps)
		if req.Steps != nil {
			steps = *req.Steps
		}
		def, err := definition(typ, trig, steps, pending)
		if err == nil {
			err = h.references(r.Context(), def.steps)
		}
		if err != nil {
			h.fail(w, r, err)
			return
		}
		p.TriggerType, p.Trigger, p.Steps, p.Program, p.Setup = &typ, def.trigger, def.steps, def.program, def.setup
		pending = def.pending
	}
	active := cur.Active
	if req.Active != nil {
		active = *req.Active
	}
	if err := setupBlocks(active, pending); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Layout != nil {
		steps := p.Steps
		if steps == nil {
			steps = cur.Steps
		}
		lj, err := layoutJSON(req.Layout, steps)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		p.Layout = lj
	}
	var a db.Automation
	err := h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
		// Switching a flow on counts against the plan's active flows; a
		// flow that is on stays on after a lower plan (Phase 17 WP8).
		if active && !cur.Active {
			if err := h.svc.checkActive(r.Context(), q, &id); err != nil {
				return err
			}
		}
		var err error
		a, err = q.UpdateAutomation(r.Context(), p)
		if err != nil {
			return err
		}
		if redefined {
			// Runs in progress follow the old program; stop them.
			if _, err := q.CancelRunsOfAutomation(r.Context(), id); err != nil {
				return err
			}
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: "automation.update", EntityType: "automation", EntityID: &id,
			Changes: map[string]any{"redefined": redefined, "active": a.Active}})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, r, http.StatusOK, toAPI(a, 0, nil))
}

// DeleteAutomation implements api.ServerInterface.
func (h *Handler) DeleteAutomation(w http.ResponseWriter, r *http.Request, id api.AutomationId) {
	err := h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
		n, err := q.DeleteAutomation(r.Context(), id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound("Automation not found.")
		}
		return audit.Record(r.Context(), q, audit.Entry{Action: "automation.delete", EntityType: "automation", EntityID: &id})
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.NoContent(w)
}

func runToAPI(x db.AutomationRun, phone string) api.AutomationRun {
	ev := map[string]any{}
	_ = json.Unmarshal(x.Event, &ev)
	out := api.AutomationRun{Id: x.ID, AutomationId: x.AutomationID, ContactId: x.ContactID, Status: api.AutomationRunStatus(x.Status),
		Event: ev, Error: x.Error, ResumeAt: x.ResumeAt, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, FinishedAt: x.FinishedAt,
		Visited: []string{}}
	if len(x.Visited) > 0 {
		_ = json.Unmarshal(x.Visited, &out.Visited)
	}
	out.ErrorCode = x.ErrorCode
	attempt := int(x.Attempt)
	out.Attempt = &attempt
	vars := map[string]string{}
	_ = json.Unmarshal(x.Vars, &vars)
	out.Vars = &vars
	if phone != "" {
		out.Phone = &phone
	}
	return out
}

// ListAutomationRuns implements api.ServerInterface.
func (h *Handler) ListAutomationRuns(w http.ResponseWriter, r *http.Request, id api.AutomationId, p api.ListAutomationRunsParams) {
	if _, ok := h.load(w, r, id); !ok {
		return
	}
	limit := 50
	if p.Limit != nil {
		limit = *p.Limit
	}
	var before *uuid.UUID
	if p.Cursor != nil {
		c, err := uuid.Parse(*p.Cursor)
		if err != nil {
			h.fail(w, r, apperr.Validation("cursor", "Invalid cursor"))
			return
		}
		before = &c
	}
	rows, err := h.svc.q.ListAutomationRuns(r.Context(), db.ListAutomationRunsParams{AutomationID: id, BeforeID: before, Lim: int32(limit + 1)}) //nolint:gosec // ≤ 201
	if err != nil {
		h.fail(w, r, err)
		return
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[limit-1].ID.String()
	}
	out := make([]api.AutomationRun, len(rows))
	for i, x := range rows {
		out[i] = runToAPI(db.AutomationRun{ID: x.ID, AutomationID: x.AutomationID, ContactID: x.ContactID, Status: x.Status, Event: x.Event,
			Error: x.Error, ResumeAt: x.ResumeAt, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, FinishedAt: x.FinishedAt, Visited: x.Visited,
			Vars: x.Vars, Attempt: x.Attempt, ErrorCode: x.ErrorCode}, x.Phone)
	}
	httpx.JSONPage(w, r, http.StatusOK, out, next)
}

// GetAutomationRun implements api.ServerInterface.
func (h *Handler) GetAutomationRun(w http.ResponseWriter, r *http.Request, id api.RunId) {
	x, err := h.svc.q.GetAutomationRun(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = apperr.NotFound("Run not found.")
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	steps, err := h.svc.q.ListAutomationRunSteps(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := runToAPI(x, "")
	list := make([]api.AutomationRunStep, len(steps))
	for i, s := range steps {
		d := map[string]any{}
		_ = json.Unmarshal(s.Detail, &d)
		list[i] = api.AutomationRunStep{Pc: int(s.Pc), Action: s.Action, Outcome: api.AutomationRunStepOutcome(s.Outcome), Detail: d, CreatedAt: s.CreatedAt}
	}
	out.Steps = &list
	httpx.JSON(w, r, http.StatusOK, out)
}
