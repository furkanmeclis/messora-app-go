//go:build integration

package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/furkanmeclis/messora-app-go/internal/modules/licensing"
	"github.com/furkanmeclis/messora-app-go/internal/platform/license"
	"github.com/furkanmeclis/messora-app-go/internal/shared/licensegate"
)

// fakeGate answers check-ins with the given status, signed with its key.
func fakeGate(t *testing.T, status *atomic.Value) (string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req license.CheckInRequest
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &req)
		p, err := license.Parse(req.Key, license.LicenseKeys)
		if err != nil {
			http.Error(w, "bad key", http.StatusBadRequest)
			return
		}
		tok, _ := license.Sign(license.GatePrefix, license.CheckInAnswer{V: 1, KeyID: "gate-test",
			LicenseID: p.ID, InstanceID: req.InstanceID, Status: status.Load().(string), At: time.Now(),
			Latest: &license.Release{Version: "9.9.9", ReleasedAt: time.Now(), Channel: "stable"}}, priv)
		_, _ = io.WriteString(w, tok)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, pub
}

func TestLicensingTrialKeysCheckInAndRestriction(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	license.LicenseKeys["test"] = pub
	t.Cleanup(func() { delete(license.LicenseKeys, "test"); delete(license.GateKeys, "gate-test") })
	var status atomic.Value
	status.Store("ok")
	gateURL, gatePub := fakeGate(t, &status)
	license.GateKeys["gate-test"] = gatePub

	h := newHarness(t)
	h.svc.licensing = licensing.New(h.pool, licensing.Options{LicenseKeys: license.LicenseKeys, GateKeys: license.GateKeys,
		GateURL: gateURL, Version: "test"})
	licensegate.Install(h.svc.licensing.Allowed)
	t.Cleanup(func() { licensegate.Install(func(context.Context) bool { return true }) })
	h.remount()
	owner, phone := h.connect()

	// A fresh installation is on trial and may send.
	r := h.do(call{method: "GET", path: "/api/v1/license", token: owner})
	h.expect(r, 200, "")
	if r.data()["status"] != "trial" || r.data()["license"] != nil {
		t.Fatalf("fresh: %s", r.raw)
	}
	if a := h.do(call{method: "GET", path: "/api/v1/appearance"}); a.data()["license"].(map[string]any)["status"] != "trial" {
		t.Fatalf("appearance: %s", a.raw)
	}

	issue := func(id string, mutate func(*license.Payload)) string {
		p := license.Payload{ID: id, KeyID: "test", Customer: "Acme", Plan: "pro", IssuedAt: time.Now().Add(-48 * time.Hour), CheckIn: license.CheckInOnline}
		if mutate != nil {
			mutate(&p)
		}
		tok, err := license.IssueLicense(p, priv)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	// Only valid keys, only the owner.
	h.expect(h.do(call{method: "PUT", path: "/api/v1/license", token: owner, body: map[string]any{"key": "msl1.not-a-real-key.at-all-really"}}), 422, "VALIDATION_ERROR")
	admin, _ := h.invite(owner, "lic-admin@example.com", "admin")
	h.expect(h.do(call{method: "PUT", path: "/api/v1/license", token: admin, body: map[string]any{"key": issue("lic_a", nil)}}), 403, "OWNER_ONLY")

	// An expired key restricts sending (after its grace period).
	past := time.Now().Add(-30 * 24 * time.Hour)
	r = h.do(call{method: "PUT", path: "/api/v1/license", token: owner, body: map[string]any{"key": issue("lic_old", func(p *license.Payload) { p.ExpiresAt = &past })}})
	h.expect(r, 200, "")
	if r.data()["status"] != "restricted" || r.data()["reason"] != "expired" {
		t.Fatalf("expired: %s", r.raw)
	}
	h.expect(h.send(owner, map[string]any{"to": "+905551112233", "phone_number_id": phone, "type": "text", "text": map[string]any{"body": "hi"}}), 403, "LICENSE_RESTRICTED")

	// A valid online key checks in at once and lifts the restriction.
	r = h.do(call{method: "PUT", path: "/api/v1/license", token: owner, body: map[string]any{"key": issue("lic_new", nil)}})
	h.expect(r, 200, "")
	d := r.data()
	if d["status"] != "active" || d["last_check_in_at"] == nil || d["license"].(map[string]any)["customer"] != "Acme" {
		t.Fatalf("active: %s", r.raw)
	}
	if rel := d["latest_release"].(map[string]any); rel["version"] != "9.9.9" || rel["update_allowed"] != true {
		t.Fatalf("release: %v", rel)
	}
	// Sending is no longer blocked by the licence (the 24-hour window still applies).
	if r := h.send(owner, map[string]any{"to": "+905551112233", "phone_number_id": phone, "type": "text", "text": map[string]any{"body": "hi"}}); strings.Contains(string(r.raw), "LICENSE_RESTRICTED") {
		t.Fatalf("still restricted: %s", r.raw)
	}

	// The gate revokes it: a short grace period starts.
	status.Store("revoked")
	if err := h.svc.licensing.CheckIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	r = h.do(call{method: "GET", path: "/api/v1/license", token: owner})
	if r.data()["status"] != "grace" || r.data()["reason"] != "revoked" {
		t.Fatalf("revoked: %s", r.raw)
	}
	// A forged answer (other key) is ignored.
	license.GateKeys["gate-test"] = pub
	if err := h.svc.licensing.CheckIn(context.Background()); err == nil {
		t.Fatal("answer signed with the wrong key accepted")
	}
}
