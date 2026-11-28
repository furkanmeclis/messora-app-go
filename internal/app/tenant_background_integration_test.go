//go:build integration

package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/contacts"
	"github.com/furkanmeclis/messora-app-go/internal/modules/realtime"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// Phase 17 WP3: background work, realtime, Redis and media of two tenants.
// One harness (two containers, migrations) serves every check: each
// harness costs several seconds under -race, and the checks only need two
// tenants.
func TestTenantBackgroundWork(t *testing.T) {
	h := newHarness(t)
	ownerA, _ := h.setupOwner()
	hb := h.tenant("beta")
	ownerB := hb.setupOwnerAs("owner@beta.test")
	// Realtime first: the failed-jobs check writes ops hints to the outbox.
	t.Run("realtime events stay in their tenant's channels", func(t *testing.T) { checkRealtimeChannels(t, h, hb) })
	t.Run("signed media link is bound to the tenant host", func(t *testing.T) { checkSignedMediaLink(t, h, hb) })
	t.Run("API rate limit keys are tenant-scoped", func(t *testing.T) { checkRateLimitKeys(t, h, ownerA) })
	t.Run("import retention is per tenant", func(t *testing.T) { checkImportRetention(t, h, hb) })
	t.Run("failed jobs are tenant-scoped", func(t *testing.T) { checkFailedJobsScope(t, h, hb, ownerA, ownerB) })
}

// A presigned local media link of tenant A works only at A's addresses:
// at tenant B's host it is refused although its signature is valid.
func checkSignedMediaLink(t *testing.T, h, hb *harness) {
	ctxA := h.ctx()
	key, err := storage.Key(storage.TenantKey(ctxA), "media", "txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.stores.Private.Put(ctxA, key, strings.NewReader("alpha's file"), 12, "text/plain", storage.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	link, err := h.svc.stores.Private.PresignGet(storage.WithRequestHost(ctxA, h.host), key, time.Minute, storage.PresignOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "https://"+h.host+storage.SignedPrefix) {
		t.Fatalf("link %s does not point at A's host", link)
	}
	u, _ := url.Parse(link)
	get := func(host string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, h.srv.URL+u.RequestURI(), nil)
		req.Host = host
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := get(h.host); code != http.StatusOK {
		t.Fatalf("A's link at A's host: %d", code)
	}
	if code := get(hb.host); code != http.StatusForbidden {
		t.Fatalf("A's link at B's host: %d", code)
	}
	if code := get("nobody.messora.test"); code != http.StatusNotFound {
		t.Fatalf("A's link at an unknown host: %d", code)
	}
	// Tenant B cannot sign A's object either.
	if _, err := h.svc.stores.Private.PresignGet(hb.ctx(), key, time.Minute, storage.PresignOptions{}); err == nil {
		t.Fatal("tenant B signed tenant A's object")
	}
}

// Realtime events of two tenants, relayed by the system pool, reach each
// tenant's own wire channels only.
func checkRealtimeChannels(t *testing.T, h, hb *harness) {
	for _, x := range []*harness{h, hb} {
		ev := realtime.NewNotificationEvent("test.notice")
		if err := realtime.Publish(x.ctx(), x.pool, realtime.InboxChannel, ev); err != nil {
			t.Fatal(err)
		}
	}
	rec := &recordingPublisher{}
	router := outbox.NewRouter()
	router.Handle(outbox.TaskTopicPrefix, outbox.PublisherFunc(func(context.Context, outbox.Event) error { return nil }))
	router.Handle(realtime.TopicPrefix, realtime.Publisher(rec))
	relay := outbox.NewRelay(h.sys, router, outbox.RelayOptions{Logger: testutil.DiscardLogger()})
	for {
		n, err := relay.DrainOnce(tenant.System(context.Background()))
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	got := map[string]int{}
	for _, e := range rec.events {
		if !strings.HasPrefix(e.Topic, realtime.ChannelPrefix(h.tenantID)) && !strings.HasPrefix(e.Topic, realtime.ChannelPrefix(hb.tenantID)) {
			t.Fatalf("published outside both tenants: %q", e.Topic)
		}
		if strings.Contains(string(e.Payload), "test.notice") {
			got[e.Topic]++
		}
	}
	if got[h.wire(realtime.InboxChannel)] != 1 || got[hb.wire(realtime.InboxChannel)] != 1 || len(got) != 2 {
		t.Fatalf("published on %v, want each tenant's inbox:all once", got)
	}
}

// API rate limits of sessions count per tenant, under the tenant's Redis
// prefix; nothing of a session is kept under a global key.
func checkRateLimitKeys(t *testing.T, h *harness, owner string) {
	r := h.do(call{method: "GET", path: "/api/v1/me", token: owner})
	h.expect(r, 200, "")
	userID := r.data()["user"].(map[string]any)["id"].(string)
	ctx := context.Background()
	want := tenant.RedisPrefix(h.tenantID) + "rl:api:user:" + userID
	if n, err := h.rdb.Exists(ctx, want).Result(); err != nil || n != 1 {
		t.Fatalf("session limit key %q: %d %v", want, n, err)
	}
	keys, err := h.rdb.Keys(ctx, "messora:rl:api:user:*").Result()
	if err != nil || len(keys) != 0 {
		t.Fatalf("global session limit keys: %v %v", keys, err)
	}
}

// The import retention job removes a tenant's old imports and their files,
// and nothing of another tenant's.
func checkImportRetention(t *testing.T, h, hb *harness) {
	old := time.Now().Add(-contacts.ImportRetention - time.Hour)
	type imp struct {
		x        *harness
		key      string
		finished time.Time
	}
	var imps []imp
	for _, x := range []*harness{h, hb} {
		for _, finished := range []time.Time{old, time.Now()} {
			key, err := storage.Key(storage.TenantKey(x.ctx()), "imports", "csv")
			if err != nil {
				t.Fatal(err)
			}
			if err := x.svc.stores.Private.Put(x.ctx(), key, strings.NewReader("phone\n"), 6, "text/csv", storage.PutOptions{}); err != nil {
				t.Fatal(err)
			}
			if _, err := x.owner.Exec(context.Background(), `INSERT INTO contact_imports (tenant_id, filename, storage_key, size_bytes, status, finished_at)
				VALUES ($1, 'c.csv', $2, 6, 'completed', $3)`, x.tenantID, key, finished); err != nil {
				t.Fatal(err)
			}
			imps = append(imps, imp{x, key, finished})
		}
	}
	n, err := h.svc.contacts.ExpireImports(h.ctx(), contacts.ImportRetention)
	if err != nil || n != 1 {
		t.Fatalf("tenant A: removed %d, %v", n, err)
	}
	for _, i := range imps {
		_, statErr := i.x.svc.stores.Private.Stat(i.x.ctx(), i.key)
		rows := i.x.count("SELECT count(*) FROM contact_imports WHERE storage_key = $1", i.key)
		gone := i.x == h && i.finished.Equal(old)
		if gone != (statErr != nil) || gone != (rows == 0) {
			t.Errorf("tenant %s import finished %s: object err %v, rows %d, want gone=%v", i.x.host, i.finished, statErr, rows, gone)
		}
	}
	if n, err := hb.svc.contacts.ExpireImports(hb.ctx(), contacts.ImportRetention); err != nil || n != 1 {
		t.Fatalf("tenant B: removed %d, %v", n, err)
	}
}
