-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P5: AI providers, model catalogue, prices, usage and AI
-- profiles. See docs/AI.md. Everything AI reads its configuration through
-- internal/modules/ai, so per-tenant scoping (Phase 17) is added there.

-- Connections to model providers. The API key is sealed with the
-- installation's encryption key (associated data "ai_provider.key:<id>") and
-- never leaves the server; key_last4 is what the panel shows.
--   kind    openai | anthropic | gemini | deepseek | fake (tests/development only)
--   status  unknown | ok | invalid | no_balance | error (last key check)
CREATE TABLE ai_providers (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    kind          text NOT NULL CHECK (kind IN ('openai', 'anthropic', 'gemini', 'deepseek', 'fake')),
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    api_key_enc   bytea NOT NULL,
    key_last4     text NOT NULL DEFAULT '',
    -- Only for DeepSeek and OpenAI-compatible endpoints; https only.
    base_url      text CHECK (base_url ~ '^https://'),
    status        text NOT NULL DEFAULT 'unknown' CHECK (status IN ('unknown', 'ok', 'invalid', 'no_balance', 'error')),
    status_detail text NOT NULL DEFAULT '',
    checked_at    timestamptz,
    models_synced_at timestamptz,
    enabled       boolean NOT NULL DEFAULT true,
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name)
);

-- Models as each provider's list endpoint reports them. A NULL limit or
-- capability means the provider does not report it ("unknown"); it is never
-- guessed.
CREATE TABLE ai_models (
    provider_id    uuid NOT NULL REFERENCES ai_providers (id) ON DELETE CASCADE,
    model_id       text NOT NULL CHECK (length(model_id) BETWEEN 1 AND 200),
    display_name   text NOT NULL DEFAULT '',
    context_window bigint,
    max_output     bigint,
    -- {"tools": bool|null, "structured_output": bool|null, "vision": bool|null, "thinking": bool|null}
    capabilities   jsonb NOT NULL DEFAULT '{}'::jsonb,
    lifecycle      text,
    synced_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, model_id)
);

-- Prices per 1M tokens. No provider publishes prices through an API, so
-- rows are seeded from the providers' pricing pages (unconfirmed), imported
-- from LiteLLM's MIT price list or entered by an admin, and confirmed by an
-- admin. model_pattern is an exact model id or a prefix ending in '*'.
-- rules holds input-size thresholds and time windows (see
-- internal/modules/ai/pricing.go). A row applies from effective_from
-- (inclusive) until effective_to (exclusive). Costs already recorded in
-- ai_usage never change when a price is edited.
CREATE TABLE ai_model_prices (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    provider_kind  text NOT NULL CHECK (provider_kind IN ('openai', 'anthropic', 'gemini', 'deepseek', 'fake')),
    model_pattern  text NOT NULL CHECK (length(model_pattern) BETWEEN 1 AND 200),
    currency       text NOT NULL DEFAULT 'USD' CHECK (currency ~ '^[A-Z]{3}$'),
    input          numeric(12, 6) NOT NULL CHECK (input >= 0),
    cached_input   numeric(12, 6) CHECK (cached_input >= 0),
    cache_write_5m numeric(12, 6) CHECK (cache_write_5m >= 0),
    cache_write_1h numeric(12, 6) CHECK (cache_write_1h >= 0),
    output         numeric(12, 6) NOT NULL CHECK (output >= 0),
    rules          jsonb NOT NULL DEFAULT '{}'::jsonb,
    effective_from date NOT NULL,
    effective_to   date CHECK (effective_to > effective_from),
    source         text NOT NULL DEFAULT 'manual' CHECK (source IN ('seed', 'litellm', 'manual')),
    source_url     text NOT NULL DEFAULT '',
    notes          text NOT NULL DEFAULT '',
    confirmed_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    confirmed_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_kind, model_pattern, effective_from)
);

