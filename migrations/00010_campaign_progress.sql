-- +goose Up
SET LOCAL lock_timeout = '5s';

-- Live campaign counts are published at most every two seconds per campaign
-- while delivery statuses arrive; updates that fall into a quiet period
-- mark the campaign dirty and the reaper publishes them.
ALTER TABLE campaigns ADD COLUMN progress_published_at timestamptz;
-- squawk-ignore adding-field-with-default
ALTER TABLE campaigns ADD COLUMN progress_dirty boolean NOT NULL DEFAULT false;

-- +goose Down
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS progress_dirty;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS progress_published_at;
