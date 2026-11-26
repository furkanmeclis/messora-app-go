package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/activity"
	"github.com/furkanmeclis/messora-app-go/internal/modules/ai"
	"github.com/furkanmeclis/messora-app-go/internal/modules/automation/trigger"
	"github.com/furkanmeclis/messora-app-go/internal/modules/campaigns"
	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/modules/whatsapp"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/safehttp"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/licensegate"
)

// Task types.
const (
	TypeRun    = "automation:run"
	TypeResume = "automation:resume"
	TypeReaper = "automation:reaper"
	// TypeScan finds conversations for the time-based triggers
	// (window_closing, no_reply); the scheduler enqueues it every minute.
	TypeScan = "automation:scan"
)

// maxInstructionsPerRun bounds a run even though programs only jump forward.
const maxInstructionsPerRun = 500

// ErrRetriesExhausted is returned by Run when a step failed after its last
// retry: the run is marked failed, and the worker hands the task to the
// dead-letter queue (archived, listed on the failed jobs page).
var ErrRetriesExhausted = errors.New("automation: step failed after its last retry")

// Failure codes of a run (automation_runs.error_code).
const (
	CodeAIDisabled       = "ai_disabled"
	CodeAINotConfigured  = "ai_not_configured"
	CodeAINotAllowed     = "ai_not_allowed"
	CodeAINoProfile      = "ai_no_profile"
	CodeAIError          = "ai_error"
	CodeRetriesExhausted = "retries_exhausted"
	CodeStepFailed       = "step_failed"
)

// StepError is a step failure with a machine-readable code; Retryable
// failures follow the step's retry policy.
type StepError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *StepError) Error() string { return e.Message }

// Doer sends HTTP requests to user-supplied URLs; production uses the
// SSRF-safe client from platform/safehttp.
type Doer interface {
	Do(*http.Request) (*safehttp.Response, error)
}

// Service is the automation engine.
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	msg    *messaging.Service
	mail   mailer.Sender
	http   Doer
	log    *slog.Logger
	ai     *ai.Service
	inbox  Inbox
	fields FieldWriter
	now    func() time.Time
}

// New returns the engine. httpClient may be nil, which disables HTTP steps.
func New(pool *pgxpool.Pool, msg *messaging.Service, mail mailer.Sender, httpClient Doer, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), msg: msg, mail: mail, http: httpClient, log: log, fields: directFields{}, now: time.Now}
}

// SetAI connects the AI module (AI steps fail with ai_not_configured
// without it).
func (s *Service) SetAI(a *ai.Service) { s.ai = a }

// SetInbox connects the conversations module (handoff, note and the
// conversation-aware AI steps are skipped without it).
func (s *Service) SetInbox(i Inbox) { s.inbox = i }

// SetFieldWriter replaces how AI steps store contact fields (the contacts
// module's provenance store, once it exists).
func (s *Service) SetFieldWriter(f FieldWriter) { s.fields = f }

