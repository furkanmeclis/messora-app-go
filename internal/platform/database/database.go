// Package database owns the PostgreSQL connection pool, transactions and
// schema migrations.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/furkanmeclis/messora-app-go/internal/platform/startup"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tracing"
	"github.com/furkanmeclis/messora-app-go/migrations"
)

// PoolOptions configures Open.
type PoolOptions struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	ApplicationName string
	// Traced records a span per query (OpenTelemetry).
	Traced bool
	// TenantScoped makes every connection carry the tenant of the context
	// that acquires it (see TenantHooks). Required for the application
	// pool (messora_app); the owner and system pools leave it off.
	TenantScoped bool
}

// Open creates a pgx pool and verifies connectivity, retrying for up to
// connectTimeout so containers can start in any order.
func Open(ctx context.Context, o PoolOptions, connectTimeout time.Duration, log *slog.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(o.DSN)
	if err != nil {
		return nil, errors.New("database: invalid connection string") // never echo the DSN
	}
	if o.MaxConns > 0 {
		cfg.MaxConns = o.MaxConns
	}
	if o.MinConns > 0 {
		cfg.MinConns = o.MinConns
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	if o.ApplicationName != "" {
		cfg.ConnConfig.RuntimeParams["application_name"] = o.ApplicationName
	}
	// Store and compare all timestamps in UTC regardless of server defaults.
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	if o.Traced {
		cfg.ConnConfig.Tracer = tracing.PgxTracer{}
	}
	if o.TenantScoped {
		cfg.PrepareConn, cfg.AfterRelease = TenantHooks()
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}
	if err := startup.Retry(ctx, connectTimeout, log, "postgres", func(ctx context.Context) error { return pool.Ping(tenant.System(ctx)) }); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// resetTimeout bounds the RESET a released connection runs before it goes
// back to the pool.
const resetTimeout = 5 * time.Second

// TenantHooks returns the pgxpool hooks of a tenant-scoped pool. On every
// acquire the connection's app.tenant_id is set from the context (the
// tenant's ID, or empty for tenant.System work); a context with neither
// fails with tenant.ErrNoTenant before any statement runs. On release the
// setting is reset, and a connection whose reset fails is destroyed rather
// than reused. Transactions (pool.Begin) acquire through the same hook, so
// every statement of a request or job runs as its tenant; row-level
// security does the rest (migration 00028).
func TenantHooks() (prepare func(context.Context, *pgx.Conn) (bool, error), release func(*pgx.Conn) bool) {
	prepare = func(ctx context.Context, c *pgx.Conn) (bool, error) {
		v, err := tenant.Setting(ctx)
		if err != nil {
			return true, err // the connection is untouched and goes back to the pool
		}
		if _, err := c.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", v); err != nil {
			return false, fmt.Errorf("database: set tenant: %w", err)
		}
		return true, nil
	}
	release = func(c *pgx.Conn) bool {
		ctx, cancel := context.WithTimeout(context.Background(), resetTimeout)
		defer cancel()
		_, err := c.Exec(ctx, "RESET app.tenant_id")
		return err == nil
	}
	return prepare, release
}

// InTx runs fn inside a transaction, committing on nil and rolling back on
// error or panic.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// migrationLockID is the advisory lock key goose takes so that concurrent
// migrate runs serialize instead of racing. Value: crc32("messora-migrations").
const migrationLockID int64 = 0x4d53524d

func newProvider(pool *pgxpool.Pool, fsys fs.FS) (*goose.Provider, *sql.DB, error) {
	db := stdlib.OpenDBFromPool(pool)
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(migrationLockID))
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return p, db, nil
}

// Migrate applies all pending embedded migrations and returns the resulting
// schema version.
func Migrate(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) (int64, error) {
	p, db, err := newProvider(pool, migrations.FS)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	results, err := p.Up(ctx)
	for _, r := range results {
		log.Info("migration applied", "version", r.Source.Version, "path", r.Source.Path, "duration_ms", r.Duration.Milliseconds())
	}
	if err != nil {
		return 0, fmt.Errorf("migrate up: %w", err)
	}
	return p.GetDBVersion(ctx)
}

// MigrateDownAll rolls back every migration. Used by tests to prove that Down
// sections are correct; never wired to a production command.
func MigrateDownAll(ctx context.Context, pool *pgxpool.Pool) error {
	p, db, err := newProvider(pool, migrations.FS)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = p.DownTo(ctx, 0)
	return err
}

// SchemaStatus reports the applied version and the newest embedded version.
type SchemaStatus struct {
	Current int64 `json:"current"`
	Target  int64 `json:"target"`
}

// UpToDate reports whether no migrations are pending.
func (s SchemaStatus) UpToDate() bool { return s.Current >= s.Target }

// Status returns the schema version without changing anything. It does not
// take the migration lock, so readiness probes never block on a migration.
func Status(ctx context.Context, pool *pgxpool.Pool) (SchemaStatus, error) {
	ctx = tenant.System(ctx) // reads only goose_db_version
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return SchemaStatus{}, err
	}
	cur, target, err := p.GetVersions(ctx)
	if err != nil {
		return SchemaStatus{}, err
	}
	return SchemaStatus{Current: cur, Target: target}, nil
}

// Database roles created by migration 00024.
const (
	AppRole    = "messora_app"
	SystemRole = "messora_system"
)

// SetRolePasswords lets each role log in with its password (empty
// passwords are skipped). It runs as the owner after migrating; the
// statement is built by format() so PostgreSQL quotes the password.
func SetRolePasswords(ctx context.Context, owner *pgxpool.Pool, passwords map[string]string) error {
	for _, role := range []string{AppRole, SystemRole} {
		pw := passwords[role]
		if pw == "" {
			continue
		}
		var stmt string
		if err := owner.QueryRow(ctx, "SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', $1::text, $2::text)", role, pw).Scan(&stmt); err != nil {
			return fmt.Errorf("database: role %s: %w", role, err)
		}
		if _, err := owner.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("database: role %s: %w", role, err)
		}
	}
	return nil
}
