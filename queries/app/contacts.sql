-- Contact listing with filters and segments is built in Go (contacts/list.go)
-- because segment conditions are dynamic; everything else lives here.

-- name: InsertContact :one
INSERT INTO contacts (phone_e164, name, email, var1, var2, var3, var4, var5, attributes, source)
VALUES (@phone_e164, @name, sqlc.narg(email), sqlc.narg(var1), sqlc.narg(var2), sqlc.narg(var3), sqlc.narg(var4),
        sqlc.narg(var5), @attributes, @source)
RETURNING *;

-- name: UpdateContact :one
UPDATE contacts SET
    name       = coalesce(sqlc.narg(name), name),
    email      = CASE WHEN @set_email::boolean THEN sqlc.narg(email) ELSE email END,
    var1       = CASE WHEN @set_var1::boolean THEN sqlc.narg(var1) ELSE var1 END,
    var2       = CASE WHEN @set_var2::boolean THEN sqlc.narg(var2) ELSE var2 END,
    var3       = CASE WHEN @set_var3::boolean THEN sqlc.narg(var3) ELSE var3 END,
    var4       = CASE WHEN @set_var4::boolean THEN sqlc.narg(var4) ELSE var4 END,
    var5       = CASE WHEN @set_var5::boolean THEN sqlc.narg(var5) ELSE var5 END,
    attributes = coalesce(sqlc.narg(attributes), attributes),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DeleteContact :execrows
DELETE FROM contacts WHERE id = @id;

-- name: GetContactByPhone :one
SELECT * FROM contacts WHERE phone_e164 = @phone_e164;

-- name: LockContact :one
SELECT * FROM contacts WHERE id = @id FOR UPDATE;

-- name: SetContactConsent :one
-- Sets the consent status of the chosen categories and keeps the legacy
-- opted_out_at flag equal to "both categories opted out".
UPDATE contacts SET
    marketing_consent        = CASE WHEN @marketing::boolean THEN @status::text ELSE marketing_consent END,
    marketing_consent_at     = CASE WHEN @marketing::boolean THEN now() ELSE marketing_consent_at END,
    marketing_consent_source = CASE WHEN @marketing::boolean THEN @source::text ELSE marketing_consent_source END,
    marketing_consent_reason = CASE WHEN @marketing::boolean THEN sqlc.narg(reason)::text ELSE marketing_consent_reason END,
    service_consent          = CASE WHEN @service::boolean THEN @status::text ELSE service_consent END,
    service_consent_at       = CASE WHEN @service::boolean THEN now() ELSE service_consent_at END,
    service_consent_source   = CASE WHEN @service::boolean THEN @source::text ELSE service_consent_source END,
    service_consent_reason   = CASE WHEN @service::boolean THEN sqlc.narg(reason)::text ELSE service_consent_reason END,
    opted_out_at             = CASE WHEN (CASE WHEN @marketing::boolean THEN @status::text ELSE marketing_consent END) = 'opted_out'
                                     AND (CASE WHEN @service::boolean THEN @status::text ELSE service_consent END) = 'opted_out'
                                    THEN coalesce(opted_out_at, now()) ELSE NULL END,
    opt_out_reason           = CASE WHEN (CASE WHEN @marketing::boolean THEN @status::text ELSE marketing_consent END) = 'opted_out'
                                     AND (CASE WHEN @service::boolean THEN @status::text ELSE service_consent END) = 'opted_out'
                                    THEN coalesce(sqlc.narg(reason)::text, opt_out_reason) ELSE NULL END,
    updated_at               = now()
WHERE id = @id
RETURNING *;

-- name: SetMetaMarketingStopped :execrows
-- Only an actual change touches the row (and returns 1).
UPDATE contacts SET
    meta_marketing_stopped_at = CASE WHEN @stopped::boolean THEN @at::timestamptz ELSE NULL END,
    updated_at = now()
WHERE id = @id AND (meta_marketing_stopped_at IS NULL) = @stopped::boolean;

