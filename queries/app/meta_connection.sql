-- ── Meta connection (Phase 17 WP5) ─────────────────────────────────────────

-- name: GetMetaConnection :one
SELECT * FROM meta_connections;

-- name: InsertMetaConnection :one
-- Returns no row when another request created the tenant's row first.
INSERT INTO meta_connections (webhook_key_enc, verify_token_enc)
VALUES (@webhook_key_enc, @verify_token_enc)
ON CONFLICT (tenant_id) DO NOTHING
RETURNING *;

-- name: SetMetaWebhookKey :one
UPDATE meta_connections SET webhook_key_enc = @webhook_key_enc, webhook_key_rotated_at = now(),
    previous_key_expires_at = sqlc.narg(previous_key_expires_at), updated_at = now()
RETURNING *;

-- name: SetMetaVerifyToken :one
UPDATE meta_connections SET verify_token_enc = @verify_token_enc, verify_token_rotated_at = now(), updated_at = now()
RETURNING *;

-- name: TouchMetaWebhook :exec
-- At most one write a minute however many webhooks arrive.
UPDATE meta_connections SET last_webhook_at = now()
WHERE last_webhook_at IS NULL OR last_webhook_at < now() - interval '1 minute';

-- name: RecordMetaSignatureFailure :exec
UPDATE meta_connections SET signature_failures = signature_failures + 1, last_signature_failure_at = now();

-- name: RecordMetaWABAMismatch :exec
UPDATE meta_connections SET waba_mismatches = waba_mismatches + 1, last_waba_mismatch_at = now(),
    last_mismatch_waba_id = @waba_id;

-- name: ListWABAIDs :many
SELECT waba_id FROM whatsapp_accounts;
