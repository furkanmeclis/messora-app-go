# Plans and entitlements (quota hooks)

Every workspace (tenant) belongs to a plan. The plan sets **limits** (request
budgets, monthly quotas, how many of something a workspace may create) and
**feature flags** (docs/PHASE-17.md §1.5). Phase 17 WP4 introduced the
interface and enforced the API-related parts; WP8 added the plan catalogue,
the per-company overrides, the AI credits and wired every remaining hook
(§5.12). Billing (invoices, payments, trial end) is WP9.

Code: `internal/platform/entitlements` (plans, checks), `internal/modules/plans`
(plan pages, platform panel), `internal/modules/credits` (AI credits).

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
    Key string                       // the plan's key; "default" without a plan

    SessionRequestsPerMinute   int   // GCRA budget of one panel session
    APIKeyRequestsPerMinute    int   // GCRA budget of one API key
    APICallsPerMonth           int64 // all API keys together, per calendar month (UTC)

    WebhookEndpoints           int   // endpoints that may be registered
    WebhookDeliveriesPerMinute int   // delivery attempts of all endpoints together

    Contacts                   int64
    Users                      int
    PhoneNumbers               int
    AIAgents                   int
    AutomationFlows            int   // active flows at once (WP8)
    AICreditsPerMonth          int64 // included AI credits per month (WP8)
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
| `custom_smtp` | sending through the company's own SMTP server | WP8 (`settings.Mail`, `PUT /settings/smtp`, test send) |
| `byo_ai_keys` | the company's own AI provider keys | WP8 (provider create/update/check/sync; `ai.Service.resolve`) |
| `custom_domain` | adding a custom domain | WP8 (platform panel, `POST /platform/tenants/{id}/domains`) |
| `ai_agents` | AI replies, AI steps, AI profiles | WP8 (`ai.Service.resolve`, `CreateProfile`) |

Helpers on `Plan`: `Has(f)`, `Require(f) error` (403 below), `Count(r)`,
`Limited(r)`, `Allow(r, current) error` (409 below when `current` already
reaches the limit, i.e. creating one more would exceed it) and
`AllowAdd(r, current, n)` (adding `n` at once: numbers of a WhatsApp
account, bytes of an upload). `entitlements.Check(ctx, q, provider, r, count)`
and `CheckAdd` do the whole check inside the creating transaction: without a
cap nothing is counted; with one they take the tenant's advisory lock for the
resource (`messora.plan:<tenant>:<resource>`), count and allow, so two
concurrent creations cannot both take the last slot.

The 409 names the resource, the limit and the current count:
`error.details` = `[{"field": "limit", "code": "<resource>", "message": "<limit>"},
{"field": "current", "code": "<resource>", "message": "<current>"}]`.
Resources: `users`, `phone_numbers`, `ai_agents`, `automation_flows`,
`contacts`, `webhook_endpoints`, `storage_bytes`.

## Providers

**`entitlements.DBProvider`** (the application's provider since WP8) reads a
tenant's plan from the catalogue: `tenants.plan_key` → `plans.limits` (a JSON
object with the field names above) and `plans.features` (exactly the features
listed as `true`; a feature the catalogue does not name is **off**, so a new
feature is never switched on by accident), then the tenant's own overrides
`tenants.plan_overrides` (same field names plus an optional `features`
object; the keys present replace the plan's — the enterprise plan's custom
limits). Plans are cached per tenant for 30 seconds; the platform panel
invalidates this replica's cache at once when it assigns a plan
(`Invalidate(id)`) or edits a catalogue plan (`InvalidateAll()`), other
replicas follow within the 30 seconds. A failed read keeps the last known
plan; without one the call fails (500 in the guard). `Describe` also
returns what the panel shows (key, names, overrides, subscription status).

