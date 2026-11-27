package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

func TestValidateNewSlug(t *testing.T) {
	for _, ok := range []string{"acme", "acme-store", "a1b", "x23456789012345678901234567890"} {
		if err := validateNewSlug(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"ab", "Acme", "-acme", "acme-", "a--b", "a_b", "x234567890123456789012345678901"} {
		e, ok := apperr.As(validateNewSlug(bad))
		if !ok || e.Code != "VALIDATION_ERROR" || e.Field != "workspace_slug" {
			t.Errorf("%q: %v", bad, e)
		}
	}
	for _, r := range tenant.ReservedSlugs {
		if err := validateNewSlug(r); !errors.Is(err, errSlugReserved) {
			t.Errorf("%q: %v, want SLUG_RESERVED", r, err)
		}
	}
}

func TestScopeOfBindsSignInChallenges(t *testing.T) {
	id := uuid.New()
	if got := scopeOf(tenant.WithID(context.Background(), id)); got != "tenant:"+id.String() {
		t.Fatalf("tenant scope %q", got)
	}
	if got := scopeOf(tenant.Platform(tenant.WithID(context.Background(), id))); got != "platform" {
		t.Fatalf("platform scope %q", got)
	}
	if mfaKey("tenant:"+id.String(), "msm_x") == mfaKey("platform", "msm_x") {
		t.Fatal("the same challenge token maps to one key in two scopes")
	}
}

func TestDeviceAndRoleLabels(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Chrome/131.0 Safari/537.36": "Chrome, macOS",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 Version/18.1 Mobile/15E148 Safari/604.1": "Safari, iOS",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0":                                     "Firefox, Windows",
		"": "",
	}
	for ua, want := range cases {
		if got := deviceOf(ua); got != want {
			t.Errorf("deviceOf(%q) = %q, want %q", ua, got, want)
		}
	}
	if roleLabel("admin", "tr") != "Yönetici" || roleLabel("admin", "en") != "Admin" || roleLabel("custom", "tr") != "custom" {
		t.Fatal("role labels")
	}
}

func TestWorkspaceLinks(t *testing.T) {
	w := workspace{URL: "https://acme.messora.tech", Name: "Acme"}
	if got := w.link("/verify-email", "token", "msv_a+b"); got != "https://acme.messora.tech/verify-email?token=msv_a%2Bb" {
		t.Fatalf("link %q", got)
	}
	if c := w.common("Ayşe Yılmaz"); c.Name != "Ayşe" || c.TenantName != "Acme" || c.TenantLogoURL != "" {
		t.Fatalf("common %+v", c)
	}
}
