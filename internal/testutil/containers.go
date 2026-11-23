//go:build integration

// Package testutil starts real PostgreSQL and Redis containers for
// integration tests (run with `make test-integration`, requires Docker).
package testutil

import (
	"context"
	"io"
	"log/slog"
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

// Postgres starts a fresh PostgreSQL container and returns a pool to it. The
// database is empty (no migrations applied).
func Postgres(t testing.TB) *pgxpool.Pool {
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
	return pool
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
