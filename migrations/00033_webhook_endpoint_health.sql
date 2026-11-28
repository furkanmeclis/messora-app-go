-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP4: outgoing webhook endpoints track their health, switch
-- themselves off after a long run of failures (the tenant's admins get an
-- e-mail) and keep the previous signing secret for an overlap period after
-- a rotation (docs/WEBHOOKS.md, docs/PHASE-17.md §5.11). The table is a
-- tenant table (RLS since 00028); the new columns follow it.
ALTER TABLE webhook_endpoints
    -- Delivery attempts that failed in a row; a success resets it.
    ADD COLUMN failure_streak             bigint NOT NULL DEFAULT 0,
    -- When the current run of failures began (NULL while healthy).
    ADD COLUMN failing_since              timestamptz,
    ADD COLUMN last_success_at            timestamptz,
    ADD COLUMN last_failure_at            timestamptz,
    -- When the endpoint was switched off automatically (NULL otherwise).
    ADD COLUMN disabled_at                timestamptz,
    -- The signing secret before the last rotation, sealed like secret_enc
    -- (same associated data); deliveries are signed with both until
    -- previous_secret_expires_at.
    ADD COLUMN previous_secret_enc        bytea,
    ADD COLUMN previous_secret_expires_at timestamptz;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS previous_secret_expires_at;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS previous_secret_enc;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS disabled_at;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS last_failure_at;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS last_success_at;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS failing_since;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS failure_streak;
