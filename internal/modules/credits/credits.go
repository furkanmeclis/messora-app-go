// Package credits keeps a company's AI credits: the plan's monthly
// allowance, manual grants of the platform admin and (WP9) purchased
// top-ups, as lots in ai_credit_lots, and the conversion of an AI call's
// cost into credits (docs/AI.md "AI credits", docs/ENTITLEMENTS.md).
//
// Amounts are millicredits (1 credit = 1000) everywhere in code and in the
// database; the API shows credits with up to three decimals.
//
// AI calls made with the platform's keys are metered: the AI module asks
// Available before a call and Consume after it, in the transaction that
// records the usage row. Calls with a company's own keys (enterprise plans,
// byo_ai_keys) consume nothing.
package credits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Milli is the number of millicredits in one credit.
const Milli = 1000

// SettingKey is the platform setting with the conversion (Settings).
const SettingKey = "ai_credits"

// Settings converts a call's cost into credits:
//
//	credits = cost (USD, at the platform price table) × Markup ÷ USDPerCredit
//
// rounded up to a millicredit. A call whose model has no price costs
// UnpricedCallCredits when it succeeded (never free: the platform pays the
// provider), nothing when it failed without usage.
type Settings struct {
	USDPerCredit        string `json:"usd_per_credit"`
	Markup              string `json:"markup"`
	UnpricedCallCredits string `json:"unpriced_call_credits"`
}

// DefaultSettings is the built-in rate: 1 credit = 0.001 USD of provider
// cost, no markup, 5 credits for an unpriced call.
func DefaultSettings() Settings {
	return Settings{USDPerCredit: "0.001", Markup: "1", UnpricedCallCredits: "5"}
}

// Validate checks the three decimals (USDPerCredit and Markup above 0).
func (s Settings) Validate() error {
	for _, f := range []struct {
		name, v  string
		positive bool
	}{{"usd_per_credit", s.USDPerCredit, true}, {"markup", s.Markup, true}, {"unpriced_call_credits", s.UnpricedCallCredits, false}} {
		r, ok := new(big.Rat).SetString(strings.TrimSpace(f.v))
		if !ok || r.Sign() < 0 || (f.positive && r.Sign() == 0) || r.Cmp(big.NewRat(1_000_000_000, 1)) > 0 {
			return &FieldError{Field: f.name, Message: "Enter a decimal number above 0, e.g. 0.001."}
		}
	}
	return nil
}

func (s Settings) normalized() Settings {
	d := DefaultSettings()
	if s.USDPerCredit == "" {
		s.USDPerCredit = d.USDPerCredit
	}
	if s.Markup == "" {
		s.Markup = d.Markup
	}
	if s.UnpricedCallCredits == "" {
		s.UnpricedCallCredits = d.UnpricedCallCredits
	}
	if s.Validate() != nil {
		return d
	}
	return s
}

// FieldError is an invalid setting or grant.
type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

// Charge returns the millicredits of a call: cost is the USD cost at the
// platform's price ("" when the model has no price), ok whether the call
// succeeded.
func (s Settings) Charge(cost string, ok bool) int64 {
	s = s.normalized()
	if cost == "" {
		if !ok {
			return 0
		}
		return ceilMilli(rat(s.UnpricedCallCredits))
	}
	c, valid := new(big.Rat).SetString(cost)
	if !valid || c.Sign() <= 0 {
		return 0
	}
	c.Mul(c, rat(s.Markup))
	c.Quo(c, rat(s.USDPerCredit))
	return ceilMilli(c)
}

func rat(v string) *big.Rat {
	r, _ := new(big.Rat).SetString(strings.TrimSpace(v))
	if r == nil {
		return new(big.Rat)
	}
	return r
}

