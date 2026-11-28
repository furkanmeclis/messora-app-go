package entitlements

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Plan keys of the launch catalogue (migration 00034). The catalogue is
// data: the platform admin may add plans; these keys are what code needs
// to know.
const (
	PlanStarter    = "starter"
	PlanGrowth     = "growth"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
	// TrialPlan is the plan of a company during its trial (WP8 decision:
	// the growth plan's limits). What happens at the trial's end is WP9.
	TrialPlan = PlanGrowth
	// DefaultKey is the key of a tenant without a plan (the platform's
	// default plan, Default overlaid with platform_settings.default_plan).
	DefaultKey = "default"
)

// Info is what the panel shows about a tenant's plan next to its limits.
type Info struct {
	// Key is the plan's key (DefaultKey without a plan).
	Key    string
	NameTR string
	NameEN string
	// Overrides is the tenant's own limits over the plan (raw JSON object;
	// "{}" when none).
	Overrides          json.RawMessage
	SubscriptionStatus string
	TrialEndsAt        *time.Time
}

// Overridden reports whether the tenant has limits of its own.
func (i Info) Overridden() bool { return len(i.Overrides) > 2 }

type dbEntry struct {
	plan    Plan
	info    Info
	expires time.Time
}

// DBProvider reads each tenant's plan from the plan catalogue
// (tenants.plan_key → plans) and the tenant's own overrides
// (tenants.plan_overrides). A tenant without a plan gets the fallback
// provider's plan (the platform's default plan). Plans are cached per
// tenant for TTL; the platform panel invalidates them when it changes a
// plan or a tenant's assignment (other replicas follow within TTL).
type DBProvider struct {
	q        db.Querier
	fallback Provider
	ttl      time.Duration
	now      func() time.Time

	mu    sync.Mutex
	cache map[uuid.UUID]dbEntry
	// gen is bumped by every invalidation; a read that started before it
	// is not cached (it may have seen the old plan).
	gen uint64
}

// DBProviderTTL is how long a tenant's plan is used before it is read again.
const DBProviderTTL = 30 * time.Second

// NewDBProvider reads plans through pool (platform tables, readable
// without a tenant). fallback answers tenants without a plan (nil =
// Default).
func NewDBProvider(pool *pgxpool.Pool, fallback Provider) *DBProvider {
	return newDBProvider(db.New(pool), fallback)
}

func newDBProvider(q db.Querier, fallback Provider) *DBProvider {
	if fallback == nil {
		fallback = Static(Default())
	}
	return &DBProvider{q: q, fallback: fallback, ttl: DBProviderTTL, now: time.Now, cache: map[uuid.UUID]dbEntry{}}
}

// Limits implements Provider.
func (p *DBProvider) Limits(ctx context.Context, id uuid.UUID) (Plan, error) {
	pl, _, err := p.Describe(ctx, id)
	return pl, err
}

// Describe returns the tenant's plan and what the panel shows about it.
// A failed read keeps the last known plan (stale) and, without one, fails:
// a database hiccup never loosens or tightens anyone's limits silently.
func (p *DBProvider) Describe(ctx context.Context, id uuid.UUID) (Plan, Info, error) {
	now := p.now()
	p.mu.Lock()
	e, ok := p.cache[id]
	gen := p.gen
	p.mu.Unlock()
	if ok && now.Before(e.expires) {
		return e.plan, e.info, nil
	}
	pl, info, err := p.read(ctx, id)
	if err != nil {
		if ok {
			return e.plan, e.info, nil
		}
		return Plan{}, Info{}, err
	}
	p.mu.Lock()
	if p.gen == gen {
		p.cache[id] = dbEntry{plan: pl, info: info, expires: now.Add(p.ttl)}
	}
	p.mu.Unlock()
	return pl, info, nil
}

// Invalidate drops the cached plans of ids (a tenant's plan changed).
func (p *DBProvider) Invalidate(ids ...uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gen++
	for _, id := range ids {
		delete(p.cache, id)
	}
}

// InvalidateAll drops every cached plan (a catalogue plan changed).
func (p *DBProvider) InvalidateAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gen++
	p.cache = map[uuid.UUID]dbEntry{}
}

// ErrUnknownTenant is returned for a tenant id that does not exist.
var ErrUnknownTenant = errors.New("entitlements: unknown tenant")

