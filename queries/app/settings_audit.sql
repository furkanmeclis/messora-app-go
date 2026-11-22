-- name: GetSetting :one
SELECT value FROM settings WHERE key = @key;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_by, updated_at)
VALUES (@key, @value, sqlc.narg(updated_by), now())
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_by = excluded.updated_by, updated_at = now();

-- name: InsertSettingIfAbsent :execrows
INSERT INTO settings (key, value) VALUES (@key, @value) ON CONFLICT (key) DO NOTHING;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE key = @key;

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (actor_type, actor_id, action, entity_type, entity_id, request_id, ip, user_agent, changes)
VALUES (@actor_type, sqlc.narg(actor_id), @action, sqlc.narg(entity_type), sqlc.narg(entity_id),
        sqlc.narg(request_id), sqlc.narg(ip), sqlc.narg(user_agent), @changes);

-- name: ListAuditLogs :many
SELECT a.*,
       coalesce(u.name, k.name, '')::text AS actor_name
FROM audit_logs a
LEFT JOIN users u ON a.actor_type = 'user' AND u.id = a.actor_id
LEFT JOIN api_keys k ON a.actor_type = 'api_key' AND k.id = a.actor_id
WHERE (sqlc.narg(before_id)::uuid IS NULL OR a.id < sqlc.narg(before_id)::uuid)
  AND (sqlc.narg(action)::text IS NULL OR a.action = sqlc.narg(action)::text)
  AND (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(entity_type)::text IS NULL OR a.entity_type = sqlc.narg(entity_type)::text)
  AND (sqlc.narg(entity_id)::uuid IS NULL OR a.entity_id = sqlc.narg(entity_id)::uuid)
ORDER BY a.id DESC
LIMIT @page_size;
