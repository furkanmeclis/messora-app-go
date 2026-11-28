// Package principal describes who is making a request — a signed-in panel
// user or an API key — and carries it through context.
package principal

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Kind of principal.
type Kind string

// Principal kinds.
const (
	KindUser   Kind = "user"
	KindAPIKey Kind = "api_key"
)

// Principal is an authenticated caller.
type Principal struct {
	Kind Kind
	// TenantID is the tenant the session or key belongs to; the API guard
	// refuses it at any other tenant's address.
	TenantID    uuid.UUID
	ID          uuid.UUID // user ID or API key ID
	Name        string
	SessionID   uuid.UUID // users only
	RoleKey     string    // users only
	RoleRank    int       // users only; lower is more privileged
	Permissions []string  // role permissions, or API key scopes (sorted)
	ReauthAt    *time.Time
	ExpiresAt   time.Time
	// MFAPending is true when the user's role requires two-factor
	// authentication that is not enabled yet; the session is then limited
	// to enrolment.
	MFAPending bool
	// Email of a user (audit entries of platform actions).
	Email string
	// Platform is true for a platform admin's session of the admin panel
	// (admin host, msp_ token); TenantID is then uuid.Nil.
	Platform bool
	// Support is set for a platform admin who opened this tenant's panel
	// through support access.
	Support *Support
}

// Support describes a support-access session.
type Support struct {
	GrantID   uuid.UUID
	ReadOnly  bool
	Reason    string
	AdminName string
	ExpiresAt time.Time
}

// ReadOnly reports whether the principal may only read (a read-only
// support session).
func (p *Principal) ReadOnly() bool { return p.Support != nil && p.Support.ReadOnly }

// Has reports whether the principal holds every permission.
func (p *Principal) Has(perms ...string) bool {
	for _, perm := range perms {
		if _, ok := slices.BinarySearch(p.Permissions, perm); !ok {
			return false
		}
	}
	return true
}

// ReauthValid reports whether the user re-authenticated within window.
func (p *Principal) ReauthValid(window time.Duration, now time.Time) bool {
	return p.ReauthAt != nil && now.Sub(*p.ReauthAt) < window
}

// Key is a stable identifier for rate limits and idempotency.
func (p *Principal) Key() string { return string(p.Kind) + ":" + p.ID.String() }

type ctxKey int

const (
	principalKey ctxKey = iota
	metaKey
)

// With stores p in ctx.
func With(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// From returns the principal, or nil for anonymous requests.
func From(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey).(*Principal)
	return p
}

// RequestMeta describes the origin of a request for audit and rate limits.
type RequestMeta struct {
	RequestID string
	IP        netip.Addr // invalid when unknown
	UserAgent string
}

// WithMeta stores request metadata in ctx.
func WithMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, metaKey, m)
}

// Meta returns request metadata (zero value outside HTTP requests).
func Meta(ctx context.Context) RequestMeta {
	m, _ := ctx.Value(metaKey).(RequestMeta)
	return m
}
