// Package lock implements a Redis lease lock: a key holding a random token
// with a TTL. Only the holder of the token can renew or release it, so a
// process that stalls past its TTL cannot release a lock someone else now
// holds.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ErrNotHeld is returned when renewing or releasing a lease that has expired
// or was taken over by another holder.
var ErrNotHeld = errors.New("lock: lease not held")

var renewScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0`)

var releaseScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)

// Lease is a held lock.
type Lease struct {
	rdb   goredis.UniversalClient
	key   string
	token string
	ttl   time.Duration
}

// TryAcquire attempts to take the lock once. ok is false when another holder
// has it.
func TryAcquire(ctx context.Context, rdb goredis.UniversalClient, key string, ttl time.Duration) (lease *Lease, ok bool, err error) {
	if ttl < time.Millisecond {
		return nil, false, errors.New("lock: ttl must be at least 1ms")
	}
	token, err := randomToken()
	if err != nil {
		return nil, false, err
	}
	res, err := rdb.SetArgs(ctx, key, token, goredis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if res != "OK" {
		return nil, false, nil
	}
	return &Lease{rdb: rdb, key: key, token: token, ttl: ttl}, true, nil
}

// Renew extends the lease by its TTL. ErrNotHeld means leadership is lost.
func (l *Lease) Renew(ctx context.Context) error {
	n, err := renewScript.Run(ctx, l.rdb, []string{l.key}, l.token, l.ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotHeld
	}
	return nil
}

// Release gives the lock up if still held.
func (l *Lease) Release(ctx context.Context) error {
	n, err := releaseScript.Run(ctx, l.rdb, []string{l.key}, l.token).Int()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotHeld
	}
	return nil
}

// Key returns the Redis key of the lease.
func (l *Lease) Key() string { return l.key }

// TTL returns the lease duration.
func (l *Lease) TTL() time.Duration { return l.ttl }

// IsHeldByAnyone reports whether the key currently exists.
func IsHeldByAnyone(ctx context.Context, rdb goredis.UniversalClient, key string) (bool, error) {
	n, err := rdb.Exists(ctx, key).Result()
	return n == 1, err
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
