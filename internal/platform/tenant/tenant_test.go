package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/storage"
)

// The tenant of a context also scopes its stored objects, and the objects
// stored before tenants belong to the tenant that took over their rows.
func TestWithIDScopesStorage(t *testing.T) {
	id := uuid.New()
	if got := storage.TenantKey(WithID(context.Background(), id)); got != id.String() {
		t.Fatalf("storage tenant = %q", got)
	}
	if got := storage.TenantKey(WithID(context.Background(), uuid.Nil)); got != "" {
		t.Fatalf("nil tenant reached storage: %q", got)
	}
	if storage.LegacyTenant != DefaultID.String() {
		t.Fatalf("legacy objects belong to %s, legacy rows to %s", storage.LegacyTenant, DefaultID)
	}
}

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
	for _, ok := range []string{"acme", "a1b", "acme-store", "x" + strings.Repeat("y", 28) + "z", "123", "login", "demo"} {
		if err := ValidateSlug(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ab", "-acme", "acme-", "Acme", "ac me", "a_b", "açme", "a--b", strings.Repeat("a", 31), "acme.store"} {
		if err := ValidateSlug(bad); !errors.Is(err, ErrSlugFormat) {
			t.Errorf("%q: %v, want ErrSlugFormat", bad, err)
		}
	}
	for _, r := range []string{"www", "admin", "api", "app", "hooks", "dokploy", "cdn", "mail", "docs", "status", "help", "support"} {
		if err := ValidateSlug(r); !errors.Is(err, ErrSlugReserved) {
			t.Errorf("%q not reserved: %v", r, err)
		}
	}
}

// The sign-up form refuses the same addresses as the API.
func TestReservedSlugsMatchTheFrontend(t *testing.T) {
	src, err := os.ReadFile("../../../frontend/src/lib/auth/slug.ts")
	if err != nil {
		t.Skip("frontend sources not present:", err)
	}
	block := regexp.MustCompile(`(?s)RESERVED_SLUGS[^=]*=\s*\[(.*?)\]`).FindSubmatch(src)
	if block == nil {
		t.Fatal("RESERVED_SLUGS not found in slug.ts")
	}
	var ts []string
	for _, m := range regexp.MustCompile(`'([a-z0-9-]+)'`).FindAllSubmatch(block[1], -1) {
		ts = append(ts, string(m[1]))
	}
	got, want := slices.Sorted(slices.Values(ts)), slices.Sorted(slices.Values(ReservedSlugs))
	if !slices.Equal(got, want) {
		t.Fatalf("frontend reserved slugs %v, Go %v", got, want)
	}
	for _, re := range []string{`SLUG_MIN = ` + strconv.Itoa(SlugMin) + `;`, `SLUG_MAX = ` + strconv.Itoa(SlugMax) + `;`} {
		if !strings.Contains(string(src), re) {
			t.Errorf("slug.ts lacks %q", re)
		}
	}
}

func TestHosts(t *testing.T) {
	h := NewHosts("https://app.messora.tech", "Messora.Tech")
	for host, want := range map[string]HostKind{
		"admin.messora.tech": HostAdmin, "messora.tech": HostPlatform, "www.messora.tech": HostPlatform,
		"app.messora.tech:443": HostPlatform, "acme.messora.tech": HostTenant, "panel.acme.com": HostTenant,
	} {
		if got := h.Kind(host); got != want {
			t.Errorf("Kind(%q) = %v, want %v", host, got, want)
		}
	}
	if h.URL("acme.messora.tech") != "https://acme.messora.tech" || h.AdminURL() != "https://admin.messora.tech" {
		t.Fatalf("urls: %s %s", h.URL("acme.messora.tech"), h.AdminURL())
	}
	dev := NewHosts("http://localhost:8090", "messora.localhost")
	if dev.URL("acme.messora.localhost") != "http://acme.messora.localhost:8090" || dev.Secure() {
		t.Fatalf("dev url %s", dev.URL("acme.messora.localhost"))
	}
	if !h.IsPlatformLabel("app.messora.tech") || !h.IsPlatformLabel("support.messora.tech") || h.IsPlatformLabel("acme.messora.tech") {
		t.Fatal("platform labels")
	}
	none := NewHosts("http://localhost", "")
	if none.Kind("admin.localhost") != HostTenant || none.AdminURL() != "" {
		t.Fatal("no base domain: no platform hosts")
	}
}

func TestPlatformContext(t *testing.T) {
	a := WithID(context.Background(), uuid.New())
	p := Platform(a)
	if _, ok := FromCtx(p); ok || !IsPlatform(p) || Role(p) != PlatformRole {
		t.Fatal("platform work drops the tenant and switches role")
	}
	if v, err := Setting(p); err != nil || v != "" {
		t.Fatalf("platform setting %q %v", v, err)
	}
	back := WithID(p, uuid.New())
	if IsPlatform(back) || Role(back) != "" {
		t.Fatal("a tenant's work is never platform work")
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
	var kind HostKind
	var platform bool
	hosts := NewHosts("https://app.messora.tech", "messora.tech")
	h := httpx.IdentifyBFF(secret)(Middleware(r, hosts, slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen = ID(req.Context())
		leaked = req.Header.Get(HostHeader)
		kind, platform = HostKindOf(req.Context()), IsPlatform(req.Context())
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
	// The platform's own hosts carry no tenant: the admin host is platform
	// work, the public one tenant-less system work.
	if code, _ := do("messora-api:8080", "admin.messora.tech", true); code != http.StatusNoContent || seen != uuid.Nil || kind != HostAdmin || !platform {
		t.Fatalf("admin host: %d %s %v %v", code, seen, kind, platform)
	}
	if code, _ := do("app.messora.tech", "", false); code != http.StatusNoContent || seen != uuid.Nil || kind != HostPlatform || platform {
		t.Fatalf("platform host: %d %s %v %v", code, seen, kind, platform)
	}
	if code, _ := do("acme.messora.tech", "", false); code != http.StatusNoContent || kind != HostTenant || platform {
		t.Fatalf("tenant host kind: %d %v %v", code, kind, platform)
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
