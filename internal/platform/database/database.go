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

	"github.com/furkanmeclis/messora-app-go/internal/platform/tracing"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/furkanmeclis/messora-app-go/internal/platform/startup"
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

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}
	if err := startup.Retry(ctx, connectTimeout, log, "postgres", func(ctx context.Context) error { return pool.Ping(ctx) }); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
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
