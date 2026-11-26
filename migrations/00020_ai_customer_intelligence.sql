-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P7: AI customer intelligence (lead capture for unknown
-- numbers, per-field provenance), campaign attribution of AI usage, AI
-- reply classification of campaign replies. See docs/AI.md ("Müşteri
-- bilgisi toplama", "Maliyet ve kullanım").

-- A contact created by an inbound message from a number nobody knew
-- (created_by_inbound_at) gets a profile the AI may build:
--   profile_status   NULL (not built by the AI) | new | collecting | complete
--   profile_progress {"required": n, "filled": m, "missing": ["email", …]}
-- Existing contacts keep NULL: the AI never treats them as unknown numbers.
ALTER TABLE contacts ADD COLUMN created_by_inbound_at timestamptz;
ALTER TABLE contacts ADD COLUMN profile_status text;
ALTER TABLE contacts ADD COLUMN profile_progress jsonb;
ALTER TABLE contacts ADD CONSTRAINT contacts_profile_status_check
    CHECK (profile_status IS NULL OR profile_status IN ('new', 'collecting', 'complete')) NOT VALID;
-- Contacts list filter "AI tarafından oluşturulan"; every row is NULL now.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX contacts_profile_status_idx ON contacts (id DESC) WHERE profile_status IN ('collecting', 'complete');

-- Where a contact field's value came from. A field without a row was set
-- by people, an import or the API before provenance existed and counts as
-- verified: the AI never overwrites it.
--   field   name | email | var1…var5 | attr:<key>
--   source  ai | user | import | api
CREATE TABLE contact_field_sources (
    contact_id   uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    field        text NOT NULL CHECK (field ~ '^(name|email|var[1-5]|attr:[A-Za-z0-9_.-]{1,64})$'),
    source       text NOT NULL CHECK (source IN ('ai', 'user', 'import', 'api')),
    verified     boolean NOT NULL DEFAULT false,
    collected_at timestamptz NOT NULL DEFAULT now(),
    ai_usage_id  uuid REFERENCES ai_usage (id) ON DELETE SET NULL,
    conversation_id uuid REFERENCES conversations (id) ON DELETE SET NULL,
    verified_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    verified_at  timestamptz,
    PRIMARY KEY (contact_id, field)
);

-- New timeline item: the AI completed the profile of a new contact.
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_activity DROP CONSTRAINT conversation_activity_kind_check;
ALTER TABLE conversation_activity ADD CONSTRAINT conversation_activity_kind_check CHECK (kind IN (
    'handed_to_ai', 'handed_to_human', 'ai_mode_changed', 'assignee_changed', 'closed', 'reopened',
    'note_added', 'ai_action', 'ai_reply_sent', 'ai_draft_created', 'ai_draft_decided', 'ai_error',
    'escalation', 'handoff_accepted', 'automation_started', 'automation_completed', 'automation_failed',
    'message_sent', 'message_failed', 'contact_ai_disabled', 'contact_ai_enabled', 'profile_completed')) NOT VALID;

-- New escalation reasons: the monthly AI budget is used up; the profile of
-- a new contact is complete and the AI profile hands it to the team.
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_reason_check;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_reason_check CHECK (reason IN (
    'customer_request', 'no_answer', 'low_confidence', 'critical_intent', 'unauthorized_action', 'window_closed',
    'ai_error', 'manual', 'ai_budget', 'profile_complete')) NOT VALID;

-- Campaign attribution looks up the latest campaign message a contact got
-- on a number before an AI request.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX messages_campaign_reach_idx ON messages (contact_id, phone_number_id, sent_at DESC)
    WHERE campaign_id IS NOT NULL AND direction = 'outbound';

-- The AI's reading of a customer's first reply to a campaign (company
-- setting "classify campaign replies"). P3's reaction pipeline reads it as
-- a system-computed positive/negative reaction. One row per campaign and
-- contact: the first reply is the one classified.
--   status    pending | done | failed | skipped
--   sentiment positive | negative | neutral (NULL until done)
CREATE TABLE campaign_reply_classifications (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id     uuid NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    contact_id      uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    conversation_id uuid REFERENCES conversations (id) ON DELETE SET NULL,
    message_id      uuid REFERENCES messages (id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed', 'skipped')),
    sentiment       text CHECK (sentiment IN ('positive', 'negative', 'neutral')),
    confidence      double precision,
    reason          text NOT NULL DEFAULT '' CHECK (length(reason) <= 500),
    error           text NOT NULL DEFAULT '' CHECK (length(error) <= 200),
    ai_usage_id     uuid REFERENCES ai_usage (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    classified_at   timestamptz,
    UNIQUE (campaign_id, contact_id)
);

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_reply_classifications;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS messages_campaign_reach_idx;
UPDATE conversation_handoffs SET reason = 'ai_error' WHERE reason IN ('ai_budget', 'profile_complete');
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_reason_check;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_reason_check CHECK (reason IN (
    'customer_request', 'no_answer', 'low_confidence', 'critical_intent', 'unauthorized_action', 'window_closed',
    'ai_error', 'manual')) NOT VALID;
DELETE FROM conversation_activity WHERE kind = 'profile_completed';
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_activity DROP CONSTRAINT conversation_activity_kind_check;
ALTER TABLE conversation_activity ADD CONSTRAINT conversation_activity_kind_check CHECK (kind IN (
    'handed_to_ai', 'handed_to_human', 'ai_mode_changed', 'assignee_changed', 'closed', 'reopened',
    'note_added', 'ai_action', 'ai_reply_sent', 'ai_draft_created', 'ai_draft_decided', 'ai_error',
    'escalation', 'handoff_accepted', 'automation_started', 'automation_completed', 'automation_failed',
    'message_sent', 'message_failed', 'contact_ai_disabled', 'contact_ai_enabled')) NOT VALID;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contact_field_sources;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS contacts_profile_status_idx;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_profile_status_check;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS profile_progress;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS profile_status;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS created_by_inbound_at;