-- name: SetContactUndeliverable :execrows
UPDATE contacts SET undeliverable_at = now(), undeliverable_code = @code, updated_at = now()
WHERE id = @id AND undeliverable_at IS NULL;

-- name: ClearContactUndeliverable :execrows
UPDATE contacts SET undeliverable_at = NULL, undeliverable_code = NULL, updated_at = now()
WHERE id = @id AND undeliverable_at IS NOT NULL;

-- name: InsertConsentEvent :one
INSERT INTO consent_events (contact_id, kind, source, note, actor_id, category, status)
VALUES (@contact_id, @kind, @source, @note, sqlc.narg(actor_id), sqlc.narg(category), sqlc.narg(status))
RETURNING *;

-- name: ListConsentEvents :many
SELECT e.*, coalesce(u.name, k.name, '')::text AS actor_name
FROM consent_events e
LEFT JOIN users u ON u.id = e.actor_id
LEFT JOIN api_keys k ON k.id = e.actor_id
WHERE e.contact_id = @contact_id ORDER BY e.id DESC LIMIT 100;

-- ── Tags ────────────────────────────────────────────────────────────────────

-- name: ListTags :many
SELECT t.*, (SELECT count(*) FROM contact_tags ct WHERE ct.tag_id = t.id) AS contact_count
FROM tags t ORDER BY lower(t.name);

-- name: UpsertTag :one
-- Tags are matched case-insensitively; the first spelling wins.
INSERT INTO tags (name, color) VALUES (@name, @color)
ON CONFLICT (tenant_id, lower(name)) DO UPDATE SET name = tags.name
RETURNING *;

-- name: UpdateTag :one
UPDATE tags SET name = coalesce(sqlc.narg(name), name), color = coalesce(sqlc.narg(color), color)
WHERE id = @id RETURNING *;

-- name: DeleteTag :execrows
DELETE FROM tags WHERE id = @id;

-- name: ContactTags :many
SELECT t.* FROM tags t JOIN contact_tags ct ON ct.tag_id = t.id WHERE ct.contact_id = @contact_id ORDER BY lower(t.name);

-- name: TagsForContacts :many
SELECT ct.contact_id, t.id, t.name, t.color
FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id
WHERE ct.contact_id = ANY(@contact_ids::uuid[])
ORDER BY lower(t.name);

-- name: AddContactTag :exec
INSERT INTO contact_tags (contact_id, tag_id) VALUES (@contact_id, @tag_id) ON CONFLICT DO NOTHING;

-- name: ClearContactTags :exec
DELETE FROM contact_tags WHERE contact_id = @contact_id;

-- ── Groups ──────────────────────────────────────────────────────────────────

-- name: ListContactGroups :many
SELECT g.*, (SELECT count(*) FROM contact_group_members m WHERE m.group_id = g.id) AS member_count
FROM contact_groups g ORDER BY lower(g.name);

-- name: GetContactGroup :one
SELECT * FROM contact_groups WHERE id = @id;

-- name: InsertContactGroup :one
INSERT INTO contact_groups (name, description, created_by) VALUES (@name, @description, sqlc.narg(created_by))
RETURNING *;

-- name: UpdateContactGroup :one
UPDATE contact_groups SET name = coalesce(sqlc.narg(name), name),
    description = coalesce(sqlc.narg(description), description), updated_at = now()
WHERE id = @id RETURNING *;

-- name: DeleteContactGroup :execrows
DELETE FROM contact_groups WHERE id = @id;

-- name: AddGroupMembers :execrows
INSERT INTO contact_group_members (group_id, contact_id)
SELECT @group_id, c.id FROM contacts c WHERE c.id = ANY(@contact_ids::uuid[])
ON CONFLICT DO NOTHING;

-- name: RemoveGroupMembers :execrows
DELETE FROM contact_group_members WHERE group_id = @group_id AND contact_id = ANY(@contact_ids::uuid[]);

-- name: ContactGroupIDs :many
SELECT group_id FROM contact_group_members WHERE contact_id = @contact_id;

