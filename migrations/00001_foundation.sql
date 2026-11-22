-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Typed key/value application settings. Keys are defined in code.
CREATE TABLE settings (
    key        text PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{0,99}$'),
    value      jsonb NOT NULL,
    updated_by uuid,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Transactional outbox: written in the same transaction as the business
-- change, published to the queue / realtime server by the worker relay.
CREATE TABLE outbox_events (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic        text NOT NULL CHECK (topic <> ''),
    payload      jsonb NOT NULL DEFAULT '{}'::jsonb,
    available_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    attempts     integer NOT NULL DEFAULT 0,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX outbox_events_unpublished_idx ON outbox_events (available_at, id) WHERE published_at IS NULL;
CREATE INDEX outbox_events_published_at_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION outbox_notify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('outbox', '');
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- One notification per statement, not per row: the relay drains in batches.
CREATE TRIGGER outbox_events_notify
    AFTER INSERT ON outbox_events
    FOR EACH STATEMENT EXECUTE FUNCTION outbox_notify();

-- +goose Down
DROP TRIGGER IF EXISTS outbox_events_notify ON outbox_events;
DROP FUNCTION IF EXISTS outbox_notify();
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS settings;
-- Extensions are left in place: other objects may depend on them.