-- One row per AI request made through the service, successful or not.
-- Token columns follow internal/modules/ai.Usage: input excludes cached
-- reads and cache writes; output includes reasoning. cost is computed with
-- the price valid at request time and never recomputed. campaign_id is
-- filled by campaign attribution (P7); a request belongs to at most one
-- campaign.
--   purpose reply | suggest | classify | extract | summarize | automation | test
--   status  ok | error
CREATE TABLE ai_usage (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    provider_id        uuid REFERENCES ai_providers (id) ON DELETE SET NULL,
    provider_kind      text NOT NULL,
    model              text NOT NULL,
    purpose            text NOT NULL CHECK (purpose IN ('reply', 'suggest', 'classify', 'extract', 'summarize', 'automation', 'test')),
    profile_id         uuid,
    conversation_id    uuid REFERENCES conversations (id) ON DELETE SET NULL,
    contact_id         uuid REFERENCES contacts (id) ON DELETE SET NULL,
    campaign_id        uuid REFERENCES campaigns (id) ON DELETE SET NULL,
    automation_run_id  uuid REFERENCES automation_runs (id) ON DELETE SET NULL,
    input_tokens       bigint NOT NULL DEFAULT 0,
    cached_tokens      bigint NOT NULL DEFAULT 0,
    cache_write_tokens bigint NOT NULL DEFAULT 0,
    cache_write_1h_tokens bigint NOT NULL DEFAULT 0,
    output_tokens      bigint NOT NULL DEFAULT 0,
    reasoning_tokens   bigint NOT NULL DEFAULT 0,
    cost               numeric(12, 6),
    currency           text,
    price_id           uuid REFERENCES ai_model_prices (id) ON DELETE SET NULL,
    latency_ms         bigint NOT NULL DEFAULT 0,
    status             text NOT NULL CHECK (status IN ('ok', 'error')),
    error_kind         text,
    created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ai_usage_created_idx ON ai_usage (created_at DESC);
CREATE INDEX ai_usage_conversation_idx ON ai_usage (conversation_id) WHERE conversation_id IS NOT NULL;
CREATE INDEX ai_usage_campaign_idx ON ai_usage (campaign_id) WHERE campaign_id IS NOT NULL;

-- How the AI behaves on one WhatsApp number (at most one profile per
-- number). handoff.on_request is always true: Meta's Business Messaging
-- Policy requires a clear path to a human.
--   disclosure_mode always_first_message | on_question | off
--   status          active | paused
CREATE TABLE ai_profiles (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    name            text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    phone_number_id uuid UNIQUE REFERENCES phone_numbers (id) ON DELETE SET NULL,
    system_prompt   text NOT NULL DEFAULT '' CHECK (length(system_prompt) <= 200000),
    provider_id     uuid REFERENCES ai_providers (id) ON DELETE SET NULL,
    model           text,
    auto_reply      boolean NOT NULL DEFAULT false,
    handoff         jsonb NOT NULL DEFAULT '{}'::jsonb,
    data_collection jsonb NOT NULL DEFAULT '{}'::jsonb,
    allowed_actions jsonb NOT NULL DEFAULT '[]'::jsonb,
    disclosure_mode text NOT NULL DEFAULT 'always_first_message' CHECK (disclosure_mode IN ('always_first_message', 'on_question', 'off')),
    status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused')),
    created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

INSERT INTO permissions (key, module, description) VALUES
    ('ai.read', 'ai', 'View AI providers, models, prices and profiles'),
    ('ai.manage', 'ai', 'Connect AI providers, edit prices, AI settings and profiles');

-- Owner and admin manage; managers can look.
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, p.key FROM roles r JOIN permissions p ON p.key IN ('ai.read', 'ai.manage')
WHERE r.key IN ('owner', 'admin')
ON CONFLICT DO NOTHING;
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, 'ai.read' FROM roles r WHERE r.key = 'manager'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_key IN ('ai.read', 'ai.manage');
DELETE FROM permissions WHERE key IN ('ai.read', 'ai.manage');
DELETE FROM settings WHERE key IN ('ai', 'ai_prices_seeded');
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS ai_profiles;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS ai_usage;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS ai_model_prices;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS ai_models;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS ai_providers;
