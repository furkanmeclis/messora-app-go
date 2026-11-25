-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P6: conversation ownership, the AI reply loop, internal notes,
-- handoffs (escalations) and the activity timeline. See docs/AI.md
-- ("Sohbet devri").
--
-- Ownership of a conversation (the state machine is
-- internal/modules/conversations/ownership.go):
--   owner_type  none | human | ai
--   ai_mode     off | suggest            while a person (or nobody) owns it
--               auto_reply | awaiting_approval   while the AI owns it
-- ai_status is the AI's live state in the thread (idle | thinking | error);
-- ai_handled_message_id is the last customer message the AI answered (an
-- ai:reply task acts once per inbound message). ai_mode_set_at is NULL
-- until the mode was first decided (from the number's AI profile on the
-- first customer message, or by a person). pending_handoff_id is the
-- escalation a person has not taken yet.
ALTER TABLE conversations ADD COLUMN owner_type text NOT NULL DEFAULT 'none';
ALTER TABLE conversations ADD COLUMN ai_mode text NOT NULL DEFAULT 'off';
ALTER TABLE conversations ADD COLUMN ai_profile_id uuid;
ALTER TABLE conversations ADD COLUMN ai_paused_reason text;
ALTER TABLE conversations ADD COLUMN ai_status text NOT NULL DEFAULT 'idle';
ALTER TABLE conversations ADD COLUMN ai_status_detail text;
ALTER TABLE conversations ADD COLUMN ai_mode_set_at timestamptz;
ALTER TABLE conversations ADD COLUMN ai_handled_message_id uuid;
ALTER TABLE conversations ADD COLUMN ai_disclosed_at timestamptz;
ALTER TABLE conversations ADD COLUMN pending_handoff_id uuid;
-- Existing rows hold the defaults, which satisfy the checks.
ALTER TABLE conversations ADD CONSTRAINT conversations_owner_type_check
    CHECK (owner_type IN ('none', 'human', 'ai')) NOT VALID;
ALTER TABLE conversations ADD CONSTRAINT conversations_ai_mode_check
    CHECK (ai_mode IN ('off', 'suggest', 'auto_reply', 'awaiting_approval')) NOT VALID;
ALTER TABLE conversations ADD CONSTRAINT conversations_ai_status_check
    CHECK (ai_status IN ('idle', 'thinking', 'error')) NOT VALID;
-- New column, every row NULL: the constraint holds without a scan.
-- squawk-ignore adding-foreign-key-constraint
ALTER TABLE conversations ADD CONSTRAINT conversations_ai_profile_id_fkey
    FOREIGN KEY (ai_profile_id) REFERENCES ai_profiles (id) ON DELETE SET NULL NOT VALID;
-- A conversation someone was assigned to is owned by a person.
UPDATE conversations SET owner_type = 'human' WHERE assignee_id IS NOT NULL;

-- Per-contact switch: the AI never answers or drafts for this contact.
ALTER TABLE contacts ADD COLUMN ai_disabled boolean NOT NULL DEFAULT false;

-- Messages the AI wrote: sent_by says who wrote an outbound message
-- (user | api_key | system | ai; NULL for inbound and older rows) and
-- ai_usage_id the AI request that produced it. source gains 'ai'.
ALTER TABLE messages ADD COLUMN sent_by text;
ALTER TABLE messages ADD COLUMN ai_usage_id uuid;
ALTER TABLE messages ADD CONSTRAINT messages_sent_by_check
    CHECK (sent_by IS NULL OR sent_by IN ('user', 'api_key', 'system', 'ai')) NOT VALID;
-- squawk-ignore adding-foreign-key-constraint
ALTER TABLE messages ADD CONSTRAINT messages_ai_usage_id_fkey
    FOREIGN KEY (ai_usage_id) REFERENCES ai_usage (id) ON DELETE SET NULL NOT VALID;
-- Replaced by a wider check right below.
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_source_check;
ALTER TABLE messages ADD CONSTRAINT messages_source_check
    CHECK (source IN ('api', 'panel', 'webhook', 'campaign', 'automation', 'ai')) NOT VALID;

