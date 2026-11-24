-- ── Conversations (inbox) ───────────────────────────────────────────────────

-- name: LockConversationByThread :one
SELECT * FROM conversations WHERE contact_id = @contact_id AND phone_number_id = @phone_number_id FOR UPDATE;

-- name: GetConversationByThread :one
SELECT * FROM conversations WHERE contact_id = @contact_id AND phone_number_id = @phone_number_id;

-- name: TouchConversation :one
-- Records a new message in its thread. A customer message (inbound) counts
-- as unread and reopens a closed conversation; opens_window extends the
-- 24-hour customer service window. Late (out of order) messages never move
-- the recency backwards.
INSERT INTO conversations (contact_id, phone_number_id, last_message_id, last_message_at, last_inbound_at, unread_count)
VALUES (@contact_id, @phone_number_id, @message_id, @at,
        CASE WHEN @opens_window::bool THEN @at::timestamptz END,
        CASE WHEN @inbound::bool THEN 1 ELSE 0 END)
ON CONFLICT (contact_id, phone_number_id) DO UPDATE SET
    last_message_id = CASE WHEN excluded.last_message_at >= conversations.last_message_at
                           THEN excluded.last_message_id ELSE conversations.last_message_id END,
    last_message_at = greatest(conversations.last_message_at, excluded.last_message_at),
    last_inbound_at = CASE WHEN excluded.last_inbound_at IS NULL THEN conversations.last_inbound_at
                           ELSE greatest(conversations.last_inbound_at, excluded.last_inbound_at) END,
    unread_count    = conversations.unread_count + excluded.unread_count,
    status          = CASE WHEN @inbound::bool THEN 'open' ELSE conversations.status END,
    closed_at       = CASE WHEN @inbound::bool THEN NULL ELSE conversations.closed_at END,
    updated_at      = now()
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations WHERE id = @id;

-- name: GetConversationRow :one
SELECT c.*, ct.phone_e164, ct.name AS contact_name, ct.profile_name, ct.opted_out_at,
       pn.display_phone_number, pn.verified_name, pn.account_id AS phone_account_id,
       m.direction AS last_direction, m.type AS last_type, m.body AS last_body, m.status AS last_status,
       u.name AS assignee_name
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
JOIN phone_numbers pn ON pn.id = c.phone_number_id
LEFT JOIN messages m ON m.id = c.last_message_id
LEFT JOIN users u ON u.id = c.assignee_id
WHERE c.id = @id;

-- name: ListConversations :many
-- see_all is false for users who may only read conversations assigned to
-- them or to nobody (conversations.read without conversations.read_all).
SELECT c.*, ct.phone_e164, ct.name AS contact_name, ct.profile_name, ct.opted_out_at,
       pn.display_phone_number, pn.verified_name, pn.account_id AS phone_account_id,
       m.direction AS last_direction, m.type AS last_type, m.body AS last_body, m.status AS last_status,
       u.name AS assignee_name
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
JOIN phone_numbers pn ON pn.id = c.phone_number_id
LEFT JOIN messages m ON m.id = c.last_message_id
LEFT JOIN users u ON u.id = c.assignee_id
WHERE (@see_all::bool OR c.assignee_id IS NULL OR c.assignee_id = @viewer_id::uuid)
  AND (sqlc.narg(status)::text IS NULL OR c.status = sqlc.narg(status)::text)
  AND (NOT @only_unassigned::bool OR c.assignee_id IS NULL)
  AND (sqlc.narg(assignee_id)::uuid IS NULL OR c.assignee_id = sqlc.narg(assignee_id)::uuid)
  AND (sqlc.narg(phone_number_id)::uuid IS NULL OR c.phone_number_id = sqlc.narg(phone_number_id)::uuid)
  AND (NOT @only_unread::bool OR c.unread_count > 0)
  AND (sqlc.narg(q)::text IS NULL OR ct.name ILIKE sqlc.narg(q)::text OR ct.profile_name ILIKE sqlc.narg(q)::text
       OR ct.phone_e164 LIKE sqlc.narg(q)::text)
  AND (sqlc.narg(before_at)::timestamptz IS NULL
       OR (c.last_message_at, c.id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY c.last_message_at DESC, c.id DESC
LIMIT @lim;

-- name: SetConversationAssignee :one
UPDATE conversations SET assignee_id = sqlc.narg(assignee_id), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetConversationStatus :one
UPDATE conversations SET status = @status,
    closed_at = CASE WHEN @status::text = 'closed' THEN now() END,
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: MarkConversationRead :one
UPDATE conversations SET unread_count = 0, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: InsertConversationEvent :one
INSERT INTO conversation_events (conversation_id, kind, actor_type, actor_id, assignee_id)
VALUES (@conversation_id, @kind, @actor_type, sqlc.narg(actor_id), sqlc.narg(assignee_id))
RETURNING *;

-- name: ListConversationEvents :many
SELECT e.*, actor.name AS actor_name, assignee.name AS assignee_name
FROM conversation_events e
LEFT JOIN users actor ON actor.id = e.actor_id AND e.actor_type = 'user'
LEFT JOIN users assignee ON assignee.id = e.assignee_id
WHERE e.conversation_id = @conversation_id
ORDER BY e.id DESC
LIMIT @lim;

-- name: ListThreadMessages :many
-- A conversation's messages, newest first, before the cursor message.
SELECT * FROM messages
WHERE contact_id = @contact_id AND phone_number_id = @phone_number_id
  AND (sqlc.narg(before_id)::uuid IS NULL OR id < sqlc.narg(before_id)::uuid)
ORDER BY id DESC
LIMIT @lim;

-- name: ListMediaForMessages :many
SELECT * FROM media_files WHERE message_id = ANY(@message_ids::uuid[]) ORDER BY id;

-- name: GetAssignableUser :one
-- Conversations can only be assigned to active users who can read them.
SELECT u.id, u.name FROM users u
WHERE u.id = @id AND u.status = 'active' AND EXISTS (
    SELECT 1 FROM user_roles ur JOIN role_permissions rp ON rp.role_id = ur.role_id
    WHERE ur.user_id = u.id AND rp.permission_key IN ('conversations.read', 'conversations.read_all'));