A tenant **without a plan** (`plan_key` NULL: only tenants inserted by hand,
e.g. E2E fixtures) gets the fallback: **`entitlements.NewSettingsProvider(pool)`**,
`entitlements.Default()` overlaid with the platform setting
`platform_settings.default_plan` (a JSON object with the field names above;
missing fields keep the default, `features` keys replace only those
features). The value is read at most every 30 seconds; a failed read keeps
the last good value. The default is generous and switches every feature on:

| Limit | Default |
|---|---|
| session / API key requests per minute | 600 / 300 |
| API calls per month | unlimited |
| webhook endpoints | 50 (the former hard limit) |
| webhook deliveries per minute | 1200 |
| contacts, users, phone numbers, AI agents, flows, storage, campaign msg/s, AI credits | unlimited |
| features | all on |

Example (staging): `UPDATE platform_settings SET value = '{"api_calls_per_month": 100000, "features": {"custom_smtp": false}}' WHERE key = 'default_plan'`.

## The plan catalogue (WP8)

Migration 00034 seeds four plans (docs/design/rakip-fiyat-analizi.md; prices
TRY, VAT excluded, for display only — billing is WP9). `0` = no limit.

| | Başlangıç (`starter`) | Büyüme (`growth`) | Pro (`pro`) | Kurumsal (`enterprise`) |
|---|---|---|---|---|
| Monthly / yearly list price | 1.290 / 12.900 | 2.990 / 29.900 | 6.490 / 64.900 | from 14.990, per offer |
| Users (seats) | 2 | 5 | 12 | 0 (custom via overrides) |
| WhatsApp numbers | 1 | 1 | 3 | 0 |
| AI agents (profiles) | 1 | 3 | 10 | 0 |
| Active automation flows | 3 | 20 | 0 | 0 |
| AI credits / month | 300 | 2.000 | 8.000 | 20.000 (custom pool) |
| API calls / month | — (no `public_api`) | 100.000 | 1.000.000 | 0 |
| Webhook endpoints / deliveries per min | 3 / 300 | 10 / 600 | 25 / 1200 | 50 / 1200 |
| Storage | 5 GiB | 20 GiB | 100 GiB | 0 |
| Campaign messages / second | 20 | 50 | 80 | 0 (fairness setting) |
| Contacts | 0 (fair use) | 0 | 0 | 0 |
| `webhooks`, `ai_agents` | ✓ | ✓ | ✓ | ✓ |
| `public_api` | – | ✓ | ✓ | ✓ |
| `custom_domain` | – | – | ✓ | ✓ |
| `custom_smtp`, `byo_ai_keys` | – | – | – | ✓ |

Session and API key budgets are 600 / 300 per minute on every plan. Who gets
what:

- **New companies** (sign-up, platform panel) start their 14-day trial on
  the **growth** plan (`entitlements.TrialPlan`); a company opened without a
  trial gets **starter**. The trial's end is WP9.
- **Companies from before plans** (migration 00034): on trial → growth,
  every other → **enterprise**, so none loses a feature it had; the platform
  admin reassigns.
