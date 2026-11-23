package config

import (
	"encoding/base64"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

var validKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

func TestLoadPrecedence(t *testing.T) {
	env := map[string]string{"POSTGRES_PASSWORD": "from-env", "LOG_LEVEL": ""}
	file := map[string]string{"POSTGRES_PASSWORD": "from-file", "REDIS_PASSWORD": "redis-file", "LOG_LEVEL": "debug"}
	cfg, err := loadFrom(envFrom(env), file)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Database.Password.Reveal(); got != "from-env" {
		t.Errorf("env must win over file, got %q", got)
	}
	if got := cfg.Redis.Password.Reveal(); got != "redis-file" {
		t.Errorf("file must fill missing env, got %q", got)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("empty env must not shadow file, got %q", cfg.LogLevel)
	}
	if cfg.Env != EnvProduction {
		t.Errorf("default env must be production, got %q", cfg.Env)
	}
}

func TestDSNEscapesPassword(t *testing.T) {
	d := DatabaseConfig{Host: "postgres", Port: 5432, User: "messora", Password: "p@ss/w:rd?", Name: "messora", SSLMode: "disable"}
	got := d.DSN()
	want := "postgres://messora:p%40ss%2Fw%3Ard%3F@postgres:5432/messora?sslmode=disable"
	if got != want {
		t.Fatalf("DSN = %q, want %q", got, want)
	}
	d.URL = "postgres://override"
	if d.DSN() != "postgres://override" {
		t.Fatal("DATABASE_URL must win")
	}
}

func TestSecretNeverPrints(t *testing.T) {
	s := Secret("hunter2")
	//nolint:staticcheck // deliberately exercising every fmt verb
	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%v", s), fmt.Sprintf("%s", s), fmt.Sprintf("%#v", s), fmt.Sprintf("%+v", struct{ P Secret }{s})} {
		if strings.Contains(out, "hunter2") {
			t.Fatalf("secret leaked: %q", out)
		}
	}
}

func TestEncryptionKeyValidation(t *testing.T) {
	cases := map[string]string{
		"not base64": "!!!",
		"too short":  base64.StdEncoding.EncodeToString(make([]byte, 16)),
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadFrom(envFrom(map[string]string{"APP_ENCRYPTION_KEY": v}), nil); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	cfg, err := loadFrom(envFrom(map[string]string{"APP_ENCRYPTION_KEY": validKey}), nil)
	if err != nil || len(cfg.EncryptionKey) != 32 {
		t.Fatalf("valid key rejected: %v", err)
	}
}

func TestValidateByRole(t *testing.T) {
	base := map[string]string{
		"POSTGRES_PASSWORD":      "pw",
		"APP_ENCRYPTION_KEY":     validKey,
		"BFF_SHARED_SECRET":      strings.Repeat("x", 32),
		"CENTRIFUGO_API_KEY":     "k",
		"CENTRIFUGO_HMAC_SECRET": "h",
	}
	cfg, err := loadFrom(envFrom(base), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []Role{RoleAPI, RoleWorker, RoleScheduler, RoleMigrate} {
		if err := cfg.Validate(r); err != nil {
			t.Errorf("%s: unexpected error %v", r, err)
		}
	}

	migrateOnly, _ := loadFrom(envFrom(map[string]string{"POSTGRES_PASSWORD": "pw"}), nil)
	if err := migrateOnly.Validate(RoleMigrate); err != nil {
		t.Errorf("migrate needs only the database: %v", err)
	}
	if err := migrateOnly.Validate(RoleWorker); err == nil || !strings.Contains(err.Error(), "APP_ENCRYPTION_KEY") {
		t.Errorf("worker must require encryption key, got %v", err)
	}
	if err := migrateOnly.Validate(RoleAPI); err == nil || !strings.Contains(err.Error(), "BFF_SHARED_SECRET") || !strings.Contains(err.Error(), "CENTRIFUGO_HMAC_SECRET") {
		t.Errorf("api must require the BFF and token secrets, got %v", err)
	}
	if err := migrateOnly.Validate(RoleWorker); err == nil || !strings.Contains(err.Error(), "CENTRIFUGO_API_KEY") {
		t.Errorf("worker publishes realtime events and needs the Centrifugo API key, got %v", err)
	}

	short := maps.Clone(base)
	short["BFF_SHARED_SECRET"] = "short"
	c2, _ := loadFrom(envFrom(short), nil)
	if err := c2.Validate(RoleAPI); err == nil {
		t.Error("short BFF secret must be rejected")
	}
}

func TestInvalidNumbersAreReported(t *testing.T) {
	_, err := loadFrom(envFrom(map[string]string{"DB_MAX_CONNS": "lots", "SHUTDOWN_TIMEOUT": "soon"}), nil)
	if err == nil || !strings.Contains(err.Error(), "DB_MAX_CONNS") || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
		t.Fatalf("expected both errors, got %v", err)
	}
	cfg, err := loadFrom(envFrom(map[string]string{"SHUTDOWN_TIMEOUT": "5s"}), nil)
	if err != nil || cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("got %v %v", cfg, err)
	}
}

func TestParseQueues(t *testing.T) {
	q, err := ParseQueues("critical=6, default=3 ,bulk=1")
	if err != nil || q["critical"] != 6 || q["default"] != 3 || q["bulk"] != 1 {
		t.Fatalf("got %v %v", q, err)
	}
	for _, bad := range []string{"", "critical", "critical=0", "critical=x"} {
		if _, err := ParseQueues(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestParseDotenv(t *testing.T) {
	in := "# comment\n\nexport A=1\nB = \"two words\"\nC='x=y'\nD=\n"
	m, err := ParseDotenv(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if m["A"] != "1" || m["B"] != "two words" || m["C"] != "x=y" || m["D"] != "" {
		t.Fatalf("got %#v", m)
	}
	if _, err := ParseDotenv(strings.NewReader("no-equals")); err == nil {
		t.Error("expected error for missing '='")
	}
	if _, err := ParseDotenv(strings.NewReader("1BAD=x")); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestMetaSettings(t *testing.T) {
	cfg, err := loadFrom(envFrom(map[string]string{"APP_ENV": "test", "META_DRY_RUN": "false", "META_GRAPH_API_VERSION": "v27.0"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Meta.DryRun || cfg.Meta.GraphVersion != "v27.0" {
		t.Fatalf("test environment must force dry-run: %+v", cfg.Meta)
	}
	if _, err := loadFrom(envFrom(map[string]string{"META_DRY_RUN": "maybe"}), nil); err == nil {
		t.Fatal("invalid boolean accepted")
	}
	for _, env := range []map[string]string{
		{"POSTGRES_PASSWORD": "pw", "META_GRAPH_API_VERSION": "26.0"},
		{"POSTGRES_PASSWORD": "pw", "META_WABA_ID": "123"},
		{"POSTGRES_PASSWORD": "pw", "META_ACCESS_TOKEN": "EAAG-token"},
	} {
		c, err := loadFrom(envFrom(env), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Validate(RoleMigrate); err == nil {
			t.Errorf("%v accepted", env)
		}
	}
}
