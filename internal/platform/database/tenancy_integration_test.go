//go:build integration

package database_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
	"github.com/furkanmeclis/messora-app-go/migrations"
)

// platformTables have no tenant: the tenant registry, platform settings,
// the permission catalogue and the price tables shared by every tenant.
// A new table must be tenant-scoped or be added here deliberately.
var platformTables = []string{
	"goose_db_version", "tenants", "tenant_domains", "platform_settings",
	"permissions", "roles", "role_permissions",
	"meta_rates", "meta_rate_markets", "ai_model_prices", "meta_webhook_keys", "meta_routes",
	// Global accounts (WP2): a person, their password and second factor
	// belong to no company; memberships and sessions do.
	"users", "user_recovery_codes",
	// The platform admin panel.
	"platform_sessions", "email_verification_tokens", "platform_audit_logs",
	// Plans and Messora's own AI keys (WP8).
	"plans", "platform_ai_providers", "platform_ai_models",
	// Incoming bank transfers (WP9): recorded by the platform admin before
	// any company is known; tenant_id names the company once matched.
	"bank_transfers",
}

// routingTables are platform tables whose tenant_id names the tenant a
// row is about: routing (read before any tenant is known), the sign-up to
// verify, the audit subject.
var routingTables = []string{"tenant_domains", "meta_webhook_keys", "email_verification_tokens", "platform_audit_logs", "meta_routes", "bank_transfers"}

// nullableTenant may hold rows without a tenant (platform events).
var nullableTenant = []string{"outbox_events"}

// TestEveryTableIsTenantScoped is the CI guard of docs/SECURITY.md: every
// table outside the platform allowlist has a NOT NULL tenant_id, row-level
// security enabled and forced, and the tenant_isolation policy with both a
// USING and a WITH CHECK expression on app_tenant_id().
func TestEveryTableIsTenantScoped(t *testing.T) {
	db := testutil.TenantDB(t)
	rows, err := db.Owner.Query(context.Background(), `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity,
		       a.attname IS NOT NULL, coalesce(a.attnotnull, false),
		       coalesce(p.qual, ''), coalesce(p.with_check, ''), p.cmd = 'ALL'
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
		LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped
		LEFT JOIN pg_policies p ON p.schemaname = 'public' AND p.tablename = c.relname AND p.policyname = 'tenant_isolation'
		WHERE c.relkind IN ('r', 'p')
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	scoped := 0
	for rows.Next() {
		var name, qual, check string
		var rls, force, hasCol, notNull bool
		var all *bool
		if err := rows.Scan(&name, &rls, &force, &hasCol, &notNull, &qual, &check, &all); err != nil {
			t.Fatal(err)
		}
		if slices.Contains(platformTables, name) {
			// tenant_domains belongs to the registry: its tenant_id names
			// the tenant a host routes to, read before any tenant is known.
			if hasCol && !slices.Contains(routingTables, name) {
				t.Errorf("platform table %s has a tenant_id: scope it or drop it from the allowlist", name)
			}
			continue
		}
		scoped++
		switch {
		case !hasCol:
			t.Errorf("%s has no tenant_id (tenant-scope it, or add it to platformTables if it is truly platform-wide)", name)
		case !notNull && !slices.Contains(nullableTenant, name):
			t.Errorf("%s.tenant_id is nullable", name)
		}
		if !rls || !force {
			t.Errorf("%s: row-level security enabled=%v forced=%v", name, rls, force)
		}
		if all == nil || !*all || !strings.Contains(qual, "app_tenant_id()") || !strings.Contains(check, "app_tenant_id()") {
			t.Errorf("%s: tenant_isolation policy missing or incomplete (using %q, check %q)", name, qual, check)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if scoped < 50 {
		t.Fatalf("only %d tenant tables checked", scoped)
	}
}

// The service roles: messora_app cannot bypass row-level security and owns
// nothing; messora_system bypasses it; neither is a superuser.
func TestDatabaseRoles(t *testing.T) {
	db := testutil.TenantDB(t)
	ctx := context.Background()
	for role, wantBypass := range map[string]bool{database.AppRole: false, database.SystemRole: true} {
		var super, bypass, login bool
		if err := db.Owner.QueryRow(ctx, `SELECT rolsuper, rolbypassrls, rolcanlogin FROM pg_roles WHERE rolname = $1`, role).Scan(&super, &bypass, &login); err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		if super || bypass != wantBypass || !login {
			t.Errorf("%s: super=%v bypassrls=%v login=%v", role, super, bypass, login)
		}
		var owned int
		if err := db.Owner.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_roles r ON r.oid = c.relowner WHERE r.rolname = $1`, role).Scan(&owned); err != nil || owned != 0 {
			t.Errorf("%s owns %d relations (%v)", role, owned, err)
		}
	}
	// The app role reads, but cannot change, the tenant registry.
	app := tenant.System(ctx)
	var n int
	if err := db.App.QueryRow(app, "SELECT count(*) FROM tenants").Scan(&n); err != nil || n != 1 {
		t.Fatalf("app role reads tenants: %d %v", n, err)
	}
	for _, stmt := range []string{
		"UPDATE tenants SET status = 'active'",
		"INSERT INTO tenant_domains (tenant_id, hostname, kind) VALUES ('" + tenant.DefaultID.String() + "', 'x.test', 'custom')",
		"INSERT INTO platform_settings (key, value) VALUES ('x', '{}')",
	} {
		if _, err := db.App.Exec(app, stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s: %v, want permission denied", stmt, err)
		}
	}
}

