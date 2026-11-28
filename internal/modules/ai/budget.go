package ai

import (
	"context"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
)

// Budget is the state of the monthly AI budget.
type Budget struct {
	// Limit is the monthly budget in USD ("" = none).
	Limit string
	// Spent is the USD cost recorded since MonthStart.
	Spent      string
	MonthStart time.Time
	Exceeded   bool
}

// MonthStart is the first instant of t's calendar month in UTC.
func MonthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// OverBudget reports whether spent reached limit (both decimal strings;
// an empty or invalid limit is no budget).
func OverBudget(limit, spent string) bool {
	if limit == "" {
		return false
	}
	l, err := ParseAmount(limit)
	if err != nil || l.Sign() <= 0 {
		return false
	}
	sp, ok := new(big.Rat).SetString(spent)
	if !ok {
		return false
	}
	return sp.Cmp(l) >= 0
}

// BudgetStatus reads the month's spend against the budget.
func (s *Service) BudgetStatus(ctx context.Context) (Budget, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return Budget{}, err
	}
	return s.budgetOf(ctx, s.q, st)
}

func (s *Service) budgetOf(ctx context.Context, q db.Querier, st Settings) (Budget, error) {
	b := Budget{Limit: st.MonthlyBudget, MonthStart: MonthStart(s.opts.Now())}
	spent, err := q.AICostSince(ctx, b.MonthStart)
	if err != nil {
		return b, err
	}
	b.Spent = trimZeros(spent)
	b.Exceeded = OverBudget(b.Limit, spent)
	return b, nil
}

// budgetNoticeKey marks the month whose exceeded budget admins were told
// about.
func budgetNoticeKey(month time.Time) string { return "ai_budget_notified." + month.Format("2006_01") }

// NotifyBudgetExceeded tells the people who manage AI (once per month)
// that the budget is used up and AI auto replies are paused, and writes
// the audit log. It runs in the caller's transaction.
func (s *Service) NotifyBudgetExceeded(ctx context.Context, tx pgx.Tx, q *db.Queries, b Budget) error {
	n, err := q.InsertSettingIfAbsent(ctx, db.InsertSettingIfAbsentParams{Key: budgetNoticeKey(b.MonthStart),
		Value: []byte(`{"limit":"` + b.Limit + `","spent":"` + b.Spent + `"}`)})
	if err != nil || n == 0 {
		return err
	}
	users, err := q.ListAIManagers(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		channels := make([]string, len(users))
		for i, u := range users {
			channels[i] = realtime.PersonalChannel(u)
		}
		if err := realtime.PublishAll(ctx, tx, channels, realtime.NewNotificationEvent("ai.budget_exceeded")); err != nil {
			return err
		}
	}
	return audit.Record(ctx, q, audit.Entry{Action: "ai.budget_exceeded", EntityType: "ai_settings",
		Changes: map[string]any{"limit": b.Limit, "spent": b.Spent, "month": b.MonthStart.Format("2006-01")}})
}

// creditsNoticeKey marks the month whose used-up AI credits admins were
// told about.
func creditsNoticeKey(month time.Time) string {
	return "ai_credits_notified." + month.Format("2006_01")
}

// NotifyCreditsExhausted tells the people who manage AI (once per month)
// that the company's AI credits are used up and AI with Messora's keys is
// paused, and writes the audit log. It runs in the caller's transaction.
func (s *Service) NotifyCreditsExhausted(ctx context.Context, tx pgx.Tx, q *db.Queries) error {
	month := MonthStart(s.opts.Now())
	n, err := q.InsertSettingIfAbsent(ctx, db.InsertSettingIfAbsentParams{Key: creditsNoticeKey(month), Value: []byte(`{}`)})
	if err != nil || n == 0 {
		return err
	}
	users, err := q.ListAIManagers(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		channels := make([]string, len(users))
		for i, u := range users {
			channels[i] = realtime.PersonalChannel(u)
		}
		if err := realtime.PublishAll(ctx, tx, channels, realtime.NewNotificationEvent("ai.credits_exhausted")); err != nil {
			return err
		}
	}
	return audit.Record(ctx, q, audit.Entry{Action: "ai.credits_exhausted", EntityType: "ai_settings",
		Changes: map[string]any{"month": month.Format("2006-01")}})
}
