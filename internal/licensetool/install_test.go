package licensetool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
)

func TestInstallCompose(t *testing.T) {
	compose, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	caddy, err := os.ReadFile("../../deployments/caddy/Caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	out, err := InstallCompose(compose, caddy, "registry.example", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, bad := range []string{"build:", "context:", "./deployments", "./frontend"} {
		if strings.Contains(s, bad) {
			t.Errorf("install file still contains %q", bad)
		}
	}
	for _, want := range []string{
		"${MESSORA_IMAGE:-registry.example/messora}:${MESSORA_VERSION:-1.2.3}",
		"${MESSORA_WEB_IMAGE:-registry.example/messora-web}:${MESSORA_VERSION:-1.2.3}",
		"{$$TRUSTED_PROXIES:127.0.0.1/32}",
		"docker login registry.example",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install file lacks %q", want)
		}
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("not YAML: %v", err)
	}

	// The real check: Compose accepts it and hands Caddy its placeholders.
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), out, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, docker, "compose", "-f", "compose.yml", "config")
	cmd.Dir = dir
	cfg, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(cfg), "unknown") && strings.Contains(string(cfg), "compose") {
			t.Skip("docker compose not available")
		}
		t.Fatalf("docker compose config: %v\n%s", err, cfg)
	}
	c := string(cfg)
	for _, want := range []string{
		// `compose config` prints content re-escaped ($$); CI checks the
		// file Caddy actually receives.
		"trusted_proxies static {$$TRUSTED_PROXIES:127.0.0.1/32}",
		"image: registry.example/messora:1.2.3",
		"image: registry.example/messora-web:1.2.3",
		"target: /etc/caddy/Caddyfile",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("compose config lacks %q", want)
		}
	}
	if strings.Contains(c, "build:") {
		t.Error("compose config has a build section")
	}
}