// SetClock replaces the clock of the scanner (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Evaluate starts a run for every active automation the event matches. A
// redelivered event finds its runs already present and starts nothing.
func (s *Service) Evaluate(ctx context.Context, e trigger.Event) error {
	if licensegate.Check(ctx) != nil {
		return nil // restricted mode: automations do not start
	}
	autos, err := s.q.ActiveAutomationsFor(ctx, e.Type)
	if err != nil || len(autos) == 0 {
		return err
	}
	contact, err := s.q.GetContact(ctx, e.ContactID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, a := range autos {
		if _, err := s.start(ctx, a, e, contact); err != nil {
			return err
		}
	}
	return nil
}

// start starts a run of a for e when the trigger matches and neither a
// mute nor the cooldown holds it back; false when nothing started (also
// when the run for this event exists already).
func (s *Service) start(ctx context.Context, a db.Automation, e trigger.Event, contact db.Contact) (bool, error) {
	if contact.AutomationsMutedUntil != nil && contact.AutomationsMutedUntil.After(time.Now()) {
		return false, nil
	}
	var t Trigger
	_ = json.Unmarshal(a.Trigger, &t)
	if !MatchTrigger(t, e) {
		return false, nil
	}
	if t.CooldownSeconds > 0 {
		recent, err := s.q.RecentRunForContact(ctx, db.RecentRunForContactParams{AutomationID: a.ID, ContactID: &e.ContactID,
			WithinSeconds: float64(t.CooldownSeconds)})
		if err != nil || recent {
			return false, err
		}
	}
	raw, _ := json.Marshal(e)
	started := false
	err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		id, err := q.InsertAutomationRun(ctx, db.InsertAutomationRunParams{AutomationID: a.ID, AutomationVersion: a.Version,
			EventKey: e.Key, Event: raw, ContactID: &e.ContactID, ConversationID: e.ConversationID, StartedBy: "system"})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already ran for this event
		}
		if err != nil {
			return err
		}
		started = true
		if _, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeRun), map[string]string{"run_id": id.String()}, 0); err != nil {
			return err
		}
		return activity.AnnounceRun(ctx, tx, q, id, true)
	})
	return started, err
}

// stepResult is what one executed instruction did.
type stepResult struct {
	outcome string // ok | skipped | failed | waiting
	detail  map[string]any
	next    int
	wait    time.Duration
	err     error // stops the run (or retries the step, see RetryPolicy)
	// vars are run variables the step produced.
	vars map[string]string
	// apply finishes an external step inside the transaction that records
	// it (sends, contact changes, handoffs); its result replaces this one.
	apply func(ctx context.Context, tx pgx.Tx, q *db.Queries) stepResult
}

// runCtx is what a step sees of its run.
type runCtx struct {
	id      uuid.UUID
	pc      int
	attempt int
	ev      trigger.Event
	contact db.Contact
	vars    map[string]string
}

// textVars are the placeholder values of a run: {{message}} and
// {{var:<name>}}.
func (rc runCtx) textVars() map[string]string {
	v := map[string]string{"message": rc.ev.Body}
	for k, x := range rc.vars {
		v["var:"+k] = x
	}
	return v
}

func (rc runCtx) fill(s string) string { return campaigns.FillText(s, rc.contact, rc.textVars()) }

// external reports whether an instruction runs outside a transaction
// (network I/O).
func external(in Instr) bool {
	if in.Op != "action" || in.Step == nil {
		return false
	}
	return in.Step.Type == "http_request" || in.Step.Type == "send_email" || IsAIStep(in.Step.Type)
}

// Run executes a run from its current instruction until it finishes, fails
// or waits. Each database action commits together with the advanced
// position, so a crash never repeats it; HTTP, e-mail and AI calls happen
// at least once, and their effects (sends included) exactly once.
func (s *Service) Run(ctx context.Context, id uuid.UUID) error {
	for range maxInstructionsPerRun {
		done, err := s.step(ctx, id)
		if err != nil || done {
			return err
		}
	}
	return s.q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "failed", Error: ptr("instruction limit reached")})
}

