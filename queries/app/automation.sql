-- name: HasActiveAutomation :one
SELECT EXISTS (SELECT 1 FROM automations WHERE active AND trigger_type = @trigger_type);

-- name: InsertAutomation :one
INSERT INTO automations (name, description, active, trigger_type, trigger, steps, program, layout, created_by, setup, preset_key, preset_version)
VALUES (@name, @description, @active, @trigger_type, @trigger, @steps, @program, @layout, sqlc.narg(created_by), @setup,
        sqlc.narg(preset_key), sqlc.narg(preset_version))
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
    setup = coalesce(sqlc.narg(setup), setup),
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
INSERT INTO automation_runs (automation_id, automation_version, event_key, event, contact_id, conversation_id, started_by)
VALUES (@automation_id, @automation_version, @event_key, @event, @contact_id, sqlc.narg(conversation_id), @started_by)
ON CONFLICT (automation_id, event_key) DO NOTHING
RETURNING id;

-- name: RecentRunForContact :one
SELECT EXISTS (SELECT 1 FROM automation_runs WHERE automation_id = @automation_id AND contact_id = @contact_id
               AND created_at > now() - make_interval(secs => @within_seconds::double precision));

-- name: LockAutomationRun :one
SELECT * FROM automation_runs WHERE id = @id FOR UPDATE;

-- name: AdvanceAutomationRun :exec
UPDATE automation_runs SET pc = @pc, steps_executed = steps_executed + 1, status = 'running', attempt = 0, updated_at = now() WHERE id = @id;

-- name: WaitAutomationRun :exec
UPDATE automation_runs SET pc = @pc, status = 'waiting', steps_executed = steps_executed + 1, attempt = 0,
    resume_at = now() + make_interval(secs => @wait_seconds::double precision), updated_at = now()
WHERE id = @id;

-- name: FinishAutomationRun :exec
UPDATE automation_runs SET status = @status, error = sqlc.narg(error), error_code = sqlc.narg(error_code), finished_at = now(),
    updated_at = now(), resume_at = NULL
WHERE id = @id;

-- name: RetryAutomationRun :exec
-- A failed attempt of the current step: wait, then run the same step again.
UPDATE automation_runs SET status = 'waiting', attempt = attempt + 1,
    resume_at = now() + make_interval(secs => @wait_seconds::double precision), updated_at = now()
WHERE id = @id;

-- name: MergeAutomationRunVars :exec
UPDATE automation_runs SET vars = vars || @vars::jsonb, updated_at = now() WHERE id = @id;

-- name: GetAutomationSend :one
SELECT message_id FROM automation_sends WHERE run_id = @run_id AND pc = @pc;

-- name: InsertAutomationSend :exec
INSERT INTO automation_sends (run_id, pc, message_id) VALUES (@run_id, @pc, @message_id);

-- name: WindowClosingCandidates :many
-- Conversations whose customer service window (24 hours after the
-- customer's last window-opening message) closes soon: the window opened
-- between @opened_after (now - 24 h) and @opened_before (now - 24 h + the
-- automation's minutes).
-- last_message_* is the latest message of the thread, reactions, system
-- notices and failed sends aside: inbound means the customer is waiting
-- for an answer. inbound_message_id (the message that opened the window)
-- identifies the window.
SELECT c.id, c.contact_id, c.phone_number_id, lm.id AS last_message_id, lm.direction AS last_direction,
       li.id AS inbound_message_id, c.last_inbound_at
FROM conversations c
CROSS JOIN LATERAL (
    SELECT m.id, m.direction FROM messages m
    WHERE m.contact_id = c.contact_id AND m.phone_number_id = c.phone_number_id
      AND m.type NOT IN ('reaction', 'unsupported', 'system')
      AND NOT (m.direction = 'outbound' AND m.status IN ('failed', 'cancelled'))
    ORDER BY m.id DESC LIMIT 1) lm
CROSS JOIN LATERAL (
    SELECT m.id FROM messages m
    WHERE m.contact_id = c.contact_id AND m.phone_number_id = c.phone_number_id AND m.direction = 'inbound'
      AND m.type NOT IN ('reaction', 'unsupported', 'system')
    ORDER BY m.id DESC LIMIT 1) li
WHERE c.last_inbound_at IS NOT NULL
  AND c.last_inbound_at > @opened_after::timestamptz
  AND c.last_inbound_at <= @opened_before::timestamptz
  AND (sqlc.narg(phone_number_id)::uuid IS NULL OR c.phone_number_id = sqlc.narg(phone_number_id)::uuid)
ORDER BY c.last_inbound_at
LIMIT 1000;

-- name: NoReplyCandidates :many
-- Conversations whose latest message (reactions, system notices and failed
-- sends aside) went in @direction between @after and @before: outbound =
-- the customer has not answered us, inbound = the team has not answered
-- the customer. Campaign messages count as ours only with
-- @include_campaigns; for outbound the customer must have written before.
SELECT c.id, c.contact_id, c.phone_number_id, lm.id AS message_id
FROM conversations c
CROSS JOIN LATERAL (
    SELECT m.id, m.direction, m.campaign_id, m.created_at FROM messages m
    WHERE m.contact_id = c.contact_id AND m.phone_number_id = c.phone_number_id
      AND m.type NOT IN ('reaction', 'unsupported', 'system')
      AND NOT (m.direction = 'outbound' AND m.status IN ('failed', 'cancelled'))
    ORDER BY m.id DESC LIMIT 1) lm
WHERE c.last_message_at > @after::timestamptz AND c.last_message_at <= @before::timestamptz
  AND lm.direction = @direction::text
  AND lm.created_at <= @before::timestamptz
  AND (@direction::text = 'inbound' OR (c.last_inbound_at IS NOT NULL AND (@include_campaigns::boolean OR lm.campaign_id IS NULL)))
  AND (sqlc.narg(phone_number_id)::uuid IS NULL OR c.phone_number_id = sqlc.narg(phone_number_id)::uuid)
ORDER BY c.last_message_at
LIMIT 1000;

-- name: CampaignRepliedTo :one
-- The campaign an inbound message answers: the campaign message it quotes,
-- else the thread's latest outbound message when that is a campaign
-- message of the last 72 hours and the customer has not written since.
-- The nil uuid means none.
SELECT coalesce(
    (SELECT q.campaign_id FROM messages q WHERE sqlc.narg(reply_to_wamid)::text IS NOT NULL AND q.wamid = sqlc.narg(reply_to_wamid)::text),
    (SELECT o.campaign_id FROM messages o
     WHERE o.contact_id = @contact_id AND o.phone_number_id = @phone_number_id AND o.direction = 'outbound'
       AND o.id < @message_id AND o.created_at > now() - interval '72 hours'
       AND NOT EXISTS (SELECT 1 FROM messages i WHERE i.contact_id = o.contact_id AND i.phone_number_id = o.phone_number_id
                       AND i.direction = 'inbound' AND i.id > o.id AND i.id < @message_id)
     ORDER BY o.id DESC LIMIT 1),
    '00000000-0000-0000-0000-000000000000'
)::uuid AS campaign_id;

-- name: CampaignOfWAMID :one
SELECT campaign_id FROM messages WHERE wamid = @wamid;

-- name: ListAutomationsByTypes :many
SELECT * FROM automations WHERE active AND trigger_type = ANY(@trigger_types::text[]) ORDER BY created_at;

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
