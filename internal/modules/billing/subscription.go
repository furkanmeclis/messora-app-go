package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// Cycles.
const (
	CycleMonthly = "monthly"
	CycleYearly  = "yearly"
)

// istanbul is the time zone of dates printed on documents.
var istanbul = func() *time.Location {
	l, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return time.FixedZone("TRT", 3*3600)
	}
	return l
}()

// addMonths adds n calendar months, keeping the day or, when the month is
// shorter, its last day (31 January + 1 month = 28/29 February).
func addMonths(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d, last)-1)
}

// AddCycle is the end of a period of cycle starting at t.
func AddCycle(t time.Time, cycle string) time.Time {
	if cycle == CycleYearly {
		return addMonths(t, 12)
	}
	return addMonths(t, 1)
}

// numericKurus converts a numeric(12,2) list price into kuruş.
func numericKurus(n pgtype.Numeric) (int64, bool) {
	if !n.Valid {
		return 0, false
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return 0, false
	}
	k, err := ParseKurus(fmt.Sprint(v))
	return k, err == nil
}

// PlanPrice is a plan's price for one period of cycle, VAT excluded: the
// yearly list price, else ten monthly prices (12 months for 10); custom
// (the company's own price) wins.
func PlanPrice(p db.Plan, cycle string, custom *int64) (int64, bool) {
	if custom != nil {
		return *custom, true
	}
	monthly, ok := numericKurus(p.PriceMonthly)
	if cycle == CycleYearly {
		if y, ok := numericKurus(p.PriceYearly); ok {
			return y, true
		}
		if !ok {
			return 0, false
		}
		return monthly * 10, true
	}
	return monthly, ok
}

// monthlyValue is the monthly recurring value of a period price.
func monthlyValue(price int64, cycle string) int64 {
	if cycle == CycleYearly {
		return roundHalfUp(ratOf(price, 12))
	}
	return price
}

// ensure returns the tenant's subscription row, locked, creating it with a
// fresh company code the first time.
func (s *Service) ensure(ctx context.Context, q *db.Queries) (db.Subscription, error) {
	for range 8 {
		sub, err := q.GetSubscriptionForUpdate(ctx)
		if err == nil {
			return sub, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return sub, err
		}
		code, err := NewCode()
		if err != nil {
			return sub, err
		}
		// A taken code inserts nothing: try another one.
		if _, err := q.InsertSubscription(ctx, code); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return sub, err
		}
	}
	return db.Subscription{}, errors.New("billing: could not assign a company code")
}

// Ensure creates the context tenant's subscription row if it is missing
// (a platform transaction; the overview and the job call it).
func (s *Service) Ensure(ctx context.Context) (db.Subscription, error) {
	var sub db.Subscription
	err := s.tenantTx(ctx, tenant.ID(ctx), func(ctx context.Context, q *db.Queries, _ *effects) error {
		var err error
		sub, err = s.ensure(ctx, q)
		return err
	})
	return sub, err
}

func subParams(sub db.Subscription) db.UpdateSubscriptionParams {
	return db.UpdateSubscriptionParams{BillingCycle: sub.BillingCycle, PeriodStart: sub.PeriodStart, PeriodEnd: sub.PeriodEnd,
		PendingPlanKey: sub.PendingPlanKey, PendingCycle: sub.PendingCycle, CustomPriceKurus: sub.CustomPriceKurus,
		GraceEndsAt: sub.GraceEndsAt, ReadOnlySince: sub.ReadOnlySince, StatusReason: sub.StatusReason,
		AccountCreditKurus: sub.AccountCreditKurus}
}

func (s *Service) save(ctx context.Context, q *db.Queries, sub *db.Subscription) error {
	out, err := q.UpdateSubscription(ctx, subParams(*sub))
	if err != nil {
		return err
	}
	*sub = out
	return nil
}

