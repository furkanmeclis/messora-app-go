package tenant

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/logger"
)

// HostHeader carries the host the browser asked for from the panel BFF,
// which reaches the API on its internal address. It is believed only on
// requests that also carry the BFF secret (httpx.IdentifyBFF).
const HostHeader = "X-Messora-Tenant-Host"

// Tenant statuses.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusDeleted   = "deleted"
)

// Info is a resolved tenant.
type Info struct {
	ID     uuid.UUID
	Slug   string
	Name   string
	Status string
}

// Errors of Resolve.
var (
	ErrUnknownHost = errors.New("tenant: no tenant at this host")
	ErrSuspended   = errors.New("tenant: suspended")
)

// Resolver maps request hosts to tenants through tenant_domains, with a
// short in-process cache. Unknown hosts are cached too (briefly), so random
// Host headers cannot turn every request into a query.
type Resolver struct {
	find             func(ctx context.Context, host string) (Info, bool, error)
	ttl, negativeTTL time.Duration
	now              func() time.Time

	mu      sync.Mutex
	entries map[string]resolved
}

type resolved struct {
	info    Info
	found   bool
	expires time.Time
}

// maxCachedHosts bounds the cache; past it the cache starts over.
const maxCachedHosts = 10_000

// NewResolver returns a resolver reading pool (the application pool: the
// tables it reads are platform tables, readable without a tenant). A
// suspension or a removed domain takes effect within ttl on every replica,
// at once on the replica that made the change (Invalidate).
func NewResolver(pool *pgxpool.Pool, ttl time.Duration) *Resolver {
	return newResolver(func(ctx context.Context, host string) (Info, bool, error) { return lookup(ctx, pool, host) }, ttl)
}

func newResolver(find func(ctx context.Context, host string) (Info, bool, error), ttl time.Duration) *Resolver {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	return &Resolver{find: find, ttl: ttl, negativeTTL: min(ttl, 5*time.Second), now: time.Now, entries: map[string]resolved{}}
}

// NormalizeHost lowercases host, drops a port and a trailing dot, and
// returns "" when what is left is not a plausible DNS name.
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" || len(host) > 253 {
		return ""
	}
	for _, c := range host {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '.' {
			return ""
		}
	}
	return host
}

// Resolve returns the tenant reached at host. Deleted tenants and
// unverified domains are unknown; suspended tenants are returned together
// with ErrSuspended.
func (r *Resolver) Resolve(ctx context.Context, host string) (Info, error) {
	host = NormalizeHost(host)
	if host == "" {
		return Info{}, ErrUnknownHost
	}
	now := r.now()
	r.mu.Lock()
	e, ok := r.entries[host]
	r.mu.Unlock()
	if !ok || !now.Before(e.expires) {
		info, found, err := r.find(ctx, host)
		if err != nil {
			return Info{}, err
		}
		ttl := r.ttl
		if !found {
			ttl = r.negativeTTL
		}
		e = resolved{info: info, found: found, expires: now.Add(ttl)}
		r.mu.Lock()
		if len(r.entries) >= maxCachedHosts {
			clear(r.entries)
		}
		r.entries[host] = e
		r.mu.Unlock()
	}
	switch {
	case !e.found || e.info.Status == StatusDeleted:
		return Info{}, ErrUnknownHost
	case e.info.Status != StatusActive:
		return e.info, ErrSuspended
	}
	return e.info, nil
}

func lookup(ctx context.Context, pool *pgxpool.Pool, host string) (Info, bool, error) {
	var i Info
	err := pool.QueryRow(System(ctx), `
		SELECT t.id, t.slug, t.name, t.status
		FROM tenant_domains d JOIN tenants t ON t.id = d.tenant_id
		WHERE d.hostname = $1 AND d.verified_at IS NOT NULL`, host).Scan(&i.ID, &i.Slug, &i.Name, &i.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Info{}, false, nil
	}
	if err != nil {
		return Info{}, false, err
	}
	return i, true, nil
}

// Invalidate forgets the cached answer for each host (all hosts when none
// is given), after a domain or tenant status change.
func (r *Resolver) Invalidate(hosts ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(hosts) == 0 {
		clear(r.entries)
		return
	}
	for _, h := range hosts {
		delete(r.entries, NormalizeHost(h))
	}
}

// RequestHost returns the host a request was addressed to: the BFF's
// HostHeader on requests that proved the BFF secret, the Host header
// otherwise. The edge proxy (Caddy) passes Host through unchanged;
// X-Forwarded-Host is not used, since a direct caller could set it.
func RequestHost(r *http.Request) string {
	if httpx.FromBFF(r.Context()) {
		if h := r.Header.Get(HostHeader); h != "" {
			return h
		}
	}
	return r.Host
}

// Middleware resolves the request's tenant and stores it in the context.
// It must run after httpx.IdentifyBFF. The platform's own hosts (see
// Hosts) carry no tenant: the admin host runs as platform work, the public
// platform host as tenant-less system work; the API guard decides which
// operations each kind of host serves. Unknown hosts get 404, suspended
// tenants 403 TENANT_SUSPENDED.
func Middleware(res *Resolver, hosts Hosts, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := RequestHost(r)
			// Nothing downstream may read (or forward) the claimed host.
			r.Header.Del(HostHeader)
			switch hosts.Kind(host) {
			case HostAdmin:
				ctx := WithHostKind(Platform(r.Context()), HostAdmin)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			case HostPlatform:
				ctx := WithHostKind(System(r.Context()), HostPlatform)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			info, err := res.Resolve(r.Context(), host)
			switch {
			case errors.Is(err, ErrUnknownHost):
				httpx.Error(w, r, http.StatusNotFound, "TENANT_NOT_FOUND", "There is no Messora workspace at this address")
				return
			case errors.Is(err, ErrSuspended):
				httpx.Error(w, r, http.StatusForbidden, "TENANT_SUSPENDED", "This workspace is suspended")
				return
			case err != nil:
				log.ErrorContext(r.Context(), "resolve tenant", "err", err)
				httpx.Error(w, r, http.StatusServiceUnavailable, httpx.CodeUnavailable, "Service temporarily unavailable")
				return
			}
			ctx := WithHostKind(WithID(r.Context(), info.ID), HostTenant)
			ctx = logger.WithAttrs(ctx, slog.String("tenant_id", info.ID.String()))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
