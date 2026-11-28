package entitlements

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// The default plan keeps every feature on and the documented API budgets,
// so no workspace loses anything before WP8.
func TestDefaultPlan(t *testing.T) {
	p := Default()
	for _, f := range AllFeatures {
		if !p.Has(f) {
			t.Errorf("default plan lacks %s", f)
		}
	}
	if p.SessionRequestsPerMinute != 600 || p.APIKeyRequestsPerMinute != 300 || p.WebhookEndpoints != 50 {
		t.Fatalf("default budgets: %+v", p)
	}
	if p.APICallsPerMonth != Unlimited || p.Count(Contacts) != Unlimited {
		t.Fatal("default plan should not cap contacts or monthly calls")
	}
}

func TestOverlay(t *testing.T) {
	p, err := Overlay(Default(), []byte(`{"key":"starter","api_calls_per_month":10000,"webhook_endpoints":2,
		"api_key_requests_per_minute":0,"features":{"webhooks":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "starter" || p.APICallsPerMonth != 10000 || p.WebhookEndpoints != 2 {
		t.Fatalf("overlay: %+v", p)
	}
	// Missing or invalid budgets keep the default; features merge.
	if p.APIKeyRequestsPerMinute != 300 || p.SessionRequestsPerMinute != 600 {
		t.Fatalf("budgets: %+v", p)
	}
	if p.Has(Webhooks) || !p.Has(PublicAPI) || !p.Has(CustomSMTP) {
		t.Fatalf("features: %v", p.Features)
	}
	// The base is not modified.
	if !Default().Has(Webhooks) {
		t.Fatal("overlay changed the default")
	}
	if _, err := Overlay(Default(), []byte(`[`)); err == nil {
		t.Fatal("bad JSON accepted")
	}
}

func TestErrors(t *testing.T) {
	p := Default()
	p.WebhookEndpoints = 2
	if err := p.Allow(WebhookEndpoints, 1); err != nil {
		t.Fatal(err)
	}
	err := p.Allow(WebhookEndpoints, 2)
	e := ErrLimit(WebhookEndpoints, 2)
	if err == nil || e.Status != http.StatusConflict || e.Code != CodePlanLimitReached || e.Details[0].Code != "webhook_endpoints" || e.Details[0].Message != "2" {
		t.Fatalf("limit: %v %+v", err, e)
	}
	if p.Allow(Contacts, 1<<40) != nil {
		t.Fatal("unlimited count refused")
	}
	p.Features[AIAgents] = false
	if err := p.Require(AIAgents); err == nil {
		t.Fatal("missing feature allowed")
	}
	if f := ErrFeature(AIAgents); f.Status != http.StatusForbidden || f.Details[0].Code != "ai_agents" {
		t.Fatalf("feature error: %+v", f)
	}
}

// For asks the provider for the context's tenant; platform work has no
// plan (the default).
func TestFor(t *testing.T) {
	id := uuid.New()
	var got uuid.UUID
	prov := ProviderFunc(func(_ context.Context, t uuid.UUID) (Plan, error) {
		got = t
		p := Default()
		p.Key = "custom"
		return p, nil
	})
	p, err := For(tenant.WithID(context.Background(), id), prov)
	if err != nil || p.Key != "custom" || got != id {
		t.Fatalf("For: %+v %v %v", p, err, got)
	}
	if p, _ := For(context.Background(), prov); p.Key != "default" {
		t.Fatal("no tenant should give the default plan")
	}
	if p, _ := NewSettingsProvider(nil).Limits(context.Background(), id); p.Key != "default" {
		t.Fatal("settings provider without a pool")
	}
}
