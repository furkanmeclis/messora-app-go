package storage

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestKeyScheme(t *testing.T) {
	k, err := keyAt("acme", "media", ".jpg", time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^t/acme/media/2026/03/[0-9a-f-]{36}\.jpg$`).MatchString(k) {
		t.Fatalf("key %q", k)
	}
	if k2, _ := Key("acme", "exports", ""); !regexp.MustCompile(`^t/acme/exports/\d{4}/\d{2}/[0-9a-f-]{36}$`).MatchString(k2) {
		t.Fatalf("key without extension %q", k2)
	}
	for _, bad := range [][3]string{{"", "media", ""}, {"../x", "media", ""}, {"a/b", "media", ""}, {"acme", "", ""},
		{"acme", "me/dia", ""}, {"acme", "media", "../x"}, {"acme", "media", "toolongextension"}} {
		if _, err := Key(bad[0], bad[1], bad[2]); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Key%v accepted", bad)
		}
	}
}

func TestValidateKey(t *testing.T) {
	good := []string{"t/acme/media/2026/01/x.jpg", "ab/cd/legacy-1234", "imports/uploads/f.csv", "exports/1.xlsx"}
	for _, k := range good {
		if err := ValidateKey(k); err != nil {
			t.Errorf("%q rejected: %v", k, err)
		}
	}
	bad := []string{"", "/etc/passwd", "../etc/passwd", "a/../../b", "a/./b", "a//b", "a/b/", "a\\b", "..", ".",
		"a/..", "_staging/x", "a/_x", "a/.hidden", "a/b._meta/../c", "a\x00b", "a/\x01", string(make([]byte, 600))}
	for _, k := range bad {
		if err := ValidateKey(k); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%q accepted", k)
		}
	}
}

func TestCheckTenantKey(t *testing.T) {
	cases := []struct {
		tenant, key string
		ok          bool
	}{
		{"acme", "t/acme/media/2026/01/x", true},
		{"acme", "t/other/media/2026/01/x", false},
		{"acme", "t/acme", false},
		{"acme", "t/acme/", false},
		{"acme", "t/acmex/media/x", false},
		{"acme", "t/acme/../other/x", false},
		{"acme", "ab/cd/legacy", false},
		{DefaultTenant, "ab/cd/legacy", true},
		{DefaultTenant, "t/acme/media/x", false},
		{"../x", "t/../x/y", false},
	}
	for _, c := range cases {
		if err := CheckTenantKey(c.tenant, c.key); (err == nil) != c.ok {
			t.Errorf("CheckTenantKey(%q, %q) = %v, want ok=%v", c.tenant, c.key, err, c.ok)
		}
	}
}

func TestTenantFromContext(t *testing.T) {
	if got := TenantKey(context.Background()); got != DefaultTenant {
		t.Fatalf("no tenant: %q", got)
	}
	if got := TenantKey(WithTenant(context.Background(), "acme")); got != "acme" {
		t.Fatalf("tenant: %q", got)
	}
	// A malformed tenant fails closed, never falls back to the default.
	evil := TenantKey(WithTenant(context.Background(), "../evil"))
	if _, err := Key(evil, "media", ""); err == nil {
		t.Fatal("key built for a malformed tenant")
	}
	if CheckTenantKey(evil, "ab/cd/legacy") == nil {
		t.Fatal("malformed tenant reached default-tenant keys")
	}
}

func TestExtFor(t *testing.T) {
	if ExtFor("image/JPEG; q=1") != "jpg" || ExtFor("application/x-unknown") != "" {
		t.Fatal("ExtFor")
	}
}
