// Package provision implements `messora init`: it generates any secret the
// operator did not supply and writes one file per consumer, each readable
// only by that consumer's UID. It is idempotent: values are generated once
// and reused on every later run.
package provision

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/furkanmeclis/messora-app-go/internal/config"
)

// Secret keys managed by init. These are variable NAMES, not values.
//
//nolint:gosec // G101 false positive: no credentials here
const (
	KeyPostgresPassword = "POSTGRES_PASSWORD"
	// The roles the services connect as (migration 00024); `messora
	// migrate` sets their passwords.
	KeyPostgresAppPassword    = "POSTGRES_APP_PASSWORD"
	KeyPostgresSystemPassword = "POSTGRES_SYSTEM_PASSWORD"
	KeyRedisPassword          = "REDIS_PASSWORD"
	KeyEncryptionKey          = "APP_ENCRYPTION_KEY"
	KeyBFFSecret              = "BFF_SHARED_SECRET"
	KeyCentrifugoAPIKey       = "CENTRIFUGO_API_KEY"
	KeyCentrifugoHMAC         = "CENTRIFUGO_HMAC_SECRET"
	KeyMetaVerifyToken        = "META_VERIFY_TOKEN"
)

type generator func() (string, error)

var managed = map[string]generator{
	KeyPostgresPassword:       urlToken(24),
	KeyPostgresAppPassword:    urlToken(24),
	KeyPostgresSystemPassword: urlToken(24),
	KeyRedisPassword:          urlToken(24),
	KeyEncryptionKey:          stdKey(32),
	KeyBFFSecret:              urlToken(32),
	KeyCentrifugoAPIKey:       urlToken(32),
	KeyCentrifugoHMAC:         urlToken(48),
	KeyMetaVerifyToken:        urlToken(24),
}

// ManagedKeys lists the keys in a stable order.
func ManagedKeys() []string { return slices.Sorted(maps.Keys(managed)) }

// Owners are the UIDs/GIDs of the containers reading each file. Defaults match
// the pinned images: distroless nonroot 65532 (api/worker/scheduler and the
// distroless Node web image), postgres 999, redis 999, centrifugo 1000.
type Owners struct {
	App, Web, Postgres, Redis, Centrifugo int
}

// DefaultOwners returns the UIDs of the pinned images.
func DefaultOwners() Owners {
	return Owners{App: 65532, Web: 65532, Postgres: 999, Redis: 999, Centrifugo: 1000}
}

// Options configures Run.
type Options struct {
	Dir          string
	Getenv       func(string) string
	PublicAppURL string
	Owners       Owners
	Logger       *slog.Logger
}

// Result reports what Run did.
type Result struct {
	Generated []string // keys created in this run
	Overrides []string // keys taken from the environment
}

// Run provisions the secrets directory.
func Run(o Options) (Result, error) {
	var res Result
	if o.Dir == "" {
		return res, errors.New("provision: empty directory")
	}
	statePath := filepath.Join(o.Dir, ".state", "secrets.env")
	stored, err := config.LoadDotenvFile(statePath)
	if err != nil {
		return res, fmt.Errorf("provision: read state: %w", err)
	}

	values := map[string]string{}
	for _, k := range ManagedKeys() {
		envVal := strings.TrimSpace(o.Getenv(k))
		switch {
		case envVal != "":
			if prev, ok := stored[k]; ok && prev != envVal {
				res.Overrides = append(res.Overrides, k)
				if k == KeyPostgresPassword {
					o.Logger.Warn("POSTGRES_PASSWORD in .env differs from the provisioned value. " +
						"PostgreSQL only reads it when the data volume is first created; change the role's password with ALTER ROLE as well, or the services will fail to connect.")
				}
			}
			values[k] = envVal
		case stored[k] != "":
			values[k] = stored[k]
		default:
			v, err := managed[k]()
			if err != nil {
				return res, err
			}
			values[k] = v
			res.Generated = append(res.Generated, k)
		}
	}
	if err := validate(values); err != nil {
		return res, err
	}

	cfJSON, err := CentrifugoConfig(values[KeyCentrifugoAPIKey], values[KeyCentrifugoHMAC], o.PublicAppURL)
	if err != nil {
		return res, err
	}

	files := []struct {
		path    string
		owner   int
		content []byte
	}{
		{filepath.Join(".state", "secrets.env"), 0, dotenv(values, ManagedKeys())},
		{filepath.Join("app", "secrets.env"), o.Owners.App, dotenv(values, ManagedKeys())},
		{filepath.Join("web", "secrets.env"), o.Owners.Web, dotenv(values, []string{KeyBFFSecret})},
		{filepath.Join("postgres", "password"), o.Owners.Postgres, []byte(values[KeyPostgresPassword])},
		{filepath.Join("redis", "redis.conf"), o.Owners.Redis, RedisConf(values[KeyRedisPassword])},
		{filepath.Join("centrifugo", "config.json"), o.Owners.Centrifugo, cfJSON},
	}
	for _, f := range files {
		if err := writeOwned(filepath.Join(o.Dir, f.path), f.owner, f.content); err != nil {
			return res, err
		}
	}
	return res, nil
}

