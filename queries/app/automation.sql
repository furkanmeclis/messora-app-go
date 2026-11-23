-- name: HasActiveAutomation :one
SELECT EXISTS (SELECT 1 FROM automations WHERE active AND trigger_type = @trigger_type);

-- name: InsertAutomation :one
INSERT INTO automations (name, description, active, trigger_type, trigger, steps, program, layout, created_by)
VALUES (@name, @description, @active, @trigger_type, @trigger, @steps, @program, @layout, sqlc.narg(created_by))
RETURNING *;

-- name: UpdateAutomation :one
UPDATE automations SET
    name = coalesce(sqlc.narg(name), name),
    description = coalesce(sqlc.narg(description), description),
    active = coalesce(sqlc.narg(active), active),
    trigger_type = coalesce(sqlc.narg(trigger_type), trigger_type),
    trigger = coalesce(sqlc.narg(trigger), trigger),
    steps = coalesce(sqlc.narg(steps), steps),
    program = coalesce(sqlc.narg(program), program),
    -- Moving nodes on the canvas is not a new version: runs continue.
    layout = coalesce(sqlc.narg(layout), layout),
    version = CASE WHEN sqlc.narg(program)::jsonb IS NOT NULL OR sqlc.narg(trigger)::jsonb IS NOT NULL THEN version + 1 ELSE version END,
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: GetAutomation :one
SELECT * FROM automations WHERE id = @id;

-- name: ListAutomations :many
-- The LEFT JOIN makes run statistics nullable: an automation may never
-- have run.
SELECT a.*, coalesce(s.run_count, 0)::bigint AS run_count, (s.last_run_at IS NOT NULL)::boolean AS has_run,
       coalesce(s.last_run_at, '1970-01-01T00:00:00Z')::timestamptz AS last_run_at
FROM automations a
LEFT JOIN (SELECT automation_id, count(*) AS run_count, max(created_at) AS last_run_at
           FROM automation_runs GROUP BY automation_id) s ON s.automation_id = a.id
ORDER BY a.created_at;

-- name: GetAutomationWithStats :one
SELECT a.*, coalesce(s.run_count, 0)::bigint AS run_count, (s.last_run_at IS NOT NULL)::boolean AS has_run,
       coalesce(s.last_run_at, '1970-01-01T00:00:00Z')::timestamptz AS last_run_at
FROM automations a
LEFT JOIN (SELECT automation_id, count(*) AS run_count, max(created_at) AS last_run_at
           FROM automation_runs WHERE automation_id = @id GROUP BY automation_id) s ON s.automation_id = a.id
WHERE a.id = @id;

-- name: DeleteAutomation :execrows
DELETE FROM automations WHERE id = @id;

-- name: ActiveAutomationsFor :many
SELECT * FROM automations WHERE active AND trigger_type = @trigger_type ORDER BY created_at;

-- name: InsertAutomationRun :one
-- No row when this automation already ran for this event.
INSERT INTO automation_runs (automation_id, automation_version, event_key, event, contact_id)
VALUES (@automation_id, @automation_version, @event_key, @event, @contact_id)
ON CONFLICT (automation_id, event_key) DO NOTHING
RETURNING id;

-- name: RecentRunForContact :one
SELECT EXISTS (SELECT 1 FROM automation_runs WHERE automation_id = @automation_id AND contact_id = @contact_id
               AND created_at > now() - make_interval(secs => @within_seconds::double precision));

-- name: LockAutomationRun :one
SELECT * FROM automation_runs WHERE id = @id FOR UPDATE;

-- name: AdvanceAutomationRun :exec
UPDATE automation_runs SET pc = @pc, steps_executed = steps_executed + 1, status = 'running', updated_at = now() WHERE id = @id;

-- name: WaitAutomationRun :exec
UPDATE automation_runs SET pc = @pc, status = 'waiting', steps_executed = steps_executed + 1,
    resume_at = now() + make_interval(secs => @wait_seconds::double precision), updated_at = now()
WHERE id = @id;

-- name: FinishAutomationRun :exec
UPDATE automation_runs SET status = @status, error = sqlc.narg(error), finished_at = now(), updated_at = now(), resume_at = NULL
WHERE id = @id;

-- name: AppendAutomationRunVisited :exec
-- Run trace: the step a run just executed.
UPDATE automation_runs SET visited = coalesce(visited, '[]'::jsonb) || jsonb_build_array(@step_id::text)
WHERE id = @id;

-- name: InsertAutomationRunStep :exec
INSERT INTO automation_run_steps (run_id, pc, action, outcome, detail) VALUES (@run_id, @pc, @action, @outcome, @detail);

-- name: ListAutomationRuns :many
SELECT r.*, coalesce(c.phone_e164, '')::text AS phone
FROM automation_runs r LEFT JOIN contacts c ON c.id = r.contact_id
WHERE r.automation_id = @automation_id
  AND (sqlc.narg(before_id)::uuid IS NULL OR r.id < sqlc.narg(before_id)::uuid)
ORDER BY r.id DESC LIMIT @lim;

-- name: ListAutomationRunSteps :many
SELECT * FROM automation_run_steps WHERE run_id = @run_id ORDER BY id;

-- name: GetAutomationRun :one
SELECT * FROM automation_runs WHERE id = @id;

-- name: OverdueWaitingRuns :many
-- Waiting runs whose resume task may have been lost.
SELECT id FROM automation_runs WHERE status = 'waiting' AND resume_at < now() - interval '1 minute' LIMIT 500;

-- name: StaleRunningRuns :many
SELECT id FROM automation_runs WHERE status IN ('queued', 'running') AND updated_at < now() - interval '5 minutes' LIMIT 500;

-- name: MuteContactAutomations :exec
UPDATE contacts SET automations_muted_until = now() + make_interval(secs => @seconds::double precision), updated_at = now()
WHERE id = @id;

-- name: CancelRunsOfAutomation :execrows
UPDATE automation_runs SET status = 'cancelled', finished_at = now(), updated_at = now(), resume_at = NULL
WHERE automation_id = @automation_id AND status IN ('queued', 'running', 'waiting');

-- name: RemoveContactTagByName :execrows
DELETE FROM contact_tags ct USING tags t
WHERE ct.tag_id = t.id AND ct.contact_id = @contact_id AND lower(t.name) = lower(@name::text);

-- name: ContactHasTag :one
SELECT EXISTS (SELECT 1 FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id
               WHERE ct.contact_id = @contact_id AND lower(t.name) = lower(@name::text));