// step runs one instruction; done reports that the run stopped (finished,
// failed or waiting).
func (s *Service) step(ctx context.Context, id uuid.UUID) (done bool, err error) {
	var ext *Instr
	var rc runCtx
	var conv *uuid.UUID // the run's conversation: its progress is announced there
	exhausted := false
	announce := func(tx pgx.Tx, q *db.Queries, err error) error {
		if err != nil || conv == nil {
			return err
		}
		return activity.AnnounceRun(ctx, tx, q, id, false)
	}
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		return announce(tx, q, func() error {
			run, err := q.LockAutomationRun(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				done = true
				return nil
			}
			if err != nil {
				return err
			}
			switch run.Status {
			case "completed", "failed", "cancelled":
				done = true
				return nil
			case "waiting":
				if run.ResumeAt != nil && run.ResumeAt.After(time.Now().Add(time.Second)) {
					done = true // woken early; the resume task comes later
					return nil
				}
			}
			conv = run.ConversationID
			a, err := q.GetAutomation(ctx, run.AutomationID)
			if errors.Is(err, pgx.ErrNoRows) {
				done = true
				return nil
			}
			if err != nil {
				return err
			}
			if a.Version != run.AutomationVersion {
				done = true
				return q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "cancelled", Error: ptr("The automation was edited while this run was in progress.")})
			}
			var prog []Instr
			if err := json.Unmarshal(a.Program, &prog); err != nil {
				return err
			}
			if int(run.Pc) >= len(prog) {
				done = true
				return q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "completed"})
			}
			rc = runCtx{id: id, pc: int(run.Pc), attempt: int(run.Attempt), vars: map[string]string{}}
			_ = json.Unmarshal(run.Event, &rc.ev)
			_ = json.Unmarshal(run.Vars, &rc.vars)
			if run.ContactID == nil {
				done = true
				return q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "failed", Error: ptr("The contact was deleted.")})
			}
			if rc.contact, err = q.GetContact(ctx, *run.ContactID); err != nil {
				return err
			}
			in := prog[run.Pc]
			if in.StepID == "" && in.Op != "jump" {
				in.StepID = legacyStepID(a.Steps, int(run.Pc))
			}
			if external(in) {
				ext = &in
				return nil // runs outside the transaction (network I/O)
			}
			res := s.exec(ctx, tx, q, in, rc)
			return s.record(ctx, tx, q, rc, in, res, &done, &exhausted)
		}())
	})
	if err != nil || ext == nil {
		return done, s.exhausted(err, exhausted)
	}
	res := s.external(ctx, ext.Step, rc)
	res.next = rc.pc + 1
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		run, err := q.LockAutomationRun(ctx, id)
		if err != nil {
			return err
		}
		if int(run.Pc) != rc.pc || int(run.Attempt) != rc.attempt || (run.Status != "queued" && run.Status != "running" && run.Status != "waiting") {
			done = true // someone else advanced it meanwhile
			return nil
		}
		if res.apply != nil && res.err == nil {
			applied := res.apply(ctx, tx, q)
			applied.next = res.next
			if applied.detail == nil {
				applied.detail = map[string]any{}
			}
			for k, v := range res.detail {
				if _, ok := applied.detail[k]; !ok {
					applied.detail[k] = v
				}
			}
			if applied.vars == nil {
				applied.vars = res.vars
			}
			res = applied
		}
		return announce(tx, q, s.record(ctx, tx, q, rc, *ext, res, &done, &exhausted))
	})
	return done, s.exhausted(err, exhausted)
}

func (s *Service) exhausted(err error, exhausted bool) error {
	if err == nil && exhausted {
		return ErrRetriesExhausted
	}
	return err
}

// retryWait is the wait before retry number attempt+1: the policy's
// backoff, doubled for each earlier retry, at most an hour.
func retryWait(p RetryPolicy, attempt int) time.Duration {
	w := time.Duration(p.Backoff) * time.Second
	for range attempt {
		w *= 2
		if w >= time.Hour {
			return time.Hour
		}
	}
	return min(w, time.Hour)
}

