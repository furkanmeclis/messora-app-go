// Package config loads and validates Messora's runtime configuration.
//
// Values are resolved in this order: a non-empty process environment
// variable, then the secrets file referenced by MESSORA_SECRETS_FILE (written
// by `messora init` on first start), then the built-in default.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment names.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
	EnvTest        = "test"
)

// Role identifies which process is being configured; validation differs.
type Role string

// Process roles.
const (
	RoleAPI       Role = "api"
	RoleWorker    Role = "worker"
	RoleScheduler Role = "scheduler"
	RoleMigrate   Role = "migrate"
)

// Secret is a string that never prints its value.
type Secret string

// String implements fmt.Stringer and always redacts.
func (Secret) String() string { return "[redacted]" }

// GoString implements fmt.GoStringer and always redacts.
func (Secret) GoString() string { return "[redacted]" }

// Reveal returns the underlying value. Call sites are easy to audit.
func (s Secret) Reveal() string { return string(s) }

// IsSet reports whether a value is present.
func (s Secret) IsSet() bool { return s != "" }

// Config is the full process configuration.
type Config struct {
	Env         string
	LogLevel    string
	LogFormat   string
	ServiceName string

	HTTPAddr        string // public API listener (api role)
	InternalAddr    string // metrics + health listener (all long-running roles)
	ShutdownTimeout time.Duration

	PublicAppURL string

	Database DatabaseConfig
	Redis    RedisConfig

	EncryptionKey   []byte // 32 bytes, decoded from APP_ENCRYPTION_KEY
	BFFSharedSecret Secret

	Centrifugo CentrifugoConfig

	Worker WorkerConfig
}

// DatabaseConfig describes the PostgreSQL connection.
type DatabaseConfig struct {
	URL      Secret // full DSN; when set it wins over the discrete fields
	Host     string
	Port     int
	User     string
	Password Secret
	Name     string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

// DSN returns the connection string.
func (d DatabaseConfig) DSN() string {
	if d.URL.IsSet() {
		return d.URL.Reveal()
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password.Reveal()),
		Host:     net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:     "/" + d.Name,
		RawQuery: url.Values{"sslmode": {d.SSLMode}}.Encode(),
	}
	return u.String()
}

// RedisConfig describes the Redis connection.
type RedisConfig struct {
	URL      Secret // redis://[:password@]host:port/db — wins over discrete fields
	Addr     string
	Password Secret
	DB       int
}

// CentrifugoConfig describes the realtime server.
type CentrifugoConfig struct {
	APIURL     string
	APIKey     Secret
	HMACSecret Secret
}

// WorkerConfig tunes the asynq server.
type WorkerConfig struct {
	Concurrency int
	Queues      map[string]int
}

// Load reads configuration from the environment and the optional secrets file.
func Load() (*Config, error) {
	file := map[string]string{}
	if p := os.Getenv("MESSORA_SECRETS_FILE"); p != "" {
		m, err := LoadDotenvFile(p)
		if err != nil {
			return nil, fmt.Errorf("read secrets file: %w", err)
		}
		file = m
	}
	return loadFrom(os.Getenv, file)
}

