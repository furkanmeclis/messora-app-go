//go:build integration

package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furkanmeclis/messora-app-go/internal/platform/database"
	"github.com/furkanmeclis/messora-app-go/internal/platform/outbox"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

func TestOutboxIsTransactionalAndIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testutil.MigratedPostgres(t)
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()
	insp := queue.NewInspector(rdb)
	defer insp.Close()

	router := outbox.NewRouter()
	router.Handle(outbox.TaskTopicPrefix, outbox.TaskPublisher(client))
	relay := outbox.NewRelay(pool, router, outbox.RelayOptions{Logger: testutil.DiscardLogger(), Registerer: prometheus.NewRegistry()})

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
	ctx := context.Background()
	pool := testutil.MigratedPostgres(t)
	relay := outbox.NewRelay(pool, outbox.NewRouter(), outbox.RelayOptions{Logger: testutil.DiscardLogger()})

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testutil.MigratedPostgres(t)

	got := make(chan int64, 1)
	router := outbox.NewRouter()
	router.Handle("test:", outbox.PublisherFunc(func(_ context.Context, e outbox.Event) error {
		got <- e.ID
		return nil
	}))
	// A long poll interval proves delivery came from LISTEN/NOTIFY.
	relay := outbox.NewRelay(pool, router, outbox.RelayOptions{Logger: testutil.DiscardLogger(), PollInterval: time.Minute})
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
	pool := testutil.MigratedPostgres(t)
	if _, err := pool.Exec(ctx, `
		INSERT INTO outbox_events (topic, published_at) VALUES
		  ('t', now() - interval '8 days'),
		  ('t', now() - interval '1 day'),
		  ('t', NULL)`); err != nil {
		t.Fatal(err)
	}
	h := outbox.CleanupHandler(pool, 7*24*time.Hour, testutil.DiscardLogger())
	if err := h.ProcessTask(ctx, asynq.NewTask(queue.TypeOutboxCleanup, nil)); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events").Scan(&left)
	if left != 2 {
		t.Fatalf("left %d rows, want 2 (recent published + unpublished)", left)
	}
}
