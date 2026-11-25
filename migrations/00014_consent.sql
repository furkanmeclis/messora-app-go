-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P1: consent per category, Meta-sourced reachability flags and
-- send blocking. See docs/CONSENT.md.
--
-- Each contact has a consent status for marketing and for service
-- (utility, authentication and replies):
--   opted_in | unknown | opt_out_requested | opted_out | resubscribed
-- with when, from where (source) and why (reason) it last changed.
-- opted_out_at stays the legacy "opted out of everything" flag: it is set
-- exactly when both categories are opted_out.
ALTER TABLE contacts ADD COLUMN marketing_consent text NOT NULL DEFAULT 'unknown';
ALTER TABLE contacts ADD COLUMN marketing_consent_at timestamptz;
ALTER TABLE contacts ADD COLUMN marketing_consent_source text;
ALTER TABLE contacts ADD COLUMN marketing_consent_reason text;
ALTER TABLE contacts ADD COLUMN service_consent text NOT NULL DEFAULT 'unknown';
ALTER TABLE contacts ADD COLUMN service_consent_at timestamptz;
ALTER TABLE contacts ADD COLUMN service_consent_source text;
ALTER TABLE contacts ADD COLUMN service_consent_reason text;
-- The customer chose "Stop" for this business's marketing in WhatsApp
-- (user_preferences webhook or error 131050); cleared by "Resume".
ALTER TABLE contacts ADD COLUMN meta_marketing_stopped_at timestamptz;
-- Meta could not deliver to this number (131026); cleared by the next
-- inbound message from it.
ALTER TABLE contacts ADD COLUMN undeliverable_at timestamptz;
ALTER TABLE contacts ADD COLUMN undeliverable_code text;
-- Existing rows hold the defaults, which satisfy the checks; NOT VALID
-- avoids a full-table scan under the lock.
ALTER TABLE contacts ADD CONSTRAINT contacts_marketing_consent_check
    CHECK (marketing_consent IN ('opted_in', 'unknown', 'opt_out_requested', 'opted_out', 'resubscribed')) NOT VALID;
ALTER TABLE contacts ADD CONSTRAINT contacts_service_consent_check
    CHECK (service_consent IN ('opted_in', 'unknown', 'opt_out_requested', 'opted_out', 'resubscribed')) NOT VALID;

-- An existing opt-out applied to everything: both categories.
UPDATE contacts c SET
    marketing_consent = 'opted_out', marketing_consent_at = c.opted_out_at,
    marketing_consent_reason = c.opt_out_reason,
    marketing_consent_source = coalesce((SELECT e.source FROM consent_events e
        WHERE e.contact_id = c.id AND e.kind = 'opt_out' ORDER BY e.id DESC LIMIT 1), 'manual'),
    service_consent = 'opted_out', service_consent_at = c.opted_out_at,
    service_consent_reason = c.opt_out_reason,
    service_consent_source = coalesce((SELECT e.source FROM consent_events e
        WHERE e.contact_id = c.id AND e.kind = 'opt_out' ORDER BY e.id DESC LIMIT 1), 'manual')
WHERE c.opted_out_at IS NOT NULL;

-- consent_events learns the category (NULL = both, as every older row) and
-- the resulting status, and more kinds and sources.
ALTER TABLE consent_events ADD COLUMN category text;
ALTER TABLE consent_events ADD COLUMN status text;
-- Replaced by wider checks right below.
-- squawk-ignore ban-drop-constraint
ALTER TABLE consent_events DROP CONSTRAINT consent_events_kind_check;
-- squawk-ignore ban-drop-constraint
ALTER TABLE consent_events DROP CONSTRAINT consent_events_source_check;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_kind_check CHECK (kind IN (
    'opt_in', 'opt_out', 'opt_out_requested', 'resubscribed', 'unknown',
    'meta_marketing_stopped', 'meta_marketing_resumed', 'undeliverable', 'deliverable')) NOT VALID;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_source_check CHECK (source IN (
    'manual', 'bulk', 'api', 'import', 'keyword', 'meta_user_preferences', 'meta_error', 'automation', 'inbound')) NOT VALID;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_category_check
    CHECK (category IS NULL OR category IN ('marketing', 'service')) NOT VALID;