- The platform admin edits plans (Plans page; `PATCH /platform/plans/{key}`,
  `plan.update` in the platform audit log), assigns a plan with the company's
  own limits (`PUT /platform/tenants/{id}/plan`; `tenant.plan_change` in the
  platform log and `plan.change` in the company's) and grants AI credits.
  `active = false` plans cannot be assigned; `public = false` plans are not
  in the workspaces' comparison (`GET /plans`).
- Workspaces see their plan, limits, usage and credits on Settings → Plan
  (`GET /plan`) and ask for another plan (`POST /plan/change-requests`, one
  open request, the platform admins get a `plan_change_request` e-mail; the
  request inbox can assign the plan in the same step). `GET /me` carries
  `tenant.plan` (key, names, included features, `ai_credits_exhausted`) so
  the panel hides what the plan lacks; the backend checks stay
  authoritative.

## Downgrades: nothing is deleted

A lower plan (or lower overrides) never deletes or switches off anything:

- **Counts** (users, numbers, AI agents, active flows, webhook endpoints,
  contacts, storage): what exists keeps working; only creating more is
  refused (409). The plan page marks the bar `over` and says so. Existing
  members keep their access, extra numbers stay connected **and keep
  sending**, flows that are on stay on (and stay editable; switching another
  one on is refused), AI profiles keep answering, existing files stay
  downloadable. Open invitations hold a seat; accepting one past the limit
  is refused (409), so a downgrade cannot grow the team further.
- **Features:** `custom_smtp` off → mail goes through the platform sender
  (the stored server is kept); `byo_ai_keys` off → calls use Messora's keys
  and credits (stored own keys are kept, listed and deletable, never used);
  `custom_domain` off → existing domains keep working, new ones are refused;
  `public_api` / `webhooks` off → as WP4 (API keys get 403, no new
  deliveries; lists, reads and deletes stay).

Rationale: a plan change is a business decision that must never break a
running inbox or lose data; the platform admin or the company resolves the
overage (WP9 may add a grace period and a read-only mode for unpaid plans).

## What is enforced where (WP8, complete)

| Hook | Where | Answer |
|---|---|---|
| users | `identity.invite` (seats = active members + open invitations; the platform's owner invitation is not capped) and `AcceptInvitation` (active members) | 409 `users` |
| contacts | `contacts.Create`; imports refuse the rows that would create contacts past the limit (failed rows "the plan's contact limit is reached"; existing ones still update) | 409 `contacts` |
| phone numbers | `channels.Connect` (all numbers of the WABA, numbers of disabled accounts do not count) and Tech Provider `StartOnboarding` (one number) | 409 `phone_numbers` |
| AI agents | `ai.CreateProfile` (also needs `ai_agents`) | 409 `ai_agents` / 403 |
| active flows | creating a flow switched on, switching a flow on (`PATCH active: true`); presets install switched off | 409 `automation_flows` |
| storage | contact import upload, logo upload; the meter is `StorageBytesUsed` (stored media, import files, finished exports). Inbound customer media is always stored (never lose a customer's message) and counts | 409 `storage_bytes` |
| campaign msg/s | `campaignCap` in `internal/app/server.go`: the lower of `platform_settings.fairness.tenant_campaign_mps` and the plan's `campaign_messages_per_second` | paced |
| API calls / month | API guard; every API key call is now counted (the plan page shows it), only a plan with a quota refuses | 429 `QUOTA_EXCEEDED` |
| `custom_smtp` | `PUT /settings/smtp`, `POST /settings/smtp/test` (`x-messora-feature`), `settings.Mail` | 403 |
| `byo_ai_keys` | create/update/check/sync of AI providers (`x-messora-feature`); `ai.Service.resolve` | 403 |
| `custom_domain` | platform `AddDomain` | 403 |
| `ai_agents` | `ai.Service.resolve` (every AI call), `CreateProfile` | 403 / `not_in_plan` |
| AI credits | `ai.Service.Generate` with Messora's keys (docs/AI.md "AI credits") | `AI_CREDITS_EXHAUSTED` (409), reply loop handoff `ai_credits`, AI steps `ai_credits_exhausted` |

## API budgets, quota and API features (WP4)

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

## Tests

`internal/platform/entitlements` (unit: overlay, catalogue composition,
cache and invalidation, stale-on-failure, `AllowAdd`), `internal/platform/apiguard`
(`TestPlanFeatures`), `internal/modules/credits` (credit math),
`internal/app/wp4_integration_test.go` (`TestPlanEnforcement` with a fake
provider per tenant) and `internal/app/wp8_integration_test.go`
(`TestPlansAndEntitlements`: catalogue, trial → growth, every count and
feature, downgrade, numbers, campaign cap, plan edit audit, change requests,
public rate card, isolation; `TestAICreditsAndKeys`; `TestEmailSendersByPlan`).
