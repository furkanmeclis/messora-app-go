//go:build integration

package edge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/furkanmeclis/messora-app-go/internal/platform/edge"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// TestEdgeAgainstTenantDomains runs both edge endpoints on a migrated
// database, as the API does: the application role (messora_app, tenant
// scoped) reading the platform tables tenants and tenant_domains.
func TestEdgeAgainstTenantDomains(t *testing.T) {
	db := testutil.TenantDB(t)
	ctx := context.Background()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// beta: active, a sub-domain, a verified and an unverified custom
	// domain. gamma: suspended, with a verified custom domain.
	mustExec(`INSERT INTO tenants (id, slug, name) VALUES ('00000000-0000-0000-0000-00000000000b', 'beta', 'Beta')`)
	mustExec(`INSERT INTO tenants (id, slug, name, status, suspended_at) VALUES ('00000000-0000-0000-0000-00000000000c', 'gamma', 'Gamma', 'suspended', now())`)
	mustExec(`INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary) VALUES
		('00000000-0000-0000-0000-00000000000b', 'beta.messora.tech', 'subdomain', now(), true),
		('00000000-0000-0000-0000-00000000000b', 'panel.beta.example', 'custom', now(), false),
		('00000000-0000-0000-0000-00000000000b', 'pending.beta.example', 'custom', NULL, false),
		('00000000-0000-0000-0000-00000000000c', 'chat.gamma.example', 'custom', now(), false)`)
	// Tenant #1 as `messora migrate` registers it: its sub-domain and the
	// PUBLIC_APP_URL host.
	if _, err := tenant.BootstrapDomains(ctx, db.Owner, "https://legacy.example.org", "messora.tech"); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	edge.Mount(r, edge.Options{Resolver: tenant.NewResolver(db.App, time.Second), Pool: db.App, BaseDomain: "messora.tech"})
	srv := httptest.NewServer(r)
	defer srv.Close()
	get := func(path string) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var body []byte
		if res.StatusCode == http.StatusOK {
			var v any
			if json.NewDecoder(res.Body).Decode(&v) == nil {
				body, _ = json.Marshal(v)
			}
		}
		return res.StatusCode, body
	}

	for host, want := range map[string]int{
		"admin.messora.tech":   http.StatusOK,
		"hooks.messora.tech":   http.StatusOK,
		"beta.messora.tech":    http.StatusOK,
		"default.messora.tech": http.StatusOK, // tenant #1
		"legacy.example.org":   http.StatusOK,
		"panel.beta.example":   http.StatusOK,
		"pending.beta.example": http.StatusForbidden, // not verified
		"chat.gamma.example":   http.StatusForbidden, // tenant suspended
		"nobody.messora.tech":  http.StatusForbidden,
		"stranger.example":     http.StatusForbidden,
	} {
		if got, _ := get(edge.AllowPath + "?domain=" + url.QueryEscape(host)); got != want {
			t.Errorf("ask %s = %d, want %d", host, got, want)
		}
	}

	hosts, err := edge.CustomDomains(ctx, db.App, "messora.tech")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hosts, []string{"legacy.example.org", "panel.beta.example"}) {
		t.Fatalf("custom domains = %v", hosts)
	}
	code, body := get(edge.TraefikPath + "?upstream=" + url.QueryEscape("http://staging-edge:80"))
	if code != http.StatusOK {
		t.Fatalf("traefik provider = %d", code)
	}
	var doc struct {
		HTTP struct {
			Routers map[string]struct {
				Rule string `json:"rule"`
			} `json:"routers"`
		} `json:"http"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var rules []string
	for _, r := range doc.HTTP.Routers {
		rules = append(rules, r.Rule)
	}
	slices.Sort(rules)
	rules = slices.Compact(rules)
	if !slices.Equal(rules, []string{"Host(`legacy.example.org`)", "Host(`panel.beta.example`)"}) {
		t.Fatalf("rules = %v", rules)
	}
}
