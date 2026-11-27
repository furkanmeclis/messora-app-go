-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP0: Messora runs only as a hosted SaaS (docs/PHASE-17.md §1).
-- The settings of the removed features are dropped; nothing reads them any
-- more.

-- The licence record (licence key, instance id, check-in state).
DELETE FROM settings WHERE key = 'license';

-- +goose Down
-- The removed features have no code left to read their settings, so there
-- is nothing to restore.
SELECT 1;
