//go:build integration

package ratelimit_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

func TestBurstThenRefill(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(testutil.Redis(t), "rl")
	lim := ratelimit.Limit{Rate: 5, Period: time.Second, Burst: 5}

	for i := range 5 {
		r, err := l.Allow(ctx, "k", lim)
		if err != nil || !r.Allowed {
			t.Fatalf("request %d within burst rejected: %+v %v", i, r, err)
		}
	}
	r, _ := l.Allow(ctx, "k", lim)
	if r.Allowed || r.RetryAfter <= 0 || r.RetryAfter > 250*time.Millisecond {
		t.Fatalf("6th request: %+v", r)
	}
	time.Sleep(r.RetryAfter + 20*time.Millisecond)
	if r, _ := l.Allow(ctx, "k", lim); !r.Allowed {
		t.Fatal("token must refill after RetryAfter")
	}
	// Keys are independent; Reset clears.
	if r, _ := l.Allow(ctx, "other", lim); !r.Allowed {
		t.Fatal("independent key limited")
	}
	_ = l.Reset(ctx, "k")
	for range 5 {
		if r, _ := l.Allow(ctx, "k", lim); !r.Allowed {
			t.Fatal("reset did not restore burst")
		}
	}
}

func TestConcurrentCallersNeverExceedBurst(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(testutil.Redis(t), "rl")
	lim := ratelimit.Limit{Rate: 10, Period: time.Hour, Burst: 10}
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if r, err := l.Allow(ctx, "shared", lim); err == nil && r.Allowed {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("allowed %d, want exactly 10", allowed.Load())
	}
}
