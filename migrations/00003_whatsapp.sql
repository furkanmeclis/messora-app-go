-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- A WhatsApp Business Account connected with a system-user access token.
-- Secrets are sealed with APP_ENCRYPTION_KEY (AES-GCM, associated data is
-- the row id) and never leave the server.
CREATE TABLE whatsapp_accounts (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    waba_id          text NOT NULL UNIQUE CHECK (waba_id ~ '^[0-9]{1,32}$'),
    app_id           text CHECK (app_id ~ '^[0-9]{1,32}$'),
    access_token_enc bytea NOT NULL,
    app_secret_enc   bytea,
    status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'error', 'disabled')),
    last_error       text,
    last_synced_at   timestamptz,
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE phone_numbers (
    id                       uuid PRIMARY KEY DEFAULT uuidv7(),
    account_id               uuid NOT NULL REFERENCES whatsapp_accounts (id) ON DELETE CASCADE,
    phone_number_id          text NOT NULL UNIQUE CHECK (phone_number_id ~ '^[0-9]{1,32}$'),
    display_phone_number     text NOT NULL DEFAULT '',
    verified_name            text NOT NULL DEFAULT '',
    quality_rating           text NOT NULL DEFAULT 'UNKNOWN',
    messaging_limit          text NOT NULL DEFAULT '',
    throughput_level         text NOT NULL DEFAULT '',
    code_verification_status text NOT NULL DEFAULT '',
    meta_status              text NOT NULL DEFAULT '',
    is_default               boolean NOT NULL DEFAULT false,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX phone_numbers_account_idx ON phone_numbers (account_id);
-- At most one default sender.
CREATE UNIQUE INDEX phone_numbers_one_default ON phone_numbers (is_default) WHERE is_default;

-- Minimal contact record so messages have a counterpart; Phase 4 extends it.
CREATE TABLE contacts (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    phone_e164      text NOT NULL UNIQUE CHECK (phone_e164 ~ '^\+[1-9][0-9]{6,14}$'),
    wa_id           text,
    name            text NOT NULL DEFAULT '' CHECK (length(name) <= 200),
    profile_name    text NOT NULL DEFAULT '' CHECK (length(profile_name) <= 200),
    last_inbound_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Every inbound and outbound WhatsApp message.
--
-- status_rank makes webhook processing monotonic: an update only moves a
-- message forward (pending 0 < sending 1 < unknown 2 < sent 3 < delivered 4
-- < read 5), and "failed" (6) is accepted only before delivery. Timestamps
-- are recorded even when their status arrives late.
CREATE TABLE messages (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    phone_number_id uuid NOT NULL REFERENCES phone_numbers (id) ON DELETE RESTRICT,
    contact_id      uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    direction       text NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    wamid           text UNIQUE,
    type            text NOT NULL CHECK (length(type) BETWEEN 1 AND 40),
    body            text NOT NULL DEFAULT '',
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    status          text NOT NULL CHECK (status IN ('pending', 'sending', 'unknown', 'sent', 'delivered', 'read', 'failed', 'received')),
    -- squawk-ignore prefer-bigint-over-smallint
    status_rank     smallint NOT NULL DEFAULT 0,
    reply_to_wamid  text,
    error_code      text,
    error_title     text,
    -- squawk-ignore prefer-bigint-over-int
    attempts        integer NOT NULL DEFAULT 0,
    source          text NOT NULL DEFAULT 'api' CHECK (source IN ('api', 'panel', 'webhook', 'campaign', 'automation')),
    created_by      uuid,
    sent_at         timestamptz,
    delivered_at    timestamptz,
    read_at         timestamptz,
    failed_at       timestamptz,
    occurred_at     timestamptz NOT NULL DEFAULT now(), -- Meta's timestamp for inbound
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX messages_contact_idx ON messages (contact_id, id DESC);
CREATE INDEX messages_status_idx ON messages (status) WHERE status IN ('pending', 'sending', 'unknown');

-- Media referenced by messages. Inbound files are downloaded by the worker
-- before Meta's 7-day expiry and kept under MEDIA_DIR.
CREATE TABLE media_files (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    message_id    uuid REFERENCES messages (id) ON DELETE CASCADE,
    meta_media_id text,
    mime_type     text NOT NULL DEFAULT '',
    filename      text NOT NULL DEFAULT '',
    size_bytes    bigint,
    sha256        text NOT NULL DEFAULT '',
    storage_key   text,
    status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'stored', 'failed')),
    last_error    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX media_files_message_idx ON media_files (message_id);

-- Raw webhook deliveries, stored before processing. Meta retries for up to
-- 7 days, so identical bodies are deduplicated by hash.
CREATE TABLE webhook_events (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    payload_sha256 bytea NOT NULL UNIQUE,
    body           jsonb NOT NULL,
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processed', 'failed')),
    -- squawk-ignore prefer-bigint-over-int
    attempts       integer NOT NULL DEFAULT 0,
    last_error     text,
    received_at    timestamptz NOT NULL DEFAULT now(),
    processed_at   timestamptz
);
CREATE INDEX webhook_events_received_idx ON webhook_events (received_at);

CREATE TABLE message_templates (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    account_id       uuid NOT NULL REFERENCES whatsapp_accounts (id) ON DELETE CASCADE,
    meta_template_id text NOT NULL,
    name             text NOT NULL CHECK (name ~ '^[a-z0-9_]+$' AND length(name) <= 512),
    language         text NOT NULL CHECK (length(language) BETWEEN 2 AND 15),
    category         text NOT NULL,
    status           text NOT NULL,
    parameter_format text NOT NULL DEFAULT 'POSITIONAL',
    components       jsonb NOT NULL DEFAULT '[]'::jsonb,
    rejected_reason  text NOT NULL DEFAULT '',
    quality_score    text NOT NULL DEFAULT '',
    synced_at        timestamptz NOT NULL DEFAULT now(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, name, language),
    UNIQUE (account_id, meta_template_id)
);

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS message_templates;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS webhook_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS media_files;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS messages;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contacts;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS phone_numbers;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS whatsapp_accounts;
