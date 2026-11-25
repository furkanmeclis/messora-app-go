-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P2: the campaign delivery engine. See docs/ARCHITECTURE.md §6.3
-- and §7.
--
-- Campaigns learn a priority (their weight in the per-number send
-- scheduler), an optional rate cap, why and when they were paused, and when
-- a paused campaign resumes by itself (paused_limit: when the portfolio's
-- messaging limit frees up; any pause: a resume scheduled by an operator).
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT campaigns_status_check;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_status_check CHECK (status IN ('draft', 'scheduled', 'preparing', 'running',
    'paused', 'paused_limit', 'completed', 'cancelled', 'failed')) NOT VALID;
ALTER TABLE campaigns ADD COLUMN priority text NOT NULL DEFAULT 'normal';
ALTER TABLE campaigns ADD CONSTRAINT campaigns_priority_check CHECK (priority IN ('high', 'normal', 'low')) NOT VALID;
-- Messages per second this campaign may use at most (NULL = its fair share).
ALTER TABLE campaigns ADD COLUMN max_mps double precision;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_max_mps_check CHECK (max_mps IS NULL OR max_mps > 0) NOT VALID;
ALTER TABLE campaigns ADD COLUMN paused_reason text;
ALTER TABLE campaigns ADD COLUMN paused_at timestamptz;
ALTER TABLE campaigns ADD COLUMN paused_by uuid;
ALTER TABLE campaigns ADD COLUMN resume_at timestamptz;
-- Measured dispatch rate (messages per second) for the live ETA: the
-- dispatcher counts messages it queued since rate_window_start and folds
-- them into send_rate every few seconds.
ALTER TABLE campaigns ADD COLUMN send_rate double precision;
ALTER TABLE campaigns ADD COLUMN rate_window_start timestamptz;
-- squawk-ignore adding-field-with-default
ALTER TABLE campaigns ADD COLUMN rate_window_queued bigint NOT NULL DEFAULT 0;
-- The auto-resume job looks for paused campaigns with a resume time.
-- campaigns is small; the partial index is empty today.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX campaigns_resume_idx ON campaigns (resume_at) WHERE resume_at IS NOT NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX campaigns_number_running_idx ON campaigns (phone_number_id) WHERE status = 'running';

-- Every pause, with what the campaign looked like at that moment.
CREATE TABLE campaign_pauses (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id uuid NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    status      text NOT NULL,
    reason      text NOT NULL,
    detail      jsonb NOT NULL DEFAULT '{}'::jsonb,
    paused_at   timestamptz NOT NULL DEFAULT now(),
    paused_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    resume_at   timestamptz,
    -- Counts by recipient state, message status and unreached class.
    snapshot    jsonb NOT NULL DEFAULT '{}'::jsonb,
    resumed_at  timestamptz,
    resumed_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    -- What the operator changed when resuming (priority, rate cap, name …).
    changes     jsonb
);
CREATE INDEX campaign_pauses_campaign_idx ON campaign_pauses (campaign_id, id DESC);

-- Recipients whose message Meta withheld for ecosystem health (131049)
-- after accepting it (status webhook) wait in "retry" until next_attempt_at
-- (>= 24 h); the dispatcher then sends them a new message. resends counts
-- those new messages (three attempts in all).
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT campaign_recipients_state_check;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_state_check
    CHECK (state IN ('pending', 'queued', 'skipped', 'cancelled', 'retry')) NOT VALID;
ALTER TABLE campaign_recipients ADD COLUMN next_attempt_at timestamptz;
-- squawk-ignore adding-field-with-default,prefer-bigint-over-int
ALTER TABLE campaign_recipients ADD COLUMN resends integer NOT NULL DEFAULT 0;
-- No recipient is in "retry" yet; the partial index is empty.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX campaign_recipients_retry_idx ON campaign_recipients (campaign_id, next_attempt_at) WHERE state = 'retry';

-- A resend names the message it replaces; one resend per message.
ALTER TABLE messages ADD COLUMN retry_of uuid;
-- No message is a resend yet; the partial index is empty.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX messages_retry_of_idx ON messages (retry_of) WHERE retry_of IS NOT NULL;

-- Portfolio messaging limit: the unique customers each WhatsApp account
-- messaged outside their customer service window, with the latest such
-- message. "Messaged in the last 24 h" is last_at > now() - 24 h.
CREATE TABLE portfolio_reach (
    account_id uuid NOT NULL REFERENCES whatsapp_accounts (id) ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    last_at    timestamptz NOT NULL,
    PRIMARY KEY (account_id, contact_id)
);
CREATE INDEX portfolio_reach_window_idx ON portfolio_reach (account_id, last_at);

-- Why a recipient of a campaign was not reached, from its recipient row and
-- its (latest) message; NULL = reached, or possibly reached (sent,
-- delivered, read, unknown, sending). Used by the "previous campaign's
-- unreached customers" audience and its summary.
--   never_sent        never queued (pending, cancelled) or cancelled before sending
--   blocked_by_consent / user_stopped_marketing / undeliverable / permanent / temporary /
--   rejected_by_meta / ecosystem_limit   as messages.failure_class (and skip reasons)
-- +goose StatementBegin
CREATE FUNCTION campaign_recipient_class(state text, skip_reason text, msg_status text, failure_class text)
RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE
        WHEN state = 'skipped' THEN CASE
            WHEN skip_reason = 'contact_deleted' THEN NULL
            WHEN skip_reason = 'meta_marketing_stopped' THEN 'user_stopped_marketing'
            WHEN skip_reason = 'undeliverable' THEN 'undeliverable'
            WHEN skip_reason IN ('opted_out', 'opt_out_requested', 'consent_unknown', 'replied_after_opt_out') THEN 'blocked_by_consent'
            ELSE 'permanent' END
        WHEN state IN ('pending', 'cancelled') THEN 'never_sent'
        WHEN state = 'retry' THEN 'ecosystem_limit'
        WHEN msg_status IS NULL THEN 'never_sent'
        WHEN msg_status IN ('failed', 'skipped') THEN CASE
            WHEN failure_class = 'cancelled' THEN 'never_sent'
            ELSE coalesce(failure_class, 'rejected_by_meta') END
        WHEN msg_status IN ('cancelled', 'held', 'pending') THEN 'never_sent'
        WHEN msg_status = 'deferred' THEN 'ecosystem_limit'
        ELSE NULL
    END
$$;
-- +goose StatementEnd

-- +goose Down
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_recipient_class(text, text, text, text);
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS portfolio_reach;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS messages_retry_of_idx;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS retry_of;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS campaign_recipients_retry_idx;
UPDATE campaign_recipients SET state = 'queued' WHERE state = 'retry';
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS resends;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS next_attempt_at;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT campaign_recipients_state_check;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_state_check
    CHECK (state IN ('pending', 'queued', 'skipped', 'cancelled')) NOT VALID;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_pauses;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS campaigns_number_running_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS campaigns_resume_idx;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS rate_window_queued;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS rate_window_start;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS send_rate;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS resume_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS paused_by;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS paused_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS paused_reason;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT campaigns_max_mps_check;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS max_mps;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT campaigns_priority_check;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS priority;
UPDATE campaigns SET status = 'paused' WHERE status = 'paused_limit';
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT campaigns_status_check;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_status_check CHECK (status IN ('draft', 'scheduled', 'preparing', 'running',
    'paused', 'completed', 'cancelled', 'failed')) NOT VALID;
