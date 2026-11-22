// Package redis builds the shared go-redis client.
package redis

import (
	"context"
	"errors"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/platform/startup"
)

// Open connects to Redis and waits until it answers PING.
func Open(ctx context.Context, c config.RedisConfig, connectTimeout time.Duration, log *slog.Logger) (*goredis.Client, error) {
	var opts *goredis.Options
	if c.URL.IsSet() {
		o, err := goredis.ParseURL(c.URL.Reveal())
		if err != nil {
			return nil, errors.New("redis: invalid REDIS_URL") // never echo the URL
		}
		opts = o
	} else {
		opts = &goredis.Options{Addr: c.Addr, Password: c.Password.Reveal(), DB: c.DB}
	}
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := goredis.NewClient(opts)
	err := startup.Retry(ctx, connectTimeout, log, "redis", func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