func (s *Service) record(ctx context.Context, tx pgx.Tx, q *db.Queries, rc runCtx, in Instr, res stepResult, done, exhausted *bool) error {
	id, pc := rc.id, int32(rc.pc) //nolint:gosec // program size is bounded
	action := in.Op
	if in.Step != nil {
		action = in.Step.Type
	}
	if res.detail == nil {
		res.detail = map[string]any{}
	}
	if rc.attempt > 0 {
		res.detail["attempt"] = rc.attempt + 1
	}
	code := ""
	if res.err != nil {
		res.outcome = "failed"
		res.detail["error"] = res.err.Error()
		code = CodeStepFailed
		retry := true // network, provider and database errors
		var se *StepError
		if errors.As(res.err, &se) {
			code, retry = se.Code, se.Retryable
			res.detail["code"] = se.Code
		}
		if p := in.Step.retryPolicy(); retry && p != nil {
			if rc.attempt < p.Max {
				wait := retryWait(*p, rc.attempt)
				res.outcome = "retrying"
				res.detail["attempt"], res.detail["max_retries"], res.detail["retry_in_seconds"] = rc.attempt+1, p.Max, int(wait.Seconds())
				if err := s.logStep(ctx, q, id, pc, in, action, res); err != nil {
					return err
				}
				*done = true
				if err := q.RetryAutomationRun(ctx, db.RetryAutomationRunParams{ID: id, WaitSeconds: wait.Seconds()}); err != nil {
					return err
				}
				_, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeResume), map[string]string{"run_id": id.String()}, wait)
				return err
			}
			code, *exhausted = CodeRetriesExhausted, true
		}
	}
	if err := s.logStep(ctx, q, id, pc, in, action, res); err != nil {
		return err
	}
	if len(res.vars) > 0 {
		raw, _ := json.Marshal(res.vars)
		if err := q.MergeAutomationRunVars(ctx, db.MergeAutomationRunVarsParams{ID: id, Vars: raw}); err != nil {
			return err
		}
	}
	switch {
	case res.err != nil:
		*done = true
		msg := res.err.Error()
		if code == CodeRetriesExhausted {
			msg = fmt.Sprintf("Failed after %d retries: %s", in.Step.Retry.Max, msg)
		}
		return q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "failed", Error: ptr(truncate(msg, 500)), ErrorCode: &code})
	case res.outcome == "waiting":
		*done = true
		if err := q.WaitAutomationRun(ctx, db.WaitAutomationRunParams{ID: id, Pc: int32(res.next), WaitSeconds: res.wait.Seconds()}); err != nil { //nolint:gosec // program size is bounded
			return err
		}
		_, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeResume), map[string]string{"run_id": id.String()}, res.wait)
		return err
	default:
		return q.AdvanceAutomationRun(ctx, db.AdvanceAutomationRunParams{ID: id, Pc: int32(res.next)}) //nolint:gosec // bounded
	}
}

// logStep appends the step log entry and the run trace (visited[i] belongs
// to the i-th log entry; a retried step appears once per attempt).
func (s *Service) logStep(ctx context.Context, q *db.Queries, id uuid.UUID, pc int32, in Instr, action string, res stepResult) error {
	if in.Op == "jump" {
		return nil
	}
	dj, _ := json.Marshal(res.detail)
	if err := q.InsertAutomationRunStep(ctx, db.InsertAutomationRunStepParams{RunID: id, Pc: pc, Action: action, Outcome: res.outcome, Detail: dj}); err != nil {
		return err
	}
	if in.StepID != "" {
		return q.AppendAutomationRunVisited(ctx, db.AppendAutomationRunVisitedParams{ID: id, StepID: in.StepID})
	}
	return nil
}

// retryPolicy is the step's policy when it retries at all.
func (st *Step) retryPolicy() *RetryPolicy {
	if st == nil || st.Retry == nil || st.Retry.Max <= 0 || !retryable(st.Type) {
		return nil
	}
	return st.Retry
}

// sendOnce queues a message for step pc of a run at most once: a step that
// runs again (a retry, a recovered run) finds its message and sends
// nothing. Refusals (window closed, opted out …) skip the step.
func (s *Service) sendOnce(ctx context.Context, tx pgx.Tx, q *db.Queries, rc runCtx, in messaging.SendInput) stepResult {
	next := rc.pc + 1
	pc := int32(rc.pc) //nolint:gosec // bounded
	prev, err := q.GetAutomationSend(ctx, db.GetAutomationSendParams{RunID: rc.id, Pc: pc})
	if err == nil {
		return stepResult{outcome: "ok", next: next, detail: map[string]any{"message_id": prev, "deduplicated": true}}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return stepResult{err: err}
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return stepResult{err: err}
	}
	m, err := s.msg.QueueTx(ctx, sp, in)
	var ae *apperr.Error
	if errors.As(err, &ae) {
		_ = sp.Rollback(ctx)
		// Window closed, opted out, template not approved: record and
		// carry on with the next step.
		return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": ae.Code, "message": ae.Message}}
	}
	if err != nil {
		_ = sp.Rollback(ctx)
		return stepResult{err: fmt.Errorf("queue the message: %w", err)}
	}
	if err := sp.Commit(ctx); err != nil {
		return stepResult{err: err}
	}
	if err := q.InsertAutomationSend(ctx, db.InsertAutomationSendParams{RunID: rc.id, Pc: pc, MessageID: m.ID}); err != nil {
		return stepResult{err: err}
	}
	return stepResult{outcome: "ok", next: next, detail: map[string]any{"message_id": m.ID}}
}