func TestReservedSlugsMatchTheDatabase(t *testing.T) {
	db := testutil.TenantDB(t)
	var dbList []string
	if err := db.Owner.QueryRow(context.Background(), "SELECT tenant_reserved_slugs()").Scan(&dbList); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dbList, tenant.ReservedSlugs) {
		t.Fatalf("tenant_reserved_slugs() %v\ntenant.ReservedSlugs %v", dbList, tenant.ReservedSlugs)
	}
	if _, err := db.Owner.Exec(context.Background(), "INSERT INTO tenants (slug, name) VALUES ('admin', 'x')"); err == nil {
		t.Fatal("a reserved slug was accepted")
	}
}

// twoTenants returns tenant #1 and a second tenant with a contact each.
func twoTenants(t *testing.T, db *testutil.DB) (a, b uuid.UUID, contactA, contactB uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a = tenant.DefaultID
	if err := db.Owner.QueryRow(ctx, "INSERT INTO tenants (slug, name) VALUES ('beta', 'Beta') RETURNING id").Scan(&b); err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		tid uuid.UUID
		out *uuid.UUID
	}{{a, &contactA}, {b, &contactB}} {
		// The same phone number in both tenants: unique per tenant only.
		if err := db.App.QueryRow(tenant.WithID(ctx, x.tid), "INSERT INTO contacts (phone_e164, name) VALUES ('+905550000001', 'c') RETURNING id").Scan(x.out); err != nil {
			t.Fatal(err)
		}
	}
	return a, b, contactA, contactB
}

func TestRowLevelSecurityIsolatesTenants(t *testing.T) {
	db := testutil.TenantDB(t)
	a, b, contactA, contactB := twoTenants(t, db)
	ctxA, ctxB := tenant.WithID(context.Background(), a), tenant.WithID(context.Background(), b)

	// Each tenant sees only its own rows, even when asking by ID.
	var ids []uuid.UUID
	rows, err := db.App.Query(ctxB, "SELECT id FROM contacts")
	if err != nil {
		t.Fatal(err)
	}
	ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil || len(ids) != 1 || ids[0] != contactB {
		t.Fatalf("tenant B sees %v (%v)", ids, err)
	}
	var n int
	if err := db.App.QueryRow(ctxB, "SELECT count(*) FROM contacts WHERE id = $1", contactA).Scan(&n); err != nil || n != 0 {
		t.Fatalf("tenant B reads A's contact: %d %v", n, err)
	}
	// Writes cannot reach the other tenant's rows ...
	if tag, err := db.App.Exec(ctxB, "UPDATE contacts SET name = 'pwned' WHERE id = $1", contactA); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("tenant B updated A's contact: %v %v", tag, err)
	}
	if tag, err := db.App.Exec(ctxB, "DELETE FROM contacts WHERE id = $1", contactA); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("tenant B deleted A's contact: %v %v", tag, err)
	}
	// ... and cannot create rows in it or move a row to it.
	if _, err := db.App.Exec(ctxB, "INSERT INTO contacts (tenant_id, phone_e164) VALUES ($1, '+905550000002')", a); err == nil ||
		!strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("tenant B inserted into A: %v", err)
	}
	if _, err := db.App.Exec(ctxB, "UPDATE contacts SET tenant_id = $1 WHERE id = $2", a, contactB); err == nil {
		t.Fatal("tenant B moved its row to A")
	}
	// A row written without naming the tenant takes the connection's.
	var got uuid.UUID
	if err := db.App.QueryRow(ctxB, "INSERT INTO tags (name) VALUES ('vip') RETURNING tenant_id").Scan(&got); err != nil || got != b {
		t.Fatalf("default tenant_id = %s %v", got, err)
	}
	// Tenant-less (system) work on the app pool sees no tenant row and
	// cannot write one.
	sys := tenant.System(context.Background())
	if err := db.App.QueryRow(sys, "SELECT count(*) FROM contacts").Scan(&n); err != nil || n != 0 {
		t.Fatalf("system context on the app pool sees %d contacts (%v)", n, err)
	}
	if _, err := db.App.Exec(sys, "INSERT INTO tags (name) VALUES ('x')"); err == nil {
		t.Fatal("system context on the app pool inserted a tenant row")
	}
	// The system pool bypasses row-level security (relay, maintenance).
	if err := db.System.QueryRow(sys, "SELECT count(*) FROM contacts").Scan(&n); err != nil || n != 2 {
		t.Fatalf("system pool sees %d contacts (%v)", n, err)
	}
	// A unchanged.
	var name string
	if err := db.App.QueryRow(ctxA, "SELECT name FROM contacts WHERE id = $1", contactA).Scan(&name); err != nil || name != "c" {
		t.Fatalf("A's contact: %q %v", name, err)
	}
	// A malformed setting fails rather than widening access.
	err = database.InTx(ctxA, db.App, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctxA, "SELECT set_config('app.tenant_id', 'not-a-uuid', true)"); err != nil {
			return err
		}
		return tx.QueryRow(ctxA, "SELECT count(*) FROM contacts").Scan(&n)
	})
	if err == nil {
		t.Fatalf("a malformed tenant setting read %d rows", n)
	}
}