-- Internal notes of the team. Never sent to the customer; notes with
-- for_ai are given to the AI as operator instructions (never as customer
-- content).
--   author_type user | system | ai
CREATE TABLE conversation_notes (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    author_type     text NOT NULL CHECK (author_type IN ('user', 'system', 'ai')),
    author_id       uuid REFERENCES users (id) ON DELETE SET NULL,
    body            text NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    visibility      text NOT NULL DEFAULT 'internal' CHECK (visibility = 'internal'),
    for_ai          boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversation_notes_conversation_idx ON conversation_notes (conversation_id, id);

-- Every change of owner. An escalation (AI → person) carries the reason,
-- the AI's (or a deterministic) summary, the customer's need, the
-- suggested next action and the collected fields; accepted_* is the person
-- who took it.
--   from_type ai | human | none        to_type ai | human | pool
CREATE TABLE conversation_handoffs (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    conversation_id       uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    from_type             text NOT NULL CHECK (from_type IN ('ai', 'human', 'none')),
    from_user_id          uuid REFERENCES users (id) ON DELETE SET NULL,
    to_type               text NOT NULL CHECK (to_type IN ('ai', 'human', 'pool')),
    to_user_id            uuid REFERENCES users (id) ON DELETE SET NULL,
    reason                text NOT NULL CHECK (reason IN ('customer_request', 'no_answer', 'low_confidence', 'critical_intent',
                                                          'unauthorized_action', 'window_closed', 'ai_error', 'manual')),
    reason_detail         text NOT NULL DEFAULT '' CHECK (length(reason_detail) <= 500),
    summary               text NOT NULL DEFAULT '' CHECK (length(summary) <= 4000),
    summary_source        text NOT NULL DEFAULT 'none' CHECK (summary_source IN ('ai', 'fallback', 'none')),
    detected_need         text NOT NULL DEFAULT '' CHECK (length(detected_need) <= 1000),
    suggested_next_action text NOT NULL DEFAULT '' CHECK (length(suggested_next_action) <= 1000),
    collected_fields      jsonb NOT NULL DEFAULT '{}'::jsonb,
    actor_type            text NOT NULL CHECK (actor_type IN ('user', 'api_key', 'system', 'ai')),
    actor_id              uuid,
    created_at            timestamptz NOT NULL DEFAULT now(),
    accepted_by           uuid REFERENCES users (id) ON DELETE SET NULL,
    accepted_at           timestamptz
);
CREATE INDEX conversation_handoffs_conversation_idx ON conversation_handoffs (conversation_id, id DESC);

-- AI drafts: a suggestion for the agent (suggest mode) or a reply waiting
-- for approval (awaiting_approval mode).
--   kind   suggestion | approval
--   status pending | accepted | edited | rejected | superseded
CREATE TABLE conversation_ai_drafts (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    message_id      uuid REFERENCES messages (id) ON DELETE SET NULL,
    kind            text NOT NULL CHECK (kind IN ('suggestion', 'approval')),
    body            text NOT NULL CHECK (length(body) <= 4096),
    confidence      double precision NOT NULL DEFAULT 0,
    intent          text NOT NULL DEFAULT '',
    ai_usage_id     uuid REFERENCES ai_usage (id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'edited', 'rejected', 'superseded')),
    decided_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    decided_at      timestamptz,
    sent_message_id uuid REFERENCES messages (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversation_ai_drafts_conversation_idx ON conversation_ai_drafts (conversation_id, id DESC);

-- The conversation's activity timeline: one typed stream written next to
-- every change (handoffs, assignee changes, notes, AI actions and replies,
-- automation runs, sent and failed messages, escalations).
CREATE TABLE conversation_activity (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN (
        'handed_to_ai', 'handed_to_human', 'ai_mode_changed', 'assignee_changed', 'closed', 'reopened',
        'note_added', 'ai_action', 'ai_reply_sent', 'ai_draft_created', 'ai_draft_decided', 'ai_error',
        'escalation', 'handoff_accepted', 'automation_started', 'automation_completed', 'automation_failed',
        'message_sent', 'message_failed', 'contact_ai_disabled', 'contact_ai_enabled')),
    actor_type      text NOT NULL CHECK (actor_type IN ('user', 'api_key', 'system', 'ai')),
    actor_id        uuid,
    data            jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversation_activity_conversation_idx ON conversation_activity (conversation_id, id);

-- Automation runs started for a conversation (by a trigger, a person or
-- the AI) are shown and announced in it.
ALTER TABLE automation_runs ADD COLUMN conversation_id uuid;
ALTER TABLE automation_runs ADD COLUMN started_by text NOT NULL DEFAULT 'system';
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_started_by_check
    CHECK (started_by IN ('system', 'user', 'ai')) NOT VALID;
-- squawk-ignore adding-foreign-key-constraint
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_conversation_id_fkey
    FOREIGN KEY (conversation_id) REFERENCES conversations (id) ON DELETE SET NULL NOT VALID;
UPDATE automation_runs SET conversation_id = (event ->> 'conversation_id')::uuid
WHERE event ? 'conversation_id' AND EXISTS (SELECT 1 FROM conversations c WHERE c.id = (event ->> 'conversation_id')::uuid);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX automation_runs_conversation_idx ON automation_runs (conversation_id, id DESC) WHERE conversation_id IS NOT NULL;
-- The inbox filters "AI'da" and "İnsan bekleyen".
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX conversations_owner_idx ON conversations (owner_type, last_message_at DESC) WHERE owner_type = 'ai';
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX conversations_pending_handoff_idx ON conversations (last_message_at DESC) WHERE pending_handoff_id IS NOT NULL;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS conversations_pending_handoff_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS conversations_owner_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS automation_runs_conversation_idx;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS started_by;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS conversation_id;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS conversation_activity;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS conversation_ai_drafts;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS conversation_handoffs;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS conversation_notes;
UPDATE messages SET source = 'automation' WHERE source = 'ai';
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_source_check;
ALTER TABLE messages ADD CONSTRAINT messages_source_check
    CHECK (source IN ('api', 'panel', 'webhook', 'campaign', 'automation')) NOT VALID;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS ai_usage_id;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS sent_by;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS ai_disabled;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS pending_handoff_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_disclosed_at;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_handled_message_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_mode_set_at;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_status_detail;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_status;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_paused_reason;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_profile_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS ai_mode;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS owner_type;
