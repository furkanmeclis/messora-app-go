// Package ratelimit implements a distributed GCRA (generic cell rate
// algorithm) limiter in Redis. The check is one atomic Lua call and uses the
// Redis server clock, so every API replica shares the same view.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Limit allows Rate events per Period with bursts of up to Burst.
type Limit struct {
	Rate   int
	Period time.Duration
	Burst  int
}

// PerMinute is a convenience constructor (burst = rate).
func PerMinute(n int) Limit { return Limit{Rate: n, Period: time.Minute, Burst: n} }

// Result of a check.
type Result struct {
	Allowed    bool
	RetryAfter time.Duration
	// Remaining is how many more events the caller could make right now.
	Remaining int
	// Reset is the time until the full burst is available again.
	Reset time.Duration
}

var gcra = goredis.NewScript(`
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local interval = tonumber(ARGV[1])
local tolerance = tonumber(ARGV[2])
local tat = tonumber(redis.call('GET', KEYS[1]) or now)
if tat < now then tat = now end
local new_tat = tat + interval
local allow_at = new_tat - tolerance
if now < allow_at then
  return {0, math.ceil(allow_at - now), 0, math.ceil(tat - now)}
end
redis.call('SET', KEYS[1], new_tat, 'PX', math.ceil(new_tat - now) + 1)
return {1, 0, math.floor((tolerance - (new_tat - now)) / interval), math.ceil(new_tat - now)}
`)

// Limiter checks limits under a key prefix.
type Limiter struct {
	rdb    goredis.UniversalClient
	prefix string
}

// New returns a limiter; keys are stored as "<prefix>:<key>".
func New(rdb goredis.UniversalClient, prefix string) *Limiter {
	return &Limiter{rdb: rdb, prefix: prefix}
}

// Allow consumes one event for key.
func (l *Limiter) Allow(ctx context.Context, key string, lim Limit) (Result, error) {
	if lim.Rate <= 0 || lim.Period <= 0 {
		return Result{}, fmt.Errorf("ratelimit: invalid limit %+v", lim)
	}
	burst := lim.Burst
	if burst < 1 {
		burst = 1
	}
	intervalMs := float64(lim.Period.Milliseconds()) / float64(lim.Rate)
	res, err := gcra.Run(ctx, l.rdb, []string{l.prefix + ":" + key}, intervalMs, intervalMs*float64(burst)).Int64Slice()
	if err != nil {
		return Result{}, err
	}
	if len(res) < 4 {
		return Result{}, fmt.Errorf("ratelimit: unexpected reply %v", res)
	}
	return Result{
		Allowed:    res[0] == 1,
		RetryAfter: time.Duration(res[1]) * time.Millisecond,
		Remaining:  max(int(res[2]), 0),
		Reset:      time.Duration(res[3]) * time.Millisecond,
	}, nil
}

// Reset clears the counter for key (e.g. after a successful login).
func (l *Limiter) Reset(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, l.prefix+":"+key).Err()
}