// ceilMilli is ⌈credits × 1000⌉.
func ceilMilli(credits *big.Rat) int64 {
	m := new(big.Rat).Mul(credits, big.NewRat(Milli, 1))
	q, r := new(big.Int).QuoRem(m.Num(), m.Denom(), new(big.Int))
	if r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 1<<62 - 1
	}
	return q.Int64()
}

// Credits renders millicredits as credits (for the API).
func Credits(milli int64) float64 { return float64(milli) / Milli }

// MonthStart is the first instant of t's calendar month in UTC (the plan
// lots' period; WP9 aligns it to billing periods).
func MonthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Service keeps the credit ledger of the context's tenant.
type Service struct {
	pool  *pgxpool.Pool
	q     *db.Queries
	plans entitlements.Provider
	now   func() time.Time

	mu       sync.Mutex
	settings Settings
	expires  time.Time
}

// New returns the service; plans gives each tenant's monthly allowance.
func New(pool *pgxpool.Pool, plans entitlements.Provider) *Service {
	return &Service{pool: pool, q: db.New(pool), plans: plans, now: time.Now}
}

// SetClock replaces the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// SetPlans replaces the plan provider.
func (s *Service) SetPlans(p entitlements.Provider) { s.plans = p }

// settingsTTL is how long the conversion is used before it is read again.
const settingsTTL = 30 * time.Second

// Settings returns the platform's conversion (defaults when unset; a
// failed read keeps the last value).
func (s *Service) Settings(ctx context.Context) Settings {
	now := s.now()
	s.mu.Lock()
	if now.Before(s.expires) {
		st := s.settings
		s.mu.Unlock()
		return st
	}
	s.mu.Unlock()
	st, err := ReadSettings(ctx, s.q)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.settings = st
	} else if s.settings == (Settings{}) {
		s.settings = DefaultSettings()
	}
	s.expires = now.Add(settingsTTL)
	return s.settings
}

// InvalidateSettings drops the cached conversion (after a change).
func (s *Service) InvalidateSettings() {
	s.mu.Lock()
	s.expires = time.Time{}
	s.mu.Unlock()
}

