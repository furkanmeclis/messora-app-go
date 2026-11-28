//go:build integration

package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/furkanmeclis/messora-app-go/internal/config"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/settings"
	"github.com/furkanmeclis/messora-app-go/internal/platform/centrifugo"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/httpx"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer"
	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/testutil"
)

// rawUpload sends a file body the way the BFF does.
func (h *harness) rawUpload(method, path, token, contentType string, body []byte) resp {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(httpx.BFFHeader, bffSecret)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: raw}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func TestTwoFactorIsOptionalByDefault(t *testing.T) {
	h := newHarness(t)
	token := h.ownerSession("o@example.com", "O")
	r := h.do(call{method: "GET", path: "/api/v1/me", token: token})
	if r.data()["mfa_required"] != false {
		t.Fatalf("a fresh owner must not be forced to enrol: %s", r.raw)
	}
	r = h.do(call{method: "GET", path: "/api/v1/settings", token: token})
	h.expect(r, 200, "")
	if roles := r.data()["mfa_required_roles"].([]any); len(roles) != 0 {
		t.Fatalf("default policy: %v", roles)
	}
	if setup := r.data()["setup"].(map[string]any); setup["completed"] != false {
		t.Fatalf("wizard must be pending on a fresh install: %v", setup)
	}
}

func TestWorkspaceSettingsRegionalAndWizard(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()

	r := h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{
		"default_locale": "tr", "date_format": "dd/MM/yyyy", "time_format": "12h",
		"setup_steps": []string{"organization", "language", "organization"},
	}})
	h.expect(r, 200, "")
	d := r.data()
	if d["default_locale"] != "tr" || d["date_format"] != "dd/MM/yyyy" || d["time_format"] != "12h" {
		t.Fatalf("regional settings: %s", r.raw)
	}
	if _, ok := d["theme"]; ok {
		t.Fatalf("the theme engine is gone: %s", r.raw)
	}
	if steps := d["setup"].(map[string]any)["steps"].([]any); len(steps) != 2 {
		t.Fatalf("steps must be de-duplicated: %v", steps)
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"default_locale": "de"}}), 422, "")
	// The removed theme field and wizard step are rejected.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"theme": map[string]any{"preset": "ocean"}}}), 422, "")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"setup_steps": []string{"theme"}}}), 422, "")

	// The appearance is public to the panel (sign-in page) and matches.
	r = h.do(call{method: "GET", path: "/api/v1/appearance"})
	h.expect(r, 200, "")
	if r.data()["default_locale"] != "tr" {
		t.Fatalf("appearance: %s", r.raw)
	}

	// An admin may change the workspace settings too.
	admin, _ := h.invite(owner, "admin@example.com", "admin")
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: admin, body: map[string]any{"organization_name": "Acme Two"}}), 200, "")

	r = h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"setup_completed": true}})
	h.expect(r, 200, "")
	if r.data()["setup"].(map[string]any)["completed"] != true {
		t.Fatal("wizard completion not stored")
	}
}

func TestSettingsFromEnvironmentAreLocked(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	if err := h.svc.settings.SetOverrides(map[string]string{"organization_name": "From Env", "default_locale": "tr"}); err != nil {
		t.Fatal(err)
	}
	r := h.do(call{method: "GET", path: "/api/v1/settings", token: owner})
	h.expect(r, 200, "")
	if r.data()["organization_name"] != "From Env" || r.data()["default_locale"] != "tr" {
		t.Fatalf("overrides not applied: %s", r.raw)
	}
	if locked := r.data()["locked"].([]any); len(locked) != 2 {
		t.Fatalf("locked: %v", locked)
	}
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"organization_name": "Panel"}}), 409, "SETTING_MANAGED_BY_ENV")
	// Unlocked fields still change, and the environment value is not stored.
	h.expect(h.do(call{method: "PATCH", path: "/api/v1/settings", token: owner, body: map[string]any{"time_format": "12h"}}), 200, "")
	raw, err := db.New(h.pool).GetSetting(h.ctx(), "workspace")
	if err != nil || strings.Contains(string(raw), "From Env") {
		t.Fatalf("environment value persisted: %s %v", raw, err)
	}
	if err := h.svc.settings.SetOverrides(map[string]string{"default_locale": "de"}); err == nil {
		t.Fatal("invalid environment value accepted")
	}
}