// setStatus changes the tenant's subscription status (the tenant registry,
// platform role) and records the change in both audit logs.
func (s *Service) setStatus(ctx context.Context, q *db.Queries, id uuid.UUID, from, to, reason string, fx *effects) error {
	if from == to {
		return nil
	}
	if _, err := q.SetTenantSubscriptionStatus(ctx, db.SetTenantSubscriptionStatusParams{ID: id, SubscriptionStatus: to}); err != nil {
		return err
	}
	fx.invalidate = append(fx.invalidate, id)
	ch := map[string]any{"subscription_status": []string{from, to}, "reason": reason}
	if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.status_change", EntityType: "subscription", EntityID: &id, Changes: ch}); err != nil {
		return err
	}
	return audit.Record(ctx, q, audit.Entry{Action: "tenant.subscription_status", EntityType: "tenant", EntityID: &id, Tenant: &id, Changes: ch})
}

func actorID(p *principal.Principal) *uuid.UUID {
	if p == nil || p.Support != nil {
		return nil
	}
	id := p.ID
	return &id
}

// planOf reads the tenant's plan row (nil without a plan).
func planOf(ctx context.Context, q *db.Queries, key *string) (*db.Plan, error) {
	if key == nil {
		return nil, nil
	}
	p, err := q.GetPlan(tenant.System(ctx), *key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ── Plan changes (self-serve) ──────────────────────────────────────────────

// Plan change effects.
const (
	EffectProforma  = "proforma"
	EffectScheduled = "scheduled"
)

// ChangeResult is what a plan change did.
type ChangeResult struct {
	Effect   string
	Proforma *db.Proforma
}

// ChangePlan applies a workspace's choice of plan and cycle
// (docs/BILLING.md "Plan changes"):
//
//   - without a paid period (trial, read-only, canceled, or a company that
//     never had a period) a proforma for one full period of the plan; the
//     period starts when it is paid;
//   - during a paid period a higher plan in the same cycle: a prorated
//     upgrade proforma (the difference × the days left ÷ the period's
//     days); the plan changes when it is paid;
//   - a lower plan or another cycle: scheduled for the next renewal;
//   - inside the renewal window (an unpaid renewal is open) every change is
//     scheduled and the renewal is re-issued with the new terms.
func (s *Service) ChangePlan(ctx context.Context, actor *principal.Principal, planKey, cycle string) (ChangeResult, error) {
	id := tenant.ID(ctx)
	if cycle != CycleMonthly && cycle != CycleYearly {
		return ChangeResult{}, apperr.Validation("billing_cycle", "Choose monthly or yearly.")
	}
	var out ChangeResult
	err := s.tenantTx(ctx, id, func(ctx context.Context, q *db.Queries, fx *effects) error {
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		t, err := q.GetTenantForUpdate(ctx, id)
		if err != nil {
			return err
		}
		want, err := q.GetPlan(ctx, planKey)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!want.Active || !want.Public)) {
			return apperr.Validation("plan_key", "This plan cannot be chosen.")
		}
		if err != nil {
			return err
		}
		if want.CustomPricing {
			return apperr.Conflict("PLAN_NOT_SELF_SERVE", "This plan is priced per offer; ask Messora for an offer.")
		}
		price, ok := PlanPrice(want, cycle, nil)
		if !ok || price <= 0 {
			return apperr.Conflict("PLAN_NOT_SELF_SERVE", "This plan has no list price; ask Messora for an offer.")
		}
		now := s.now()
		paid := sub.PeriodEnd != nil && now.Before(*sub.PeriodEnd) && (t.SubscriptionStatus == StatusActive || t.SubscriptionStatus == StatusPastDue)
		if !paid {
			if err := s.replaceOpenPlanProforma(ctx, q, "replaced by a new plan choice"); err != nil {
				return err
			}
			f, err := s.issue(ctx, q, &sub, fx, draft{kind: KindSubscription, planKey: &want.Key, cycle: &cycle,
				lines: []line{planLine(want, cycle, price, nil, nil)}, net: price, createdBy: actorID(actor)})
			if err != nil {
				return err
			}
			out = ChangeResult{Effect: EffectProforma, Proforma: &f}
			return s.auditChange(ctx, q, id, "billing.plan_choice", map[string]any{"plan": want.Key, "billing_cycle": cycle, "proforma": f.Number})
		}
		if t.SubscriptionStatus == StatusPastDue {
			return apperr.Conflict("RENEWAL_DUE", "Pay the open renewal first; the plan can be changed afterwards.")
		}
		cur, err := planOf(ctx, q, t.PlanKey)
		if err != nil {
			return err
		}
		renewal, err := q.GetRenewalProforma(ctx, sub.PeriodEnd)
		openRenewal := err == nil && (renewal.Status == "open" || renewal.Status == "partially_paid")
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		curPrice := int64(0)
		if cur != nil {
			curPrice, _ = PlanPrice(*cur, sub.BillingCycle, sub.CustomPriceKurus)
		}
		samePlan := cur != nil && cur.Key == want.Key
		if samePlan && cycle == sub.BillingCycle {
			if sub.PendingPlanKey == nil && sub.PendingCycle == nil {
				return apperr.Validation("plan_key", "The workspace is on this plan and cycle already.")
			}
		}
		newSameCycle, _ := PlanPrice(want, sub.BillingCycle, nil)
		if !openRenewal && !samePlan && cycle == sub.BillingCycle && newSameCycle > curPrice {
			if err := s.replaceOpenPlanProforma(ctx, q, "replaced by a new upgrade"); err != nil {
				return err
			}
			net := Prorate(newSameCycle-curPrice, now, *sub.PeriodStart, *sub.PeriodEnd)
			if net <= 0 {
				return apperr.Conflict("PERIOD_ENDING", "The period ends today; choose the plan for the next period.")
			}
			_, pricing, err := s.settings(ctx)
			if err != nil {
				return err
			}
			due := now.AddDate(0, 0, pricing.API.DueDays)
			if due.After(*sub.PeriodEnd) {
				due = *sub.PeriodEnd
			}
			start, end := now, *sub.PeriodEnd
			f, err := s.issue(ctx, q, &sub, fx, draft{kind: KindUpgrade, planKey: &want.Key, cycle: &sub.BillingCycle,
				periodStart: &start, periodEnd: &end, due: &due, net: net, createdBy: actorID(actor),
				lines: []line{upgradeLine(cur, want, sub.BillingCycle, net, start, end)}})
			if err != nil {
				return err
			}
			out = ChangeResult{Effect: EffectProforma, Proforma: &f}
			return s.auditChange(ctx, q, id, "billing.upgrade", map[string]any{"plan": []any{t.PlanKey, want.Key}, "proforma": f.Number,
				"prorated": FormatKurus(net)})
		}
		// Scheduled for the next renewal.
		sub.PendingPlanKey, sub.PendingCycle = nil, nil
		if !samePlan {
			sub.PendingPlanKey = &want.Key
		}
		if cycle != sub.BillingCycle {
			sub.PendingCycle = &cycle
		}
		if err := s.save(ctx, q, &sub); err != nil {
			return err
		}
		if openRenewal {
			if err := s.reissueRenewal(ctx, q, &sub, t, renewal, fx); err != nil {
				return err
			}
		}
		out = ChangeResult{Effect: EffectScheduled}
		return s.auditChange(ctx, q, id, "billing.change_scheduled", map[string]any{"pending_plan": sub.PendingPlanKey, "pending_cycle": sub.PendingCycle})
	})
	return out, err
}