// exec runs an instruction that only touches the database.
func (s *Service) exec(ctx context.Context, tx pgx.Tx, q *db.Queries, in Instr, rc runCtx) stepResult {
	next := rc.pc + 1
	ev, c := rc.ev, rc.contact
	switch in.Op {
	case "jump":
		return stepResult{outcome: "ok", next: in.Target}
	case "branch":
		ok, err := s.evalCondition(ctx, q, *in.Step.If, rc)
		if err != nil {
			return stepResult{err: err}
		}
		if !ok {
			next = in.Target
		}
		return stepResult{outcome: "ok", next: next, detail: map[string]any{"result": ok}}
	}
	st := in.Step
	res := stepResult{outcome: "ok", next: next}
	switch st.Type {
	case "reply", "send_template":
		in := messaging.SendInput{PhoneNumberID: ev.PhoneNumberID, To: c.PhoneE164, Source: "automation"}
		if st.Type == "reply" {
			in.Type, in.Text = "text", &whatsapp.Text{Body: rc.fill(st.Text)}
		} else {
			// A template whose header media or variables are not filled
			// would fail at Meta; skip it with the reason instead.
			sh, err := checkTemplateStep(ctx, q, ev.PhoneNumberID, *st.Template)
			if err != nil {
				return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "template_params", "message": err.Error()}}
			}
			tpl, missing := campaigns.Render(sh, st.Template.Params(), c, st.Template.Name, st.Template.Language)
			if missing != "" {
				return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "missing_" + missing}}
			}
			in.Type, in.Template = "template", &tpl
		}
		return s.sendOnce(ctx, tx, q, rc, in)
	case "add_tag":
		t, err := q.UpsertTag(ctx, db.UpsertTagParams{Name: strings.TrimSpace(st.Tag), Color: "gray"})
		if err == nil {
			err = q.AddContactTag(ctx, db.AddContactTagParams{ContactID: c.ID, TagID: t.ID})
		}
		res.err = err
	case "remove_tag":
		_, res.err = q.RemoveContactTagByName(ctx, db.RemoveContactTagByNameParams{ContactID: c.ID, Name: st.Tag})
	case "add_to_group":
		n, err := q.AddGroupMembers(ctx, db.AddGroupMembersParams{GroupID: *st.GroupID, ContactIds: []uuid.UUID{c.ID}})
		if err != nil {
			return stepResult{err: err}
		}
		res.detail = map[string]any{"added": n > 0}
	case "set_field":
		v := rc.fill(st.Value)
		p := db.UpdateContactParams{ID: c.ID}
		val := nullIfEmpty(v)
		switch st.Field {
		case "name":
			p.Name = &v
		case "email":
			p.SetEmail, p.Email = true, val
		case "var1":
			p.SetVar1, p.Var1 = true, val
		case "var2":
			p.SetVar2, p.Var2 = true, val
		case "var3":
			p.SetVar3, p.Var3 = true, val
		case "var4":
			p.SetVar4, p.Var4 = true, val
		case "var5":
			p.SetVar5, p.Var5 = true, val
		}
		_, res.err = q.UpdateContact(ctx, p)
	case "assign", "close_conversation":
		conv, err := s.conversation(ctx, q, ev, c)
		if err != nil {
			return stepResult{err: err}
		}
		if conv == nil {
			return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "no_conversation"}}
		}
		kind := "closed"
		if st.Type == "assign" {
			kind = "assigned"
			if st.AssigneeID == nil {
				kind = "unassigned"
			} else if _, err := q.GetAssignableUser(ctx, *st.AssigneeID); err != nil {
				return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "assignee_unavailable"}}
			}
			_, err = q.SetConversationAssignee(ctx, db.SetConversationAssigneeParams{ID: conv.ID, AssigneeID: st.AssigneeID})
		} else {
			_, err = q.SetConversationStatus(ctx, db.SetConversationStatusParams{ID: conv.ID, Status: "closed"})
		}
		if err == nil {
			_, err = q.InsertConversationEvent(ctx, db.InsertConversationEventParams{ConversationID: conv.ID, Kind: kind,
				AssigneeID: st.AssigneeID, ActorType: "system"})
		}
		if err == nil {
			ak, d := activity.Closed, activity.Data{}
			if kind != "closed" {
				ak, d = activity.AssigneeChanged, activity.Data{"assignee_id": st.AssigneeID}
			}
			d["by"] = "automation"
			_, err = activity.Record(ctx, tx, q, conv.ID, ak, activity.System, d)
		}
		res.err = err
	case "handoff_to_ai", "handoff_to_human", "add_internal_note":
		return s.inboxStep(ctx, tx, q, st, rc, nil)
	case "mute":
		res.err = q.MuteContactAutomations(ctx, db.MuteContactAutomationsParams{ID: c.ID, Seconds: float64(st.Seconds)})
	case "delay":
		return stepResult{outcome: "waiting", next: next, wait: time.Duration(st.Seconds) * time.Second,
			detail: map[string]any{"seconds": st.Seconds}}
	default:
		res.err = fmt.Errorf("unsupported step %q", st.Type)
	}
	return res
}

