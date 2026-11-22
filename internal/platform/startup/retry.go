// Package startup holds helpers for bringing up dependencies in containers
// that may start in any order.
package startup

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Retry calls fn with exponential backoff (250ms doubling to 4s) until it
// succeeds or total elapses. Each attempt gets its own 5s timeout.
func Retry(ctx context.Context, total time.Duration, log *slog.Logger, what string, fn func(context.Context) error) error {
	deadline := time.Now().Add(total)
	wait := 250 * time.Millisecond
	for attempt := 1; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := fn(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().Add(wait).After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("%s unreachable after %d attempts: %w", what, attempt, err)
		}
		log.Warn("dependency not ready, retrying", "dependency", what, "attempt", attempt, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		if wait < 4*time.Second {
			wait *= 2
		}
	}
}