func validate(v map[string]string) error {
	key, err := base64.StdEncoding.DecodeString(v[KeyEncryptionKey])
	if err != nil || len(key) != 32 {
		return errors.New("provision: APP_ENCRYPTION_KEY must be standard base64 of 32 bytes (generate with: openssl rand -base64 32)")
	}
	if len(v[KeyBFFSecret]) < 32 {
		return errors.New("provision: BFF_SHARED_SECRET must be at least 32 characters")
	}
	for _, k := range []string{KeyPostgresPassword, KeyPostgresAppPassword, KeyPostgresSystemPassword, KeyRedisPassword, KeyCentrifugoAPIKey, KeyCentrifugoHMAC, KeyMetaVerifyToken} {
		if len(v[k]) < 16 {
			return fmt.Errorf("provision: %s must be at least 16 characters", k)
		}
		if strings.ContainsAny(v[k], " \t\r\n\"'\\") {
			return fmt.Errorf("provision: %s must not contain whitespace, quotes or backslashes", k)
		}
	}
	return nil
}

// RedisConf renders the Redis configuration: password, AOF persistence
// (fsync every second) and noeviction, which asynq requires so that queued
// jobs are never silently evicted.
func RedisConf(password string) []byte {
	return []byte(strings.Join([]string{
		"# Generated by `messora init`. Do not edit; set REDIS_PASSWORD in .env instead.",
		"requirepass " + password,
		"appendonly yes",
		"appendfsync everysec",
		"maxmemory-policy noeviction",
		"save 900 1 300 100",
		"dir /data",
		"",
	}, "\n"))
}

// CentrifugoConfig renders the Centrifugo v6 configuration. Clients connect
// only with JWTs issued by the API; subscriptions require subscription tokens
// except for user-limited personal channels.
func CentrifugoConfig(apiKey, hmacSecret, publicAppURL string) ([]byte, error) {
	origin := strings.TrimRight(publicAppURL, "/")
	if origin == "" {
		origin = "http://localhost"
	}
	recovering := func(name string, size int, ttl string) map[string]any {
		return map[string]any{
			"name":           name,
			"history_size":   size,
			"history_ttl":    ttl,
			"force_recovery": true,
		}
	}
	// Ten minutes of history: a client that reconnects within that window
	// gets the missed events replayed; after it, the panel re-syncs through
	// GET /realtime/sync.
	const historySize, historyTTL = 200, "600s"
	personal := recovering("personal", historySize, historyTTL)
	personal["allow_user_limited_channels"] = true
	cfg := map[string]any{
		"log": map[string]any{"level": "info"},
		"client": map[string]any{
			"allowed_origins":    []string{origin},
			"token":              map[string]any{"hmac_secret_key": hmacSecret},
			"subscription_token": map[string]any{"enabled": true, "hmac_secret_key": hmacSecret},
		},
		"http_api":   map[string]any{"key": apiKey},
		"health":     map[string]any{"enabled": true},
		"prometheus": map[string]any{"enabled": true},
		"admin":      map[string]any{"enabled": false},
		"channel": map[string]any{
			"namespaces": []map[string]any{
				personal,
				recovering("campaign", historySize, historyTTL),
				recovering("conversation", historySize, historyTTL),
				recovering("inbox", historySize, historyTTL),
				recovering("system", 20, "300s"),
			},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func dotenv(v map[string]string, keys []string) []byte {
	var b bytes.Buffer
	b.WriteString("# Generated by `messora init`. Values in .env take precedence.\n")
	for _, k := range keys {
		b.WriteString(k + "=" + v[k] + "\n")
	}
	return b.Bytes()
}

// writeOwned writes content atomically with mode 0400 inside a 0500 directory,
// both owned by uid when running as root.
func writeOwned(path string, uid int, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Re-runs: the directory was left read-only (0500) by the previous run.
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // directories need the execute bit; owner-only
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o400); err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(tmpName, uid, uid); err != nil {
			return err
		}
		if err := os.Chown(dir, uid, uid); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return os.Chmod(dir, 0o500) //nolint:gosec // owner read+traverse only
}

func urlToken(n int) generator {
	return func() (string, error) {
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
}

func stdKey(n int) generator {
	return func() (string, error) {
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(b), nil
	}
}

// OwnersFromEnv lets operators with custom images override UIDs, e.g.
// MESSORA_UID_WEB=1001.
func OwnersFromEnv(getenv func(string) string) (Owners, error) {
	o := DefaultOwners()
	for name, dst := range map[string]*int{
		"MESSORA_UID_APP": &o.App, "MESSORA_UID_WEB": &o.Web, "MESSORA_UID_POSTGRES": &o.Postgres,
		"MESSORA_UID_REDIS": &o.Redis, "MESSORA_UID_CENTRIFUGO": &o.Centrifugo,
	} {
		if raw := getenv(name); raw != "" {
			v, err := strconv.Atoi(raw)
			if err != nil || v < 0 {
				return o, fmt.Errorf("%s must be a non-negative integer", name)
			}
			*dst = v
		}
	}
	return o, nil
}