// inboxStep runs a step that changes the conversation through the
// conversations module: a handoff (sum is the AI summary of
// ai_route_to_agent) or an internal note.
func (s *Service) inboxStep(ctx context.Context, tx pgx.Tx, q *db.Queries, st *Step, rc runCtx, sum *Summary) stepResult {
	next := rc.pc + 1
	if s.inbox == nil {
		return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "inbox_unavailable"}}
	}
	conv, err := s.conversation(ctx, q, rc.ev, rc.contact)
	if err != nil {
		return stepResult{err: err}
	}
	if conv == nil {
		return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "no_conversation"}}
	}
	if st.Type == "add_internal_note" {
		body := strings.TrimSpace(rc.fill(st.Text))
		if body == "" {
			return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "empty_note"}}
		}
		id, err := s.inbox.AutomationNote(ctx, tx, q, conv.ID, truncateRunes(body, 4000), st.ForAI)
		if err != nil {
			return stepResult{err: err}
		}
		return stepResult{outcome: "ok", next: next, detail: map[string]any{"note_id": id}}
	}
	h := HandoffRequest{To: "human", AssigneeID: st.AssigneeID, Note: truncateRunes(strings.TrimSpace(rc.fill(st.Text)), 500), Summary: sum}
	if st.Type == "handoff_to_ai" {
		h.To, h.Mode = "ai", st.Mode
	}
	out, err := s.inbox.AutomationHandOff(ctx, tx, q, conv.ID, h)
	if err != nil {
		return stepResult{err: err}
	}
	if out.Skipped != "" {
		return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": out.Skipped}}
	}
	d := map[string]any{"to": h.To}
	if out.HandoffID != nil {
		d["handoff_id"] = *out.HandoffID
	}
	return stepResult{outcome: "ok", next: next, detail: d}
}

