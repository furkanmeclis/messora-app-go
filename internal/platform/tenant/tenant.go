// Package tenant carries the tenant (the company whose data a request or
// job touches) through context, and resolves it from the request host.
//
// The database pool reads it on every connection acquire and sets the
// PostgreSQL setting app.tenant_id, which row-level security compares with
// each row's tenant_id (docs/SECURITY.md "Kiracı yalıtımı"). Access is
// fail-closed: a context with neither a tenant nor the System marker cannot
// acquire a connection at all (ErrNoTenant).
package tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// DefaultID is tenant #1, which owns every row that existed before tenants
// (migration 00024).
var DefaultID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// ErrNoTenant is returned when database access is attempted from a context
// that names no tenant and is not marked as system work.
var ErrNoTenant = errors.New("tenant: no tenant in context")

type ctxKey int

const (
	idKey ctxKey = iota
	systemKey
)

// WithID returns ctx scoped to tenant id. A nil id is ignored, so it can
// never stand in for a real tenant.
func WithID(ctx context.Context, id uuid.UUID) context.Context {
	if id == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, idKey, id)
}

// FromCtx returns the tenant of ctx.
func FromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(idKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// ID returns the tenant of ctx, or uuid.Nil.
func ID(ctx context.Context) uuid.UUID {
	id, _ := FromCtx(ctx)
	return id
}

// System marks ctx as deliberate tenant-less work. On the application pool
// (messora_app) such a connection has no tenant, so row-level security
// hides every tenant row and rejects every tenant write: only the platform
// tables (tenants, tenant_domains, the permission catalogue, prices) are
// usable, e.g. for host resolution and health checks. On the system pool
// (messora_system, BYPASSRLS) it is the context of the few cross-tenant
// paths: the outbox relay and maintenance.
func System(ctx context.Context) context.Context {
	return context.WithValue(ctx, systemKey, true)
}

// IsSystem reports whether ctx carries the System marker.
func IsSystem(ctx context.Context) bool {
	ok, _ := ctx.Value(systemKey).(bool)
	return ok
}

// Setting returns the value the pool stores in app.tenant_id for ctx: the
// tenant's ID, "" for system work, or ErrNoTenant.
func Setting(ctx context.Context) (string, error) {
	if id, ok := FromCtx(ctx); ok {
		return id.String(), nil
	}
	if IsSystem(ctx) {
		return "", nil
	}
	return "", ErrNoTenant
}
