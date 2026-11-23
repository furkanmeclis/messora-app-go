-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- A bulk template send to an audience. See docs/ARCHITECTURE.md §7.
CREATE TABLE campaigns (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    name               text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    status             text NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft', 'scheduled', 'preparing', 'running', 'paused', 'completed', 'cancelled', 'failed')),
    phone_number_id    uuid NOT NULL REFERENCES phone_numbers (id) ON DELETE RESTRICT,
    template_name      text NOT NULL,
    template_language  text NOT NULL,
    -- Variable mapping per component, e.g. {"body": ["{{name}}", "{{var1}}"]}.
    template_params    jsonb NOT NULL DEFAULT '{}'::jsonb,
    -- {"segment_id": …} | {"group_id": …} | {"tag_id": …} | {"all": true}
    audience           jsonb NOT NULL,
    scheduled_at       timestamptz,
    started_at         timestamptz,
    finished_at        timestamptz,
    -- Materialisation resumes after this contact id.
    materialize_cursor uuid,
    -- Dispatcher generation: bumped on every (re)start so a stale
    -- dispatcher chain stops by itself.
    -- squawk-ignore prefer-bigint-over-int
    dispatch_gen       integer NOT NULL DEFAULT 0,
    -- squawk-ignore prefer-bigint-over-int
    window_size        integer NOT NULL DEFAULT 0 CHECK (window_size BETWEEN 0 AND 10000), -- 0 = auto
    total_recipients   bigint NOT NULL DEFAULT 0,
    last_error         text,
    created_by         uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX campaigns_status_idx ON campaigns (status) WHERE status IN ('scheduled', 'preparing', 'running');

CREATE TABLE campaign_recipients (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id  uuid NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    contact_id   uuid REFERENCES contacts (id) ON DELETE SET NULL,
    phone_e164   text NOT NULL,
    state        text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'queued', 'skipped', 'cancelled')),
    skip_reason  text,
    message_id   uuid,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, phone_e164)
);
CREATE INDEX campaign_recipients_pending_idx ON campaign_recipients (campaign_id, id) WHERE state = 'pending';
CREATE INDEX campaign_recipients_state_idx ON campaign_recipients (campaign_id, state);

CREATE TABLE campaign_events (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    kind        text NOT NULL,
    detail      jsonb NOT NULL DEFAULT '{}'::jsonb,
    actor_id    uuid,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX campaign_events_campaign_idx ON campaign_events (campaign_id, id DESC);

-- Messages learn which campaign they belong to, and two campaign states:
-- held (campaign paused when the send was due) and cancelled.
-- campaigns was created above and is empty; the new column is all NULL.
-- squawk-ignore adding-foreign-key-constraint
ALTER TABLE messages ADD COLUMN campaign_id uuid REFERENCES campaigns (id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN meta_status text;
-- Replaced by a wider check right below.
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_status_check;
-- The messages table is young; validating the new check at once is cheap.
-- squawk-ignore constraint-missing-not-valid
ALTER TABLE messages ADD CONSTRAINT messages_status_check CHECK (status IN ('pending', 'sending', 'unknown', 'sent', 'delivered', 'read', 'failed', 'received', 'held', 'cancelled'));
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX messages_campaign_status_idx ON messages (campaign_id, status) WHERE campaign_id IS NOT NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX messages_stuck_idx ON messages (updated_at) WHERE status IN ('pending', 'sending');

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS messages_stuck_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS messages_campaign_status_idx;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_status_check;
-- squawk-ignore constraint-missing-not-valid
ALTER TABLE messages ADD CONSTRAINT messages_status_check CHECK (status IN ('pending', 'sending', 'unknown', 'sent', 'delivered', 'read', 'failed', 'received'));
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS meta_status;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS campaign_id;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_recipients;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaigns;
