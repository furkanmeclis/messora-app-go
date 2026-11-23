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
	steps := []api.AutomationStep{}
	_ = json.Unmarshal(a.Steps, &steps)
	var t api.AutomationTrigger
	_ = json.Unmarshal(a.Trigger, &t)
	out = api.Automation{Id: a.ID, Name: a.Name, Description: a.Description, Active: a.Active,
		TriggerType: api.AutomationTriggerType(a.TriggerType), Trigger: t, Steps: steps, Version: int(a.Version),
		RunCount: runs, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
	out.LastRunAt = last
	return out
}

// definition validates and compiles a trigger and steps.
func definition(typ string, trig any, steps any) (triggerJSON, stepsJSON, program []byte, err error) {
	var t Trigger
	if trig != nil {
		if err := recode(trig, &t); err != nil {
			return nil, nil, nil, apperr.Validation("trigger", "Invalid trigger.")
		}
	}
	if err := ValidateTrigger(typ, t); err != nil {
		return nil, nil, nil, err
	}
	var st []Step
	if err := recode(steps, &st); err != nil {
		return nil, nil, nil, apperr.Validation("steps", "Invalid steps.")
	}
	prog, err := Compile(st)
	if err != nil {
		return nil, nil, nil, err
	}
	triggerJSON, _ = json.Marshal(t)
	stepsJSON, _ = json.Marshal(st)
	program, _ = json.Marshal(prog)
	return triggerJSON, stepsJSON, program, nil
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
			Trigger: a.Trigger, Steps: a.Steps, Program: a.Program, Version: a.Version, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt},
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
	tj, sj, pj, err := definition(string(req.TriggerType), trig, req.Steps)
	if err == nil {
		err = h.references(r.Context(), sj)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := principal.From(r.Context())
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	var a db.Automation
	err = h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
		a, err = q.InsertAutomation(r.Context(), db.InsertAutomationParams{Name: strings.TrimSpace(req.Name), Description: desc,
			Active: req.Active != nil && *req.Active, TriggerType: string(req.TriggerType), Trigger: tj, Steps: sj, Program: pj, CreatedBy: &p.ID})
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
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}, a.RunCount, lastRun(a.HasRun, a.LastRunAt)))
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
		tj, sj, pj, err := definition(typ, trig, steps)
		if err == nil {
			err = h.references(r.Context(), sj)
		}
		if err != nil {
			h.fail(w, r, err)
			return
		}
		p.TriggerType, p.Trigger, p.Steps, p.Program = &typ, tj, sj, pj
	}
	var a db.Automation
	err := h.svc.tx(r.Context(), func(_ pgx.Tx, q *db.Queries) error {
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
		Event: ev, Error: x.Error, ResumeAt: x.ResumeAt, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, FinishedAt: x.FinishedAt}
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
			Error: x.Error, ResumeAt: x.ResumeAt, CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, FinishedAt: x.FinishedAt}, x.Phone)
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
