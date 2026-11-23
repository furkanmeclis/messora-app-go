-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Workflow canvas (Phase 14). Where the editor draws each node: step id
-- (or "trigger") → {"x": …, "y": …}. Positions only; the steps stay the
-- source of truth for the engine.
-- squawk-ignore adding-field-with-default
ALTER TABLE automations ADD COLUMN layout jsonb DEFAULT '{}'::jsonb;

-- Run trace: the step ids a run went through, in execution order. The
-- engine appends one id per executed instruction (jumps excluded).
-- squawk-ignore adding-field-with-default
ALTER TABLE automation_runs ADD COLUMN visited jsonb DEFAULT '[]'::jsonb;

-- +goose Down
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS visited;
-- squawk-ignore ban-drop-column
ALTER TABLE automations DROP COLUMN IF EXISTS layout;
