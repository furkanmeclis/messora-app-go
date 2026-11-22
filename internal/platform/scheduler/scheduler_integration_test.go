//go:build integration

package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/platform/lock"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/scheduler"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

func TestLeaseLock(t *testing.T) {
	ctx := context.Background()
	rdb := testutil.Redis(t)

	a, ok, err := lock.TryAcquire(ctx, rdb, "k", 500*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("first acquire: %v %v", ok, err)
	}
	if _, ok, _ := lock.TryAcquire(ctx, rdb, "k", time.Second); ok {
		t.Fatal("second holder acquired a held lock")
	}
	if err := a.Renew(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(700 * time.Millisecond) // expire
	b, ok, err := lock.TryAcquire(ctx, rdb, "k", time.Second)
	if err != nil || !ok {
		t.Fatalf("acquire after expiry: %v %v", ok, err)
	}
	// The stale holder must not be able to renew or release b's lease.
	if err := a.Renew(ctx); !errors.Is(err, lock.ErrNotHeld) {
		t.Fatalf("stale renew = %v", err)
	}
	if err := a.Release(ctx); !errors.Is(err, lock.ErrNotHeld) {
		t.Fatalf("stale release = %v", err)
	}
	if held, _ := lock.IsHeldByAnyone(ctx, rdb, "k"); !held {
		t.Fatal("stale release removed the new holder's lock")
	}
	if err := b.Release(ctx); err != nil {
		t.Fatal(err)
	}
}

// Three schedulers run concurrently through several slots and a leader
// change; every slot must be enqueued exactly once.
func TestSchedulersNeverDuplicate(t *testing.T) {
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()
	insp := queue.NewInspector(rdb)
	defer insp.Close()

	jobs := []scheduler.Job{{Name: "every_second", Every: time.Second, TaskType: queue.TypeOutboxCleanup}}
	opts := scheduler.Options{LeaseTTL: 600 * time.Millisecond, Tick: 100 * time.Millisecond, Logger: testutil.DiscardLogger()}

	ctxs := make([]context.CancelFunc, 3)
	var wg sync.WaitGroup
	for i := range 3 {
		s, err := scheduler.New(rdb, client, jobs, opts)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		ctxs[i] = cancel
		wg.Go(func() { _ = s.Run(ctx) })
	}

	start := time.Now()
	time.Sleep(2500 * time.Millisecond)
	ctxs[0]() // stop one (possibly the leader): leadership must move on
	time.Sleep(2500 * time.Millisecond)
	ctxs[1]()
	ctxs[2]()
	wg.Wait()
	elapsed := time.Since(start)

	tasks, err := insp.ListPendingTasks(queue.Route(queue.TypeOutboxCleanup))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, ti := range tasks {
		if seen[ti.ID] {
			t.Fatalf("duplicate task %s", ti.ID)
		}
		seen[ti.ID] = true
	}
	// One task per elapsed one-second slot (allowing for the edges).
	maxSlots := int(elapsed/time.Second) + 2
	if len(tasks) < 3 || len(tasks) > maxSlots {
		t.Fatalf("got %d tasks over %s, want between 3 and %d", len(tasks), elapsed, maxSlots)
	}
	if held, _ := lock.IsHeldByAnyone(context.Background(), rdb, scheduler.LeaderKey); held {
		t.Fatal("leader lease not released on shutdown")
	}
}
