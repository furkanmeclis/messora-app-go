-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P0: Meta cost capture and account signals. See docs/WHATSAPP.md
-- ("Cost and account signals").

-- What Meta billed for an outbound message. The status webhook's pricing
-- object arrives on either "sent" or "delivered" (never both) and carries no
-- amount; cost_amount is computed from the imported rate card (meta_rates)
-- in the account's currency. NULL cost = no matching rate was imported.
--   pricing_type      regular (billable) | free_customer_service | free_entry_point | …
--   pricing_category  marketing | utility | authentication | authentication-international | service | …
--   recipient_country ISO 3166-1 alpha-2 derived from the recipient's E.164 number
--   cost_source       rate_card
ALTER TABLE messages ADD COLUMN pricing_type text;
ALTER TABLE messages ADD COLUMN pricing_category text;
ALTER TABLE messages ADD COLUMN pricing_model text;
ALTER TABLE messages ADD COLUMN billable boolean;
ALTER TABLE messages ADD COLUMN recipient_country text;
ALTER TABLE messages ADD COLUMN cost_amount numeric(12, 6);
ALTER TABLE messages ADD COLUMN cost_currency text;
ALTER TABLE messages ADD COLUMN cost_source text;

-- The WABA's billing currency (GET /{waba-id}?fields=currency).
ALTER TABLE whatsapp_accounts ADD COLUMN currency text;

-- Throughput read from Meta (GET /{phone-number-id}?fields=throughput):
-- messages per second the number may send; the send throttle uses it as its
-- ceiling (80 when unknown). quality_rating and messaging_limit already
-- exist and are refreshed by the same hourly signal sync.
-- squawk-ignore prefer-bigint-over-int
ALTER TABLE phone_numbers ADD COLUMN throughput_mps integer;
ALTER TABLE phone_numbers ADD COLUMN signals_checked_at timestamptz;

-- Meta's per-message rate card, imported from CSV (Meta publishes no API).
-- A market is a country or a group of countries ("Rest of Western Europe");
-- meta_rate_markets maps every country to its market. Volume tiers (utility
-- and authentication) are inclusive bounds on the message's ordinal within
-- the calendar month; both NULL = untiered.
CREATE TABLE meta_rate_markets (
    country     text PRIMARY KEY CHECK (country ~ '^[A-Z]{2}$'),
    market      text NOT NULL CHECK (length(market) BETWEEN 1 AND 120),
    imported_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meta_rates (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    currency       text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    market         text NOT NULL CHECK (length(market) BETWEEN 1 AND 120),
    category       text NOT NULL CHECK (category IN ('marketing', 'utility', 'authentication', 'authentication-international', 'service')),
    price          numeric(12, 6) NOT NULL CHECK (price >= 0),
    tier_lower     bigint CHECK (tier_lower >= 1),
    tier_upper     bigint CHECK (tier_upper >= tier_lower),
    effective_from date NOT NULL,
    effective_to   date CHECK (effective_to > effective_from),
    source         text NOT NULL DEFAULT '',
    imported_at    timestamptz NOT NULL DEFAULT now(),
    imported_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    UNIQUE NULLS NOT DISTINCT (currency, market, category, tier_lower, effective_from)
);
CREATE INDEX meta_rates_lookup_idx ON meta_rates (market, category, currency, effective_from);

-- Marketing opt-out signals from WhatsApp ("Stop/Resume" in the chat menu),
-- append-only. Consent is derived from these in a later package.
CREATE TABLE meta_user_preferences (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    phone_number_id uuid REFERENCES phone_numbers (id) ON DELETE SET NULL,
    wa_id           text,
    user_id         text,
    phone_e164      text,
    category        text NOT NULL,
    value           text NOT NULL,
    detail          text NOT NULL DEFAULT '',
    occurred_at     timestamptz NOT NULL,
    received_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meta_user_preferences_phone_idx ON meta_user_preferences (phone_e164, occurred_at DESC);
-- A webhook processed twice (retries) records the change once.
CREATE UNIQUE INDEX meta_user_preferences_once ON meta_user_preferences (wa_id, user_id, category, value, occurred_at) NULLS NOT DISTINCT;

-- Account-level alerts from Meta webhooks (account_update,
-- phone_number_quality_update, business_capability_update,
-- message_template_quality_update), kept for the operations view.
CREATE TABLE whatsapp_account_events (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    account_id      uuid NOT NULL REFERENCES whatsapp_accounts (id) ON DELETE CASCADE,
    phone_number_id uuid REFERENCES phone_numbers (id) ON DELETE SET NULL,
    kind            text NOT NULL,
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at     timestamptz NOT NULL,
    received_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX whatsapp_account_events_account_idx ON whatsapp_account_events (account_id, occurred_at DESC);
CREATE UNIQUE INDEX whatsapp_account_events_once ON whatsapp_account_events (account_id, kind, occurred_at, md5(payload::text));

-- Meta's own (approximate) cost per day from pricing_analytics, used to
-- reconcile the per-message estimates. Missing dimensions are ''. cost is
-- NULL when Meta does not report it (accounts on a partner's credit line).
CREATE TABLE meta_cost_daily (
    account_id       uuid NOT NULL REFERENCES whatsapp_accounts (id) ON DELETE CASCADE,
    day              date NOT NULL,
    phone_number     text NOT NULL DEFAULT '',
    country          text NOT NULL DEFAULT '',
    pricing_category text NOT NULL DEFAULT '',
    pricing_type     text NOT NULL DEFAULT '',
    volume           bigint NOT NULL DEFAULT 0,
    cost             numeric(14, 6),
    currency         text,
    fetched_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, day, phone_number, country, pricing_category, pricing_type)
);

-- +goose Down
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_cost_daily;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS whatsapp_account_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_user_preferences;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_rates;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_rate_markets;
-- squawk-ignore ban-drop-column
ALTER TABLE phone_numbers DROP COLUMN IF EXISTS signals_checked_at;
-- squawk-ignore ban-drop-column
ALTER TABLE phone_numbers DROP COLUMN IF EXISTS throughput_mps;
-- squawk-ignore ban-drop-column
ALTER TABLE whatsapp_accounts DROP COLUMN IF EXISTS currency;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS cost_source;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS cost_currency;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS cost_amount;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS recipient_country;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS billable;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS pricing_model;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS pricing_category;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS pricing_type;