func (s *Service) conversation(ctx context.Context, q *db.Queries, ev trigger.Event, c db.Contact) (*db.Conversation, error) {
	if ev.ConversationID != nil {
		conv, err := q.GetConversation(ctx, *ev.ConversationID)
		if err == nil {
			return &conv, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if ev.PhoneNumberID == nil {
		return nil, nil
	}
	conv, err := q.GetConversationByThread(ctx, db.GetConversationByThreadParams{ContactID: c.ID, PhoneNumberID: *ev.PhoneNumberID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &conv, err
}

// external runs HTTP, e-mail and AI steps (at least once).
func (s *Service) external(ctx context.Context, st *Step, rc runCtx) stepResult {
	if IsAIStep(st.Type) {
		return s.aiStep(ctx, st, rc)
	}
	switch st.Type {
	case "send_email":
		if s.mail == nil || !s.mail.Enabled() {
			return stepResult{outcome: "skipped", detail: map[string]any{"reason": "email_not_configured"}}
		}
		if err := s.mail.Send(ctx, mailer.Message{To: rc.fill(st.To), Subject: rc.fill(st.Subject), Body: rc.fill(st.Body)}); err != nil {
			return stepResult{err: fmt.Errorf("e-mail: %w", err)}
		}
		return stepResult{outcome: "ok", detail: map[string]any{"to": rc.fill(st.To)}}
	case "http_request":
		if s.http == nil {
			return stepResult{outcome: "skipped", detail: map[string]any{"reason": "http_not_configured"}}
		}
		method := st.Method
		if method == "" {
			method = http.MethodPost
		}
		var body io.Reader
		if st.Body != "" {
			body = bytes.NewBufferString(rc.fill(st.Body))
		}
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(rctx, method, rc.fill(st.URL), body)
		if err != nil {
			return stepResult{err: &StepError{Code: CodeStepFailed, Message: err.Error()}}
		}
		req.Header.Set("User-Agent", "Messora-Automation/1")
		if st.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range st.Headers {
			req.Header.Set(k, rc.fill(v))
		}
		start := time.Now()
		resp, err := s.http.Do(req)
		if err != nil {
			if safehttp.IsBlocked(err) {
				return stepResult{err: &StepError{Code: CodeStepFailed, Message: "http request refused: the address is not public"}}
			}
			return stepResult{err: fmt.Errorf("http request: %w", err)}
		}
		detail := map[string]any{"status": resp.StatusCode, "duration_ms": time.Since(start).Milliseconds()}
		if resp.StatusCode >= 400 {
			// Client errors (except 408 and 429) do not get better by
			// trying again.
			retry := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout
			return stepResult{err: &StepError{Code: CodeStepFailed, Message: fmt.Sprintf("http request answered %d", resp.StatusCode), Retryable: retry}, detail: detail}
		}
		return stepResult{outcome: "ok", detail: detail}
	}
	return stepResult{err: fmt.Errorf("unsupported step %q", st.Type)}
}

// evalCondition evaluates a branch condition against the event, the
// contact and the run variables.
func (s *Service) evalCondition(ctx context.Context, q *db.Queries, cnd Condition, rc runCtx) (bool, error) {
	ev, c := rc.ev, rc.contact
	var v string
	switch cnd.Field {
	case "message.body":
		v = ev.Body
	case "message.type":
		v = ev.MessageType
	case "event.tag":
		v = ev.Tag
	case "event.status":
		v = ev.Status
	case "event.subscription":
		v = ev.Subscription
	case "event.reaction":
		v = ev.Reaction
	case "event.reason":
		v = ev.Reason
	case "contact.name":
		v = c.Name
	case "contact.phone":
		v = c.PhoneE164
	case "contact.email":
		v = deref(c.Email)
	case "contact.var1":
		v = deref(c.Var1)
	case "contact.var2":
		v = deref(c.Var2)
	case "contact.var3":
		v = deref(c.Var3)
	case "contact.var4":
		v = deref(c.Var4)
	case "contact.var5":
		v = deref(c.Var5)
	case "contact.opted_out":
		return boolOp(cnd.Operator, c.OptedOutAt != nil), nil
	case "conversation.window_open":
		open := false
		if conv, err := s.conversation(ctx, q, ev, c); err != nil {
			return false, err
		} else if conv != nil {
			open = messaging.WindowOpen(conv.LastInboundAt, time.Now())
		}
		return boolOp(cnd.Operator, open), nil
	case "contact.tag":
		has, err := q.ContactHasTag(ctx, db.ContactHasTagParams{ContactID: c.ID, Name: cnd.Value})
		if err != nil {
			return false, err
		}
		switch cnd.Operator {
		case "equals", "contains":
			return has, nil
		case "not_equals", "not_contains":
			return !has, nil
		}
		return false, nil
	default:
		if name, ok := strings.CutPrefix(cnd.Field, "var."); ok {
			v = rc.vars[name]
		}
	}
	isSet := strings.TrimSpace(v) != ""
	lv, lw := strings.ToLower(v), strings.ToLower(cnd.Value)
	switch cnd.Operator {
	case "contains":
		return strings.Contains(lv, lw), nil
	case "not_contains":
		return !strings.Contains(lv, lw), nil
	case "equals":
		return lv == lw, nil
	case "not_equals":
		return lv != lw, nil
	case "starts_with":
		return strings.HasPrefix(lv, lw), nil
	case "is_set":
		return isSet, nil
	case "is_not_set":
		return !isSet, nil
	case "is_true":
		return isSet && lv != "false" && lv != "0", nil
	case "is_false":
		return !isSet || lv == "false" || lv == "0", nil
	}
	return false, nil
}

func boolOp(op string, v bool) bool {
	if op == "is_false" || op == "is_not_set" {
		return !v
	}
	return v
}

// Reap restarts runs whose task was lost (waiting past their time, or stuck
// running after a crash).
func (s *Service) Reap(ctx context.Context) (int, error) {
	var ids []uuid.UUID
	w, err := s.q.OverdueWaitingRuns(ctx)
	if err != nil {
		return 0, err
	}
	r, err := s.q.StaleRunningRuns(ctx)
	if err != nil {
		return 0, err
	}
	ids = append(append(ids, w...), r...)
	err = s.tx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		for _, id := range ids {
			if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(TypeRun), map[string]string{"run_id": id.String()}, 0); err != nil {
				return err
			}
		}
		return nil
	})
	return len(ids), err
}

func (s *Service) tx(ctx context.Context, fn func(pgx.Tx, *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// legacyStepID names the step of instruction pc of a program compiled
// before step ids existed: compiling the stored steps with their derived
// ids (LegacyIDs, as the API shows them) yields the same instructions.
func legacyStepID(stepsJSON []byte, pc int) string {
	var st []Step
	if err := json.Unmarshal(stepsJSON, &st); err != nil {
		return ""
	}
	LegacyIDs(st)
	prog, err := Compile(st)
	if err != nil || pc >= len(prog) {
		return ""
	}
	return prog[pc].StepID
}

func ptr[T any](v T) *T { return &v }

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// checkTemplateStep verifies a template step's values against the template
// on the sender's account. Without a known sender (the default number is
// used) or template, Meta has the last word.
func checkTemplateStep(ctx context.Context, q *db.Queries, phoneNumberID *uuid.UUID, ref TemplateRef) (campaigns.Shape, error) {
	if phoneNumberID == nil {
		return campaigns.Shape{}, nil
	}
	pn, err := q.GetPhoneNumber(ctx, *phoneNumberID)
	if err != nil {
		return campaigns.Shape{}, nil //nolint:nilerr // unknown sender: the send itself reports it
	}
	t, err := q.FindApprovedTemplate(ctx, db.FindApprovedTemplateParams{AccountID: pn.AccountID, Name: ref.Name, Language: ref.Language})
	if err != nil {
		return campaigns.Shape{}, nil //nolint:nilerr // unknown template: the send itself reports it
	}
	sh := campaigns.ShapeOf(t.Components, t.ParameterFormat, t.Category)
	return sh, campaigns.CheckParams(sh, ref.Params())
}
