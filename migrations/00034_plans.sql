-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP8: plans and entitlements, the platform's AI keys and the AI
-- credit ledger, plan change requests (docs/ENTITLEMENTS.md,
-- docs/PHASE-17.md §1.5 and §5.12).

-- ── Plans ──────────────────────────────────────────────────────────────────
-- The plan catalogue (platform table). limits uses the JSON names of
-- entitlements.Plan (0 = no limit); features maps every feature key to
-- whether the plan includes it (a missing key = not included). Prices are
-- for display only (TRY, VAT excluded); billing is WP9.
CREATE TABLE plans (
    key             text PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_-]{1,30}$'),
    name_tr         text NOT NULL CHECK (length(name_tr) BETWEEN 1 AND 60),
    name_en         text NOT NULL CHECK (length(name_en) BETWEEN 1 AND 60),
    description_tr  text NOT NULL DEFAULT '' CHECK (length(description_tr) <= 500),
    description_en  text NOT NULL DEFAULT '' CHECK (length(description_en) <= 500),
    sort_order      bigint NOT NULL DEFAULT 0,
    limits          jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(limits) = 'object'),
    features        jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(features) = 'object'),
    currency        text NOT NULL DEFAULT 'TRY' CHECK (currency ~ '^[A-Z]{3}$'),
    -- Monthly and yearly list prices, VAT excluded; NULL = not shown.
    price_monthly   numeric(12, 2) CHECK (price_monthly >= 0),
    price_yearly    numeric(12, 2) CHECK (price_yearly >= 0),
    -- Priced per offer ("from" price_monthly), e.g. the enterprise plan.
    custom_pricing  boolean NOT NULL DEFAULT false,
    -- active: may be assigned to a company; public: listed in the
    -- workspaces' plan comparison.
    active          boolean NOT NULL DEFAULT true,
    public          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- The four launch plans (docs/design/rakip-fiyat-analizi.md, owner's WP8
-- decisions): only the enterprise plan has its own SMTP server and its own
-- AI keys; the starter plan has webhooks but no public API.
INSERT INTO plans (key, name_tr, name_en, description_tr, description_en, sort_order, limits, features,
                   price_monthly, price_yearly, custom_pricing) VALUES
('starter', 'Başlangıç', 'Starter',
 'Tek numara ve küçük ekipler için gelen kutusu, kampanya ve otomasyon.',
 'Inbox, campaigns and automations for one number and a small team.', 10,
 '{"session_requests_per_minute": 600, "api_key_requests_per_minute": 300, "api_calls_per_month": 0,
   "webhook_endpoints": 3, "webhook_deliveries_per_minute": 300, "contacts": 0, "users": 2, "phone_numbers": 1,
   "ai_agents": 1, "automation_flows": 3, "ai_credits_per_month": 300, "storage_bytes": 5368709120,
   "campaign_messages_per_second": 20}',
 '{"custom_smtp": false, "byo_ai_keys": false, "custom_domain": false, "public_api": false, "webhooks": true, "ai_agents": true}',
 1290, 12900, false),
('growth', 'Büyüme', 'Growth',
 'Büyüyen ekipler için daha fazla kullanıcı, AI ajanı, otomasyon ve public API.',
 'More users, AI agents and automations, and the public API, for growing teams.', 20,
 '{"session_requests_per_minute": 600, "api_key_requests_per_minute": 300, "api_calls_per_month": 100000,
   "webhook_endpoints": 10, "webhook_deliveries_per_minute": 600, "contacts": 0, "users": 5, "phone_numbers": 1,
   "ai_agents": 3, "automation_flows": 20, "ai_credits_per_month": 2000, "storage_bytes": 21474836480,
   "campaign_messages_per_second": 50}',
 '{"custom_smtp": false, "byo_ai_keys": false, "custom_domain": false, "public_api": true, "webhooks": true, "ai_agents": true}',
 2990, 29900, false),
('pro', 'Pro', 'Pro',
 'Birden çok numara, sınırsız otomasyon, özel alan adı ve yüksek API kotası.',
 'Several numbers, unlimited automations, a custom domain and a high API quota.', 30,
 '{"session_requests_per_minute": 600, "api_key_requests_per_minute": 300, "api_calls_per_month": 1000000,
   "webhook_endpoints": 25, "webhook_deliveries_per_minute": 1200, "contacts": 0, "users": 12, "phone_numbers": 3,
   "ai_agents": 10, "automation_flows": 0, "ai_credits_per_month": 8000, "storage_bytes": 107374182400,
   "campaign_messages_per_second": 80}',
 '{"custom_smtp": false, "byo_ai_keys": false, "custom_domain": true, "public_api": true, "webhooks": true, "ai_agents": true}',
 6490, 64900, false),
