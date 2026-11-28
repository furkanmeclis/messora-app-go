// Package entitlements answers what a tenant's plan allows: request
// budgets, monthly quotas, counts of things it may create and feature
// flags (docs/ENTITLEMENTS.md, docs/PHASE-17.md §1.5).
//
// Callers ask a Provider for the tenant's Plan and enforce it where the
// action happens: the API guard enforces the request budgets, the monthly
// API quota and the public_api flag; the webhook module the endpoint count,
// the delivery rate and the webhooks flag. WP8 replaces the default provider
// with one that reads the plan table; nothing else changes.
package entitlements

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// Feature is a capability a plan switches on or off.
type Feature string

// Features. The keys are part of the API (FEATURE_NOT_IN_PLAN details).
const (
	// CustomSMTP lets a company send through its own SMTP server.
	CustomSMTP Feature = "custom_smtp"
	// BYOAIKeys lets a company use its own AI provider keys.
	BYOAIKeys Feature = "byo_ai_keys"
	// CustomDomain lets a company add its own domain.
	CustomDomain Feature = "custom_domain"
	// PublicAPI lets a company's API keys call the API.
	PublicAPI Feature = "public_api"
	// Webhooks lets a company register outgoing webhook endpoints.
	Webhooks Feature = "webhooks"
	// AIAgents lets a company run AI agents (AI replies and steps).
	AIAgents Feature = "ai_agents"
)

// AllFeatures lists every feature in a stable order.
var AllFeatures = []Feature{CustomSMTP, BYOAIKeys, CustomDomain, PublicAPI, Webhooks, AIAgents}

// Resource is something a plan counts.
type Resource string

// Counted resources. The keys are part of the API (PLAN_LIMIT_REACHED
// details).
const (
	WebhookEndpoints Resource = "webhook_endpoints"
	Contacts         Resource = "contacts"
	Users            Resource = "users"
	PhoneNumbers     Resource = "phone_numbers"
	AIAgentCount     Resource = "ai_agents"
)

// Unlimited is the value of a count or quota that has no limit.
const Unlimited = 0

// Plan is what one tenant may do. Counts and quotas use Unlimited (0) for
// "no limit"; a resource the plan does not include at all is switched off
// by its feature flag, not by a zero.
type Plan struct {
	// Key names the plan ("default" for the built-in one).
	Key string `json:"key"`

	// API request budgets per minute (GCRA, burst = rate) of one panel
	// session and of one API key.
	SessionRequestsPerMinute int `json:"session_requests_per_minute"`
	APIKeyRequestsPerMinute  int `json:"api_key_requests_per_minute"`
	// APICallsPerMonth caps the calls of all API keys of the tenant per
	// calendar month (UTC). Panel sessions do not count.
	APICallsPerMonth int64 `json:"api_calls_per_month"`

	// WebhookEndpoints is how many endpoints may be registered.
	WebhookEndpoints int `json:"webhook_endpoints"`
	// WebhookDeliveriesPerMinute is the delivery attempts of all of the
	// tenant's endpoints together.
	WebhookDeliveriesPerMinute int `json:"webhook_deliveries_per_minute"`

	// Counts, enforced when the resource is created (WP8 for all but
	// webhook endpoints).
	Contacts     int64 `json:"contacts"`
	Users        int   `json:"users"`
	PhoneNumbers int   `json:"phone_numbers"`
	AIAgents     int   `json:"ai_agents"`
	// StorageBytes caps media and file storage.
	StorageBytes int64 `json:"storage_bytes"`
	// CampaignMessagesPerSecond caps the tenant's campaign send rate
	// (together with the fairness setting, the lower one wins).
	CampaignMessagesPerSecond int `json:"campaign_messages_per_second"`

	// Features maps every feature to whether the plan includes it; a
	// missing key means "not included".
	Features map[Feature]bool `json:"features"`
}

// Has reports whether the plan includes f.
func (p Plan) Has(f Feature) bool { return p.Features[f] }

// Count returns the limit of r (Unlimited when there is none).
func (p Plan) Count(r Resource) int64 {
	switch r {
	case WebhookEndpoints:
		return int64(p.WebhookEndpoints)
	case Contacts:
		return p.Contacts
	case Users:
		return int64(p.Users)
	case PhoneNumbers:
		return int64(p.PhoneNumbers)
	case AIAgentCount:
		return int64(p.AIAgents)
	}
	return Unlimited
}

// Default is the built-in plan until WP8 brings plans: generous limits and
// every feature on, so no existing workspace loses anything. The request
// budgets are the documented defaults of the API (600 per session, 300 per
// key per minute).
func Default() Plan {
	return Plan{
		Key:                        "default",
		SessionRequestsPerMinute:   600,
		APIKeyRequestsPerMinute:    300,
		APICallsPerMonth:           Unlimited,
		WebhookEndpoints:           50,
		WebhookDeliveriesPerMinute: 1200,
		Contacts:                   Unlimited,
		Users:                      Unlimited,
		PhoneNumbers:               Unlimited,
		AIAgents:                   Unlimited,
		StorageBytes:               Unlimited,
		CampaignMessagesPerSecond:  Unlimited,
		Features: map[Feature]bool{
			CustomSMTP: true, BYOAIKeys: true, CustomDomain: true,
			PublicAPI: true, Webhooks: true, AIAgents: true,
		},
	}
}

