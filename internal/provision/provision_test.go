package provision

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/config"
)

// tempDir is t.TempDir whose read-only (0500) subdirectories are made
// writable again before the testing package removes it.
func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(p, 0o700)
			}
			return nil
		})
	})
	return dir
}

func run(t *testing.T, dir string, env map[string]string) Result {
	t.Helper()
	res, err := Run(Options{
		Dir: dir, Getenv: func(k string) string { return env[k] },
		PublicAppURL: "http://localhost:8088/", Owners: DefaultOwners(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	m, err := config.LoadDotenvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGeneratesOnceAndIsIdempotent(t *testing.T) {
	dir := tempDir(t)
	res := run(t, dir, nil)
	if len(res.Generated) != len(ManagedKeys()) {
		t.Fatalf("first run must generate all keys, got %v", res.Generated)
	}
	app := readEnv(t, filepath.Join(dir, "app", "secrets.env"))
	key, err := base64.StdEncoding.DecodeString(app[KeyEncryptionKey])
	if err != nil || len(key) != 32 {
		t.Fatalf("encryption key invalid: %v", err)
	}
	if len(app[KeyBFFSecret]) < 32 {
		t.Fatal("bff secret too short")
	}

	res2 := run(t, dir, nil)
	if len(res2.Generated) != 0 {
		t.Fatalf("second run must not regenerate, got %v", res2.Generated)
	}
	if app2 := readEnv(t, filepath.Join(dir, "app", "secrets.env")); app2[KeyRedisPassword] != app[KeyRedisPassword] {
		t.Fatal("values changed between runs")
	}
}

func TestEnvOverridesAndFanOut(t *testing.T) {
	dir := tempDir(t)
	env := map[string]string{KeyRedisPassword: "operator-chosen-redis-pw", KeyPostgresPassword: "operator-chosen-pg-pw"}
	run(t, dir, env)

	app := readEnv(t, filepath.Join(dir, "app", "secrets.env"))
	if app[KeyRedisPassword] != "operator-chosen-redis-pw" {
		t.Fatal("env value must win")
	}
	web := readEnv(t, filepath.Join(dir, "web", "secrets.env"))
	if len(web) != 1 || web[KeyBFFSecret] != app[KeyBFFSecret] {
		t.Fatalf("web must receive only the BFF secret, got %v", web)
	}
	pg, _ := os.ReadFile(filepath.Join(dir, "postgres", "password"))
	if string(pg) != "operator-chosen-pg-pw" {
		t.Fatalf("postgres password file = %q", pg)
	}
	rc, _ := os.ReadFile(filepath.Join(dir, "redis", "redis.conf"))
	for _, want := range []string{"requirepass operator-chosen-redis-pw", "appendonly yes", "maxmemory-policy noeviction"} {
		if !strings.Contains(string(rc), want) {
			t.Errorf("redis.conf missing %q", want)
		}
	}

	var cf map[string]any
	raw, _ := os.ReadFile(filepath.Join(dir, "centrifugo", "config.json"))
	if err := json.Unmarshal(raw, &cf); err != nil {
		t.Fatal(err)
	}
	client := cf["client"].(map[string]any)
	if client["allowed_origins"].([]any)[0] != "http://localhost:8088" {
		t.Errorf("origin = %v", client["allowed_origins"])
	}
	if client["token"].(map[string]any)["hmac_secret_key"] != app[KeyCentrifugoHMAC] {
		t.Error("hmac mismatch")
	}
	if cf["http_api"].(map[string]any)["key"] != app[KeyCentrifugoAPIKey] {
		t.Error("api key mismatch")
	}

	res := run(t, dir, map[string]string{KeyRedisPassword: "a-different-redis-password"})
	if len(res.Overrides) != 1 || res.Overrides[0] != KeyRedisPassword {
		t.Fatalf("override not reported: %v", res.Overrides)
	}
}

func TestFilePermissions(t *testing.T) {
	dir := tempDir(t)
	run(t, dir, nil)
	for _, p := range []string{"app/secrets.env", "web/secrets.env", "postgres/password", "redis/redis.conf", "centrifugo/config.json", ".state/secrets.env"} {
		st, err := os.Stat(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o400 {
			t.Errorf("%s mode = %v, want 0400", p, st.Mode().Perm())
		}
		dst, _ := os.Stat(filepath.Dir(filepath.Join(dir, p)))
		if dst.Mode().Perm() != 0o500 {
			t.Errorf("%s dir mode = %v, want 0500", p, dst.Mode().Perm())
		}
	}
}

func TestRejectsWeakOperatorValues(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"short key":   {KeyEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 16))},
		"short bff":   {KeyBFFSecret: "tooshort"},
		"quote in pw": {KeyRedisPassword: `abcdefghijklmnop"q`},
		"short redis": {KeyRedisPassword: "short"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Run(Options{Dir: tempDir(t), Getenv: func(k string) string { return env[k] }, Owners: DefaultOwners(),
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestOwnersFromEnv(t *testing.T) {
	o, err := OwnersFromEnv(func(k string) string { return map[string]string{"MESSORA_UID_WEB": "1001"}[k] })
	if err != nil || o.Web != 1001 || o.App != 65532 {
		t.Fatalf("got %+v %v", o, err)
	}
	if _, err := OwnersFromEnv(func(k string) string { return "x" }); err == nil {
		t.Fatal("expected error")
	}
}

func TestAllowedOrigins(t *testing.T) {
	for _, c := range []struct {
		url, base string
		want      []string
	}{
		{"https://app.messora.tech", "messora.tech", []string{"https://app.messora.tech", "https://messora.tech", "https://*.messora.tech"}},
		{"http://localhost:8090/", "e2e.localhost", []string{"http://localhost:8090", "http://e2e.localhost:8090", "http://*.e2e.localhost:8090"}},
		{"https://messora.tech", ".Messora.Tech.", []string{"https://messora.tech", "https://*.messora.tech"}},
		{"https://panel.example.com", "", []string{"https://panel.example.com"}},
		{"", "", []string{"http://localhost"}},
	} {
		if got := AllowedOrigins(c.url, c.base); !slices.Equal(got, c.want) {
			t.Errorf("AllowedOrigins(%q, %q) = %v, want %v", c.url, c.base, got, c.want)
		}
	}
}

func TestCentrifugoConfigOrigins(t *testing.T) {
	raw, err := CentrifugoConfig("k", "h", AllowedOrigins("https://app.messora.tech", "messora.tech"))
	if err != nil {
		t.Fatal(err)
	}
	var cf struct {
		Client struct {
			AllowedOrigins []string `json:"allowed_origins"`
		} `json:"client"`
	}
	if err := json.Unmarshal(raw, &cf); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cf.Client.AllowedOrigins, "https://*.messora.tech") {
		t.Fatalf("allowed_origins = %v", cf.Client.AllowedOrigins)
	}
}
