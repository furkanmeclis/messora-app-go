-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- An automation reacts to one trigger with a list of steps. The steps are
-- stored as written (for the editor) and compiled into a flat program of
-- instructions (for the runner); see internal/modules/automation.
CREATE TABLE automations (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description  text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    active       boolean NOT NULL DEFAULT false,
    trigger_type text NOT NULL CHECK (trigger_type IN ('message_received', 'new_contact', 'tag_added', 'message_status', 'conversation_closed')),
    trigger      jsonb NOT NULL DEFAULT '{}'::jsonb,
    steps        jsonb NOT NULL,
    program      jsonb NOT NULL,
    -- squawk-ignore prefer-bigint-over-int
    version      integer NOT NULL DEFAULT 1,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX automations_trigger_idx ON automations (trigger_type) WHERE active;

-- One run per (automation, triggering event): a webhook delivered twice
-- produces the same event key and therefore no second run.
CREATE TABLE automation_runs (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    automation_id   uuid NOT NULL REFERENCES automations (id) ON DELETE CASCADE,
    -- squawk-ignore prefer-bigint-over-int
    automation_version integer NOT NULL,
    event_key       text NOT NULL,
    event           jsonb NOT NULL,
    contact_id      uuid REFERENCES contacts (id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'waiting', 'completed', 'failed', 'cancelled')),
    -- squawk-ignore prefer-bigint-over-int
    pc              integer NOT NULL DEFAULT 0, -- next instruction
    -- squawk-ignore prefer-bigint-over-int
    steps_executed  integer NOT NULL DEFAULT 0,
    resume_at       timestamptz,
    error           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz,
    UNIQUE (automation_id, event_key)
);
CREATE INDEX automation_runs_automation_idx ON automation_runs (automation_id, id DESC);
CREATE INDEX automation_runs_waiting_idx ON automation_runs (resume_at) WHERE status = 'waiting';
CREATE INDEX automation_runs_contact_idx ON automation_runs (contact_id, created_at DESC);

-- Execution log: one row per executed instruction.
CREATE TABLE automation_run_steps (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    run_id      uuid NOT NULL REFERENCES automation_runs (id) ON DELETE CASCADE,
    -- squawk-ignore prefer-bigint-over-int
    pc          integer NOT NULL,
    action      text NOT NULL,
    outcome     text NOT NULL CHECK (outcome IN ('ok', 'skipped', 'failed', 'waiting')),
    detail      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, pc, created_at)
);
CREATE INDEX automation_run_steps_run_idx ON automation_run_steps (run_id, id);

-- "Mute" action: no automation runs for a contact until muted_until.
-- squawk-ignore adding-field-with-default
ALTER TABLE contacts ADD COLUMN automations_muted_until timestamptz;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS automations_muted_until;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS automation_run_steps;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS automation_runs;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS automations;
