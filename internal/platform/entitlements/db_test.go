package entitlements

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// fakePlans answers GetTenantPlan from rows (the rest of db.Querier is
// not used by the provider).
type fakePlans struct {
	db.Querier
	mu    sync.Mutex
	rows  map[uuid.UUID]db.GetTenantPlanRow
	reads int
	fail  error
}

func (f *fakePlans) GetTenantPlan(_ context.Context, id uuid.UUID) (db.GetTenantPlanRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.fail != nil {
		return db.GetTenantPlanRow{}, f.fail
	}
	r, ok := f.rows[id]
	if !ok {
		return r, pgx.ErrNoRows
	}
	return r, nil
}

func (f *fakePlans) set(id uuid.UUID, r db.GetTenantPlanRow) {
	f.mu.Lock()
	f.rows[id] = r
	f.mu.Unlock()
}

func ptrTo[T any](v T) *T { return &v }

const starterLimits = `{"users": 2, "phone_numbers": 1, "ai_credits_per_month": 300, "api_key_requests_per_minute": 0}`

func TestDBProviderComposesPlanAndOverrides(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	f := &fakePlans{rows: map[uuid.UUID]db.GetTenantPlanRow{
		a: {ID: a, PlanKey: ptrTo("starter"), PlanOverrides: []byte(`{}`), SubscriptionStatus: "active",
			PlanLimits: []byte(starterLimits), PlanFeatures: []byte(`{"webhooks": true, "public_api": false}`),
			PlanNameTr: ptrTo("Başlangıç"), PlanNameEn: ptrTo("Starter")},
		b: {ID: b, PlanKey: ptrTo("enterprise"), PlanOverrides: []byte(`{"users": 40, "features": {"custom_domain": false}}`),
			PlanLimits: []byte(`{"users": 0}`), PlanFeatures: []byte(`{"custom_smtp": true, "custom_domain": true}`)},
		c: {ID: c, PlanOverrides: []byte(`{}`), SubscriptionStatus: "trial"},
	}}
	p := newDBProvider(f, Static(Default()))
	ctx := context.Background()

	pa, info, err := p.Describe(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if pa.Key != "starter" || pa.Users != 2 || pa.AICreditsPerMonth != 300 || info.NameTR != "Başlangıç" || info.Overridden() {
		t.Fatalf("starter: %+v %+v", pa, info)
	}
	// A feature the catalogue does not name is off; budgets ≤ 0 keep the
	// default; unnamed limits keep the base.
	if pa.Has(PublicAPI) || !pa.Has(Webhooks) || pa.Has(CustomSMTP) || pa.APIKeyRequestsPerMinute != 300 || pa.WebhookEndpoints != 50 {
		t.Fatalf("starter features/budgets: %+v", pa)
	}
	pb, info, _ := p.Describe(ctx, b)
	if pb.Key != "enterprise" || pb.Users != 40 || !pb.Has(CustomSMTP) || pb.Has(CustomDomain) || !info.Overridden() {
		t.Fatalf("enterprise with overrides: %+v", pb)
	}
	pc, info, _ := p.Describe(ctx, c)
	if pc.Key != DefaultKey || !pc.Has(CustomSMTP) || info.SubscriptionStatus != "trial" {
		t.Fatalf("no plan: %+v %+v", pc, info)
	}
	if _, err := p.Limits(ctx, uuid.New()); !errors.Is(err, ErrUnknownTenant) {
		t.Fatalf("unknown tenant: %v", err)
	}
}

func TestDBProviderCachesAndInvalidates(t *testing.T) {
	id := uuid.New()
	f := &fakePlans{rows: map[uuid.UUID]db.GetTenantPlanRow{id: {ID: id, PlanKey: ptrTo("starter"), PlanOverrides: []byte(`{}`),
		PlanLimits: []byte(starterLimits), PlanFeatures: []byte(`{}`)}}}
	p := newDBProvider(f, nil)
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	ctx := context.Background()
	for range 5 {
		if pl, _ := p.Limits(ctx, id); pl.Users != 2 {
			t.Fatalf("users = %d", pl.Users)
		}
	}
	if f.reads != 1 {
		t.Fatalf("reads = %d, want 1 (cached)", f.reads)
	}
	// A change becomes visible after invalidation…
	f.set(id, db.GetTenantPlanRow{ID: id, PlanKey: ptrTo("growth"), PlanOverrides: []byte(`{}`), PlanLimits: []byte(`{"users": 5}`), PlanFeatures: []byte(`{}`)})
	p.Invalidate(id)
	if pl, _ := p.Limits(ctx, id); pl.Key != "growth" || pl.Users != 5 {
		t.Fatalf("after Invalidate: %+v", pl)
	}
	// …or after the TTL.
	f.set(id, db.GetTenantPlanRow{ID: id, PlanKey: ptrTo("pro"), PlanOverrides: []byte(`{}`), PlanLimits: []byte(`{"users": 12}`), PlanFeatures: []byte(`{}`)})
	if pl, _ := p.Limits(ctx, id); pl.Key != "growth" {
		t.Fatal("re-read before the TTL")
	}
	now = now.Add(DBProviderTTL + time.Second)
	if pl, _ := p.Limits(ctx, id); pl.Key != "pro" {
		t.Fatalf("after the TTL: %s", pl.Key)
	}
	// A failed read keeps the last known plan.
	now = now.Add(DBProviderTTL + time.Second)
	f.fail = errors.New("database down")
	if pl, err := p.Limits(ctx, id); err != nil || pl.Key != "pro" {
		t.Fatalf("stale on failure: %v %v", pl.Key, err)
	}
	// Without a known plan the failure is reported.
	if _, err := p.Limits(ctx, uuid.New()); err == nil {
		t.Fatal("no error without a cached plan")
	}
	f.fail = nil
	p.InvalidateAll()
	if pl, _ := p.Limits(ctx, id); pl.Key != "pro" || f.reads < 4 {
		t.Fatalf("after InvalidateAll: %s (%d reads)", pl.Key, f.reads)
	}
}

func TestAllowAddAndLimitDetails(t *testing.T) {
	p := Default()
	p.StorageBytes, p.PhoneNumbers = 100, 3
	if err := p.AllowAdd(Storage, 60, 40); err != nil {
		t.Fatalf("exactly at the limit: %v", err)
	}
	e, ok := apperr.As(p.AllowAdd(Storage, 60, 41))
	if !ok || e.Details[0].Message != "100" || e.Details[1].Field != "current" || e.Details[1].Message != "60" {
		t.Fatalf("over: %+v", e)
	}
	if e, ok := apperr.As(p.Allow(PhoneNumbers, 3)); !ok || e.Status != 409 {
		t.Fatalf("count at the limit: %v", e)
	}
	if p.Limited(Contacts) || !p.Limited(PhoneNumbers) {
		t.Fatal("Limited")
	}
	if got := p.FeatureList(); len(got) != len(AllFeatures) {
		t.Fatalf("features: %v", got)
	}
	// For answers the default outside a tenant (platform work).
	if pl, _ := For(tenant.Platform(context.Background()), Static(Plan{Key: "x"})); pl.Key != "default" {
		t.Fatalf("platform: %s", pl.Key)
	}
}
