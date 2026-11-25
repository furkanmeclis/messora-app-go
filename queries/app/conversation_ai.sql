-- ── Conversation ownership, AI loop, notes, handoffs, activity (16C P6) ────

-- name: LockConversation :one
SELECT * FROM conversations WHERE id = @id FOR UPDATE;

-- name: SetConversationOwnership :one
-- Applies a transition of the ownership state machine
-- (internal/modules/conversations/ownership.go).
UPDATE conversations SET owner_type = @owner_type, ai_mode = @ai_mode,
    ai_profile_id = sqlc.narg(ai_profile_id), ai_paused_reason = sqlc.narg(ai_paused_reason),
    pending_handoff_id = sqlc.narg(pending_handoff_id), ai_mode_set_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: LockConversationsOfContact :many
SELECT * FROM conversations WHERE contact_id = @contact_id ORDER BY id FOR UPDATE;

-- name: SetConversationAIStatus :one
UPDATE conversations SET ai_status = @ai_status, ai_status_detail = sqlc.narg(ai_status_detail), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetConversationAIHandled :exec
UPDATE conversations SET ai_handled_message_id = @message_id WHERE id = @id;

-- name: MarkConversationAIDisclosed :exec
UPDATE conversations SET ai_disclosed_at = now() WHERE id = @id AND ai_disclosed_at IS NULL;

-- name: LatestInboundMessage :one
-- The customer's latest message in a thread (debounce: only the newest one
-- of a burst is answered).
SELECT * FROM messages
WHERE contact_id = @contact_id AND phone_number_id = @phone_number_id AND direction = 'inbound'
ORDER BY id DESC
LIMIT 1;

-- name: CountConversationAIRequests :one
-- AI reply and suggestion requests of a conversation in the last hour (the
-- per-conversation rate and cost guard), failures included.
SELECT count(*) FROM ai_usage
WHERE conversation_id = @conversation_id AND purpose IN ('reply', 'suggest')
  AND created_at > now() - interval '1 hour';

-- name: SetContactAIDisabled :one
UPDATE contacts SET ai_disabled = @ai_disabled, updated_at = now() WHERE id = @id RETURNING *;

-- name: SetContactAttribute :exec
UPDATE contacts SET attributes = attributes || jsonb_build_object(@key::text, @value::text), updated_at = now()
WHERE id = @id;

-- name: ListConversationReaders :many
-- Active users who can read conversations (escalation notices).
SELECT DISTINCT u.id, rp.permission_key = 'conversations.read_all' AS reads_all FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE u.status = 'active' AND rp.permission_key IN ('conversations.read', 'conversations.read_all');

-- ── Notes ──────────────────────────────────────────────────────────────────

-- name: InsertConversationNote :one
INSERT INTO conversation_notes (conversation_id, author_type, author_id, body, for_ai)
VALUES (@conversation_id, @author_type, sqlc.narg(author_id), @body, @for_ai)
RETURNING *;

-- name: ListConversationNotes :many
SELECT n.*, u.name AS author_name
FROM conversation_notes n
LEFT JOIN users u ON u.id = n.author_id
WHERE n.conversation_id = @conversation_id
ORDER BY n.id DESC
LIMIT @lim;

-- name: ListAINotes :many
-- Notes for the AI, newest first (the context builder keeps the latest).
SELECT * FROM conversation_notes
WHERE conversation_id = @conversation_id AND for_ai
ORDER BY id DESC
LIMIT @lim;

-- ── Handoffs ───────────────────────────────────────────────────────────────

-- name: InsertConversationHandoff :one
INSERT INTO conversation_handoffs (conversation_id, from_type, from_user_id, to_type, to_user_id, reason, reason_detail,
    summary, summary_source, detected_need, suggested_next_action, collected_fields, actor_type, actor_id, accepted_by, accepted_at)
VALUES (@conversation_id, @from_type, sqlc.narg(from_user_id), @to_type, sqlc.narg(to_user_id), @reason, @reason_detail,
    @summary, @summary_source, @detected_need, @suggested_next_action, @collected_fields, @actor_type, sqlc.narg(actor_id),
    sqlc.narg(accepted_by), sqlc.narg(accepted_at))
