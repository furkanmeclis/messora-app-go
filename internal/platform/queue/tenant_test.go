package queue

import (
	"container/heap"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

func taskOf(tid uuid.UUID, typ string) *asynq.Task {
	if tid == uuid.Nil {
		return asynq.NewTask(typ, nil)
	}
	return asynq.NewTaskWithHeaders(typ, nil, map[string]string{TenantHeader: tid.String()})
}

// A task runs as its header's tenant; one without a header runs only when
// it is a platform task (as system work) and is refused otherwise.
func TestTenantMiddleware(t *testing.T) {
	a := uuid.New()
	var gotTenant uuid.UUID
	var gotSystem, ran bool
	h := TenantMiddleware(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		ran = true
		gotTenant, _ = tenant.FromCtx(ctx)
		gotSystem = tenant.IsSystem(ctx)
		return nil
	}))
	reset := func() { ran, gotTenant, gotSystem = false, uuid.Nil, false }

	if err := h.ProcessTask(context.Background(), taskOf(a, "whatsapp:send")); err != nil || gotTenant != a || gotSystem {
		t.Fatalf("tenant task: %v %v %v", err, gotTenant, gotSystem)
	}
	reset()
	err := h.ProcessTask(context.Background(), taskOf(uuid.Nil, "whatsapp:send"))
	if ran || !errors.Is(err, asynq.SkipRetry) || !errors.Is(err, ErrNoTaskTenant) {
		t.Fatalf("headerless tenant task: ran=%v err=%v", ran, err)
	}
	reset()
	bad := asynq.NewTaskWithHeaders("campaign:start_due", nil, map[string]string{TenantHeader: "not-a-uuid"})
	if err := h.ProcessTask(context.Background(), bad); ran || !errors.Is(err, ErrNoTaskTenant) {
		t.Fatalf("malformed header: ran=%v err=%v", ran, err)
	}
	for _, typ := range []string{TypeOutboxCleanup, TypeTenantFanOut} {
		reset()
		if err := h.ProcessTask(context.Background(), taskOf(uuid.Nil, typ)); err != nil || !ran || !gotSystem || gotTenant != uuid.Nil {
			t.Fatalf("platform task %s: %v ran=%v system=%v", typ, err, ran, gotSystem)
		}
	}
	// The allowlist is exactly the two platform tasks.
	if len(PlatformTasks()) != 2 {
		t.Fatalf("platform tasks %v: keep the list short and documented", PlatformTasks())
	}
}

func TestTenantLimiterLimitsPerTenant(t *testing.T) {
	share := 0.5
	l := NewTenantLimiter(10, func(context.Context) float64 { return share })
	if l.Limit(context.Background()) != 5 {
		t.Fatalf("limit %d", l.Limit(context.Background()))
	}
	for _, c := range []struct {
		share float64
		want  int
	}{{0, 10}, {-1, 10}, {2, 10}, {0.01, 1}, {0.15, 2}, {1, 10}} {
		share = c.share
		if got := l.Limit(context.Background()); got != c.want {
			t.Errorf("share %v: limit %d, want %d", c.share, got, c.want)
		}
	}
	share = 0.5
	a, b := uuid.New(), uuid.New()
	block := make(chan struct{})
	started := make(chan struct{}, 20)
	h := l.Middleware(asynq.HandlerFunc(func(context.Context, *asynq.Task) error {
		started <- struct{}{}
		<-block
		return nil
	}))
	// Share 5 of 10 slots, headroom 3: alone, A may use 7 slots (a quiet
	// worker never slows a tenant down); the eighth would leave fewer than
	// three idle and is deferred.
	if hr := l.Headroom(5); hr != 3 {
		t.Fatalf("headroom %d", hr)
	}
	done := make(chan error, 20)
	for range 7 {
		go func() { done <- h.ProcessTask(context.Background(), taskOf(a, "whatsapp:send")) }()
	}
	for range 7 {
		<-started
	}
	// A's eighth task is deferred, not failed; B and platform tasks run.
	err := h.ProcessTask(context.Background(), taskOf(a, "whatsapp:send"))
	var ra *RetryAfterError
	if !errors.As(err, &ra) || IsFailure(err) || ra.After < 500*time.Millisecond || ra.After > 1500*time.Millisecond {
		t.Fatalf("over the share: %v", err)
	}
	go func() { done <- h.ProcessTask(context.Background(), taskOf(b, "whatsapp:send")) }()
	go func() { done <- h.ProcessTask(context.Background(), taskOf(uuid.Nil, TypeOutboxCleanup)) }()
	<-started
	<-started
	if l.Running(a) != 7 || l.Running(b) != 1 {
		t.Fatalf("running a=%d b=%d", l.Running(a), l.Running(b))
	}
	close(block)
	for range 9 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if l.Running(a) != 0 || l.Running(b) != 0 {
		t.Fatal("slots not released")
	}
	// Below its share a tenant is never deferred, however busy the worker.
	for range 8 {
		l.Acquire(a, 5, true)
	}
	for i := range 2 {
		if !l.Acquire(b, 5, false) {
			t.Fatalf("a tenant under its share was refused on a full worker (slot %d)", i+1)
		}
	}
	if l.Acquire(a, 5, false) || !l.Acquire(a, 5, true) {
		t.Fatal("over the share on a busy worker: deferred unless it is the last attempt")
	}
	// A share of the whole worker is no limit.
	if !NewTenantLimiter(2, nil).Acquire(a, 2, false) {
		t.Fatal("no limit refused")
	}
}