('enterprise', 'Kurumsal', 'Enterprise',
 'Özel limitler, kendi SMTP sunucunuz ve kendi AI anahtarlarınız; teklifle.',
 'Custom limits, your own SMTP server and your own AI keys; priced per offer.', 40,
 '{"session_requests_per_minute": 600, "api_key_requests_per_minute": 300, "api_calls_per_month": 0,
   "webhook_endpoints": 50, "webhook_deliveries_per_minute": 1200, "contacts": 0, "users": 0, "phone_numbers": 0,
   "ai_agents": 0, "automation_flows": 0, "ai_credits_per_month": 20000, "storage_bytes": 0,
   "campaign_messages_per_second": 0}',
 '{"custom_smtp": true, "byo_ai_keys": true, "custom_domain": true, "public_api": true, "webhooks": true, "ai_agents": true}',
 14990, NULL, true);

-- A company's plan and, mainly for the enterprise plan, its own limits
-- (a JSON object like plans.limits plus an optional "features" object;
-- the keys present replace the plan's). A NULL plan gets the platform's
-- default plan (platform_settings.default_plan over the built-in one).
ALTER TABLE tenants
    ADD COLUMN plan_key        text,
    ADD COLUMN plan_overrides  jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN plan_changed_at timestamptz;
-- squawk-ignore adding-foreign-key-constraint
ALTER TABLE tenants ADD CONSTRAINT tenants_plan_key_fkey FOREIGN KEY (plan_key) REFERENCES plans (key) ON UPDATE CASCADE NOT VALID;
-- The tenant registry is small: validated at once.
-- squawk-ignore constraint-missing-not-valid
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_plan_key_fkey;
ALTER TABLE tenants ADD CONSTRAINT tenants_plan_overrides_object CHECK (jsonb_typeof(plan_overrides) = 'object') NOT VALID;
-- squawk-ignore constraint-missing-not-valid
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_plan_overrides_object;

-- Existing companies: a trial gets the growth plan (the trial plan of new
-- companies), everyone else the enterprise plan, so no existing company
-- loses a feature it had before plans (the platform admin reassigns).
UPDATE tenants SET plan_key = CASE WHEN subscription_status = 'trial' THEN 'growth' ELSE 'enterprise' END,
                   plan_changed_at = now()
WHERE plan_key IS NULL;

-- ── The platform's AI keys ─────────────────────────────────────────────────
-- One connection per provider kind with Messora's own key (sealed with
-- associated data "platform_ai_provider.key:<id>"). Companies use them by
-- default; their calls are paid with AI credits. The application role
-- reads them (to call the provider for a company); only the platform role
-- writes them.
CREATE TABLE platform_ai_providers (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    kind             text NOT NULL UNIQUE CHECK (kind IN ('openai', 'anthropic', 'gemini', 'deepseek', 'fake')),
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    api_key_enc      bytea NOT NULL,
    key_last4        text NOT NULL DEFAULT '',
    base_url         text CHECK (base_url ~ '^https://'),
    status           text NOT NULL DEFAULT 'unknown' CHECK (status IN ('unknown', 'ok', 'invalid', 'no_balance', 'error')),
    status_detail    text NOT NULL DEFAULT '',
    checked_at       timestamptz,
    models_synced_at timestamptz,
    enabled          boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- The platform keys' model catalogue (like ai_models).
CREATE TABLE platform_ai_models (
    provider_id    uuid NOT NULL REFERENCES platform_ai_providers (id) ON DELETE CASCADE,
    model_id       text NOT NULL CHECK (length(model_id) BETWEEN 1 AND 200),
    display_name   text NOT NULL DEFAULT '',
    context_window bigint,
    max_output     bigint,
    capabilities   jsonb NOT NULL DEFAULT '{}'::jsonb,
    lifecycle      text,
    synced_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, model_id)
);

-- ── AI credits ─────────────────────────────────────────────────────────────
-- A company's AI credits as lots, in millicredits (1 credit = 1000): the
-- plan's monthly allowance (one lot per calendar month, UTC; created on
-- first use and expiring at the month's end), manual grants of the
-- platform admin and, in WP9, purchased top-ups. AI calls with the
-- platform's keys consume the lots that expire first; usage beyond every
-- lot is booked on the month's plan lot (used may exceed granted). Lots
-- are locked FOR UPDATE in a fixed order while consuming, so concurrent
-- calls never lose an update.
CREATE TABLE ai_credit_lots (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id    uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    source       text NOT NULL CHECK (source IN ('plan', 'grant', 'purchase')),
    -- The month of a plan lot (its first day); NULL for other lots.
    period_start date,
    granted      bigint NOT NULL CHECK (granted >= 0),
    used         bigint NOT NULL DEFAULT 0 CHECK (used >= 0),
    starts_at    timestamptz NOT NULL DEFAULT now(),
    -- NULL = never expires.
    expires_at   timestamptz,
    note         text NOT NULL DEFAULT '' CHECK (length(note) <= 500),
    -- WP9: the purchase (invoice/payment) a purchased lot belongs to.
    reference    text CHECK (length(reference) <= 200),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ai_credit_lots_plan_period CHECK ((source = 'plan') = (period_start IS NOT NULL))
);
-- One plan lot per company and month.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX ai_credit_lots_plan_month ON ai_credit_lots (tenant_id, period_start) WHERE source = 'plan';
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX ai_credit_lots_tenant_idx ON ai_credit_lots (tenant_id, expires_at);
ALTER TABLE ai_credit_lots ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_credit_lots FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_credit_lots USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- The checks below hold for every new row at once (NOT VALID); they are
-- not validated here because ai_usage, ai_profiles and
-- conversation_handoffs may be large and the existing rows cannot break
-- them (new columns are NULL, the reason list only grows).
-- Which key paid for an AI call: the company's own (tenant, no credits) or
-- the platform's (platform, credits = millicredits charged). Rows before
-- WP8 have neither.
ALTER TABLE ai_usage
    ADD COLUMN key_source           text,
    ADD COLUMN platform_provider_id uuid,
    ADD COLUMN credits              bigint;
