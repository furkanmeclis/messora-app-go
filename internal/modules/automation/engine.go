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
)

// maxInstructionsPerRun bounds a run even though programs only jump forward.
const maxInstructionsPerRun = 500

// Doer sends HTTP requests to user-supplied URLs; production uses the
// SSRF-safe client from platform/safehttp.
type Doer interface {
	Do(*http.Request) (*safehttp.Response, error)
}

// Service is the automation engine.
type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	msg  *messaging.Service
	mail mailer.Sender
	http Doer
	log  *slog.Logger
}

// New returns the engine. httpClient may be nil, which disables HTTP steps.
func New(pool *pgxpool.Pool, msg *messaging.Service, mail mailer.Sender, httpClient Doer, log *slog.Logger) *Service {
	return &Service{pool: pool, q: db.New(pool), msg: msg, mail: mail, http: httpClient, log: log}
}

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
	if contact.AutomationsMutedUntil != nil && contact.AutomationsMutedUntil.After(time.Now()) {
		return nil
	}
	raw, _ := json.Marshal(e)
	for _, a := range autos {
		var t Trigger
		_ = json.Unmarshal(a.Trigger, &t)
		if !MatchTrigger(t, e) {
			continue
		}
		if t.CooldownSeconds > 0 {
			recent, err := s.q.RecentRunForContact(ctx, db.RecentRunForContactParams{AutomationID: a.ID, ContactID: &e.ContactID,
				WithinSeconds: float64(t.CooldownSeconds)})
			if err != nil {
				return err
			}
			if recent {
				continue
			}
		}
		err := s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
			id, err := q.InsertAutomationRun(ctx, db.InsertAutomationRunParams{AutomationID: a.ID, AutomationVersion: a.Version,
				EventKey: e.Key, Event: raw, ContactID: &e.ContactID})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // already ran for this event
			}
			if err != nil {
				return err
			}
			_, err = outbox.Write(ctx, tx, outbox.TaskTopic(TypeRun), map[string]string{"run_id": id.String()}, 0)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// stepResult is what one executed instruction did.
type stepResult struct {
	outcome string // ok | skipped | failed | waiting
	detail  map[string]any
	next    int
	wait    time.Duration
	err     error // stops the run
}

// Run executes a run from its current instruction until it finishes, fails
// or waits. Each database action commits together with the advanced
// position, so a crash never repeats it; HTTP and e-mail steps are
// delivered at least once.
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
	var external *Step
	var externalID string
	var ev trigger.Event
	var contact db.Contact
	var pc int32
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
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
		_ = json.Unmarshal(run.Event, &ev)
		if run.ContactID == nil {
			done = true
			return q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "failed", Error: ptr("The contact was deleted.")})
		}
		contact, err = q.GetContact(ctx, *run.ContactID)
		if err != nil {
			return err
		}
		pc = run.Pc
		in := prog[pc]
		if in.StepID == "" && in.Op != "jump" {
			in.StepID = legacyStepID(a.Steps, int(pc))
		}
		if in.Op == "action" && (in.Step.Type == "http_request" || in.Step.Type == "send_email") {
			external, externalID = in.Step, in.StepID
			return nil // runs outside the transaction (network I/O)
		}
		res := s.exec(ctx, tx, q, in, int(pc), ev, contact)
		return s.record(ctx, tx, q, id, pc, in, res, &done)
	})
	if err != nil || external == nil {
		return done, err
	}
	res := s.external(ctx, external, ev, contact)
	res.next = int(pc) + 1
	err = s.tx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		run, err := q.LockAutomationRun(ctx, id)
		if err != nil {
			return err
		}
		if run.Pc != pc || (run.Status != "queued" && run.Status != "running" && run.Status != "waiting") {
			done = true // someone else advanced it meanwhile
			return nil
		}
		return s.record(ctx, tx, q, id, pc, Instr{Op: "action", Step: external, StepID: externalID}, res, &done)
	})
	return done, err
}

