//go:build integration

package database_test

import (
	"context"
	"sync"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

func TestMigrationsUpDownUpAndConcurrent(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Postgres(t)
	log := testutil.DiscardLogger()

	st, err := database.Status(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if st.UpToDate() || st.Target < 1 {
		t.Fatalf("fresh database must have pending migrations: %+v", st)
	}

	// Two migrators racing must both succeed and apply each migration once
	// (serialized by the advisory lock).
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() { _, errs[i] = database.Migrate(ctx, pool, log) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent migrate: %v", err)
		}
	}
	st, _ = database.Status(ctx, pool)
	if !st.UpToDate() {
		t.Fatalf("not up to date after migrate: %+v", st)
	}

	// Down sections must be valid and leave a clean schema.
	if err := database.MigrateDownAll(ctx, pool); err != nil {
		t.Fatalf("down: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('settings','outbox_events')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("tables left after down: %d %v", n, err)
	}
	// And migrations are repeatable.
	if _, err := database.Migrate(ctx, pool, log); err != nil {
		t.Fatalf("re-up: %v", err)
	}
}

func TestSessionsUseUTC(t *testing.T) {
	pool := testutil.Postgres(t)
	var tz string
	if err := pool.QueryRow(context.Background(), "SHOW timezone").Scan(&tz); err != nil || tz != "UTC" {
		t.Fatalf("timezone = %q %v", tz, err)
	}
}