RETURNING *;

-- name: AcceptConversationHandoff :one
UPDATE conversation_handoffs SET accepted_by = @accepted_by, accepted_at = now()
WHERE id = @id AND accepted_at IS NULL
RETURNING *;

-- name: ListConversationHandoffs :many
SELECT h.*, fu.name AS from_user_name, tu.name AS to_user_name, au.name AS accepted_by_name
FROM conversation_handoffs h
LEFT JOIN users fu ON fu.id = h.from_user_id
LEFT JOIN users tu ON tu.id = h.to_user_id
LEFT JOIN users au ON au.id = h.accepted_by
WHERE h.conversation_id = @conversation_id
ORDER BY h.id DESC
LIMIT @lim;

-- ── AI drafts ──────────────────────────────────────────────────────────────

-- name: InsertAIDraft :one
INSERT INTO conversation_ai_drafts (conversation_id, message_id, kind, body, confidence, intent, ai_usage_id)
VALUES (@conversation_id, sqlc.narg(message_id), @kind, @body, @confidence, @intent, sqlc.narg(ai_usage_id))
RETURNING *;

-- name: SupersedeAIDrafts :exec
-- A newer draft (or a person's own reply) replaces the pending ones.
UPDATE conversation_ai_drafts SET status = 'superseded', decided_at = now()
WHERE conversation_id = @conversation_id AND status = 'pending';

-- name: LockAIDraft :one
SELECT * FROM conversation_ai_drafts WHERE id = @id AND conversation_id = @conversation_id FOR UPDATE;

-- name: DecideAIDraft :one
UPDATE conversation_ai_drafts SET status = @status, decided_by = sqlc.narg(decided_by), decided_at = now(),
    sent_message_id = sqlc.narg(sent_message_id)
WHERE id = @id
RETURNING *;

-- name: ListAIDrafts :many
SELECT * FROM conversation_ai_drafts
WHERE conversation_id = @conversation_id AND (NOT @only_pending::bool OR status = 'pending')
ORDER BY id DESC
LIMIT @lim;

-- ── Activity ───────────────────────────────────────────────────────────────

-- name: InsertConversationActivity :one
INSERT INTO conversation_activity (conversation_id, kind, actor_type, actor_id, data)
VALUES (@conversation_id, @kind, @actor_type, sqlc.narg(actor_id), @data)
RETURNING *;

-- name: ListConversationActivity :many
-- A page of the timeline, newest first, before the cursor item (the
-- service returns it oldest first).
SELECT a.*, u.name AS actor_name
FROM conversation_activity a
LEFT JOIN users u ON u.id = a.actor_id AND a.actor_type = 'user'
WHERE a.conversation_id = @conversation_id
  AND (sqlc.narg(before_id)::uuid IS NULL OR a.id < sqlc.narg(before_id)::uuid)
ORDER BY a.id DESC
LIMIT @lim;

-- ── Automation runs of a conversation ──────────────────────────────────────

-- name: ListConversationAutomationRuns :many
SELECT r.id, r.automation_id, a.name AS automation_name, r.status, r.started_by, r.steps_executed, r.error,
       r.created_at, r.updated_at, r.finished_at,
       (SELECT count(*) FROM automation_run_steps s WHERE s.run_id = r.id)::bigint AS steps_done,
       coalesce((SELECT s.action FROM automation_run_steps s WHERE s.run_id = r.id ORDER BY s.id DESC LIMIT 1), '')::text AS last_action
FROM automation_runs r
JOIN automations a ON a.id = r.automation_id
WHERE r.conversation_id = @conversation_id
ORDER BY r.id DESC
LIMIT @lim;

-- name: GetAutomationRunSummary :one
SELECT r.id, r.automation_id, a.name AS automation_name, r.status, r.started_by, r.steps_executed, r.error,
       r.conversation_id, r.created_at, r.finished_at,
       (SELECT count(*) FROM automation_run_steps s WHERE s.run_id = r.id)::bigint AS steps_done
FROM automation_runs r
JOIN automations a ON a.id = r.automation_id
WHERE r.id = @id;
