package tenant

import (
	"context"
	"net"
	"net/url"
	"strings"
)

// HostKind says what a request host is.
type HostKind int

// Host kinds. A tenant host is a row of tenant_domains; the platform hosts
// derive from the base domain (TENANT_BASE_DOMAIN).
const (
	// HostUnknown: neither a platform host nor a known tenant domain.
	HostUnknown HostKind = iota
	// HostTenant: a company's sub-domain or verified custom domain.
	HostTenant
	// HostPlatform: the public platform address (the base domain, www.
	// and app.): sign-up and e-mail verification.
	HostPlatform
	// HostAdmin: admin.<base>, the platform admin panel.
	HostAdmin
)

func (k HostKind) String() string {
	switch k {
	case HostTenant:
		return "tenant"
	case HostPlatform:
		return "platform"
	case HostAdmin:
		return "admin"
	}
	return "unknown"
}

// AdminLabel is the sub-domain of the platform admin panel.
const AdminLabel = "admin"

// Hosts knows the platform's addresses: the base domain whose sub-domains
// are the tenants, the admin host, and how links are built (the scheme and
// port of PUBLIC_APP_URL, so a development stack on http://localhost:8090
// links to http://acme.messora.localhost:8090).
type Hosts struct {
	// Base is the platform domain (e.g. messora.tech); empty disables the
	// platform hosts.
	Base   string
	scheme string
	port   string
}

// NewHosts derives the platform addresses from PUBLIC_APP_URL and the base
// domain.
func NewHosts(publicAppURL, base string) Hosts {
	h := Hosts{Base: NormalizeHost(base), scheme: "https"}
	if u, err := url.Parse(publicAppURL); err == nil && u.Host != "" {
		if u.Scheme == "http" {
			h.scheme = "http"
		}
		h.port = u.Port()
	}
	return h
}

// Secure reports whether links use https.
func (h Hosts) Secure() bool { return h.scheme == "https" }

// Admin returns the admin panel host ("" without a base domain).
func (h Hosts) Admin() string {
	if h.Base == "" {
		return ""
	}
	return AdminLabel + "." + h.Base
}

// TenantHost returns the platform sub-domain of slug ("" without a base
// domain).
func (h Hosts) TenantHost(slug string) string {
	if h.Base == "" {
		return ""
	}
	return slug + "." + h.Base
}

// Kind classifies host without looking up tenant domains: HostAdmin,
// HostPlatform, or HostTenant for anything else (which may still be
// unknown).
func (h Hosts) Kind(host string) HostKind {
	host = NormalizeHost(host)
	if h.Base == "" || host == "" {
		return HostTenant
	}
	switch host {
	case h.Admin():
		return HostAdmin
	case h.Base, "www." + h.Base, "app." + h.Base:
		return HostPlatform
	}
	return HostTenant
}

// URL returns the origin of host: scheme://host[:port].
func (h Hosts) URL(host string) string {
	if h.port != "" {
		host = net.JoinHostPort(host, h.port)
	}
	return h.scheme + "://" + host
}

// PlatformURL is the public platform address (app.<base>), or "" without a
// base domain.
func (h Hosts) PlatformURL() string {
	if h.Base == "" {
		return ""
	}
	return h.URL("app." + h.Base)
}

// AdminURL is the origin of the admin panel ("" without a base domain).
func (h Hosts) AdminURL() string {
	if h.Base == "" {
		return ""
	}
	return h.URL(h.Admin())
}

// IsPlatformLabel reports whether host is one of the platform's own
// addresses under base (used to keep them out of tenant_domains).
func (h Hosts) IsPlatformLabel(host string) bool {
	k := h.Kind(host)
	return k == HostAdmin || k == HostPlatform || (h.Base != "" && strings.HasSuffix(NormalizeHost(host), "."+h.Base) &&
		IsReserved(strings.TrimSuffix(NormalizeHost(host), "."+h.Base)))
}

type hostKindKey struct{}

// WithHostKind records what kind of host the request came to.
func WithHostKind(ctx context.Context, k HostKind) context.Context {
	return context.WithValue(ctx, hostKindKey{}, k)
}

// HostKindOf returns the kind of host the request came to (HostUnknown
// outside requests).
func HostKindOf(ctx context.Context) HostKind {
	k, _ := ctx.Value(hostKindKey{}).(HostKind)
	return k
}