func loadFrom(getenv func(string) string, file map[string]string) (*Config, error) {
	l := &loader{getenv: getenv, file: file}

	cfg := &Config{
		Env:             l.str("APP_ENV", EnvProduction),
		LogLevel:        l.str("LOG_LEVEL", "info"),
		LogFormat:       l.str("LOG_FORMAT", "json"),
		ServiceName:     l.str("OTEL_SERVICE_NAME", "messora"),
		HTTPAddr:        l.str("API_ADDR", ":8080"),
		InternalAddr:    l.str("INTERNAL_ADDR", ":9100"),
		ShutdownTimeout: l.duration("SHUTDOWN_TIMEOUT", 30*time.Second),
		PublicAppURL:    strings.TrimRight(l.str("PUBLIC_APP_URL", "http://localhost"), "/"),
		Database: DatabaseConfig{
			URL:      Secret(l.str("DATABASE_URL", "")),
			Host:     l.str("POSTGRES_HOST", "postgres"),
			Port:     l.int("POSTGRES_PORT", 5432),
			User:     l.str("POSTGRES_USER", "messora"),
			Password: Secret(l.str("POSTGRES_PASSWORD", "")),
			Name:     l.str("POSTGRES_DB", "messora"),
			SSLMode:  l.str("POSTGRES_SSLMODE", "disable"),
			MaxConns: l.int32Range("DB_MAX_CONNS", 20, 1, 1000),
			MinConns: l.int32Range("DB_MIN_CONNS", 2, 0, 1000),
		},
		Redis: RedisConfig{
			URL:      Secret(l.str("REDIS_URL", "")),
			Addr:     l.str("REDIS_ADDR", "redis:6379"),
			Password: Secret(l.str("REDIS_PASSWORD", "")),
			DB:       l.int("REDIS_DB", 0),
		},
		BFFSharedSecret: Secret(l.str("BFF_SHARED_SECRET", "")),
		Centrifugo: CentrifugoConfig{
			APIURL:     l.str("CENTRIFUGO_API_URL", "http://centrifugo:8000/api"),
			APIKey:     Secret(l.str("CENTRIFUGO_API_KEY", "")),
			HMACSecret: Secret(l.str("CENTRIFUGO_HMAC_SECRET", "")),
		},
		Worker: WorkerConfig{
			Concurrency: l.int("WORKER_CONCURRENCY", 20),
		},
	}

	queues, err := ParseQueues(l.str("WORKER_QUEUES", "critical=6,campaigns=3,default=3,bulk=1"))
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("WORKER_QUEUES: %w", err))
	}
	cfg.Worker.Queues = queues

	if raw := l.str("APP_ENCRYPTION_KEY", ""); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		switch {
		case err != nil:
			l.errs = append(l.errs, errors.New("APP_ENCRYPTION_KEY: must be standard base64"))
		case len(key) != 32:
			l.errs = append(l.errs, fmt.Errorf("APP_ENCRYPTION_KEY: must decode to 32 bytes, got %d", len(key)))
		default:
			cfg.EncryptionKey = key
		}
	}

	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	return cfg, nil
}

// Validate checks that everything the given role needs is present.
func (c *Config) Validate(role Role) error {
	var errs []error
	req := func(ok bool, name string) {
		if !ok {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
	}

	switch c.Env {
	case EnvDevelopment, EnvProduction, EnvTest:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be one of development, production, test (got %q)", c.Env))
	}
	switch c.LogFormat {
	case "json", "text":
	default:
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be json or text (got %q)", c.LogFormat))
	}

	req(c.Database.URL.IsSet() || c.Database.Password.IsSet(), "DATABASE_URL or POSTGRES_PASSWORD")

	if role != RoleMigrate {
		req(c.Redis.URL.IsSet() || c.Redis.Addr != "", "REDIS_URL or REDIS_ADDR")
		req(len(c.EncryptionKey) == 32, "APP_ENCRYPTION_KEY")
		if c.Worker.Concurrency < 1 {
			errs = append(errs, errors.New("WORKER_CONCURRENCY must be >= 1"))
		}
	}
	if role == RoleAPI {
		req(c.BFFSharedSecret.IsSet(), "BFF_SHARED_SECRET")
		if c.BFFSharedSecret.IsSet() && len(c.BFFSharedSecret) < 32 {
			errs = append(errs, errors.New("BFF_SHARED_SECRET must be at least 32 characters"))
		}
		req(c.Centrifugo.APIKey.IsSet(), "CENTRIFUGO_API_KEY")
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("SHUTDOWN_TIMEOUT must be positive"))
	}
	return errors.Join(errs...)
}

// ParseQueues parses "name=priority,name=priority".
func ParseQueues(s string) (map[string]int, error) {
	out := map[string]int{}
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, prio, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("%q: expected name=priority", part)
		}
		p, err := strconv.Atoi(strings.TrimSpace(prio))
		if err != nil || p < 1 {
			return nil, fmt.Errorf("%q: priority must be a positive integer", part)
		}
		out[strings.TrimSpace(name)] = p
	}
	if len(out) == 0 {
		return nil, errors.New("at least one queue is required")
	}
	return out, nil
}

type loader struct {
	getenv func(string) string
	file   map[string]string
	errs   []error
}

func (l *loader) str(key, def string) string {
	if v := strings.TrimSpace(l.getenv(key)); v != "" {
		return v
	}
	if v := strings.TrimSpace(l.file[key]); v != "" {
		return v
	}
	return def
}

func (l *loader) int(key string, def int) int {
	raw := l.str(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s: must be an integer", key))
		return def
	}
	return v
}

// int32Range reads an integer that must lie within [lo, hi].
func (l *loader) int32Range(key string, def, lo, hi int32) int32 {
	v := l.int(key, int(def))
	if v < int(lo) || v > int(hi) {
		l.errs = append(l.errs, fmt.Errorf("%s: must be between %d and %d", key, lo, hi))
		return def
	}
	return int32(v) //nolint:gosec // bounds checked above
}

func (l *loader) duration(key string, def time.Duration) time.Duration {
	raw := l.str(key, "")
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s: must be a Go duration such as 30s", key))
		return def
	}
	return v
}
