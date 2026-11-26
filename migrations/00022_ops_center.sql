-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P9: the operations center (GET /ops/overview, see
-- docs/ARCHITECTURE.md §12 "Operasyon merkezi").

-- The overview lists the automation runs in progress and the runs that
-- failed in the last 24 hours; both are a small part of a table that grows
-- with every trigger.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX automation_runs_active_idx ON automation_runs (created_at DESC)
    WHERE status IN ('queued', 'running', 'waiting');
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX automation_runs_failed_idx ON automation_runs (finished_at DESC) WHERE status = 'failed';

-- ops_hint announces that a section of the overview changed, for the
-- changes no other realtime event reaches the overview with: an
-- automation run started or ended, a Meta account alert arrived (number
-- quality changes included), an account's state or token warning changed.
-- It is an ops.updated event (the realtime envelope plus "section") on
-- system:ops, written to the outbox in the transaction of the change; it
-- carries no IDs, and the client re-reads the section with its own
-- permissions.
-- +goose StatementBegin
CREATE FUNCTION ops_hint() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO outbox_events (topic, payload)
    VALUES ('rt:system:ops', jsonb_build_object('type', 'ops.updated', 'v', 1, 'id', uuidv7(),
        'ts', to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), 'section', TG_ARGV[0]));
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- A run started, or reached completed, failed or cancelled (moving between
-- queued, running and waiting does not change what the overview shows).
CREATE TRIGGER automation_runs_ops_started
    AFTER INSERT ON automation_runs
    FOR EACH ROW EXECUTE FUNCTION ops_hint('automations');
CREATE TRIGGER automation_runs_ops_finished
    AFTER UPDATE OF status ON automation_runs
    FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status AND NEW.status IN ('completed', 'failed', 'cancelled'))
    EXECUTE FUNCTION ops_hint('automations');

-- Number quality changes reach whatsapp_account_events too (00019).
CREATE TRIGGER whatsapp_account_events_ops
    AFTER INSERT ON whatsapp_account_events
    FOR EACH ROW EXECUTE FUNCTION ops_hint('alerts');
CREATE TRIGGER whatsapp_accounts_ops_state
    AFTER UPDATE OF status, token_warning, last_error ON whatsapp_accounts
    FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status OR OLD.token_warning IS DISTINCT FROM NEW.token_warning
                       OR OLD.last_error IS DISTINCT FROM NEW.last_error)
    EXECUTE FUNCTION ops_hint('alerts');

-- +goose Down
-- Down migrations intentionally drop what Up created.
DROP TRIGGER IF EXISTS whatsapp_accounts_ops_state ON whatsapp_accounts;
DROP TRIGGER IF EXISTS whatsapp_account_events_ops ON whatsapp_account_events;
DROP TRIGGER IF EXISTS automation_runs_ops_finished ON automation_runs;
DROP TRIGGER IF EXISTS automation_runs_ops_started ON automation_runs;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS ops_hint();
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS automation_runs_failed_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS automation_runs_active_idx;