ALTER TABLE ai_usage ADD CONSTRAINT ai_usage_key_source_check CHECK (key_source IN ('tenant', 'platform')) NOT VALID;
ALTER TABLE ai_usage ADD CONSTRAINT ai_usage_credits_check CHECK (credits >= 0) NOT VALID;

-- An AI profile may name one of the platform's keys instead of one of the
-- company's own (provider_id); at most one of the two is set.
ALTER TABLE ai_profiles ADD COLUMN platform_provider_id uuid;
ALTER TABLE ai_profiles ADD CONSTRAINT ai_profiles_one_provider CHECK (provider_id IS NULL OR platform_provider_id IS NULL) NOT VALID;

-- A conversation the AI could not answer because the company's AI credits
-- are used up goes to a person with its own reason.
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_reason_check;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_reason_check CHECK (reason IN (
    'customer_request', 'no_answer', 'low_confidence', 'critical_intent', 'unauthorized_action', 'window_closed',
    'ai_error', 'manual', 'ai_budget', 'ai_credits', 'profile_complete', 'automation')) NOT VALID;

-- ── Plan change requests ───────────────────────────────────────────────────
-- "Change plan" in a workspace until self-service billing (WP9): a request
-- the platform admins see in their inbox (and get by e-mail). One open
-- request per company.
CREATE TABLE plan_change_requests (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id       uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    current_plan    text,
    requested_plan  text NOT NULL REFERENCES plans (key) ON UPDATE CASCADE,
    billing_cycle   text NOT NULL DEFAULT 'monthly' CHECK (billing_cycle IN ('monthly', 'yearly')),
    message         text NOT NULL DEFAULT '' CHECK (length(message) <= 2000),
    requested_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done', 'declined')),
    resolution_note text NOT NULL DEFAULT '' CHECK (length(resolution_note) <= 1000),
    resolved_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX plan_change_requests_one_open ON plan_change_requests (tenant_id) WHERE status = 'open';
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX plan_change_requests_status_idx ON plan_change_requests (status, created_at DESC);
ALTER TABLE plan_change_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_change_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON plan_change_requests USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- ── Privileges ─────────────────────────────────────────────────────────────
-- The catalogue and the platform keys: read by the application, written by
-- the platform panel.
REVOKE INSERT, UPDATE, DELETE ON plans, platform_ai_providers, platform_ai_models FROM messora_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON plans, platform_ai_providers, platform_ai_models TO messora_platform;
-- The platform panel grants credits and reads a company's ledger inside
-- that company (app.tenant_id set: the usual policy applies), and works
-- the requests of every company.
GRANT SELECT, INSERT, UPDATE ON ai_credit_lots TO messora_platform;
GRANT SELECT, UPDATE ON plan_change_requests TO messora_platform;
CREATE POLICY platform_access ON plan_change_requests TO messora_platform USING (true) WITH CHECK (true);
-- A company's requests are made by the company; the application may not
-- resolve them.
REVOKE DELETE ON plan_change_requests FROM messora_app;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS plan_change_requests;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_reason_check;
UPDATE conversation_handoffs SET reason = 'ai_budget' WHERE reason = 'ai_credits';
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_reason_check CHECK (reason IN (
    'customer_request', 'no_answer', 'low_confidence', 'critical_intent', 'unauthorized_action', 'window_closed',
    'ai_error', 'manual', 'ai_budget', 'profile_complete', 'automation')) NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_profiles DROP CONSTRAINT IF EXISTS ai_profiles_one_provider;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_profiles DROP COLUMN IF EXISTS platform_provider_id;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_usage DROP CONSTRAINT IF EXISTS ai_usage_credits_check;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_usage DROP CONSTRAINT IF EXISTS ai_usage_key_source_check;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_usage DROP COLUMN IF EXISTS credits;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_usage DROP COLUMN IF EXISTS platform_provider_id;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_usage DROP COLUMN IF EXISTS key_source;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS ai_credit_lots;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS platform_ai_models;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS platform_ai_providers;
-- squawk-ignore ban-drop-constraint
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_plan_overrides_object;
-- squawk-ignore ban-drop-constraint
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_plan_key_fkey;
-- squawk-ignore ban-drop-column
ALTER TABLE tenants DROP COLUMN IF EXISTS plan_changed_at;
-- squawk-ignore ban-drop-column
ALTER TABLE tenants DROP COLUMN IF EXISTS plan_overrides;
-- squawk-ignore ban-drop-column
ALTER TABLE tenants DROP COLUMN IF EXISTS plan_key;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS plans;