func (s *Service) auditChange(ctx context.Context, q *db.Queries, id uuid.UUID, action string, ch map[string]any) error {
	return audit.RecordInTenant(ctx, q, audit.Entry{Action: action, EntityType: "subscription", EntityID: &id, Changes: ch})
}

// CancelChange drops a scheduled plan or cycle change.
func (s *Service) CancelChange(ctx context.Context) error {
	id := tenant.ID(ctx)
	return s.tenantTx(ctx, id, func(ctx context.Context, q *db.Queries, fx *effects) error {
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		if sub.PendingPlanKey == nil && sub.PendingCycle == nil {
			return nil
		}
		sub.PendingPlanKey, sub.PendingCycle = nil, nil
		if err := s.save(ctx, q, &sub); err != nil {
			return err
		}
		if sub.PeriodEnd != nil {
			if r, err := q.GetRenewalProforma(ctx, sub.PeriodEnd); err == nil && r.Status == "open" {
				t, err := q.GetTenantForUpdate(ctx, id)
				if err != nil {
					return err
				}
				if err := s.reissueRenewal(ctx, q, &sub, t, r, fx); err != nil {
					return err
				}
			}
		}
		return s.auditChange(ctx, q, id, "billing.change_cancel", nil)
	})
}

// replaceOpenPlanProforma voids the open plan proforma (nothing paid yet)
// before another is issued; one with money on it stops the change.
func (s *Service) replaceOpenPlanProforma(ctx context.Context, q *db.Queries, reason string) error {
	f, err := q.GetOpenPlanProforma(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if f.PaidKurus > 0 || f.Renewal {
		return apperr.Conflict("PROFORMA_OPEN", fmt.Sprintf("Proforma %s is open with a payment on it; pay it in full first.", f.Number))
	}
	_, err = q.VoidProforma(ctx, db.VoidProformaParams{ID: f.ID, Reason: reason})
	return err
}

// reissueRenewal replaces an unpaid renewal with one on the current terms
// (plan and cycle with the scheduled change).
func (s *Service) reissueRenewal(ctx context.Context, q *db.Queries, sub *db.Subscription, t db.Tenant, old db.Proforma, fx *effects) error {
	if old.PaidKurus > 0 {
		return apperr.Conflict("PROFORMA_OPEN", fmt.Sprintf("Renewal %s has a payment on it; the change applies from the next renewal.", old.Number))
	}
	if _, err := q.VoidProforma(ctx, db.VoidProformaParams{ID: old.ID, Reason: "replaced: the plan or cycle changed"}); err != nil {
		return err
	}
	_, err := s.issueRenewal(ctx, q, sub, t, fx)
	return err
}

// renewalTerms is the plan, cycle and price of the next period.
func (s *Service) renewalTerms(ctx context.Context, q *db.Queries, sub db.Subscription, t db.Tenant) (db.Plan, string, int64, bool, error) {
	key := t.PlanKey
	if sub.PendingPlanKey != nil {
		key = sub.PendingPlanKey
	}
	cycle := sub.BillingCycle
	if sub.PendingCycle != nil {
		cycle = *sub.PendingCycle
	}
	p, err := planOf(ctx, q, key)
	if err != nil || p == nil {
		return db.Plan{}, cycle, 0, false, err
	}
	custom := sub.CustomPriceKurus
	if sub.PendingPlanKey != nil && (t.PlanKey == nil || *sub.PendingPlanKey != *t.PlanKey) {
		custom = nil // a company price belongs to its plan
	}
	price, ok := PlanPrice(*p, cycle, custom)
	return *p, cycle, price, ok && price > 0, nil
}

// issueRenewal issues the proforma of the period after the current one,
// due when the current period ends.
func (s *Service) issueRenewal(ctx context.Context, q *db.Queries, sub *db.Subscription, t db.Tenant, fx *effects) (db.Proforma, error) {
	p, cycle, price, ok, err := s.renewalTerms(ctx, q, *sub, t)
	if err != nil {
		return db.Proforma{}, err
	}
	if !ok {
		return db.Proforma{}, errNoPrice
	}
	start := *sub.PeriodEnd
	end := AddCycle(start, cycle)
	due := start
	// An unpaid upgrade gives way to the renewal (one open plan proforma).
	if f, err := q.GetOpenPlanProforma(ctx); err == nil && f.Kind == KindUpgrade && f.PaidKurus == 0 {
		if _, err := q.VoidProforma(ctx, db.VoidProformaParams{ID: f.ID, Reason: "replaced by the renewal"}); err != nil {
			return db.Proforma{}, err
		}
	}
	return s.issue(ctx, q, sub, fx, draft{kind: KindSubscription, renewal: true, planKey: &p.Key, cycle: &cycle,
		periodStart: &start, periodEnd: &end, due: &due, net: price, lines: []line{planLine(p, cycle, price, &start, &end)}})
}

var errNoPrice = errors.New("billing: the plan has no price for this company")

// ── Fulfilment ─────────────────────────────────────────────────────────────

// fulfil delivers a paid proforma: the period, the plan, the credits or the
// balance; then the invoice record and the confirmation e-mail.
func (s *Service) fulfil(ctx context.Context, q *db.Queries, sub *db.Subscription, f db.Proforma, fx *effects) error {
	if f.FulfilledAt != nil || f.Status != "paid" {
		return nil
	}
	id := f.TenantID
	now := s.now()
	switch f.Kind {
	case KindSubscription:
		t, err := q.GetTenantForUpdate(ctx, id)
		if err != nil {
			return err
		}
		cycle := deref(f.BillingCycle, CycleMonthly)
		var start, end time.Time
		if f.Renewal && f.PeriodStart != nil && f.PeriodEnd != nil && t.SubscriptionStatus != StatusReadOnly && t.SubscriptionStatus != StatusCanceled &&
			sub.PeriodEnd != nil && sub.PeriodEnd.Equal(*f.PeriodStart) {
			// A renewal paid before the grace ended continues the periods.
			start, end = *f.PeriodStart, *f.PeriodEnd
		} else {
			// A first purchase, or a lapsed company coming back: the period
			// starts now (a trial's remaining days are kept).
			start = now
			base := now
			if t.SubscriptionStatus == StatusTrial && t.TrialEndsAt != nil && t.TrialEndsAt.After(now) {
				base = *t.TrialEndsAt
			}
			end = AddCycle(base, cycle)
		}
		sub.PeriodStart, sub.PeriodEnd, sub.BillingCycle = &start, &end, cycle
		sub.PendingPlanKey, sub.PendingCycle, sub.GraceEndsAt, sub.ReadOnlySince, sub.StatusReason = nil, nil, nil, nil, ""
		if f.PlanKey != nil && (t.PlanKey == nil || *t.PlanKey != *f.PlanKey) {
			if sub.CustomPriceKurus != nil {
				sub.CustomPriceKurus = nil
			}
			if _, err := q.SetTenantPlanKey(ctx, db.SetTenantPlanKeyParams{ID: id, PlanKey: f.PlanKey}); err != nil {
				return err
			}
		}
		if err := s.save(ctx, q, sub); err != nil {
			return err
		}
		if err := s.setStatus(ctx, q, id, t.SubscriptionStatus, StatusActive, "payment "+f.Number, fx); err != nil {
			return err
		}
		fx.invalidate = append(fx.invalidate, id)
	case KindUpgrade:
		if f.PlanKey != nil {
			if _, err := q.SetTenantPlanKey(ctx, db.SetTenantPlanKeyParams{ID: id, PlanKey: f.PlanKey}); err != nil {
				return err
			}
			sub.CustomPriceKurus, sub.PendingPlanKey = nil, nil
			if err := s.save(ctx, q, sub); err != nil {
				return err
			}
			fx.invalidate = append(fx.invalidate, id)
		}
	case KindAICredits:
		if s.o.Credits == nil || f.AiCredits == nil {
			return errors.New("billing: AI credits are not available")
		}
		_, pricing, err := s.settings(ctx)
		if err != nil {
			return err
		}
		exp := now.AddDate(0, pricing.API.AiCreditValidityMonths, 0)
		ref := f.Reference
		note := fmt.Sprintf("AI credit pack %s (proforma %s)", deref(f.PackKey, ""), f.Number)
		if _, err := s.o.Credits.Grant(ctx, q, creditsGrant(*f.AiCredits, exp, note, &ref)); err != nil {
			return err
		}
	case KindMessageBalance:
		if f.MessageAmountKurus == nil {
			return errors.New("billing: the top-up has no amount")
		}
		if err := s.topUp(ctx, q, *f.MessageAmountKurus*MicrosPerKurus, &f.ID, "Top-up "+f.Number, nil); err != nil {
			return err
		}
	}
	if err := q.MarkProformaFulfilled(ctx, f.ID); err != nil {
		return err
	}
	if err := s.invoice(ctx, q, f); err != nil {
		return err
	}
	ch := map[string]any{"proforma": f.Number, "kind": f.Kind, "total": FormatKurus(f.TotalKurus)}
	if err := audit.RecordInTenant(ctx, q, audit.Entry{Action: "billing.proforma_paid", EntityType: "proforma", EntityID: &f.ID, Changes: ch}); err != nil {
		return err
	}
	if err := audit.Record(ctx, q, audit.Entry{Action: "tenant.billing_paid", EntityType: "proforma", EntityID: &f.ID, Tenant: &id, Changes: ch}); err != nil {
		return err
	}
	fx.later(func(ctx context.Context) { s.mailPaid(ctx, id, f) })
	return nil
}

func deref(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

// ── The billing job ────────────────────────────────────────────────────────

// Reminder stages before the due date of a renewal.
const (
	Reminder7 = "reminder_7"
	Reminder3 = "reminder_3"
	Reminder0 = "reminder_0"
)

// TickResult is what one run of the job did (logs and tests).
type TickResult struct {
	Transitions []string
	Issued      []string
	Reminders   []string
}

// Tick runs the context tenant's billing state machine once
// (docs/BILLING.md "Timeline"). It is idempotent: the status checks and
// billing_events make a repeated or retried run do nothing again.
func (s *Service) Tick(ctx context.Context) (TickResult, error) {
	id, ok := tenant.FromCtx(ctx)
	if !ok {
		return TickResult{}, errors.New("billing: the job needs a tenant")
	}
	var res TickResult
	err := s.tenantTx(ctx, id, func(ctx context.Context, q *db.Queries, fx *effects) error {
		res = TickResult{}
		sub, err := s.ensure(ctx, q)
		if err != nil {
			return err
		}
		t, err := q.GetTenantForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if t.Status != "active" {
			return nil // suspended or deleted companies are the platform admin's
		}
		seller, pricing, err := s.settings(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		// Upgrades that were not paid in time lapse (their price was for
		// the days left when they were issued).
		stale, err := q.ListStaleUpgrades(ctx, now)
		if err != nil {
			return err
		}
		for _, f := range stale {
			if _, err := q.VoidProforma(ctx, db.VoidProformaParams{ID: f.ID, Reason: "expired: not paid by its due date"}); err != nil {
				return err
			}
		}
		switch t.SubscriptionStatus {
		case StatusTrial:
			if t.TrialEndsAt == nil || now.Before(*t.TrialEndsAt) {
				return nil
			}
			sub.ReadOnlySince, sub.StatusReason = &now, "trial_ended"
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
			if err := s.setStatus(ctx, q, id, t.SubscriptionStatus, StatusReadOnly, "trial ended", fx); err != nil {
				return err
			}
			res.Transitions = append(res.Transitions, "trial→read_only")
			if n, err := q.InsertBillingEvent(ctx, db.InsertBillingEventParams{Kind: "read_only", Key: "trial:" + t.TrialEndsAt.UTC().Format(time.RFC3339)}); err != nil {
				return err
			} else if n == 1 {
				fx.later(func(ctx context.Context) { s.mailSuspended(ctx, id, nil) })
			}
			return nil
		case StatusActive, StatusPastDue:
		default:
			return nil
		}
		if sub.PeriodEnd == nil {
			return nil // a company without a period is not renewed automatically
		}
		end := *sub.PeriodEnd
		periodKey := end.UTC().Format(time.RFC3339)
		// Issue the renewal once per period, renewal_lead_days before its
		// end (a voided renewal is not issued again).
		renewal, err := q.GetRenewalProforma(ctx, &end)
		hasRenewal := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !hasRenewal && t.SubscriptionStatus == StatusActive && !now.Before(end.AddDate(0, 0, -pricing.API.RenewalLeadDays)) {
			_, _, _, priced, err := s.renewalTerms(ctx, q, sub, t)
			if err != nil {
				return err
			}
			_, profileErr := q.GetBillingProfile(ctx)
			switch {
			case !Configured(seller):
				s.o.Logger.WarnContext(ctx, "renewal not issued: set the seller and a bank account in the platform panel", "tenant_id", id)
			case !priced:
				s.o.Logger.WarnContext(ctx, "renewal not issued: the plan has no price for this company", "tenant_id", id)
			case profileErr != nil:
				// Without invoice details no proforma can be issued; the
				// billing page asks for them.
			default:
				n, err := q.InsertBillingEvent(ctx, db.InsertBillingEventParams{Kind: "renewal", Key: periodKey})
				if err != nil {
					return err
				}
				if n == 1 {
					renewal, err = s.issueRenewal(ctx, q, &sub, t, fx)
					if err != nil {
						return err
					}
					hasRenewal = true
					res.Issued = append(res.Issued, renewal.Number)
				}
			}
		}
		open := hasRenewal && (renewal.Status == "open" || renewal.Status == "partially_paid")
		// Reminders: 7 and 3 days before the due date and on the due day,
		// each once; a late first run sends only the latest stage.
		if open && t.SubscriptionStatus == StatusActive {
			stage := ""
			switch left := renewal.DueAt.Sub(now); {
			case left <= 24*time.Hour:
				stage = Reminder0
			case left <= 3*24*time.Hour:
				stage = Reminder3
			case left <= 7*24*time.Hour:
				stage = Reminder7
			}
			if stage != "" {
				n, err := q.InsertBillingEvent(ctx, db.InsertBillingEventParams{Kind: stage, Key: renewal.ID.String(), ProformaID: &renewal.ID})
				if err != nil {
					return err
				}
				if n == 1 {
					r := renewal
					fx.later(func(ctx context.Context) { s.mailReminder(ctx, id, r, false, time.Time{}) })
					res.Reminders = append(res.Reminders, stage)
				}
			}
		}
		// The period ended unpaid: past due with grace, then read-only.
		if t.SubscriptionStatus == StatusActive && !now.Before(end) {
			grace := end.AddDate(0, 0, pricing.API.GraceDays)
			sub.GraceEndsAt, sub.StatusReason = &grace, "renewal_unpaid"
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
			if err := s.setStatus(ctx, q, id, StatusActive, StatusPastDue, "renewal unpaid", fx); err != nil {
				return err
			}
			t.SubscriptionStatus = StatusPastDue
			res.Transitions = append(res.Transitions, "active→past_due")
			if n, err := q.InsertBillingEvent(ctx, db.InsertBillingEventParams{Kind: "overdue", Key: periodKey}); err != nil {
				return err
			} else if n == 1 && open {
				r := renewal
				fx.later(func(ctx context.Context) { s.mailReminder(ctx, id, r, true, grace) })
			}
		}
		if t.SubscriptionStatus == StatusPastDue && sub.GraceEndsAt != nil && !now.Before(*sub.GraceEndsAt) {
			sub.ReadOnlySince, sub.StatusReason = &now, "grace_ended"
			if err := s.save(ctx, q, &sub); err != nil {
				return err
			}
			if err := s.setStatus(ctx, q, id, StatusPastDue, StatusReadOnly, "grace period ended", fx); err != nil {
				return err
			}
			res.Transitions = append(res.Transitions, "past_due→read_only")
			if n, err := q.InsertBillingEvent(ctx, db.InsertBillingEventParams{Kind: "read_only", Key: periodKey}); err != nil {
				return err
			} else if n == 1 {
				var r *db.Proforma
				if open {
					r = &renewal
				}
				fx.later(func(ctx context.Context) { s.mailSuspended(ctx, id, r) })
			}
		}
		return nil
	})
	return res, err
}