// Provider returns a tenant's plan. Implementations must be cheap (they are
// asked on every API request) and safe for concurrent use.
type Provider interface {
	Limits(ctx context.Context, tenantID uuid.UUID) (Plan, error)
}

// ProviderFunc adapts a function to Provider (tests, fakes).
type ProviderFunc func(ctx context.Context, tenantID uuid.UUID) (Plan, error)

// Limits implements Provider.
func (f ProviderFunc) Limits(ctx context.Context, id uuid.UUID) (Plan, error) { return f(ctx, id) }

// Static returns a provider that gives every tenant p.
func Static(p Plan) Provider {
	return ProviderFunc(func(context.Context, uuid.UUID) (Plan, error) { return p, nil })
}

// SettingKey is the platform_settings key whose JSON object overrides
// fields of Default for every tenant (missing fields keep the default).
const SettingKey = "default_plan"

// ttl is how long the platform setting is used before it is read again.
const ttl = 30 * time.Second

// SettingsProvider gives every tenant Default overlaid with the
// platform setting "default_plan". It is the provider until WP8.
type SettingsProvider struct {
	pool *pgxpool.Pool
	now  func() time.Time

	mu      sync.Mutex
	plan    Plan
	expires time.Time
}

// NewSettingsProvider reads platform_settings through pool (readable
// without a tenant). A nil pool always answers Default.
func NewSettingsProvider(pool *pgxpool.Pool) *SettingsProvider {
	return &SettingsProvider{pool: pool, now: time.Now, plan: Default()}
}

// Limits implements Provider. A failed read keeps the last good value for
// another period (Default before the first success), so a database hiccup
// never tightens anyone's limits.
func (s *SettingsProvider) Limits(ctx context.Context, _ uuid.UUID) (Plan, error) {
	now := s.now()
	s.mu.Lock()
	if s.pool == nil || now.Before(s.expires) {
		p := s.plan
		s.mu.Unlock()
		return p, nil
	}
	s.mu.Unlock()
	p, err := s.read(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.plan = p
	}
	s.expires = now.Add(ttl)
	return s.plan, nil
}

func (s *SettingsProvider) read(ctx context.Context) (Plan, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	var raw []byte
	err := s.pool.QueryRow(tenant.System(ctx), `SELECT value FROM platform_settings WHERE key = $1`, SettingKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Default(), nil
	}
	if err != nil {
		return Plan{}, err
	}
	return Overlay(Default(), raw)
}

// Overlay returns base with the fields present in raw (a JSON object)
// replaced; feature keys in raw replace only those features.
func Overlay(base Plan, raw []byte) (Plan, error) {
	features := make(map[Feature]bool, len(base.Features))
	for k, v := range base.Features {
		features[k] = v
	}
	p := base
	p.Features = nil
	if err := json.Unmarshal(raw, &p); err != nil {
		return Plan{}, fmt.Errorf("entitlements: %s: %w", SettingKey, err)
	}
	for k, v := range p.Features {
		features[k] = v
	}
	p.Features = features
	if p.SessionRequestsPerMinute <= 0 {
		p.SessionRequestsPerMinute = base.SessionRequestsPerMinute
	}
	if p.APIKeyRequestsPerMinute <= 0 {
		p.APIKeyRequestsPerMinute = base.APIKeyRequestsPerMinute
	}
	return p, nil
}

// For returns the plan of the context's tenant. Without a tenant (platform
// work) it answers Default: platform operations are not metered.
func For(ctx context.Context, p Provider) (Plan, error) {
	id, ok := tenant.FromCtx(ctx)
	if !ok || p == nil {
		return Default(), nil
	}
	return p.Limits(ctx, id)
}

// Error codes.
const (
	CodeFeatureNotInPlan = "FEATURE_NOT_IN_PLAN"
	CodePlanLimitReached = "PLAN_LIMIT_REACHED"
	CodeQuotaExceeded    = "QUOTA_EXCEEDED"
)

// ErrFeature is the 403 for a feature the plan does not include; the
// feature key is in the error details (field "feature").
func ErrFeature(f Feature) *apperr.Error {
	return &apperr.Error{Status: http.StatusForbidden, Code: CodeFeatureNotInPlan,
		Message: "Your plan does not include this feature",
		Details: []apperr.Detail{{Field: "feature", Code: string(f), Message: "Not included in the workspace's plan"}}}
}

// ErrLimit is the 409 for a count the plan caps; the resource key and the
// limit are in the details (field "limit", code = resource).
func ErrLimit(r Resource, limit int64) *apperr.Error {
	return &apperr.Error{Status: http.StatusConflict, Code: CodePlanLimitReached,
		Message: fmt.Sprintf("Your plan allows at most %d %s", limit, humanResource(r)),
		Details: []apperr.Detail{{Field: "limit", Code: string(r), Message: strconv.FormatInt(limit, 10)}}}
}

func humanResource(r Resource) string {
	switch r {
	case WebhookEndpoints:
		return "webhook endpoints"
	case PhoneNumbers:
		return "phone numbers"
	case AIAgentCount:
		return "AI agents"
	}
	return string(r)
}

// Require returns ErrFeature when the plan lacks f.
func (p Plan) Require(f Feature) error {
	if !p.Has(f) {
		return ErrFeature(f)
	}
	return nil
}

// Allow returns ErrLimit when current already reaches the plan's limit of r
// (creating one more would exceed it).
func (p Plan) Allow(r Resource, current int64) error {
	if l := p.Count(r); l != Unlimited && current >= l {
		return ErrLimit(r, l)
	}
	return nil
}
