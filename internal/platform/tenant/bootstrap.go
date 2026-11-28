package tenant

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BootstrapDomains gives tenant #1 its host names: <slug>.<baseDomain>
// (when a platform base domain is configured) and the host of
// publicAppURL. A host that already belongs to a tenant is left alone, and
// the first registered host becomes the primary one. It runs after the
// migrations (`messora migrate`), as the owner; other tenants and their
// domains are managed in the platform admin panel. A PUBLIC_APP_URL on a
// platform host (app.<base>) is not registered.
func BootstrapDomains(ctx context.Context, owner *pgxpool.Pool, publicAppURL, baseDomain string) ([]string, error) {
	hosts := NewHosts(publicAppURL, baseDomain)
	var slug string
	if err := owner.QueryRow(ctx, `SELECT slug FROM tenants WHERE id = $1`, DefaultID).Scan(&slug); err != nil {
		return nil, fmt.Errorf("tenant #1: %w", err)
	}
	type domain struct{ host, kind string }
	var want []domain
	if base := NormalizeHost(baseDomain); base != "" {
		want = append(want, domain{slug + "." + base, "subdomain"})
	}
	if u, err := url.Parse(publicAppURL); err == nil {
		// The platform's own addresses (app.<base> ...) never belong to a tenant.
		if h := NormalizeHost(u.Host); h != "" && !strings.HasPrefix(h, ".") && !hosts.IsPlatformLabel(h) {
			want = append(want, domain{h, "custom"})
		}
	}
	var added []string
	for _, d := range want {
		tag, err := owner.Exec(ctx, `
			INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary)
			VALUES ($1, $2, $3, now(), NOT EXISTS (SELECT 1 FROM tenant_domains WHERE tenant_id = $1 AND is_primary))
			ON CONFLICT (hostname) DO NOTHING`, DefaultID, d.host, d.kind)
		if err != nil {
			return added, fmt.Errorf("register %s: %w", d.host, err)
		}
		if tag.RowsAffected() > 0 {
			added = append(added, d.host)
		}
	}
	return added, nil
}
