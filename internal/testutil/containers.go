//go:build integration

// Package testutil starts real PostgreSQL and Redis containers for
// integration tests (run with `make test-integration`, requires Docker).
package testutil

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
)

// Image tags must match docker-compose.yml.
const (
	PostgresImage = "postgres:18"
	RedisImage    = "redis:8.10.3"
)

// DiscardLogger is a logger for tests; set TEST_LOG=1 to see the output.
func DiscardLogger() *slog.Logger {
	if os.Getenv("TEST_LOG") != "" {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Postgres starts a fresh PostgreSQL container and returns a pool to it, as
// the owner (a superuser, so row-level security does not apply to it). The
// database is empty (no migrations applied).
func Postgres(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, _ := postgres(t)
	return pool
}

func postgres(t testing.TB) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, PostgresImage,
		tcpostgres.WithDatabase("messora"),
		tcpostgres.WithUsername("messora"),
		tcpostgres.WithPassword("messora-test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, database.PoolOptions{DSN: dsn, MaxConns: 10}, 30*time.Second, DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, dsn
}

// MigratedPostgres is Postgres with all migrations applied.
func MigratedPostgres(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool := Postgres(t)
	if _, err := database.Migrate(context.Background(), pool, DiscardLogger()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// DB is a migrated database with a pool per role.
type DB struct {
	// Owner runs migrations; a superuser here, so it sees every tenant.
	Owner *pgxpool.Pool
	// App is the services' pool: messora_app, tenant-scoped (fail-closed,
	// row-level security applies).
	App *pgxpool.Pool
	// System is messora_system (BYPASSRLS), for the outbox relay.
	System *pgxpool.Pool
}

// Test passwords of the service roles.
const (
	appPassword    = "app-test-password"    // gitleaks:allow (test fixture)
	systemPassword = "system-test-password" // gitleaks:allow (test fixture)
)

// TenantDB starts PostgreSQL, applies the migrations and connects as each
// role, exactly as the services do.
func TenantDB(t testing.TB) *DB {
	t.Helper()
	ctx := context.Background()
	owner, dsn := postgres(t)
	if _, err := database.Migrate(ctx, owner, DiscardLogger()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.SetRolePasswords(ctx, owner, map[string]string{database.AppRole: appPassword, database.SystemRole: systemPassword}); err != nil {
		t.Fatal(err)
	}
	return &DB{Owner: owner, App: RolePool(t, dsn, database.AppRole, appPassword, true), System: RolePool(t, dsn, database.SystemRole, systemPassword, false)}
}

// RolePool connects to the database of dsn as another role.
func RolePool(t testing.TB, dsn, user, password string, tenantScoped bool) *pgxpool.Pool {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(user, password)
	pool, err := database.Open(context.Background(), database.PoolOptions{DSN: u.String(), MaxConns: 10, TenantScoped: tenantScoped}, 30*time.Second, DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Redis starts a fresh Redis container.
func Redis(t testing.TB) *goredis.Client {
	t.Helper()
	ctx := context.Background()
	c, err := tcredis.Run(ctx, RedisImage)
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	uri, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	return rdb
}
