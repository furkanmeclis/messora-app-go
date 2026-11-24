-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Access token health (Phase 16b). Filled from Meta's debug_token when the
-- token is connected or rotated, and re-checked by the scheduler.
--   token_type        SYSTEM_USER (permanent), USER, PAGE, APP …
--   token_expires_at  NULL = never expires
--   token_notified_at last "needs attention" notice, so it is sent once a day
ALTER TABLE whatsapp_accounts
    ADD COLUMN token_type text,
    ADD COLUMN token_expires_at timestamptz,
    ADD COLUMN token_data_access_expires_at timestamptz,
    ADD COLUMN token_scopes text[],
    ADD COLUMN token_checked_at timestamptz,
    ADD COLUMN token_warning text,
    ADD COLUMN token_notified_at timestamptz;

-- +goose Down
-- squawk-ignore ban-drop-column
ALTER TABLE whatsapp_accounts
    DROP COLUMN IF EXISTS token_notified_at,
    DROP COLUMN IF EXISTS token_warning,
    DROP COLUMN IF EXISTS token_checked_at,
    DROP COLUMN IF EXISTS token_scopes,
    DROP COLUMN IF EXISTS token_data_access_expires_at,
    DROP COLUMN IF EXISTS token_expires_at,
    DROP COLUMN IF EXISTS token_type;
