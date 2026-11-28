# Plans and entitlements (quota hooks)

Every workspace (tenant) belongs to a plan. The plan sets **limits** (request
budgets, monthly quotas, how many of something a workspace may create) and
**feature flags** (docs/PHASE-17.md §1.5). Phase 17 WP4 introduced the
interface and enforces the API-related parts; WP8 adds the plan table and
fills in the values. Nothing outside the provider has to change for that.

Code: `internal/platform/entitlements`.

## The interface

```go
type Provider interface {
    Limits(ctx context.Context, tenantID uuid.UUID) (Plan, error)
}
```

`Limits` is asked on **every API request** of a workspace principal (session
or API key) and whenever the webhook module creates an endpoint, fans out an
event or sends a delivery. Implementations must therefore be cheap
(in-process cache) and safe for concurrent use. An error fails the request
(500 in the API guard); prefer returning the last known plan over an error.

`entitlements.For(ctx, provider)` returns the plan of the context's tenant
and the built-in default for platform work (no tenant: the admin panel and
platform jobs are not metered).

```go
type Plan struct {
    Key string                       // "default" until WP8

    SessionRequestsPerMinute   int   // GCRA budget of one panel session
    APIKeyRequestsPerMinute    int   // GCRA budget of one API key
    APICallsPerMonth           int64 // all API keys together, per calendar month (UTC)

    WebhookEndpoints           int   // endpoints that may be registered
    WebhookDeliveriesPerMinute int   // delivery attempts of all endpoints together

    Contacts                   int64
    Users                      int
    PhoneNumbers               int
    AIAgents                   int
    StorageBytes               int64
    CampaignMessagesPerSecond  int

    Features map[Feature]bool        // missing key = not included
}
```

`0` (`entitlements.Unlimited`) means "no limit" for every count and quota. A
resource a plan does not include at all is switched off by its **feature
flag**, never by a zero count. The two request budgets cannot be unlimited:
a value `<= 0` falls back to the default (600 and 300 per minute).

Features (the keys are part of the API, see the error details below):

| Feature | Meaning | Enforced |
|---|---|---|
| `public_api` | the workspace's API keys may call the API; creating keys | WP4 (guard) |
| `webhooks` | registering, changing, testing, rotating and redelivering endpoints; new deliveries | WP4 (guard + fan-out) |
| `custom_smtp` | sending through the company's own SMTP server | WP8 (`TODO(WP8)` in `settings.Mail`) |
| `byo_ai_keys` | the company's own AI provider keys | WP8 |
| `custom_domain` | adding a custom domain | WP8 (platform panel) |
| `ai_agents` | AI replies, AI steps | WP8 |

Helpers on `Plan`: `Has(f)`, `Require(f) error` (403 below), `Count(r)` and
`Allow(r, current) error` (409 below when `current` already reaches the
limit, i.e. creating one more would exceed it).

## Default provider

`entitlements.NewSettingsProvider(pool)` gives every tenant
`entitlements.Default()` overlaid with the platform setting
`platform_settings.default_plan` (a JSON object with the field names above;
missing fields keep the default, `features` keys replace only those
features). The value is read at most every 30 seconds; a failed read keeps
the last good value. The default is generous and switches every feature on,
so no existing workspace loses anything before WP8:

| Limit | Default |
|---|---|
| session / API key requests per minute | 600 / 300 |
| API calls per month | unlimited |
| webhook endpoints | 50 (the former hard limit) |
| webhook deliveries per minute | 1200 |
| contacts, users, phone numbers, AI agents, storage, campaign msg/s | unlimited |
| features | all on |

Example (staging): `UPDATE platform_settings SET value = '{"api_calls_per_month": 100000, "features": {"custom_smtp": false}}' WHERE key = 'default_plan'`.

## What is enforced now (WP4)

**Request budgets.** The API guard takes the session and API key budgets
from the plan (`messora:t:{<tenant>}:rl:api:<principal>`, GCRA, burst =
rate). Every response carries `RateLimit-Limit`, `RateLimit-Remaining`,
`RateLimit-Reset` and `RateLimit-Policy`; over the budget the answer is
`429 RATE_LIMITED` with `Retry-After`. Platform sessions keep the fixed
default.

**Monthly API quota.** With `api_calls_per_month > 0`, every API key call
increments `messora:t:{<tenant>}:quota:api:<yyyymm>` (expires a day after
the month). Responses carry `X-Messora-Quota-Limit` and
`X-Messora-Quota-Remaining`. Past the quota: `429 QUOTA_EXCEEDED`,
`Retry-After` until the first of the next month (UTC), the `RateLimit-*`
headers describe the monthly window, and `error.details` is
`[{"field": "limit", "code": "api_calls_per_month", "message": "<limit>"}]`.
Panel sessions are not counted. Redis outages fail open, like the rate
limiter.

**Feature flags.** API keys need `public_api` for every operation. An
operation can name the feature it needs in the OpenAPI spec with
`x-messora-feature: <feature>` (checked when the guard starts: unknown
features are a spec error); today `createApiKey` (`public_api`) and the
webhook operations that create or change something (`webhooks`). Without
the feature: `403 FEATURE_NOT_IN_PLAN`, `error.details` =
`[{"field": "feature", "code": "<feature>"}]`. Lists, reads and deletes stay
available, so a downgraded workspace can still see and clean up.

**Counts.** Creating a webhook endpoint beyond `webhook_endpoints` answers
`409 PLAN_LIMIT_REACHED`, `error.details` =
`[{"field": "limit", "code": "webhook_endpoints", "message": "<limit>"}]`.

**Webhook deliveries.** `webhook_deliveries_per_minute` is a GCRA budget per
tenant (`messora:t:{<tenant>}:rl:webhook:deliveries`) shared by every
worker. Over it, a delivery task is deferred (`queue.RetryAfterError`: no
attempt and no retry budget is spent). A plan without `webhooks` creates no
deliveries.

## For WP8

1. Add the plan table (and a tenant → plan reference) and a provider that
   reads it with a short in-process cache keyed by tenant; invalidate on
   plan changes from the platform panel (like `tenant.Resolver.Invalidate`).
   Wire it in `internal/app/server.go` (`services.plans`).
2. Enforce the remaining counts where the resource is created, with
   `plan.Allow(resource, current)` inside the creating transaction (count
   under the same transaction to avoid races): users (invitations and
   accepted invitations), contacts (single create **and** imports: refuse
   the rows past the limit), phone numbers (channel connect, WP5/WP6 code),
   AI agents (AI profiles).
3. Enforce the remaining features: `custom_smtp` (`settings.Mail`, the
   `TODO(WP8)` hook), `byo_ai_keys` (AI provider create/update),
   `custom_domain` (platform panel domain creation for `kind = 'custom'`),
   `ai_agents` (AI reply loop and AI automation steps; the panel hides them).
4. `storage_bytes` and `campaign_messages_per_second` need usage meters
   (storage) and a per-tenant cap next to the fairness setting
   (`messaging.Throttle.WithTenantCampaignCap`: take the lower of the two).
5. Expose the plan to the panel (`GET /me` or a `GET /plan`), so it hides
   what the plan lacks; the backend checks stay authoritative.

Tests: `internal/platform/entitlements` (unit), `internal/platform/apiguard`
(`TestPlanFeatures`), `internal/app/wp4_integration_test.go`
(`TestPlanEnforcement` with a fake provider per tenant).