// A child row can never point at another tenant's parent, even for a
// caller that bypasses row-level security.
func TestCompositeForeignKeys(t *testing.T) {
	db := testutil.TenantDB(t)
	_, b, contactA, _ := twoTenants(t, db)
	ctx := context.Background()
	var tagB uuid.UUID
	if err := db.Owner.QueryRow(ctx, "INSERT INTO tags (tenant_id, name) VALUES ($1, 'b') RETURNING id", b).Scan(&tagB); err != nil {
		t.Fatal(err)
	}
	_, err := db.Owner.Exec(ctx, "INSERT INTO contact_tags (tenant_id, contact_id, tag_id) VALUES ($1, $2, $3)", b, contactA, tagB)
	if err == nil || !strings.Contains(err.Error(), "foreign key") {
		t.Fatalf("cross-tenant join row: %v", err)
	}
}

func TestTenantPoolFailsClosed(t *testing.T) {
	db := testutil.TenantDB(t)
	ctx := context.Background()
	// No tenant, no system marker: nothing reaches the database.
	if _, err := db.App.Exec(ctx, "SELECT 1"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("exec without a tenant: %v", err)
	}
	if _, err := db.App.Begin(ctx); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("begin without a tenant: %v", err)
	}
	if _, err := db.App.Acquire(ctx); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("acquire without a tenant: %v", err)
	}
	// Transactions run as their tenant throughout.
	a := tenant.WithID(ctx, tenant.DefaultID)
	if err := database.InTx(a, db.App, func(tx pgx.Tx) error {
		var v string
		if err := tx.QueryRow(a, "SELECT current_setting('app.tenant_id')").Scan(&v); err != nil || v != tenant.DefaultID.String() {
			t.Errorf("in tx: %q %v", v, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// A released connection is reset before anyone else gets it: with a
	// single connection, the next (system) user must see no tenant, and
	// the next tenant its own.
	cfg := db.App.Config().ConnConfig
	u := url.URL{Scheme: "postgres", User: url.UserPassword(cfg.User, cfg.Password),
		Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable"}
	one, err := database.Open(ctx, database.PoolOptions{DSN: u.String(), MaxConns: 1, TenantScoped: true}, 30*time.Second, testutil.DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	var pidA, pidSys int
	if err := one.QueryRow(a, "SELECT pg_backend_pid()").Scan(&pidA); err != nil {
		t.Fatal(err)
	}
	var v string
	if err := one.QueryRow(tenant.System(ctx), "SELECT current_setting('app.tenant_id', true), pg_backend_pid()").Scan(&v, &pidSys); err != nil {
		t.Fatal(err)
	}
	if pidSys != pidA || v != "" {
		t.Fatalf("after release: setting %q on backend %d (was %d)", v, pidSys, pidA)
	}
	other := uuid.New()
	if err := one.QueryRow(tenant.WithID(ctx, other), "SELECT current_setting('app.tenant_id')").Scan(&v); err != nil || v != other.String() {
		t.Fatalf("next tenant sees %q %v", v, err)
	}
}

// Upgrading a database with data: every row lands in tenant #1, whose slug
// comes from the organisation name, and the install answers on its
// PUBLIC_APP_URL host after `messora migrate`.
func TestMigrationMovesExistingDataToTenantOne(t *testing.T) {
	ctx := context.Background()
	owner := testutil.Postgres(t)
	sqlDB := stdlib.OpenDBFromPool(owner)
	defer sqlDB.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 23); err != nil {
		t.Fatal(err)
	}
	seed := `
INSERT INTO settings (key, value) VALUES ('workspace', '{"organization_name": "Acme Şirketi"}');
INSERT INTO audit_logs (actor_type, action) VALUES ('system', 'setup.complete');
WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc) VALUES ('a', '1', '\x00') RETURNING id),
     n AS (INSERT INTO phone_numbers (account_id, phone_number_id) SELECT id, '2' FROM a RETURNING id),
     c AS (INSERT INTO contacts (phone_e164) VALUES ('+905550000001') RETURNING id),
     g AS (INSERT INTO tags (name) VALUES ('vip') RETURNING id),
     ct AS (INSERT INTO contact_tags (contact_id, tag_id) SELECT c.id, g.id FROM c, g),
     k AS (INSERT INTO campaigns (name, phone_number_id, template_name, template_language, audience) SELECT 'k', n.id, 't', 'en', '{}' FROM n RETURNING id)
INSERT INTO messages (phone_number_id, contact_id, direction, type, body, status, campaign_id)
SELECT n.id, c.id, 'outbound', 'text', 'hi', 'sent', k.id FROM n, c, k;`
	if _, err := owner.Exec(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var slug, name string
	if err := owner.QueryRow(ctx, "SELECT slug, name FROM tenants WHERE id = $1", tenant.DefaultID).Scan(&slug, &name); err != nil || slug != "acme-sirketi" || name != "Acme Şirketi" {
		t.Fatalf("tenant #1 = %q %q %v", slug, name, err)
	}
	for _, table := range []string{"settings", "audit_logs", "whatsapp_accounts", "phone_numbers", "contacts", "tags", "contact_tags", "campaigns", "campaign_counters", "messages", "outbox_events"} {
		var total, first int
		if err := owner.QueryRow(ctx, "SELECT count(*), count(*) FILTER (WHERE tenant_id = $1) FROM "+pgx.Identifier{table}.Sanitize(), tenant.DefaultID).Scan(&total, &first); err != nil {
			t.Fatal(err)
		}
		if total != first || (total == 0 && table != "outbox_events") {
			t.Errorf("%s: %d rows, %d in tenant #1", table, total, first)
		}
	}
	added, err := tenant.BootstrapDomains(ctx, owner, "https://messora.example.com", "messora.tech")
	if err != nil || !slices.Equal(added, []string{"acme-sirketi.messora.tech", "messora.example.com"}) {
		t.Fatalf("bootstrap domains %v %v", added, err)
	}
	// Idempotent.
	if again, err := tenant.BootstrapDomains(ctx, owner, "https://messora.example.com", "messora.tech"); err != nil || len(again) != 0 {
		t.Fatalf("second bootstrap %v %v", again, err)
	}
	var primary string
	if err := owner.QueryRow(ctx, "SELECT hostname FROM tenant_domains WHERE is_primary").Scan(&primary); err != nil || primary != "acme-sirketi.messora.tech" {
		t.Fatalf("primary domain %q %v", primary, err)
	}
	res := tenant.NewResolver(owner, time.Minute)
	for _, host := range []string{"messora.example.com", "ACME-SIRKETI.messora.tech:443"} {
		if info, err := res.Resolve(ctx, host); err != nil || info.ID != tenant.DefaultID {
			t.Errorf("resolve %s: %v %v", host, info, err)
		}
	}
	if _, err := res.Resolve(ctx, "other.messora.tech"); !errors.Is(err, tenant.ErrUnknownHost) {
		t.Errorf("unknown host: %v", err)
	}
}

// WP2: accounts become global. Every user of tenant #1 becomes a member
// with their role and status, recovery codes stay with the person, and the
// account keeps working.
func TestMigrationMovesUsersIntoMemberships(t *testing.T) {
	ctx := context.Background()
	owner := testutil.Postgres(t)
	sqlDB := stdlib.OpenDBFromPool(owner)
	defer sqlDB.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 30); err != nil {
		t.Fatal(err)
	}
	seed := `
SELECT set_config('app.tenant_id', '00000000-0000-0000-0000-000000000001', false);
WITH o AS (INSERT INTO users (email, name, password_hash) VALUES ('owner@example.com', 'Owner', 'x') RETURNING id),
     a AS (INSERT INTO users (email, name, password_hash, status) VALUES ('agent@example.com', 'Agent', 'x', 'disabled') RETURNING id),
     r1 AS (INSERT INTO user_roles (user_id, role_id) SELECT o.id, r.id FROM o, roles r WHERE r.key = 'owner'),
     r2 AS (INSERT INTO user_roles (user_id, role_id) SELECT a.id, r.id FROM a, roles r WHERE r.key = 'agent')
INSERT INTO user_recovery_codes (user_id, code_hash) SELECT id, '\x01' FROM o;`
	if _, err := owner.Exec(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, "RESET app.tenant_id"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := owner.Query(ctx, `
		SELECT u.email, u.status, u.email_verified_at IS NOT NULL, m.tenant_id, r.key, m.status
		FROM users u JOIN tenant_memberships m ON m.user_id = u.id JOIN roles r ON r.id = m.role_id ORDER BY u.email`)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		email, status    string
		verified         bool
		tenantID         uuid.UUID
		role, membership string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.email, &r.status, &r.verified, &r.tenantID, &r.role, &r.membership); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	rows.Close()
	want := []row{
		{"agent@example.com", "active", true, tenant.DefaultID, "agent", "disabled"},
		{"owner@example.com", "active", true, tenant.DefaultID, "owner", "active"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("memberships %+v, want %+v", got, want)
	}
	var codes int
	if err := owner.QueryRow(ctx, "SELECT count(*) FROM user_recovery_codes").Scan(&codes); err != nil || codes != 1 {
		t.Fatalf("recovery codes %d %v", codes, err)
	}
	// Down restores the per-tenant accounts while nobody is in two tenants
	// (down to 30: later migrations are undone first).
	if _, err := p.DownTo(ctx, 30); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := owner.QueryRow(ctx, "SELECT status FROM users WHERE email = 'agent@example.com' AND tenant_id = $1", tenant.DefaultID).Scan(&status); err != nil || status != "disabled" {
		t.Fatalf("down: agent %q %v", status, err)
	}
}

// messora_platform is reached only through messora_app (SET ROLE), cannot
// log in or bypass row-level security, and owns nothing.
func TestPlatformRole(t *testing.T) {
	db := testutil.TenantDB(t)
	ctx := context.Background()
	var super, bypass, login bool
	if err := db.Owner.QueryRow(ctx, `SELECT rolsuper, rolbypassrls, rolcanlogin FROM pg_roles WHERE rolname = 'messora_platform'`).Scan(&super, &bypass, &login); err != nil {
		t.Fatal(err)
	}
	if super || bypass || login {
		t.Fatalf("messora_platform: super=%v bypassrls=%v login=%v", super, bypass, login)
	}
	// messora_app does not inherit it: it cannot write the registry...
	if _, err := db.App.Exec(tenant.System(ctx), "UPDATE tenants SET name = name"); err == nil {
		t.Fatal("messora_app wrote tenants")
	}
	if _, err := db.App.Exec(tenant.System(ctx), "UPDATE ai_model_prices SET notes = notes"); err == nil {
		t.Fatal("messora_app wrote the AI price table")
	}
	// ...but platform work can, and sees every tenant's memberships.
	pctx := tenant.Platform(ctx)
	if _, err := db.App.Exec(pctx, "UPDATE tenants SET name = name"); err != nil {
		t.Fatalf("platform work cannot write tenants: %v", err)
	}
	var role string
	if err := db.App.QueryRow(pctx, "SELECT current_user").Scan(&role); err != nil || role != "messora_platform" {
		t.Fatalf("platform connection runs as %q %v", role, err)
	}
	// The role is reset when the connection goes back to the pool.
	for range 5 {
		if err := db.App.QueryRow(tenant.System(ctx), "SELECT current_user").Scan(&role); err != nil || role != "messora_app" {
			t.Fatalf("pooled connection still %q %v", role, err)
		}
	}
	// Other tenant tables stay limited to app.tenant_id.
	var n int
	if err := db.App.QueryRow(pctx, "SELECT count(*) FROM settings").Scan(&n); err != nil || n != 0 {
		t.Fatalf("platform work sees %d settings rows without a tenant (%v)", n, err)
	}
}
