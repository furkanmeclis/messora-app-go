package crypto

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

var fast = Argon2Params{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple", fast)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected format %q", h)
	}
	if _, err := VerifyPassword("correct horse battery staple", h, fast); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPassword("wrong", h, fast); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	h2, _ := HashPassword("correct horse battery staple", fast)
	if h == h2 {
		t.Fatal("salt must make hashes unique")
	}
}

func TestRehashOnWeakerParams(t *testing.T) {
	h, _ := HashPassword("pw", fast)
	needs, err := VerifyPassword("pw", h, DefaultArgon2)
	if err != nil || !needs {
		t.Fatalf("needsRehash=%v err=%v", needs, err)
	}
	needs, _ = VerifyPassword("pw", h, fast)
	if needs {
		t.Fatal("equal params must not need rehash")
	}
}

func TestRejectsMalformedAndAbusiveHashes(t *testing.T) {
	for _, h := range []string{
		"", "plain", "$2y$10$bcrypt", "$argon2i$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$a2V5",
		"$argon2id$v=19$m=999999999,t=1,p=1$c2FsdHNhbHRzYWx0$a2V5a2V5a2V5a2V5a2V5", // memory bomb
		"$argon2id$v=19$m=8192,t=1,p=1$!!$!!",
	} {
		if _, err := VerifyPassword("x", h, fast); err == nil {
			t.Errorf("accepted %q", h)
		}
	}
}

func TestTokens(t *testing.T) {
	a, _ := NewToken("msk_live_")
	b, _ := NewToken("msk_live_")
	if a == b || !strings.HasPrefix(a, "msk_live_") || len(a) != len("msk_live_")+43 {
		t.Fatalf("tokens %q %q", a, b)
	}
	if !bytes.Equal(HashToken(a), HashToken(a)) || bytes.Equal(HashToken(a), HashToken(b)) {
		t.Fatal("hash")
	}
	d, _ := RandomDigits(6)
	if !regexp.MustCompile(`^\d{6}$`).MatchString(d) {
		t.Fatal(d)
	}
	rc, _ := RecoveryCode()
	if !regexp.MustCompile(`^[a-z2-9]{5}-[a-z2-9]{5}$`).MatchString(rc) || strings.ContainsAny(rc, "ilo01") {
		t.Fatal(rc)
	}
}

func TestSealerBindsAssociatedDataAndRotates(t *testing.T) {
	k1 := bytes.Repeat([]byte{1}, 32)
	k2 := bytes.Repeat([]byte{2}, 32)
	s1, err := NewSealer(k1, nil)
	if err != nil {
		t.Fatal(err)
	}
	ct, _ := s1.Seal([]byte("JBSWY3DPEHPK3PXP"), "totp:user-1")
	pt, err := s1.Open(ct, "totp:user-1")
	if err != nil || string(pt) != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := s1.Open(ct, "totp:user-2"); !errors.Is(err, ErrDecrypt) {
		t.Fatal("ciphertext must be bound to its associated data")
	}
	tampered := append([]byte{}, ct...)
	tampered[len(tampered)-1] ^= 1
	if _, err := s1.Open(tampered, "totp:user-1"); !errors.Is(err, ErrDecrypt) {
		t.Fatal("tampering must be detected")
	}

	// Rotation: new key is current, old ciphertexts still open.
	s2, err := NewSealer(k2, map[byte][]byte{1: k1})
	if err != nil {
		t.Fatal(err)
	}
	if pt, err := s2.Open(ct, "totp:user-1"); err != nil || string(pt) != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("old version must open after rotation: %v", err)
	}
	ct2, _ := s2.Seal([]byte("x"), "a")
	if ct2[0] != 2 {
		t.Fatalf("new ciphertexts use the new version, got %d", ct2[0])
	}
	if _, err := s1.Open(ct2, "a"); !errors.Is(err, ErrDecrypt) {
		t.Fatal("old sealer cannot open new version")
	}
	if _, err := NewSealer(make([]byte, 16), nil); err == nil {
		t.Fatal("short key accepted")
	}
}
