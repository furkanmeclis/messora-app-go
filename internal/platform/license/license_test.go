package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

func keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

var day = 24 * time.Hour

func issue(t *testing.T, priv ed25519.PrivateKey, mutate func(*Payload)) string {
	t.Helper()
	p := Payload{ID: "lic_1", KeyID: "k1", Customer: "Acme", Plan: "pro", IssuedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), CheckIn: CheckInOffline}
	if mutate != nil {
		mutate(&p)
	}
	tok, err := IssueLicense(p, priv)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestParse(t *testing.T) {
	pub, priv := keypair(t)
	other, otherPriv := keypair(t)
	keys := Keyring{"k1": pub}
	tok := issue(t, priv, nil)
	p, err := Parse(tok, keys)
	if err != nil || p.Customer != "Acme" || p.V != 1 {
		t.Fatalf("valid key: %+v %v", p, err)
	}
	// Tampering with the payload breaks the signature.
	parts := strings.Split(tok, ".")
	forged, _ := Sign(LicensePrefix, Payload{V: 1, ID: "lic_1", KeyID: "k1", Customer: "Evil", IssuedAt: p.IssuedAt, CheckIn: CheckInOffline}, otherPriv)
	fparts := strings.Split(forged, ".")
	for name, bad := range map[string]string{
		"payload swapped": parts[0] + "." + fparts[1] + "." + parts[2],
		"other signer":    forged,
		"truncated":       parts[0] + "." + parts[1],
		"wrong prefix":    "msg1." + parts[1] + "." + parts[2],
		"garbage":         "hello",
	} {
		if _, err := Parse(bad, keys); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := Parse(tok, Keyring{"k2": other}); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("unknown kid: %v", err)
	}
}

func TestEvaluate(t *testing.T) {
	pub, priv := keypair(t)
	keys := Keyring{"k1": pub}
	t0 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	exp := t0.Add(100 * day)
	expiring := issue(t, priv, func(p *Payload) { p.ExpiresAt = &exp })
	perpetual := issue(t, priv, nil)
	online := issue(t, priv, func(p *Payload) { p.CheckIn = CheckInOnline })
	licensed := t0
	checked := t0.Add(10 * day)
	revoked := t0.Add(5 * day)

	cases := []struct {
		name   string
		now    time.Time
		st     Stored
		status Status
		reason string
	}{
		{"trial", t0.Add(3 * day), Stored{InstalledAt: t0}, StatusTrial, ""},
		{"trial over", t0.Add(15 * day), Stored{InstalledAt: t0}, StatusRestricted, ReasonTrialEnded},
		{"bad key is no licence", t0.Add(15 * day), Stored{InstalledAt: t0, Key: "msl1.x.y"}, StatusRestricted, ReasonTrialEnded},
		{"perpetual", t0.Add(900 * day), Stored{InstalledAt: t0, Key: perpetual}, StatusActive, ""},
		{"active", t0.Add(10 * day), Stored{InstalledAt: t0, Key: expiring}, StatusActive, ""},
		{"expiring", exp.Add(-10 * day), Stored{InstalledAt: t0, Key: expiring}, StatusExpiring, ""},
		{"grace after expiry", exp.Add(3 * day), Stored{InstalledAt: t0, Key: expiring}, StatusGrace, ReasonExpired},
		{"restricted after grace", exp.Add(15 * day), Stored{InstalledAt: t0, Key: expiring}, StatusRestricted, ReasonExpired},
		{"clock set back", t0.Add(10 * day), Stored{InstalledAt: t0, Key: expiring, LastSeen: exp.Add(20 * day)}, StatusRestricted, ReasonExpired},
		{"revoked grace", revoked.Add(2 * day), Stored{InstalledAt: t0, Key: perpetual, RevokedAt: &revoked, Revocation: ReasonRevoked}, StatusGrace, ReasonRevoked},
		{"revoked", revoked.Add(8 * day), Stored{InstalledAt: t0, Key: perpetual, RevokedAt: &revoked, Revocation: ReasonRevoked}, StatusRestricted, ReasonRevoked},
		{"superseded", revoked.Add(8 * day), Stored{InstalledAt: t0, Key: perpetual, RevokedAt: &revoked, Revocation: ReasonSuperseded}, StatusRestricted, ReasonSuperseded},
		{"online fresh", checked.Add(5 * day), Stored{InstalledAt: t0, Key: online, LicensedAt: &licensed, LastCheckIn: &checked}, StatusActive, ""},
		{"online warning", checked.Add(20 * day), Stored{InstalledAt: t0, Key: online, LicensedAt: &licensed, LastCheckIn: &checked}, StatusGrace, ReasonCheckInOverdue},
		{"online overdue", checked.Add(31 * day), Stored{InstalledAt: t0, Key: online, LicensedAt: &licensed, LastCheckIn: &checked}, StatusRestricted, ReasonCheckInOverdue},
		{"online never checked", t0.Add(31 * day), Stored{InstalledAt: t0, Key: online, LicensedAt: &licensed}, StatusRestricted, ReasonCheckInOverdue},
		{"offline never checks in", t0.Add(400 * day), Stored{InstalledAt: t0, Key: perpetual}, StatusActive, ""},
	}
	for _, c := range cases {
		s := Evaluate(c.now, c.st, keys)
		if s.Status != c.status || s.Reason != c.reason {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, s.Status, s.Reason, c.status, c.reason)
		}
		if s.Allowed() != (c.status != StatusRestricted) {
			t.Errorf("%s: Allowed()=%v", c.name, s.Allowed())
		}
	}
}

func TestUpdatesAndGateAnswer(t *testing.T) {
	pub, priv := keypair(t)
	until := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	p := Payload{UpdatesUntil: &until}
	if !p.UpdatesAllowed(Release{ReleasedAt: until.Add(-day)}) || p.UpdatesAllowed(Release{ReleasedAt: until.Add(day)}) {
		t.Fatal("updates window")
	}
	if !(Payload{}).UpdatesAllowed(Release{ReleasedAt: until.Add(1000 * day)}) {
		t.Fatal("unlimited updates")
	}
	tok, _ := Sign(GatePrefix, CheckInAnswer{V: 1, KeyID: "g1", LicenseID: "lic_1", Status: "revoked"}, priv)
	a, err := OpenAnswer(tok, Keyring{"g1": pub})
	if err != nil || a.Status != "revoked" {
		t.Fatalf("answer: %+v %v", a, err)
	}
	if _, err := OpenAnswer(tok, Keyring{"g2": pub}); err == nil {
		t.Fatal("answer with unknown gate key accepted")
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	f.Add("msl1.e30.AAAA")
	f.Add("msl1..")
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s, Keyring{"k1": pub})
	})
}