func TestLogoUploadAndServe(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

	h.expect(h.rawUpload("PUT", "/api/v1/settings/logo", owner, "application/octet-stream", []byte("<html><script>alert(1)</script>")), 422, "VALIDATION_ERROR")
	r := h.rawUpload("PUT", "/api/v1/settings/logo", owner, "image/png", png)
	h.expect(r, 200, "")
	logo := r.data()["logo"].(map[string]any)
	if logo["content_type"] != "image/png" || logo["version"] == "" {
		t.Fatalf("logo: %s", r.raw)
	}

	req, _ := http.NewRequest("GET", h.srv.URL+"/api/v1/appearance/logo", nil)
	req.Header.Set(httpx.BFFHeader, bffSecret)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Equal(body, png) || res.Header.Get("Content-Type") != "image/png" ||
		!strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("serve: %d %v", res.StatusCode, res.Header)
	}

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`)
	h.expect(h.rawUpload("PUT", "/api/v1/settings/logo", owner, "image/svg+xml", svg), 200, "")
	h.expect(h.rawUpload("PUT", "/api/v1/settings/logo", owner, "image/png", bytes.Repeat([]byte{1}, 2<<20+1)), 422, "")

	r = h.do(call{method: "DELETE", path: "/api/v1/settings/logo", token: owner})
	h.expect(r, 200, "")
	if r.data()["logo"] != nil {
		t.Fatal("logo not cleared")
	}
	if entries, _ := os.ReadDir(filepath.Join(h.svc.cfg.Storage.LocalDir, "_public", "t", "default", "branding")); len(entries) != 0 {
		t.Fatalf("logo file left behind: %v", entries)
	}
	h.expect(h.do(call{method: "GET", path: "/api/v1/appearance/logo"}), 404, "NOT_FOUND")
}

func TestSMTPSettingsFromThePanel(t *testing.T) {
	h := newHarness(t)
	owner, _ := h.setupOwner()
	// The harness injects a capturing mailer; exercise the real one here.
	mail, err := settings.NewMail(h.svc.queries, h.sealer(), mailer.Config{})
	if err != nil {
		t.Fatal(err)
	}
	h.svc.mail = mail
	h.remount()

	r := h.do(call{method: "GET", path: "/api/v1/settings/smtp", token: owner})
	h.expect(r, 200, "")
	if r.data()["enabled"] != false || r.data()["managed_by_env"] != false {
		t.Fatalf("initial: %s", r.raw)
	}
	addr, got := fakeSMTPServer(t)
	host, port := splitHostPort(t, addr)
	cfg := map[string]any{"host": host, "port": port, "username": "mailer", "password": "Smtp-secret-pass-1", "from": "Acme <no-reply@acme.test>", "tls": "none"}

	// A test with the unsaved configuration goes out before saving.
	h.expect(h.do(call{method: "POST", path: "/api/v1/settings/smtp/test", token: owner, body: map[string]any{"to": "ops@acme.test", "config": cfg}}), 204, "")
	if msg := <-got; !strings.Contains(msg, "E-mail delivery works") {
		t.Fatalf("test message: %q", msg)
	}

	r = h.do(call{method: "PUT", path: "/api/v1/settings/smtp", token: owner, body: cfg})
	h.expect(r, 200, "")
	if r.data()["password_set"] != true || r.data()["enabled"] != true || r.data()["host"] != host {
		t.Fatalf("saved: %s", r.raw)
	}
	raw, _ := db.New(h.pool).GetSetting(h.ctx(), "smtp")
	if strings.Contains(string(raw), "Smtp-secret-pass-1") {
		t.Fatal("SMTP password stored in plain text")
	}
	// Omitting the password keeps it; the application sender uses the new
	// configuration without a restart.
	delete(cfg, "password")
	cfg["from"] = "Acme Support <support@acme.test>"
	r = h.do(call{method: "PUT", path: "/api/v1/settings/smtp", token: owner, body: cfg})
	if r.data()["password_set"] != true {
		t.Fatal("password dropped when omitted")
	}
	if !mail.Enabled(h.ctx()) {
		t.Fatal("sender still disabled after saving")
	}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/smtp", token: owner, body: map[string]any{"host": "smtp.acme.test", "port": 587, "username": "", "from": "not an address", "tls": "starttls"}}), 422, "VALIDATION_ERROR")

	// SMTP_HOST in the environment is the platform sender (Phase 17 WP8):
	// it no longer locks a company's own server (enterprise plans).
	envMail, _ := settings.NewMail(h.svc.queries, h.sealer(), mailer.Config{Host: "smtp.env.test", From: "x@env.test"})
	h.svc.mail = envMail
	h.remount()
	r = h.do(call{method: "GET", path: "/api/v1/settings/smtp", token: owner})
	if r.data()["managed_by_env"] != false || r.data()["host"] != host {
		t.Fatalf("env: %s", r.raw)
	}
	h.expect(h.do(call{method: "PUT", path: "/api/v1/settings/smtp", token: owner, body: cfg}), 200, "")
}

func (h *harness) sealer() *crypto.Sealer {
	h.t.Helper()
	s, err := crypto.NewSealer(h.svc.cfg.EncryptionKey, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	return s
}

// remount serves the API again after h.svc changed.
func (h *harness) remount() {
	h.t.Helper()
	r := chi.NewRouter()
	r.NotFound(httpx.NotFound)
	r.Use(httpx.RequestID, httpx.Recover(testutil.DiscardLogger()), httpx.IdentifyBFF(bffSecret))
	cf, _ := centrifugo.New(config.CentrifugoConfig{APIURL: "http://127.0.0.1:1/api", APIKey: "x"})
	sys := newSystemHandler(h.pool, testutil.Redis(h.t), queue.NewInspector(testutil.Redis(h.t)), cf, testutil.DiscardLogger())
	if err := mountAPI(r, h.svc, h.pool, sys, testutil.Redis(h.t), testutil.DiscardLogger()); err != nil {
		h.t.Fatal(err)
	}
	srv := httptest.NewServer(r)
	h.t.Cleanup(srv.Close)
	h.srv = srv
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(p)
	return host, port
}

// fakeSMTPServer accepts messages and returns each DATA section.
func fakeSMTPServer(t *testing.T) (string, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	ch := make(chan string, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSMTP(c, ch)
		}
	}()
	return ln.Addr().String(), ch
}

func serveSMTP(c net.Conn, ch chan<- string) {
	defer c.Close()
	r := bufio.NewReader(c)
	w := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
	w("220 fake ESMTP")
	var data strings.Builder
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if inData {
			if line == ".\r\n" {
				inData = false
				w("250 queued")
				ch <- data.String()
				data.Reset()
				continue
			}
			data.WriteString(line)
			continue
		}
		switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
		case strings.HasPrefix(cmd, "EHLO"):
			w("250-fake")
			w("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "HELO"):
			w("250 fake")
		case strings.HasPrefix(cmd, "AUTH"):
			w("235 ok")
		case strings.HasPrefix(cmd, "DATA"):
			inData = true
			w("354 go ahead")
		case strings.HasPrefix(cmd, "QUIT"):
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}
