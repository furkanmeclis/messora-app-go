package messaging

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/platform/ratelimit"
)

// Throttle paces sends per phone number with a GCRA limiter whose rate
// adapts to Meta's feedback (AIMD): a throughput error (130429) halves the
// rate, and every second of successful sending adds one message per second
// back, up to the ceiling.
type Throttle struct {
	rdb     goredis.UniversalClient
	limiter *ratelimit.Limiter
	// Ceiling is the highest rate per number (Meta's default throughput is
	// 80 messages per second); Floor is the lowest the rate can fall to.
	Ceiling, Floor int
	prefix         string
}

// NewThrottle returns a throttle using rdb.
func NewThrottle(rdb goredis.UniversalClient, ceiling int) *Throttle {
	if ceiling <= 0 {
		ceiling = 80
	}
	return &Throttle{rdb: rdb, limiter: ratelimit.New(rdb, "messora:send"), Ceiling: ceiling, Floor: 1, prefix: "messora:aimd:"}
}

func (t *Throttle) rateKey(pn uuid.UUID) string { return t.prefix + pn.String() }

// Rate returns the current rate for a number.
func (t *Throttle) Rate(ctx context.Context, pn uuid.UUID) int {
	v, err := t.rdb.Get(ctx, t.rateKey(pn)).Result()
	if err != nil {
		return t.Ceiling
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < t.Floor || n > t.Ceiling {
		return t.Ceiling
	}
	return n
}

// Wait reports how long to wait before the next send from pn (0 = now).
// When Redis is unavailable sending proceeds: Meta's own limits still
// apply and are handled as retryable errors.
func (t *Throttle) Wait(ctx context.Context, pn uuid.UUID) time.Duration {
	r := t.Rate(ctx, pn)
	res, err := t.limiter.Allow(ctx, "pn:"+pn.String(), ratelimit.Limit{Rate: r, Period: time.Second, Burst: r})
	if err != nil || res.Allowed {
		return 0
	}
	return res.RetryAfter
}

// Backoff halves the rate after a throughput error.
func (t *Throttle) Backoff(ctx context.Context, pn uuid.UUID) int {
	r := max(t.Floor, t.Rate(ctx, pn)/2)
	// The reduction decays after ten minutes without new errors.
	_ = t.rdb.Set(ctx, t.rateKey(pn), r, 10*time.Minute).Err()
	_ = t.rdb.Set(ctx, t.rateKey(pn)+":cool", 1, 5*time.Second).Err()
	return r
}

// Succeeded grows the rate by one per second of successful sends.
func (t *Throttle) Succeeded(ctx context.Context, pn uuid.UUID) {
	r := t.Rate(ctx, pn)
	if r >= t.Ceiling {
		return
	}
	// No growth right after a backoff, and at most once per second.
	if t.rdb.Exists(ctx, t.rateKey(pn)+":cool").Val() == 1 {
		return
	}
	if ok, _ := t.rdb.SetNX(ctx, t.rateKey(pn)+":inc", 1, time.Second).Result(); ok {
		_ = t.rdb.Set(ctx, t.rateKey(pn), min(t.Ceiling, r+1), 10*time.Minute).Err()
	}
}