func (p *DBProvider) read(ctx context.Context, id uuid.UUID) (Plan, Info, error) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	row, err := p.q.GetTenantPlan(tenant.System(rctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, Info{}, ErrUnknownTenant
	}
	if err != nil {
		return Plan{}, Info{}, err
	}
	base, err := p.fallback.Limits(ctx, id)
	if err != nil {
		return Plan{}, Info{}, err
	}
	info := Info{Key: DefaultKey, Overrides: json.RawMessage(row.PlanOverrides), SubscriptionStatus: row.SubscriptionStatus,
		TrialEndsAt: row.TrialEndsAt}
	if len(info.Overrides) == 0 {
		info.Overrides = json.RawMessage("{}")
	}
	pl := base
	pl.Key = DefaultKey
	if row.PlanKey != nil && row.PlanLimits != nil {
		if pl, err = FromCatalogue(base, row.PlanLimits, row.PlanFeatures); err != nil {
			return Plan{}, Info{}, fmt.Errorf("entitlements: plan %s: %w", *row.PlanKey, err)
		}
		pl.Key, info.Key = *row.PlanKey, *row.PlanKey
		info.NameTR, info.NameEN = deref(row.PlanNameTr), deref(row.PlanNameEn)
	}
	if info.Overridden() {
		key := pl.Key
		if pl, err = Overlay(pl, info.Overrides); err != nil {
			return Plan{}, Info{}, fmt.Errorf("entitlements: overrides of %s: %w", id, err)
		}
		pl.Key = key
	}
	return pl, info, nil
}

// FromCatalogue builds a catalogue plan: limits (a JSON object with the
// field names of Plan) over base, and exactly the features the plan lists
// as true (a feature the plan does not mention is not included, so a new
// feature is never switched on by accident).
func FromCatalogue(base Plan, limits, features []byte) (Plan, error) {
	p := base
	p.Features = nil
	if len(limits) > 0 {
		if err := json.Unmarshal(limits, &p); err != nil {
			return Plan{}, err
		}
	}
	p.Features = map[Feature]bool{}
	if len(features) > 0 {
		var f map[Feature]bool
		if err := json.Unmarshal(features, &f); err != nil {
			return Plan{}, err
		}
		for k, v := range f {
			if v {
				p.Features[k] = true
			}
		}
	}
	if p.SessionRequestsPerMinute <= 0 {
		p.SessionRequestsPerMinute = base.SessionRequestsPerMinute
	}
	if p.APIKeyRequestsPerMinute <= 0 {
		p.APIKeyRequestsPerMinute = base.APIKeyRequestsPerMinute
	}
	return p, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Describer is a Provider that also tells what the panel shows about the
// plan (DBProvider).
type Describer interface {
	Provider
	Describe(ctx context.Context, id uuid.UUID) (Plan, Info, error)
}

// DescribeWith returns the tenant's plan and info from p; a provider that
// cannot describe (tests, fakes) yields an Info with the plan's key.
func DescribeWith(ctx context.Context, p Provider, id uuid.UUID) (Plan, Info, error) {
	if d, ok := p.(Describer); ok {
		return d.Describe(ctx, id)
	}
	pl, err := p.Limits(ctx, id)
	return pl, Info{Key: pl.Key, Overrides: json.RawMessage("{}")}, err
}

// Check enforces the plan's limit of r inside the caller's transaction q:
// when the plan caps r it takes the tenant's lock for r (so concurrent
// creations count one after the other), counts with count and answers
// 409 PLAN_LIMIT_REACHED when one more would exceed the limit. Without a
// cap nothing is counted.
func Check(ctx context.Context, q db.Querier, p Provider, r Resource, count func() (int64, error)) error {
	plan, err := For(ctx, p)
	if err != nil {
		return err
	}
	if !plan.Limited(r) {
		return nil
	}
	if err := q.LockTenantPlanCounter(ctx, string(r)); err != nil {
		return err
	}
	n, err := count()
	if err != nil {
		return err
	}
	return plan.Allow(r, n)
}

// CheckAdd is Check for adding n of r at once (a WhatsApp account brings
// all of its numbers): 409 when current + n would exceed the limit.
func CheckAdd(ctx context.Context, q db.Querier, p Provider, r Resource, n int64, count func() (int64, error)) error {
	plan, err := For(ctx, p)
	if err != nil {
		return err
	}
	if !plan.Limited(r) || n <= 0 {
		return nil
	}
	if err := q.LockTenantPlanCounter(ctx, string(r)); err != nil {
		return err
	}
	cur, err := count()
	if err != nil {
		return err
	}
	return plan.AllowAdd(r, cur, n)
}
