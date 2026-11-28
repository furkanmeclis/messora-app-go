// Package edge serves what the edge proxy asks the API on the internal
// listener (INTERNAL_ADDR, never routed publicly), docs/OPERATIONS.md §TLS:
//
//   - GET /internal/tls/allow?domain=<host> is Caddy's on-demand TLS "ask"
//     endpoint (EDGE_MODE=caddy). It answers 200 only for the platform's own
//     hosts and for verified domains of active tenants, so a stranger
//     pointing DNS at the server cannot make Caddy request certificates.
//   - GET /internal/edge/traefik is a Traefik HTTP-provider document with
//     one HTTP-01 router per verified custom domain (EDGE_MODE=proxy behind
//     Dokploy's Traefik), so a domain verified in the platform panel gets a
//     certificate without editing the proxy by hand.
package edge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// Paths on the internal listener.
const (
	AllowPath   = "/internal/tls/allow"
	TraefikPath = "/internal/edge/traefik"
)

// PlatformLabels are the sub-domains of the base domain that belong to the
// platform itself rather than to a tenant (docs/INSTALL.md, host layout):
// the sign-up address (app, www and the bare domain), the admin panel and
// the webhook host.
var PlatformLabels = []string{"app", "www", "admin", "hooks"}

// PlatformHosts lists the platform's own host names for baseDomain (none
// without a base domain).
func PlatformHosts(baseDomain string) []string {
	base := tenant.NormalizeHost(baseDomain)
	if base == "" {
		return nil
	}
	hosts := []string{base}
	for _, l := range PlatformLabels {
		hosts = append(hosts, l+"."+base)
	}
	return hosts
}

// Resolver finds the tenant at a host (tenant.Resolver).
type Resolver interface {
	Resolve(ctx context.Context, host string) (tenant.Info, error)
}

// Options configures the handlers.
type Options struct {
	// Resolver answers which tenant a host belongs to (verified domains
	// only; suspended and deleted tenants are refused).
	Resolver Resolver
	// Pool is the application pool; tenant_domains and tenants are
	// platform tables it reads without a tenant.
	Pool *pgxpool.Pool
	// BaseDomain is TENANT_BASE_DOMAIN: its platform hosts are always
	// allowed, and domains under it are left to the wildcard certificate.
	BaseDomain string
	// AskRate and AskBurst bound the ask endpoint (default 10/s, burst 50).
	// Caddy calls it once per new host name, not per request; a flood of
	// handshakes for random names must not become a flood of queries.
	AskRate  rate.Limit
	AskBurst int
	Logger   *slog.Logger
}

// Mount registers the handlers on the internal router.
func Mount(r chi.Router, o Options) {
	r.Get(AllowPath, AllowHandler(o))
	r.Get(TraefikPath, TraefikHandler(o))
}

// AllowHandler is Caddy's on_demand_tls ask endpoint.
func AllowHandler(o Options) http.HandlerFunc {
	if o.AskRate == 0 {
		o.AskRate = 10
	}
	if o.AskBurst == 0 {
		o.AskBurst = 50
	}
	lim := rate.NewLimiter(o.AskRate, o.AskBurst)
	platform := PlatformHosts(o.BaseDomain)
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("domain")
		host := tenant.NormalizeHost(raw)
		if host == "" || host != strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), ".")) {
			// A port, a path or anything that is not a DNS name.
			http.Error(w, "invalid domain", http.StatusBadRequest)
			return
		}
		if slices.Contains(platform, host) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if !lim.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		_, err := o.Resolver.Resolve(ctx, host)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusOK)
		case errors.Is(err, tenant.ErrUnknownHost), errors.Is(err, tenant.ErrSuspended):
			http.Error(w, "not a domain of an active workspace", http.StatusForbidden)
		default:
			log.ErrorContext(r.Context(), "tls ask: resolve host", "host", host, "err", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}
	}
}