-- ── Notes ───────────────────────────────────────────────────────────────────

-- name: InsertContactNote :one
INSERT INTO contact_notes (contact_id, author_id, body) VALUES (@contact_id, sqlc.narg(author_id), @body)
RETURNING *;

-- name: ListContactNotes :many
SELECT n.*, coalesce(u.name, '')::text AS author_name
FROM contact_notes n LEFT JOIN users u ON u.id = n.author_id
WHERE n.contact_id = @contact_id ORDER BY n.id DESC LIMIT 200;

-- name: DeleteContactNote :execrows
DELETE FROM contact_notes WHERE id = @id AND contact_id = @contact_id;

-- ── Segments ────────────────────────────────────────────────────────────────

-- name: ListSegments :many
SELECT * FROM segments ORDER BY lower(name);

-- name: GetSegment :one
SELECT * FROM segments WHERE id = @id;

-- name: InsertSegment :one
INSERT INTO segments (name, description, definition, created_by)
VALUES (@name, @description, @definition, sqlc.narg(created_by))
RETURNING *;

-- name: UpdateSegment :one
UPDATE segments SET name = coalesce(sqlc.narg(name), name),
    description = coalesce(sqlc.narg(description), description),
    definition = coalesce(sqlc.narg(definition), definition), updated_at = now()
WHERE id = @id RETURNING *;

-- name: DeleteSegment :execrows
DELETE FROM segments WHERE id = @id;

-- ── Imports ─────────────────────────────────────────────────────────────────

-- name: InsertContactImport :one
INSERT INTO contact_imports (filename, storage_key, size_bytes, headers, preview, total_rows, created_by)
VALUES (@filename, @storage_key, @size_bytes, @headers, @preview, @total_rows, sqlc.narg(created_by))
RETURNING *;

-- name: GetContactImport :one
SELECT * FROM contact_imports WHERE id = @id;

-- name: ListContactImports :many
SELECT * FROM contact_imports ORDER BY id DESC LIMIT 50;

-- name: QueueContactImport :one
UPDATE contact_imports SET status = 'queued', mapping = @mapping, duplicate_policy = @duplicate_policy,
    add_tags = @add_tags, group_id = sqlc.narg(group_id), updated_at = now()
WHERE id = @id AND status = 'uploaded'
RETURNING *;

-- name: StartContactImport :one
UPDATE contact_imports SET status = 'running', started_at = coalesce(started_at, now()), updated_at = now()
WHERE id = @id AND status IN ('queued', 'running') AND NOT cancel_requested
RETURNING *;

-- name: ProgressContactImport :one
-- Counters are absolute (the worker keeps the totals), so a retried batch
-- cannot double-count. Returns whether a cancel was requested.
UPDATE contact_imports SET processed_rows = @processed_rows, created_count = @created_count,
    updated_count = @updated_count, skipped_count = @skipped_count, failed_count = @failed_count, updated_at = now()
WHERE id = @id
RETURNING cancel_requested;

-- name: FinishContactImport :exec
UPDATE contact_imports SET status = @status, error_report_key = sqlc.narg(error_report_key),
    last_error = sqlc.narg(last_error), finished_at = now(), updated_at = now()
WHERE id = @id;

-- name: RequestImportCancel :one
UPDATE contact_imports SET cancel_requested = true,
    status = CASE WHEN status IN ('uploaded', 'queued') THEN 'cancelled' ELSE status END,
    finished_at = CASE WHEN status IN ('uploaded', 'queued') THEN now() ELSE finished_at END,
    updated_at = now()
WHERE id = @id AND status IN ('uploaded', 'queued', 'running')
RETURNING *;

-- name: GetTag :one
SELECT * FROM tags WHERE id = @id;

-- name: ListContactAttributeKeys :many
SELECT k::text AS key
FROM contacts, jsonb_object_keys(attributes) AS k
WHERE jsonb_typeof(attributes) = 'object'
GROUP BY k
ORDER BY count(*) DESC, k
LIMIT 200;
