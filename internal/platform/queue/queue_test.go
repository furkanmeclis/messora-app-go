package queue

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBackoffGrowsAndCaps(t *testing.T) {
	within := func(d, want time.Duration) bool {
		return d >= time.Duration(float64(want)*0.8) && d <= time.Duration(float64(want)*1.2)
	}
	for n, want := range map[int]time.Duration{0: 2 * time.Second, 1: 4 * time.Second, 3: 16 * time.Second, 20: 10 * time.Minute} {
		for range 50 {
			if d := RetryDelay(n, errors.New("x"), nil); !within(d, want) {
				t.Fatalf("n=%d: %s not within 20%% of %s", n, d, want)
			}
		}
	}
}

func TestRetryAfterIsNotAFailure(t *testing.T) {
	ra := fmt.Errorf("wrapped: %w", &RetryAfterError{After: 7 * time.Second, Reason: "rate limited"})
	if IsFailure(ra) {
		t.Fatal("deferral must not count as failure")
	}
	if RetryDelay(5, ra, nil) != 7*time.Second {
		t.Fatal("requested delay must be honoured")
	}
	if !IsFailure(errors.New("boom")) {
		t.Fatal("plain errors are failures")
	}
}

func TestRouting(t *testing.T) {
	if Route(TypeOutboxCleanup) != Default || Route("unknown:type") != Default {
		t.Fatal("routing")
	}
}