// CustomDomains returns the verified custom domains of active tenants that
// lie outside baseDomain (those under it are covered by the wildcard
// certificate), sorted.
func CustomDomains(ctx context.Context, pool *pgxpool.Pool, baseDomain string) ([]string, error) {
	rows, err := pool.Query(tenant.System(ctx), `
		SELECT d.hostname
		FROM tenant_domains d JOIN tenants t ON t.id = d.tenant_id
		WHERE d.kind = 'custom' AND d.verified_at IS NOT NULL AND t.status = 'active'
		ORDER BY d.hostname`)
	if err != nil {
		return nil, err
	}
	hosts, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	base := tenant.NormalizeHost(baseDomain)
	return slices.DeleteFunc(hosts, func(h string) bool {
		// Single-label names (localhost) never get a public certificate.
		return !strings.Contains(h, ".") || (base != "" && (h == base || strings.HasSuffix(h, "."+base)))
	}), nil
}

// TraefikOptions are the names of the proxy's own objects, given as query
// parameters of the provider URL so the Traefik static configuration holds
// them: upstream (default http://messora-edge:80), web and websecure
// (entry points), resolver (the HTTP-01 certificate resolver, default
// letsencrypt, Dokploy's name).
type TraefikOptions struct {
	Upstream  string
	Web       string
	WebSecure string
	Resolver  string
}

var traefikName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]{0,62}$`)

// parseTraefikOptions reads and validates the query parameters.
func parseTraefikOptions(q url.Values) (TraefikOptions, error) {
	o := TraefikOptions{Upstream: "http://messora-edge:80", Web: "web", WebSecure: "websecure", Resolver: "letsencrypt"}
	for key, dst := range map[string]*string{"upstream": &o.Upstream, "web": &o.Web, "websecure": &o.WebSecure, "resolver": &o.Resolver} {
		if v := strings.TrimSpace(q.Get(key)); v != "" {
			*dst = v
		}
	}
	u, err := url.Parse(o.Upstream)
	if err != nil || u.Scheme != "http" || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		return o, errors.New("upstream must be http://host:port")
	}
	for _, v := range []string{o.Web, o.WebSecure, o.Resolver} {
		if !traefikName.MatchString(v) {
			return o, errors.New("invalid entry point or resolver name")
		}
	}
	return o, nil
}

// TraefikConfig renders the dynamic configuration for hosts: per host an
// HTTPS router with its own certificate from the HTTP-01 resolver and an
// HTTP router redirecting to it, all to one service (the stack's Caddy).
func TraefikConfig(hosts []string, o TraefikOptions) map[string]any {
	const service, redirect = "messora-custom-domains", "messora-custom-domains-https"
	routers := map[string]any{}
	for _, h := range hosts {
		sum := sha256.Sum256([]byte(h))
		name := "messora-cd-" + hex.EncodeToString(sum[:6])
		rule := "Host(`" + h + "`)"
		routers[name] = map[string]any{
			"rule": rule, "entryPoints": []string{o.WebSecure}, "service": service,
			"tls": map[string]any{"certResolver": o.Resolver},
		}
		routers[name+"-http"] = map[string]any{
			"rule": rule, "entryPoints": []string{o.Web}, "service": service, "middlewares": []string{redirect},
		}
	}
	if len(routers) == 0 {
		// Traefik rejects a service no router uses only in logs; an empty
		// document is the clean "nothing to route".
		return map[string]any{}
	}
	return map[string]any{"http": map[string]any{
		"routers": routers,
		"middlewares": map[string]any{
			redirect: map[string]any{"redirectScheme": map[string]any{"scheme": "https", "permanent": true}},
		},
		"services": map[string]any{
			service: map[string]any{"loadBalancer": map[string]any{
				"passHostHeader": true,
				"servers":        []map[string]string{{"url": o.Upstream}},
			}},
		},
	}}
}

// TraefikHandler serves TraefikConfig for the current custom domains.
func TraefikHandler(o Options) http.HandlerFunc {
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		opts, err := parseTraefikOptions(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		hosts, err := CustomDomains(ctx, o.Pool, o.BaseDomain)
		if err != nil {
			// A 5xx makes Traefik keep its last good configuration.
			log.ErrorContext(r.Context(), "traefik provider: list custom domains", "err", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, TraefikConfig(hosts, opts))
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}