func (s *Service) record(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, pc int32, in Instr, res stepResult, done *bool) error {
	action := in.Op
	if in.Step != nil {
		action = in.Step.Type
	}
	if res.detail == nil {
		res.detail = map[string]any{}
	}
	if res.err != nil {
		res.outcome = "failed"
		res.detail["error"] = res.err.Error()
	}
	if in.Op != "jump" {
		dj, _ := json.Marshal(res.detail)
		if err := q.InsertAutomationRunStep(ctx, db.InsertAutomationRunStepParams{RunID: id, Pc: pc, Action: action, Outcome: res.outcome, Detail: dj}); err != nil {
			return err
		}
		if in.StepID != "" {
			if err := q.AppendAutomationRunVisited(ctx, db.AppendAutomationRunVisitedParams{ID: id, StepID: in.StepID}); err != nil {
				return err
			}
		}
	}
	switch {
	case res.err != nil:
		*done = true
		return q.FinishAutomationRun(ctx, db.FinishAutomationRunParams{ID: id, Status: "failed", Error: ptr(truncate(res.err.Error(), 500))})
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

// exec runs an instruction that only touches the database.
func (s *Service) exec(ctx context.Context, tx pgx.Tx, q *db.Queries, in Instr, pc int, ev trigger.Event, c db.Contact) stepResult {
	next := pc + 1
	switch in.Op {
	case "jump":
		return stepResult{outcome: "ok", next: in.Target}
	case "branch":
		ok, err := s.evalCondition(ctx, q, *in.Step.If, ev, c)
		if err != nil {
			return stepResult{err: err}
		}
		if !ok {
			next = in.Target
		}
		return stepResult{outcome: "ok", next: next, detail: map[string]any{"result": ok}}
	}
	st := in.Step
	vars := map[string]string{"message": ev.Body}
	text := func(v string) string { return campaigns.FillText(v, c, vars) }
	res := stepResult{outcome: "ok", next: next}
	switch st.Type {
	case "reply", "send_template":
		in := messaging.SendInput{PhoneNumberID: ev.PhoneNumberID, To: c.PhoneE164, Source: "automation"}
		if st.Type == "reply" {
			in.Type, in.Text = "text", &whatsapp.Text{Body: text(st.Text)}
		} else {
			tpl, missing := campaigns.Render(campaigns.Params{Header: st.Template.Header, Body: st.Template.Body}, c, st.Template.Name, st.Template.Language)
			if missing != "" {
				return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": "missing_" + missing}}
			}
			in.Type, in.Template = "template", &tpl
		}
		m, err := s.msg.QueueTx(ctx, tx, in)
		var ae *apperr.Error
		if errors.As(err, &ae) {
			// Window closed, opted out, template not approved: record and
			// carry on with the next step.
			return stepResult{outcome: "skipped", next: next, detail: map[string]any{"reason": ae.Code, "message": ae.Message}}
		}
		if err != nil {
			return stepResult{err: err}
		}
		res.detail = map[string]any{"message_id": m.ID}
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
		v := text(st.Value)
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
		res.err = err
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

// external runs HTTP and e-mail steps (at least once).
func (s *Service) external(ctx context.Context, st *Step, ev trigger.Event, c db.Contact) stepResult {
	vars := map[string]string{"message": ev.Body}
	text := func(v string) string { return campaigns.FillText(v, c, vars) }
	switch st.Type {
	case "send_email":
		if s.mail == nil || !s.mail.Enabled() {
			return stepResult{outcome: "skipped", detail: map[string]any{"reason": "email_not_configured"}}
		}
		if err := s.mail.Send(ctx, mailer.Message{To: text(st.To), Subject: text(st.Subject), Body: text(st.Body)}); err != nil {
			return stepResult{err: fmt.Errorf("e-mail: %w", err)}
		}
		return stepResult{outcome: "ok", detail: map[string]any{"to": text(st.To)}}
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
			body = bytes.NewBufferString(text(st.Body))
		}
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(rctx, method, text(st.URL), body)
		if err != nil {
			return stepResult{err: err}
		}
		req.Header.Set("User-Agent", "Messora-Automation/1")
		if st.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range st.Headers {
			req.Header.Set(k, text(v))
		}
		start := time.Now()
		resp, err := s.http.Do(req)
		if err != nil {
			if safehttp.IsBlocked(err) {
				return stepResult{err: fmt.Errorf("http request refused: the address is not public")}
			}
			return stepResult{err: fmt.Errorf("http request: %w", err)}
		}
		detail := map[string]any{"status": resp.StatusCode, "duration_ms": time.Since(start).Milliseconds()}
		if resp.StatusCode >= 400 {
			return stepResult{err: fmt.Errorf("http request answered %d", resp.StatusCode), detail: detail}
		}
		return stepResult{outcome: "ok", detail: detail}
	}
	return stepResult{err: fmt.Errorf("unsupported step %q", st.Type)}
}

// evalCondition evaluates a branch condition against the event and contact.
func (s *Service) evalCondition(ctx context.Context, q *db.Queries, cnd Condition, ev trigger.Event, c db.Contact) (bool, error) {
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
