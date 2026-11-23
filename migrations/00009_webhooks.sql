-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Outgoing webhooks: receivers that integrators register, the events Messora
-- emits and one delivery per (event, endpoint). See docs/WEBHOOKS.md.
CREATE TABLE webhook_endpoints (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    url             text NOT NULL CHECK (length(url) BETWEEN 9 AND 2048),
    description     text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    -- Subscribed event types, e.g. {message.received,conversation.closed}.
    event_types     text[] NOT NULL DEFAULT '{}',
    active          boolean NOT NULL DEFAULT true,
    -- Signing secret, AES-GCM sealed (associated data "webhook_secret:<id>").
    secret_enc      bytea NOT NULL,
    secret_last4    text NOT NULL,
    -- Why the endpoint was switched off automatically (e.g. 410 Gone).
    disabled_reason text,
    created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
-- Emitters ask "does any active endpoint want this type?" on every event.
CREATE INDEX webhook_endpoints_events_idx ON webhook_endpoints USING gin (event_types) WHERE active;

-- An emitted event, written in the same transaction as the change it
-- describes. payload is the exact envelope sent to receivers.
CREATE TABLE outgoing_webhook_events (
    id         uuid PRIMARY KEY,
    type       text NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX outgoing_webhook_events_created_idx ON outgoing_webhook_events (created_at);

CREATE TABLE webhook_deliveries (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    event_id           uuid NOT NULL REFERENCES outgoing_webhook_events (id) ON DELETE CASCADE,
    endpoint_id        uuid NOT NULL REFERENCES webhook_endpoints (id) ON DELETE CASCADE,
    status             text NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'delivering', 'retrying', 'succeeded', 'failed')),
    -- squawk-ignore prefer-bigint-over-int
    attempts           integer NOT NULL DEFAULT 0,
    next_attempt_at    timestamptz NOT NULL DEFAULT now(),
    -- A worker holds a delivery while its request is in flight.
    locked_until       timestamptz,
    last_attempt_at    timestamptz,
    -- squawk-ignore prefer-bigint-over-int
    last_status_code   integer,
    -- squawk-ignore prefer-bigint-over-int
    last_duration_ms   integer,
    last_error         text,
    -- First KiB of the receiver's answer, for debugging.
    last_response_body text,
    delivered_at       timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    -- One delivery per event and endpoint, however often fan-out runs.
    UNIQUE (event_id, endpoint_id)
);
CREATE INDEX webhook_deliveries_endpoint_idx ON webhook_deliveries (endpoint_id, id DESC);

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS webhook_deliveries;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS outgoing_webhook_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS webhook_endpoints;