-- Messages: why one ended failed or skipped (failure_class), the consent
-- category it was checked against, and when a deferred one is retried.
--   deferred  Meta withheld it for ecosystem health (131049); retried at
--             next_attempt_at (>= 24 h later) and not counted in a
--             campaign's in-flight window meanwhile
--   skipped   blocked by consent when it was about to be sent
ALTER TABLE messages ADD COLUMN failure_class text;
ALTER TABLE messages ADD COLUMN consent_category text;
ALTER TABLE messages ADD COLUMN next_attempt_at timestamptz;
ALTER TABLE messages ADD CONSTRAINT messages_failure_class_check CHECK (failure_class IS NULL OR failure_class IN (
    'rejected_by_meta', 'temporary', 'permanent', 'user_stopped_marketing', 'undeliverable', 'ecosystem_limit',
    'blocked_by_consent', 'cancelled')) NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check CHECK (status IN ('pending', 'sending', 'unknown', 'sent',
    'delivered', 'read', 'failed', 'received', 'held', 'cancelled', 'deferred', 'skipped')) NOT VALID;
-- The reaper looks for deferred messages that are due. No row is deferred
-- yet, so the partial index is empty; building it still reads the table
-- once.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX messages_deferred_idx ON messages (next_attempt_at) WHERE status = 'deferred';

-- Keyword opt-outs are part of the conversation's history.
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_events DROP CONSTRAINT conversation_events_kind_check;
ALTER TABLE conversation_events ADD CONSTRAINT conversation_events_kind_check CHECK (kind IN (
    'assigned', 'unassigned', 'closed', 'reopened', 'opt_out_requested', 'opted_out', 'resubscribed')) NOT VALID;

-- Automations can start on a consent keyword (subscription_request).
-- squawk-ignore ban-drop-constraint
ALTER TABLE automations DROP CONSTRAINT automations_trigger_type_check;
ALTER TABLE automations ADD CONSTRAINT automations_trigger_type_check CHECK (trigger_type IN (
    'message_received', 'new_contact', 'tag_added', 'message_status', 'conversation_closed', 'subscription_request')) NOT VALID;

-- +goose Down
-- squawk-ignore ban-drop-constraint
ALTER TABLE automations DROP CONSTRAINT automations_trigger_type_check;
DELETE FROM automations WHERE trigger_type = 'subscription_request';
ALTER TABLE automations ADD CONSTRAINT automations_trigger_type_check CHECK (trigger_type IN (
    'message_received', 'new_contact', 'tag_added', 'message_status', 'conversation_closed')) NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_events DROP CONSTRAINT conversation_events_kind_check;
DELETE FROM conversation_events WHERE kind NOT IN ('assigned', 'unassigned', 'closed', 'reopened');
ALTER TABLE conversation_events ADD CONSTRAINT conversation_events_kind_check
    CHECK (kind IN ('assigned', 'unassigned', 'closed', 'reopened')) NOT VALID;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS messages_deferred_idx;
UPDATE messages SET status = 'pending', status_rank = 0 WHERE status = 'deferred';
UPDATE messages SET status = 'cancelled' WHERE status = 'skipped';
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_status_check;
ALTER TABLE messages ADD CONSTRAINT messages_status_check CHECK (status IN ('pending', 'sending', 'unknown', 'sent',
    'delivered', 'read', 'failed', 'received', 'held', 'cancelled')) NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_failure_class_check;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS next_attempt_at;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS consent_category;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS failure_class;
-- squawk-ignore ban-drop-constraint
ALTER TABLE consent_events DROP CONSTRAINT consent_events_category_check;
-- squawk-ignore ban-drop-constraint
ALTER TABLE consent_events DROP CONSTRAINT consent_events_source_check;
-- squawk-ignore ban-drop-constraint
ALTER TABLE consent_events DROP CONSTRAINT consent_events_kind_check;
DELETE FROM consent_events WHERE kind NOT IN ('opt_in', 'opt_out')
    OR source NOT IN ('manual', 'import', 'api', 'keyword');
ALTER TABLE consent_events ADD CONSTRAINT consent_events_kind_check CHECK (kind IN ('opt_in', 'opt_out')) NOT VALID;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_source_check
    CHECK (source IN ('manual', 'import', 'api', 'keyword')) NOT VALID;
-- squawk-ignore ban-drop-column
ALTER TABLE consent_events DROP COLUMN IF EXISTS status;
-- squawk-ignore ban-drop-column
ALTER TABLE consent_events DROP COLUMN IF EXISTS category;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT contacts_service_consent_check;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT contacts_marketing_consent_check;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS undeliverable_code;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS undeliverable_at;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS meta_marketing_stopped_at;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS service_consent_reason;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS service_consent_source;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS service_consent_at;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS service_consent;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS marketing_consent_reason;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS marketing_consent_source;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS marketing_consent_at;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS marketing_consent;