// A deterministic model of a worker process (fake clock, FIFO queue,
// asynq's deferral path) shows the fairness bound: tenant A's 10,000
// campaign sends are queued ahead of tenant B's one message. Without the
// limiter B waits until A's backlog is sent; with it, B waits only while
// the free slots skip over A's queued tasks.
func TestFairShareBoundsTheWaitBehindANoisyTenant(t *testing.T) {
	const (
		concurrency = 20
		backlog     = 10_000
		work        = 100 * time.Millisecond // one send
		skip        = 2 * time.Millisecond   // dequeue + defer of a task over the share
	)
	a, b := uuid.New(), uuid.New()
	waitOfB := func(limit bool) time.Duration {
		l := NewTenantLimiter(concurrency, func(context.Context) float64 { return 0.5 })
		l.delay = func() time.Duration { return time.Second }
		return simulate(l, limit, concurrency, backlog, work, skip, a, b)
	}
	without, with := waitOfB(false), waitOfB(true)
	// FIFO without a limit: B waits for A's backlog, 10,000 × 100 ms / 20.
	if without < 45*time.Second {
		t.Fatalf("model without the limiter: B waited %s", without)
	}
	// With the limiter: ten slots skip A's backlog at 2 ms per task (about
	// 2 s), then B runs. Bound: 5 s, whatever the size of A's campaign
	// beyond that.
	if with > 5*time.Second {
		t.Fatalf("B waited %s behind tenant A's campaign (without the limiter: %s)", with, without)
	}
	t.Logf("B waited %s with the fair share, %s without", with, without)
}

type simTask struct {
	tenant uuid.UUID
	ready  time.Duration // when it may be dequeued (deferred tasks come back)
	seq    int           // FIFO order among ready tasks
}

type simQueue []simTask

func (q simQueue) Len() int { return len(q) }
func (q simQueue) Less(i, j int) bool {
	if q[i].ready != q[j].ready {
		return q[i].ready < q[j].ready
	}
	return q[i].seq < q[j].seq
}
func (q simQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *simQueue) Push(x any)   { *q = append(*q, x.(simTask)) }
func (q *simQueue) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

// simulate runs the queue on a fake clock and returns when B's task
// finished. Each free slot takes the earliest ready task; a task over its
// tenant's share costs skip and comes back after the limiter's delay.
func simulate(l *TenantLimiter, limit bool, concurrency, backlog int, work, skip time.Duration, a, b uuid.UUID) time.Duration {
	q := &simQueue{}
	seq := 0
	for range backlog {
		heap.Push(q, simTask{tenant: a, seq: seq})
		seq++
	}
	heap.Push(q, simTask{tenant: b, seq: seq})
	seq++
	free := make([]time.Duration, concurrency) // when each slot is free
	type running struct {
		tenant uuid.UUID
		until  time.Duration
	}
	var active []running
	for q.Len() > 0 {
		// The slot that frees first takes the next task.
		s := 0
		for i := range free {
			if free[i] < free[s] {
				s = i
			}
		}
		now := free[s]
		// Release what finished by now.
		kept := active[:0]
		for _, r := range active {
			if r.until <= now {
				l.Release(r.tenant)
			} else {
				kept = append(kept, r)
			}
		}
		active = kept
		next := heap.Pop(q).(simTask)
		if next.ready > now {
			now = next.ready
		}
		if limit && !l.Acquire(next.tenant, l.Limit(context.Background()), false) {
			next.ready, next.seq = now+skip+l.delay(), seq
			seq++
			heap.Push(q, next)
			free[s] = now + skip
			continue
		}
		if !limit {
			l.Acquire(next.tenant, 1<<30, false)
		}
		if next.tenant == b {
			return now + work
		}
		active = append(active, running{next.tenant, now + work})
		free[s] = now + work
	}
	return -1
}
