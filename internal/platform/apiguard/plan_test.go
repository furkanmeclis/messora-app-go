package apiguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// An unknown x-messora-feature is a spec error.
func TestSpecFeatureNames(t *testing.T) {
	spec := func(feature string) *openapi3.T {
		s := specWith(t, "/webhook-endpoints", "session", "")
		s.Paths.Find("/webhook-endpoints").Get.Extensions = map[string]any{FeatureExtension: feature}
		return s
	}
	if _, err := New(spec("webhooks"), Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(spec("teleport"), Options{}); err == nil || !strings.Contains(err.Error(), "teleport") {
		t.Fatalf("unknown feature: %v", err)
	}
}

// Feature flags of the plan: API keys need public_api for every
// operation, an operation's x-messora-feature needs that feature, and the
// error names the feature.
func TestPlanFeatures(t *testing.T) {
	ctx := tenant.WithHostKind(tenant.WithID(context.Background(), uuid.New()), tenant.HostTenant)
	key := &principal.Principal{Kind: principal.KindAPIKey, ID: uuid.New()}
	user := &principal.Principal{Kind: principal.KindUser, ID: uuid.New()}
	off := func(f ...entitlements.Feature) *entitlements.Plan {
		p := entitlements.Default()
		p.Features = map[entitlements.Feature]bool{}
		for _, k := range entitlements.AllFeatures {
			p.Features[k] = true
		}
		for _, k := range f {
			p.Features[k] = false
		}
		return &p
	}
	g := &Guard{}
	for _, c := range []struct {
		name    string
		p       *principal.Principal
		plan    *entitlements.Plan
		feature entitlements.Feature
		want    string // feature key in the error, "" for allowed
	}{
		{"key with public_api", key, off(), "", ""},
		{"key without public_api", key, off(entitlements.PublicAPI), "", "public_api"},
		{"session without public_api", user, off(entitlements.PublicAPI), "", ""},
		{"webhooks op with webhooks", user, off(), entitlements.Webhooks, ""},
		{"webhooks op without webhooks", user, off(entitlements.Webhooks), entitlements.Webhooks, "webhooks"},
		{"key on a webhooks op without webhooks", key, off(entitlements.Webhooks), entitlements.Webhooks, "webhooks"},
		{"no plan (platform)", key, nil, entitlements.Webhooks, ""},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		err := g.entitled(httptest.NewRecorder(), r, c.p, c.plan, c.feature)
		if c.want == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		e, ok := apperr.As(err)
		if !ok || e.Status != http.StatusForbidden || e.Code != entitlements.CodeFeatureNotInPlan ||
			len(e.Details) != 1 || e.Details[0].Field != "feature" || e.Details[0].Code != c.want {
			t.Errorf("%s: %+v", c.name, err)
		}
	}
}

// The plan's request budgets replace the defaults for sessions and keys;
// platform sessions keep theirs.
func TestPlanProviderIsAskedForWorkspacePrincipalsOnly(t *testing.T) {
	asked := 0
	g := &Guard{o: Options{Plans: entitlements.ProviderFunc(func(context.Context, uuid.UUID) (entitlements.Plan, error) {
		asked++
		return entitlements.Default(), nil
	})}}
	tctx := tenant.WithHostKind(tenant.WithID(context.Background(), uuid.New()), tenant.HostTenant)
	if p, err := g.plan(tctx, &principal.Principal{Kind: principal.KindAPIKey}); err != nil || p == nil {
		t.Fatalf("workspace key: %v %v", p, err)
	}
	if p, _ := g.plan(tctx, nil); p != nil {
		t.Fatal("anonymous caller got a plan")
	}
	if p, _ := g.plan(tenant.Platform(context.Background()), &principal.Principal{Kind: principal.KindUser, Platform: true}); p != nil {
		t.Fatal("platform session got a plan")
	}
	if asked != 1 {
		t.Fatalf("provider asked %d times", asked)
	}
}
