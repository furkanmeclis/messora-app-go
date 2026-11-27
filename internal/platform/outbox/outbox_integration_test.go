//go:build integration

package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

func TestOutboxIsTransactionalAndIdempotent(t *testing.T) {
	ctx := tenant.WithID(context.Background(), tenant.DefaultID)
	db := testutil.TenantDB(t)
	pool := db.App
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()
	insp := queue.NewInspector(rdb)
	defer insp.Close()

	router := outbox.NewRouter()
	router.Handle(outbox.TaskTopicPrefix, outbox.TaskPublisher(client))
	relay := outbox.NewRelay(db.System, router, outbox.RelayOptions{Logger: testutil.DiscardLogger(), Registerer: prometheus.NewRegistry()})

	// Rolled-back transaction: the event must never exist.
	rollback := errors.New("business failure")
	err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := outbox.Write(ctx, tx, outbox.TaskTopic(queue.TypeOutboxCleanup), map[string]string{"k": "rolled-back"}, 0); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}

	// Committed transaction.
	var id int64
	if err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
		var err error
		id, err = outbox.Write(ctx, tx, outbox.TaskTopic(queue.TypeOutboxCleanup), map[string]string{"k": "v"}, 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	n, err := relay.DrainOnce(ctx)
	if err != nil || n != 1 {
		t.Fatalf("drain: n=%d err=%v (rolled-back event must not be visible)", n, err)
	}
	taskID := fmt.Sprintf("outbox:%d", id)
	info, err := insp.GetTaskInfo(queue.Route(queue.TypeOutboxCleanup), taskID)
	if err != nil {
		t.Fatalf("task not enqueued: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(info.Payload, &payload); err != nil || payload["k"] != "v" || info.Type != queue.TypeOutboxCleanup {
		t.Fatalf("unexpected task %s %s", info.Type, info.Payload)
	}
	// The task carries the writer's tenant.
	if info.Headers[queue.TenantHeader] != tenant.DefaultID.String() {
		t.Fatalf("task headers %v, want the tenant", info.Headers)
	}

	var published *time.Time
	if err := pool.QueryRow(ctx, "SELECT published_at FROM outbox_events WHERE id=$1", id).Scan(&published); err != nil || published == nil {
		t.Fatalf("not marked published: %v", err)
	}

	// Simulate "published to Redis but commit lost": un-publish the row and
	// drain again. The TaskID must make the second publication a no-op.
	if _, err := pool.Exec(ctx, "UPDATE outbox_events SET published_at=NULL WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	if n, err := relay.DrainOnce(ctx); err != nil || n != 1 {
		t.Fatalf("redrain: %d %v", n, err)
	}
	qi, err := insp.GetQueueInfo(queue.Route(queue.TypeOutboxCleanup))
	if err != nil || qi.Pending != 1 {
		t.Fatalf("duplicate task created: pending=%d err=%v", qi.Pending, err)
	}
}

func TestOutboxUnknownTopicIsRetriedNotDropped(t *testing.T) {
	ctx := tenant.WithID(context.Background(), tenant.DefaultID)
	db := testutil.TenantDB(t)
	pool := db.App
	relay := outbox.NewRelay(db.System, outbox.NewRouter(), outbox.RelayOptions{Logger: testutil.DiscardLogger()})

	id, err := outbox.Write(ctx, pool, "realtime:campaign.updated", map[string]int{"x": 1}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := relay.DrainOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var attempts int
	var lastErr *string
	var availableIn float64
	if err := pool.QueryRow(ctx, `SELECT attempts, last_error, extract(epoch from available_at - now()) FROM outbox_events WHERE id=$1 AND published_at IS NULL`, id).
		Scan(&attempts, &lastErr, &availableIn); err != nil {
		t.Fatalf("event must stay unpublished: %v", err)
	}
	if attempts != 1 || lastErr == nil || availableIn <= 0 {
		t.Fatalf("expected backoff: attempts=%d err=%v in=%f", attempts, lastErr, availableIn)
	}
	// Not yet available: the next drain must not pick it up.
	if n, _ := relay.DrainOnce(ctx); n != 0 {
		t.Fatalf("backoff not respected, drained %d", n)
	}
}

func TestRelayRunWakesOnNotify(t *testing.T) {
	ctx, cancel := context.WithCancel(tenant.WithID(context.Background(), tenant.DefaultID))
	defer cancel()
	db := testutil.TenantDB(t)
	pool := db.App

	got := make(chan int64, 1)
	router := outbox.NewRouter()
	router.Handle("test:", outbox.PublisherFunc(func(_ context.Context, e outbox.Event) error {
		if e.TenantID != tenant.DefaultID {
			t.Errorf("event tenant %s", e.TenantID)
		}
		got <- e.ID
		return nil
	}))
	// A long poll interval proves delivery came from LISTEN/NOTIFY.
	relay := outbox.NewRelay(db.System, router, outbox.RelayOptions{Logger: testutil.DiscardLogger(), PollInterval: time.Minute})
	go func() { _ = relay.Run(ctx) }()
	time.Sleep(500 * time.Millisecond) // let LISTEN register

	start := time.Now()
	id, err := outbox.Write(ctx, pool, "test:ping", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case gid := <-got:
		if gid != id {
			t.Fatalf("got %d want %d", gid, id)
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("too slow: %s", time.Since(start))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("relay did not wake on NOTIFY")
	}
}

func TestCleanupDeletesOnlyOldPublished(t *testing.T) {
	ctx := context.Background()
	db := testutil.TenantDB(t)
	pool := db.Owner
	if _, err := pool.Exec(ctx, `
		INSERT INTO outbox_events (topic, published_at) VALUES
		  ('t', now() - interval '8 days'),
		  ('t', now() - interval '1 day'),
		  ('t', NULL)`); err != nil {
		t.Fatal(err)
	}
	h := outbox.CleanupHandler(db.System, 7*24*time.Hour, testutil.DiscardLogger())
	if err := h.ProcessTask(ctx, asynq.NewTask(queue.TypeOutboxCleanup, nil)); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events").Scan(&left)
	if left != 2 {
		t.Fatalf("left %d rows, want 2 (recent published + unpublished)", left)
	}
}

// The application role cannot write an event of another tenant, nor one
// without a tenant, and sees only its own tenant's events.
func TestOutboxIsTenantScoped(t *testing.T) {
	db := testutil.TenantDB(t)
	var other uuid.UUID
	if err := db.Owner.QueryRow(context.Background(), `INSERT INTO tenants (slug, name) VALUES ('other', 'Other') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	a := tenant.WithID(context.Background(), tenant.DefaultID)
	b := tenant.WithID(context.Background(), other)
	if _, err := outbox.Write(a, db.App, "test:a", nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := outbox.Write(context.Background(), db.App, "test:none", nil, 0); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("write without a tenant: %v", err)
	}
	if _, err := outbox.Write(tenant.System(context.Background()), db.App, "test:none", nil, 0); err == nil {
		t.Fatal("the app role wrote an event without a tenant")
	}
	var n int
	if err := db.App.QueryRow(b, "SELECT count(*) FROM outbox_events").Scan(&n); err != nil || n != 0 {
		t.Fatalf("tenant B sees %d events of A (%v)", n, err)
	}
	if _, err := db.App.Exec(b, "INSERT INTO outbox_events (tenant_id, topic) VALUES ($1, 'test:forged')", tenant.DefaultID); err == nil {
		t.Fatal("tenant B wrote an event as tenant A")
	}
}
