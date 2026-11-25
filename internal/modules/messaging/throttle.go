package messaging

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// DefaultThroughput is Meta's default per-number throughput (messages per
// second), used while a number's own throughput is unknown.
const DefaultThroughput = 80

// ceilingTTL is how long a number's ceiling is cached in memory.
const ceilingTTL = time.Minute

// CeilingFunc returns a number's throughput in messages per second; 0 (or
// an error) means unknown.
type CeilingFunc func(ctx context.Context, pn uuid.UUID) (int, error)

// Throttle paces sends per phone number with a GCRA limiter whose rate
// adapts to Meta's feedback (AIMD): a throughput error (130429) halves the
// rate, and every second of successful sending adds one message per second
// back, up to the number's ceiling — the throughput Meta reports for it
// (phone_numbers.throughput_mps), or Ceiling when that is unknown.
type Throttle struct {
	rdb goredis.UniversalClient
	// Ceiling is the rate of numbers without a known throughput (Meta's
	// default is 80 messages per second); Floor is the lowest the rate can
	// fall to.
	Ceiling, Floor int
	prefix         string
	// reserved is the share of each number's rate campaigns leave free
	// (scheduler.go).
	reserved float64

	lookup CeilingFunc
	mu     sync.Mutex
	cache  map[uuid.UUID]cachedCeiling
}

type cachedCeiling struct {
	v       int
	expires time.Time
}

// NewThrottle returns a throttle using rdb.
func NewThrottle(rdb goredis.UniversalClient, ceiling int) *Throttle {
	if ceiling <= 0 {
		ceiling = DefaultThroughput
	}
	return &Throttle{rdb: rdb, Ceiling: ceiling, Floor: 1, prefix: "messora:aimd:", reserved: DefaultReservedShare,
		cache: map[uuid.UUID]cachedCeiling{}}
}

// WithCeilings makes each number's ceiling its own throughput as returned
// by fn (cached for a minute).
func (t *Throttle) WithCeilings(fn CeilingFunc) *Throttle {
	t.lookup = fn
	return t
}

// CeilingFor returns the highest rate for pn.
func (t *Throttle) CeilingFor(ctx context.Context, pn uuid.UUID) int {
	if t.lookup == nil {
		return t.Ceiling
	}
	now := time.Now()
	t.mu.Lock()
	c, ok := t.cache[pn]
	t.mu.Unlock()
	if ok && now.Before(c.expires) {
		return c.v
	}
	v, err := t.lookup(ctx, pn)
	if err != nil || v <= 0 {
		v = t.Ceiling
	}
	t.mu.Lock()
	t.cache[pn] = cachedCeiling{v: v, expires: now.Add(ceilingTTL)}
	t.mu.Unlock()
	return v
}

func (t *Throttle) rateKey(pn uuid.UUID) string { return t.prefix + pn.String() }

// Rate returns the current rate for a number.
func (t *Throttle) Rate(ctx context.Context, pn uuid.UUID) int {
	ceiling := t.CeilingFor(ctx, pn)
	v, err := t.rdb.Get(ctx, t.rateKey(pn)).Result()
	if err != nil {
		return ceiling
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < t.Floor || n > ceiling {
		return ceiling
	}
	return n
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
	ceiling := t.CeilingFor(ctx, pn)
	r := t.Rate(ctx, pn)
	if r >= ceiling {
		return
	}
	// No growth right after a backoff, and at most once per second.
	if t.rdb.Exists(ctx, t.rateKey(pn)+":cool").Val() == 1 {
		return
	}
	if ok, _ := t.rdb.SetNX(ctx, t.rateKey(pn)+":inc", 1, time.Second).Result(); ok {
		_ = t.rdb.Set(ctx, t.rateKey(pn), min(ceiling, r+1), 10*time.Minute).Err()
	}
}
