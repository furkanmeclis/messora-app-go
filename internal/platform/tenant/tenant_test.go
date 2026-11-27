package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
)

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	if _, ok := FromCtx(ctx); ok || ID(ctx) != uuid.Nil || IsSystem(ctx) {
		t.Fatal("empty context has a tenant")
	}
	if _, err := Setting(ctx); !errors.Is(err, ErrNoTenant) {
		t.Fatalf("setting without a tenant: %v", err)
	}
	if _, ok := FromCtx(WithID(ctx, uuid.Nil)); ok {
		t.Fatal("the nil UUID became a tenant")
	}
	id := uuid.New()
	a := WithID(ctx, id)
	if got, ok := FromCtx(a); !ok || got != id || ID(a) != id {
		t.Fatalf("FromCtx = %s %v", got, ok)
	}
	if v, err := Setting(a); err != nil || v != id.String() {
		t.Fatalf("setting = %q %v", v, err)
	}
	if v, err := Setting(System(ctx)); err != nil || v != "" {
		t.Fatalf("system setting = %q %v", v, err)
	}
	// A tenant wins over the system marker: the connection is scoped.
	if v, _ := Setting(System(a)); v != id.String() {
		t.Fatalf("system+tenant setting = %q", v)
	}
}

func TestValidateSlug(t *testing.T) {
	for _, ok := range []string{"acme", "a1b", "acme-store", "x" + strings.Repeat("y", 38) + "z", "123"} {
		if err := ValidateSlug(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ab", "-acme", "acme-", "Acme", "ac me", "a_b", "açme", "a--b", strings.Repeat("a", 41), "acme.store"} {
		if err := ValidateSlug(bad); !errors.Is(err, ErrSlugFormat) {
			t.Errorf("%q: %v, want ErrSlugFormat", bad, err)
		}
	}
	for _, r := range []string{"www", "api", "admin", "app", "hooks", "mail", "status", "docs", "static", "assets", "cdn"} {
		if err := ValidateSlug(r); !errors.Is(err, ErrSlugReserved) && !errors.Is(err, ErrSlugFormat) {
			t.Errorf("%q not reserved: %v", r, err)
		}
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"Acme.Messora.Tech":      "acme.messora.tech",
		"acme.messora.tech:443":  "acme.messora.tech",
		"acme.messora.tech.":     "acme.messora.tech",
		"localhost:8108":         "localhost",
		"":                       "",
		"evil/x":                 "",
		"a b":                    "",
		"[::1]:80":               "", // IP literals never name a tenant
		strings.Repeat("a", 254): "",
	}
	for in, want := range cases {
		if got := NormalizeHost(in); got != want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolverCachesAndInvalidates(t *testing.T) {
	acme := Info{ID: uuid.New(), Slug: "acme", Status: StatusActive}
	var calls atomic.Int32
	status := StatusActive
	r := newResolver(func(_ context.Context, host string) (Info, bool, error) {
		calls.Add(1)
		if host != "acme.messora.tech" {
			return Info{}, false, nil
		}
		i := acme
		i.Status = status
		return i, true, nil
	}, time.Minute)
	now := time.Unix(1_000, 0)
	r.now = func() time.Time { return now }
	ctx := context.Background()

	for range 3 {
		if got, err := r.Resolve(ctx, "ACME.messora.tech:443"); err != nil || got.ID != acme.ID {
			t.Fatalf("resolve: %v %v", got, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("%d lookups, want 1 (cached)", calls.Load())
	}
	// Unknown hosts are cached briefly too.
	for range 3 {
		if _, err := r.Resolve(ctx, "nope.messora.tech"); !errors.Is(err, ErrUnknownHost) {
			t.Fatalf("unknown host: %v", err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("%d lookups, want 2", calls.Load())
	}
	if _, err := r.Resolve(ctx, "bad host"); !errors.Is(err, ErrUnknownHost) || calls.Load() != 2 {
		t.Fatalf("malformed host: %v (lookups %d)", err, calls.Load())
	}

	// A suspension shows after invalidation (or after the TTL elsewhere).
	status = StatusSuspended
	if _, err := r.Resolve(ctx, "acme.messora.tech"); err != nil {
		t.Fatalf("still cached: %v", err)
	}
	r.Invalidate("acme.messora.tech")
	if info, err := r.Resolve(ctx, "acme.messora.tech"); !errors.Is(err, ErrSuspended) || info.ID != acme.ID {
		t.Fatalf("suspended: %v %v", info, err)
	}
	status = StatusDeleted
	now = now.Add(2 * time.Minute) // past the TTL
	if _, err := r.Resolve(ctx, "acme.messora.tech"); !errors.Is(err, ErrUnknownHost) {
		t.Fatalf("deleted tenant: %v", err)
	}

	// Lookup errors are not cached.
	boom := errors.New("db down")
	r2 := newResolver(func(context.Context, string) (Info, bool, error) { calls.Add(1); return Info{}, false, boom }, time.Minute)
	before := calls.Load()
	for range 2 {
		if _, err := r2.Resolve(ctx, "acme.messora.tech"); !errors.Is(err, boom) {
			t.Fatalf("lookup error: %v", err)
		}
	}
	if calls.Load()-before != 2 {
		t.Fatal("a failed lookup was cached")
	}
}

func TestMiddleware(t *testing.T) {
	acme := Info{ID: uuid.New(), Slug: "acme", Status: StatusActive}
	held := Info{ID: uuid.New(), Slug: "held", Status: StatusSuspended}
	r := newResolver(func(_ context.Context, host string) (Info, bool, error) {
		switch host {
		case "acme.messora.tech":
			return acme, true, nil
		case "held.messora.tech":
			return held, true, nil
		}
		return Info{}, false, nil
	}, time.Minute)
	const secret = "bff-secret-0123456789abcdef0123456789" // gitleaks:allow (test fixture)
	var seen uuid.UUID
	var leaked string
	h := httpx.IdentifyBFF(secret)(Middleware(r, slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen = ID(req.Context())
		leaked = req.Header.Get(HostHeader)
		w.WriteHeader(http.StatusNoContent)
	})))
	do := func(host, claimed string, bff bool) (int, string) {
		seen, leaked = uuid.Nil, ""
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Host = host
		if claimed != "" {
			req.Header.Set(HostHeader, claimed)
		}
		if bff {
			req.Header.Set(httpx.BFFHeader, secret)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body.Error.Code
	}

	if code, _ := do("acme.messora.tech", "", false); code != http.StatusNoContent || seen != acme.ID {
		t.Fatalf("direct request: %d %s", code, seen)
	}
	// The panel reaches the API on its internal address and names the host.
	if code, _ := do("messora-api:8080", "acme.messora.tech", true); code != http.StatusNoContent || seen != acme.ID || leaked != "" {
		t.Fatalf("BFF request: %d %s leaked=%q", code, seen, leaked)
	}
	// Without the BFF secret the claimed host is ignored.
	if code, errCode := do("held.messora.tech", "acme.messora.tech", false); code != http.StatusForbidden || errCode != "TENANT_SUSPENDED" {
		t.Fatalf("claimed host without the secret: %d %s", code, errCode)
	}
	if code, errCode := do("nope.messora.tech", "", false); code != http.StatusNotFound || errCode != "TENANT_NOT_FOUND" || seen != uuid.Nil {
		t.Fatalf("unknown host: %d %s", code, errCode)
	}
	// The BFF without a host header gets nothing either.
	if code, _ := do("messora-api:8080", "", true); code != http.StatusNotFound {
		t.Fatalf("BFF without a host: %d", code)
	}
}

func TestCache(t *testing.T) {
	c := NewCache[int](time.Minute)
	a, b := WithID(context.Background(), uuid.New()), WithID(context.Background(), uuid.New())
	c.Set(a, 1)
	c.Set(b, 2)
	c.Set(context.Background(), 3) // no tenant: never cached
	if v, ok := c.Get(a); !ok || v != 1 {
		t.Fatalf("a = %d %v", v, ok)
	}
	if v, ok := c.Get(b); !ok || v != 2 {
		t.Fatalf("b = %d %v", v, ok)
	}
	if _, ok := c.Get(System(context.Background())); ok {
		t.Fatal("a value without a tenant")
	}
	c.Invalidate(a)
	if _, ok := c.Get(a); ok {
		t.Fatal("a survived its invalidation")
	}
	if _, ok := c.Get(b); !ok {
		t.Fatal("b dropped with a")
	}
	c.Clear()
	if _, ok := c.Get(b); ok {
		t.Fatal("b survived Clear")
	}
	short := NewCache[int](time.Nanosecond)
	short.Set(a, 1)
	time.Sleep(time.Millisecond)
	if _, ok := short.Get(a); ok {
		t.Fatal("expired value returned")
	}
}
