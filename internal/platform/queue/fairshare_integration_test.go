//go:build integration

package queue_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// On a real asynq worker: tenant A's campaign (hundreds of slow sends, all
// queued first) holds at most its share of the handler slots, and tenant
// B's message, queued last, runs within a bound instead of after A's
// backlog. Tasks without a tenant are refused into the dead letter.
func TestFairShareOnARealWorker(t *testing.T) {
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()
	insp := queue.NewInspector(rdb)
	defer insp.Close()

	const (
		concurrency = 4
		backlog     = 400
		work        = 50 * time.Millisecond
	)
	a, b := uuid.New(), uuid.New()
	limiter := queue.NewTenantLimiter(concurrency, func(context.Context) float64 { return 0.5 })

	var mu sync.Mutex
	running, peakA := map[uuid.UUID]int{}, 0
	doneB := make(chan time.Time, 1)
	mux := asynq.NewServeMux()
	mux.Use(queue.TenantMiddleware, limiter.Middleware)
	mux.HandleFunc("whatsapp:send", func(ctx context.Context, _ *asynq.Task) error {
		id := tenant.ID(ctx)
		mu.Lock()
		running[id]++
		peakA = max(peakA, running[a])
		mu.Unlock()
		if id == b {
			doneB <- time.Now()
		} else {
			time.Sleep(work)
		}
		mu.Lock()
		running[id]--
		mu.Unlock()
		return nil
	})

	ctx := context.Background()
	for i := range backlog {
		if err := queue.EnqueueFor(ctx, client, a, "whatsapp:send", []byte(`{"campaign_id":"c"}`), asynq.TaskID(fmt.Sprintf("a-%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := queue.EnqueueFor(ctx, client, b, "whatsapp:send", []byte(`{"campaign_id":"d"}`), asynq.TaskID("b")); err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(ctx, client, "whatsapp:send", []byte(`{"campaign_id":"e"}`), asynq.TaskID("no-tenant")); err != nil {
		t.Fatal(err)
	}

	srv := queue.NewServer(rdb, queue.ServerOptions{Concurrency: concurrency, Queues: map[string]int{queue.Campaigns: 1},
		ShutdownTimeout: time.Second, Logger: testutil.DiscardLogger()})
	start := time.Now()
	if err := srv.Start(mux); err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	// FIFO without the limiter: B would wait backlog × work / concurrency
	// = 5 s. With it, two slots skip A's waiting tasks.
	const bound = 3 * time.Second
	select {
	case at := <-doneB:
		if wait := at.Sub(start); wait > bound {
			t.Fatalf("B waited %s behind A's campaign (bound %s)", wait, bound)
		} else {
			t.Logf("B ran after %s", wait)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("tenant B's task never ran")
	}
	mu.Lock()
	peak := peakA
	mu.Unlock()
	// Its share (2) plus the slots it may borrow while the headroom (1) stays idle.
	if peak > concurrency-limiter.Headroom(concurrency/2) {
		t.Fatalf("tenant A held %d of %d slots, share 0.5, headroom %d", peak, concurrency, limiter.Headroom(concurrency/2))
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		info, err := insp.GetTaskInfo(queue.Campaigns, "no-tenant")
		if err == nil && info.State == asynq.TaskStateArchived {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the task without a tenant was not refused: %v %v", info, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
