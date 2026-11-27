//go:build integration

package messaging_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/modules/messaging"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// The send scheduler keeps its state under the tenant's Redis prefix: one
// tenant's backoff, buckets and shares never touch another's, and every
// key of a send shares the tenant's hash tag.
func TestSendSchedulerKeysAreTenantScoped(t *testing.T) {
	rdb := testutil.Redis(t)
	a, b := uuid.New(), uuid.New()
	ctxA, ctxB := tenant.WithID(context.Background(), a), tenant.WithID(context.Background(), b)
	th := messaging.NewThrottle(rdb, 80)
	pn, camp := uuid.New(), uuid.New() // the same IDs in both tenants: no shared state

	th.Backoff(ctxA, pn)
	th.SetShare(ctxA, pn, camp, 5)
	_ = th.Wait(ctxA, pn)
	_ = th.WaitCampaign(ctxA, pn, camp)
	if r := th.Rate(ctxA, pn); r != 40 {
		t.Fatalf("A's rate after a backoff: %d", r)
	}
	if r := th.Rate(ctxB, pn); r != 80 {
		t.Fatalf("A's backoff reached B: rate %d", r)
	}
	if s := th.Share(ctxB, pn, camp); s != 0 {
		t.Fatalf("A's share reached B: %v", s)
	}
	keys, err := rdb.Keys(context.Background(), "*").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("no keys written")
	}
	for _, k := range keys {
		if !strings.HasPrefix(k, tenant.RedisPrefix(a)) {
			t.Errorf("key %q is not under tenant A's prefix %q", k, tenant.RedisPrefix(a))
		}
	}
}

// The per-tenant campaign cap (fairness setting) bounds all of a tenant's
// campaign sends together, across numbers, and leaves other tenants and
// non-campaign sends alone.
func TestTenantCampaignCap(t *testing.T) {
	rdb := testutil.Redis(t)
	a, b := uuid.New(), uuid.New()
	ctxA, ctxB := tenant.WithID(context.Background(), a), tenant.WithID(context.Background(), b)
	th := messaging.NewThrottle(rdb, 80).WithTenantCampaignCap(func(context.Context) float64 { return 2 })

	// Two campaigns on two numbers of tenant A: the tenant bucket (2/s,
	// burst 2) refuses the third send although each number has room.
	pn1, pn2, c1, c2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if w := th.WaitCampaign(ctxA, pn1, c1); w != 0 {
		t.Fatalf("first send waits %s", w)
	}
	if w := th.WaitCampaign(ctxA, pn2, c2); w != 0 {
		t.Fatalf("second send waits %s", w)
	}
	w := th.WaitCampaign(ctxA, pn1, c1)
	if w <= 0 || w > time.Second {
		t.Fatalf("third send of tenant A: wait %s, want up to 500 ms", w)
	}
	// Tenant B and A's inbox sends are not limited by A's cap.
	if w := th.WaitCampaign(ctxB, pn1, c1); w != 0 {
		t.Fatalf("tenant B waits %s", w)
	}
	if w := th.Wait(ctxA, pn2); w != 0 {
		t.Fatalf("A's inbox send waits %s", w)
	}
	// 0 turns the cap off.
	off := messaging.NewThrottle(rdb, 80).WithTenantCampaignCap(func(context.Context) float64 { return 0 })
	for range 5 {
		if w := off.WaitCampaign(ctxB, uuid.New(), uuid.New()); w != 0 {
			t.Fatalf("no cap: wait %s", w)
		}
	}
}
