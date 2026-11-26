-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P8: new triggers, AI and handoff steps, per-step retries,
-- idempotent sends and the preset library. See docs/AUTOMATIONS.md.

-- New triggers: the customer service window is about to close, nobody
-- answered for a while, a reply to a campaign message, a reaction, an AI
-- escalation.
-- squawk-ignore ban-drop-constraint
ALTER TABLE automations DROP CONSTRAINT automations_trigger_type_check;
ALTER TABLE automations ADD CONSTRAINT automations_trigger_type_check CHECK (trigger_type IN (
    'message_received', 'new_contact', 'tag_added', 'message_status', 'conversation_closed', 'subscription_request',
    'window_closing', 'no_reply', 'campaign_reply', 'reaction', 'ai_escalation')) NOT VALID;

-- Presets: an installed preset remembers where it came from, and the values
-- the user still has to choose (e.g. which template) as
-- [{step_id, field, label}]. An automation with open setup items cannot be
-- enabled.
-- squawk-ignore adding-field-with-default
ALTER TABLE automations ADD COLUMN setup jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE automations ADD COLUMN preset_key text;
-- squawk-ignore prefer-bigint-over-int
ALTER TABLE automations ADD COLUMN preset_version integer;

-- Run variables (what AI steps produced: {{var:<name>}}), the attempt of
-- the current step (per-step retries) and a machine-readable failure reason
-- (ai_disabled, ai_not_allowed, retries_exhausted …).
-- squawk-ignore adding-field-with-default
ALTER TABLE automation_runs ADD COLUMN vars jsonb NOT NULL DEFAULT '{}'::jsonb;
-- squawk-ignore prefer-bigint-over-int,adding-field-with-default
ALTER TABLE automation_runs ADD COLUMN attempt integer NOT NULL DEFAULT 0;
ALTER TABLE automation_runs ADD COLUMN error_code text;

-- A failed attempt that will be retried is logged as 'retrying'.
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_run_steps DROP CONSTRAINT automation_run_steps_outcome_check;
ALTER TABLE automation_run_steps ADD CONSTRAINT automation_run_steps_outcome_check
    CHECK (outcome IN ('ok', 'skipped', 'failed', 'waiting', 'retrying')) NOT VALID;

-- Idempotent sends: a step of a run queues at most one message, however
-- often the step is executed again (retries, recovered runs).
CREATE TABLE automation_sends (
    run_id     uuid NOT NULL REFERENCES automation_runs (id) ON DELETE CASCADE,
    -- squawk-ignore prefer-bigint-over-int
    pc         integer NOT NULL,
    message_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, pc)
);

-- Handoffs made by automation steps (handoff_to_human, ai_route_to_agent,
-- handoff_to_ai).
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_reason_check;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_reason_check CHECK (reason IN (
    'customer_request', 'no_answer', 'low_confidence', 'critical_intent', 'unauthorized_action', 'window_closed',
    'ai_error', 'manual', 'automation')) NOT VALID;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_reason_check;
UPDATE conversation_handoffs SET reason = 'manual' WHERE reason = 'automation';
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_reason_check CHECK (reason IN (
    'customer_request', 'no_answer', 'low_confidence', 'critical_intent', 'unauthorized_action', 'window_closed',
    'ai_error', 'manual')) NOT VALID;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS automation_sends;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_run_steps DROP CONSTRAINT automation_run_steps_outcome_check;
UPDATE automation_run_steps SET outcome = 'failed' WHERE outcome = 'retrying';
ALTER TABLE automation_run_steps ADD CONSTRAINT automation_run_steps_outcome_check
    CHECK (outcome IN ('ok', 'skipped', 'failed', 'waiting')) NOT VALID;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS error_code;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS attempt;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS vars;
-- squawk-ignore ban-drop-column
ALTER TABLE automations DROP COLUMN IF EXISTS preset_version;
-- squawk-ignore ban-drop-column
ALTER TABLE automations DROP COLUMN IF EXISTS preset_key;
-- squawk-ignore ban-drop-column
ALTER TABLE automations DROP COLUMN IF EXISTS setup;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automations DROP CONSTRAINT automations_trigger_type_check;
DELETE FROM automations WHERE trigger_type IN ('window_closing', 'no_reply', 'campaign_reply', 'reaction', 'ai_escalation');
ALTER TABLE automations ADD CONSTRAINT automations_trigger_type_check CHECK (trigger_type IN (
    'message_received', 'new_contact', 'tag_added', 'message_status', 'conversation_closed', 'subscription_request')) NOT VALID;