// ReadSettings reads the conversion (a platform table, readable by every
// role).
func ReadSettings(ctx context.Context, q db.Querier) (Settings, error) {
	raw, err := q.GetPlatformSetting(tenant.System(ctx), SettingKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var st Settings
	if err := json.Unmarshal(raw, &st); err != nil {
		return DefaultSettings(), nil
	}
	return st.normalized(), nil
}

// SaveSettings stores the conversion with q (a platform transaction).
func SaveSettings(ctx context.Context, q db.Querier, st Settings) error {
	if err := st.Validate(); err != nil {
		return err
	}
	raw, _ := json.Marshal(st)
	return q.UpsertPlatformSetting(ctx, db.UpsertPlatformSettingParams{Key: SettingKey, Value: raw})
}

// Balance is a tenant's credits now.
type Balance struct {
	MonthStart time.Time
	ResetsAt   time.Time
	// Included is the plan's allowance this month (millicredits).
	Included int64
	// Granted, Used and Remaining cover every usable lot.
	Granted, Used, Remaining int64
	UsedThisMonth            int64
	// Unlimited: the plan meters without a limit (never exhausted).
	Unlimited bool
}

// Exhausted reports whether AI with the platform's keys must pause.
func (b Balance) Exhausted() bool { return !b.Unlimited && b.Remaining <= 0 }

// Lot is a credit lot with its remaining amount.
type Lot struct {
	db.AiCreditLot
	Remaining int64
}

func (s *Service) allowance(ctx context.Context) (int64, bool, error) {
	plan, err := entitlements.For(ctx, s.plans)
	if err != nil {
		return 0, false, err
	}
	if plan.AICreditsPerMonth == entitlements.Unlimited {
		return 0, true, nil
	}
	return plan.AICreditsPerMonth * Milli, false, nil
}

// Balance reads the context tenant's credits without writing: the month's
// plan lot counts with the plan's allowance even before it is stored (it is
// stored by the first consumption).
func (s *Service) Balance(ctx context.Context) (Balance, error) {
	return s.balance(ctx, s.q)
}

// BalanceWith is Balance through q (a platform transaction scoped to the
// tenant).
func (s *Service) BalanceWith(ctx context.Context, q db.Querier) (Balance, error) {
	return s.balance(ctx, q)
}

func (s *Service) balance(ctx context.Context, q db.Querier) (Balance, error) {
	now := s.now()
	month := MonthStart(now)
	b := Balance{MonthStart: month, ResetsAt: month.AddDate(0, 1, 0)}
	included, unlimited, err := s.allowance(ctx)
	if err != nil {
		return b, err
	}
	b.Included, b.Unlimited = included, unlimited
	lots, err := s.lots(ctx, q, now)
	if err != nil {
		return b, err
	}
	planSeen := false
	for _, l := range lots {
		granted := l.Granted
		if l.Source == "plan" {
			planSeen = true
			granted = included // the lot follows the plan in effect
		}
		b.Granted += granted
		b.Used += l.Used
		b.Remaining += max(granted-l.Used, 0)
	}
	if !planSeen {
		b.Granted += included
		b.Remaining += included
	}
	if b.UsedThisMonth, err = q.CreditsUsedSince(ctx, month); err != nil {
		return b, err
	}
	return b, nil
}

// lots lists the usable lots (without locking).
func (s *Service) lots(ctx context.Context, q db.Querier, now time.Time) ([]db.AiCreditLot, error) {
	all, err := q.ListCreditLots(ctx, 500)
	if err != nil {
		return nil, err
	}
	month := MonthStart(now)
	out := all[:0]
	for _, l := range all {
		if l.StartsAt.After(now) || (l.ExpiresAt != nil && !l.ExpiresAt.After(now)) {
			continue
		}
		if l.Source == "plan" && (l.PeriodStart.Time.IsZero() || !l.PeriodStart.Time.Equal(month)) {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

// Lots lists the context tenant's lots, newest first, with what remains.
func (s *Service) Lots(ctx context.Context, q db.Querier, limit int32) ([]Lot, error) {
	if q == nil {
		q = s.q
	}
	rows, err := q.ListCreditLots(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Lot, len(rows))
	for i, r := range rows {
		out[i] = Lot{AiCreditLot: r, Remaining: max(r.Granted-r.Used, 0)}
	}
	return out, nil
}

// Available reports whether the context tenant may start an AI call with
// the platform's keys (credits left, or an unlimited plan).
func (s *Service) Available(ctx context.Context) (bool, error) {
	b, err := s.Balance(ctx)
	if err != nil {
		return false, err
	}
	return !b.Exhausted(), nil
}

// ErrNoTenant is returned outside a tenant.
var ErrNoTenant = errors.New("credits: no tenant in context")

// Consume books amount millicredits on the context tenant's lots inside q's
// transaction (the one recording the usage row). It stores the month's
// plan lot with the plan's allowance first, locks every usable lot
// (FOR UPDATE, in consumption order: concurrent consumers queue, no update
// is lost) and takes from the plan's month first, then from the lots that
// expire first. What no lot covers is booked on the plan lot (a call that
// started with credits left is never refused afterwards; the next one is).
func (s *Service) Consume(ctx context.Context, q *db.Queries, amount int64) error {
	if amount <= 0 {
		return nil
	}
	if _, ok := tenant.FromCtx(ctx); !ok {
		return ErrNoTenant
	}
	now := s.now()
	month := MonthStart(now)
	included, _, err := s.allowance(ctx)
	if err != nil {
		return err
	}
	if err := q.EnsurePlanCreditLot(ctx, db.EnsurePlanCreditLotParams{PeriodStart: date(month), Granted: included,
		StartsAt: month, ExpiresAt: ptr(month.AddDate(0, 1, 0))}); err != nil {
		return err
	}
	lots, err := q.LockActiveCreditLots(ctx, now)
	if err != nil {
		return err
	}
	var plan *db.AiCreditLot
	for i := range lots {
		if lots[i].Source == "plan" && lots[i].PeriodStart.Time.Equal(month) {
			plan = &lots[i]
			if plan.Granted != included { // the lot follows the plan in effect
				if _, err := q.SetPlanCreditLotGranted(ctx, db.SetPlanCreditLotGrantedParams{Granted: included, PeriodStart: date(month)}); err != nil {
					return err
				}
				plan.Granted = included
			}
		}
	}
	left := amount
	for _, l := range lots {
		if l.Source == "plan" && !l.PeriodStart.Time.Equal(month) {
			continue
		}
		avail := l.Granted - l.Used
		if avail <= 0 {
			continue
		}
		take := min(avail, left)
		if err := q.AddCreditLotUsage(ctx, db.AddCreditLotUsageParams{ID: l.ID, Amount: take}); err != nil {
			return err
		}
		left -= take
		if left == 0 {
			return nil
		}
	}
	if plan == nil {
		return fmt.Errorf("credits: the month's plan lot is missing")
	}
	return q.AddCreditLotUsage(ctx, db.AddCreditLotUsageParams{ID: plan.ID, Amount: left})
}

// Grant is a credit lot to add.
type Grant struct {
	// Credits is the amount in whole credits.
	Credits int64
	// Source is "grant" (the platform admin) or "purchase" (WP9).
	Source string
	// StartsAt is when the lot becomes usable (zero = now).
	StartsAt time.Time
	// ExpiresAt is when it stops (nil = never).
	ExpiresAt *time.Time
	Note      string
	// Reference names the purchase of a purchased lot (WP9).
	Reference *string
	CreatedBy *uuid.UUID
}

// MaxGrant bounds one lot (credits).
const MaxGrant = 100_000_000

// Grant adds a lot to the tenant q is scoped to (a platform transaction
// with app.tenant_id set, or the tenant's own). It is the entry point for
// manual grants now and for purchased top-ups in WP9.
func (s *Service) Grant(ctx context.Context, q db.Querier, g Grant) (db.AiCreditLot, error) {
	if g.Source == "" {
		g.Source = "grant"
	}
	if g.Source != "grant" && g.Source != "purchase" {
		return db.AiCreditLot{}, &FieldError{Field: "source", Message: "A lot is a grant or a purchase."}
	}
	if g.Credits <= 0 || g.Credits > MaxGrant {
		return db.AiCreditLot{}, &FieldError{Field: "credits", Message: fmt.Sprintf("Between 1 and %d credits.", MaxGrant)}
	}
	g.Note = strings.TrimSpace(g.Note)
	if len([]rune(g.Note)) < 3 || len([]rune(g.Note)) > 500 {
		return db.AiCreditLot{}, &FieldError{Field: "note", Message: "Say why the credits are granted (3–500 characters)."}
	}
	now := s.now()
	if g.StartsAt.IsZero() {
		g.StartsAt = now
	}
	if g.ExpiresAt != nil && !g.ExpiresAt.After(g.StartsAt) {
		return db.AiCreditLot{}, &FieldError{Field: "expires_at", Message: "The expiry must be in the future."}
	}
	return q.InsertCreditLot(ctx, db.InsertCreditLotParams{Source: g.Source, Granted: g.Credits * Milli, StartsAt: g.StartsAt,
		ExpiresAt: g.ExpiresAt, Note: g.Note, Reference: g.Reference, CreatedBy: g.CreatedBy})
}

func ptr[T any](v T) *T { return &v }

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// Charge converts a call's cost into millicredits with the platform's
// conversion (the AI module's meter).
func (s *Service) Charge(ctx context.Context, cost string, ok bool) int64 {
	return s.Settings(ctx).Charge(cost, ok)
}
