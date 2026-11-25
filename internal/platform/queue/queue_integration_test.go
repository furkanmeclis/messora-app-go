//go:build integration

package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

type attempt struct {
	queue   string
	retried int
}

// A campaign send runs in the campaigns queue, and a throttle deferral
// (RetryAfterError) brings it back to the same queue without spending its
// retry budget; an inbox send stays in critical.
func TestSendRoutingSurvivesDeferral(t *testing.T) {
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()

	var mu sync.Mutex
	seen := map[string][]attempt{}
	done := make(chan string, 4)
	mux := asynq.NewServeMux()
	mux.HandleFunc("whatsapp:send", func(ctx context.Context, task *asynq.Task) error {
		q, _ := asynq.GetQueueName(ctx)
		n, _ := asynq.GetRetryCount(ctx)
		id, _ := asynq.GetTaskID(ctx)
		mu.Lock()
		seen[id] = append(seen[id], attempt{q, n})
		first := len(seen[id]) == 1
		mu.Unlock()
		if first {
			return &queue.RetryAfterError{After: time.Second, Reason: "per-number rate limit"}
		}
		done <- id
		return nil
	})
	srv := queue.NewServer(rdb, queue.ServerOptions{Concurrency: 2, Queues: map[string]int{queue.Critical: 6, queue.Campaigns: 3},
		ShutdownTimeout: time.Second, Logger: testutil.DiscardLogger()})
	if err := srv.Start(mux); err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	ctx := context.Background()
	if err := queue.Enqueue(ctx, client, "whatsapp:send", []byte(`{"message_id":"m1","campaign_id":"c1"}`), asynq.TaskID("campaign")); err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(ctx, client, "whatsapp:send", []byte(`{"message_id":"m2"}`), asynq.TaskID("inbox")); err != nil {
		t.Fatal(err)
	}
	finished := map[string]bool{}
	deadline := time.After(30 * time.Second)
	for len(finished) < 2 {
		select {
		case id := <-done:
			finished[id] = true
		case <-deadline:
			t.Fatalf("tasks did not finish: %v", seen)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for id, want := range map[string]string{"campaign": queue.Campaigns, "inbox": queue.Critical} {
		got := seen[id]
		if len(got) != 2 {
			t.Fatalf("%s: %d attempts, want 2", id, len(got))
		}
		for _, a := range got {
			if a.queue != want {
				t.Errorf("%s ran in %q, want %q", id, a.queue, want)
			}
			if a.retried != 0 {
				t.Errorf("%s: deferral counted as a retry (%d)", id, a.retried)
			}
		}
	}
}

// Enqueue applies the type's retry budget unless the caller overrides it.
func TestEnqueueAppliesMaxRetry(t *testing.T) {
	rdb := testutil.Redis(t)
	client := queue.NewClient(rdb)
	defer client.Close()
	insp := queue.NewInspector(rdb)
	defer insp.Close()
	ctx := context.Background()
	if err := queue.Enqueue(ctx, client, "contacts:import", []byte(`{}`), asynq.TaskID("a")); err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(ctx, client, "contacts:import", []byte(`{}`), asynq.TaskID("b"), asynq.MaxRetry(1)); err != nil {
		t.Fatal(err)
	}
	a, err := insp.GetTaskInfo(queue.Bulk, "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := insp.GetTaskInfo(queue.Bulk, "b")
	if err != nil {
		t.Fatal(err)
	}
	if a.MaxRetry != queue.PolicyFor("contacts:import").MaxRetry || b.MaxRetry != 1 {
		t.Fatalf("max retry a=%d b=%d", a.MaxRetry, b.MaxRetry)
	}
}
