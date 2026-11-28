-- ── Accounts ────────────────────────────────────────────────────────────────

-- name: InsertWhatsAppAccount :one
INSERT INTO whatsapp_accounts (id, name, waba_id, app_id, access_token_enc, app_secret_enc, created_by)
VALUES (@id, @name, @waba_id, sqlc.narg(app_id), @access_token_enc, sqlc.narg(app_secret_enc), sqlc.narg(created_by))
RETURNING *;

-- name: GetWhatsAppAccount :one
SELECT * FROM whatsapp_accounts WHERE id = @id;

-- name: GetWhatsAppAccountByWABA :one
SELECT * FROM whatsapp_accounts WHERE waba_id = @waba_id;

-- name: ListWhatsAppAccounts :many
SELECT * FROM whatsapp_accounts ORDER BY created_at;

-- name: ListAppSecrets :many
-- The app secrets the tenant's own webhook address accepts: those of its
-- own-app accounts (Tech Provider accounts are signed by Messora's app).
SELECT id, app_secret_enc FROM whatsapp_accounts
WHERE app_secret_enc IS NOT NULL AND status <> 'disabled' AND connection_method = 'own_app';

-- name: UpdateWhatsAppAccount :one
UPDATE whatsapp_accounts SET
    name             = coalesce(sqlc.narg(name), name),
    access_token_enc = coalesce(sqlc.narg(access_token_enc), access_token_enc),
    app_secret_enc   = coalesce(sqlc.narg(app_secret_enc), app_secret_enc),
    app_id           = coalesce(sqlc.narg(app_id), app_id),
    status           = coalesce(sqlc.narg(status), status),
    last_error       = CASE WHEN sqlc.narg(status)::text = 'active' THEN NULL ELSE last_error END,
    updated_at       = now()
WHERE id = @id
RETURNING *;

-- name: SetWhatsAppTokenInfo :one
-- Records what Meta's debug_token said about the stored token.
UPDATE whatsapp_accounts SET
    token_type                   = sqlc.narg(token_type),
    token_expires_at             = sqlc.narg(token_expires_at),
    token_data_access_expires_at = sqlc.narg(token_data_access_expires_at),
    token_scopes                 = sqlc.narg(token_scopes)::text[],
    token_warning                = sqlc.narg(token_warning),
    token_checked_at             = now(),
    updated_at                   = now()
WHERE id = @id
RETURNING *;

-- name: ReactivateWhatsAppAccount :exec
-- A working token was stored: an account flagged by Meta errors is usable again.
UPDATE whatsapp_accounts SET status = 'active', last_error = NULL, token_notified_at = NULL, updated_at = now()
WHERE id = @id AND status = 'error';

-- name: MarkWhatsAppTokenNotified :exec
UPDATE whatsapp_accounts SET token_notified_at = now() WHERE id = @id;

-- name: ListWhatsAppAccountsToCheck :many
SELECT * FROM whatsapp_accounts WHERE status <> 'disabled' ORDER BY created_at;

-- name: MarkWhatsAppAccountError :exec
UPDATE whatsapp_accounts SET status = 'error', last_error = @last_error, updated_at = now()
WHERE id = @id AND status = 'active';

-- name: MarkWhatsAppAccountSynced :exec
UPDATE whatsapp_accounts SET last_synced_at = now(), updated_at = now(),
    status = CASE WHEN status = 'error' THEN 'active' ELSE status END,
    last_error = CASE WHEN status = 'error' THEN NULL ELSE last_error END
WHERE id = @id;

-- name: DeleteWhatsAppAccount :execrows
DELETE FROM whatsapp_accounts WHERE id = @id;

-- name: CountMessagesForAccount :one
SELECT count(*) FROM messages m JOIN phone_numbers p ON p.id = m.phone_number_id WHERE p.account_id = @account_id;

-- ── Phone numbers ───────────────────────────────────────────────────────────

-- name: UpsertPhoneNumber :one
-- Called with Meta's current values (connect, sync and the hourly signal
-- sync); a throughput Meta did not report keeps the stored one.
INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name, quality_rating,
                           messaging_limit, throughput_level, throughput_mps, code_verification_status, meta_status,
                           signals_checked_at)
VALUES (@account_id, @phone_number_id, @display_phone_number, @verified_name, @quality_rating,
        @messaging_limit, @throughput_level, sqlc.narg(throughput_mps), @code_verification_status, @meta_status, now())
ON CONFLICT (phone_number_id) DO UPDATE SET
    display_phone_number     = excluded.display_phone_number,
    verified_name            = excluded.verified_name,
    quality_rating           = excluded.quality_rating,
    messaging_limit          = excluded.messaging_limit,
    throughput_level         = excluded.throughput_level,
    throughput_mps           = coalesce(excluded.throughput_mps, phone_numbers.throughput_mps),
    signals_checked_at       = excluded.signals_checked_at,
    code_verification_status = excluded.code_verification_status,
    meta_status              = excluded.meta_status,
    updated_at               = now()
WHERE phone_numbers.account_id = excluded.account_id
RETURNING *;

-- name: ListPhoneNumbers :many
SELECT p.*, a.name AS account_name, a.waba_id
FROM phone_numbers p JOIN whatsapp_accounts a ON a.id = p.account_id
ORDER BY p.is_default DESC, p.created_at;

-- name: GetPhoneNumber :one
SELECT * FROM phone_numbers WHERE id = @id;

-- name: GetPhoneNumberByMetaID :one
SELECT * FROM phone_numbers WHERE phone_number_id = @phone_number_id;

-- name: GetDefaultPhoneNumber :one
SELECT * FROM phone_numbers ORDER BY is_default DESC, created_at LIMIT 1;

-- name: ClearDefaultPhoneNumber :exec
UPDATE phone_numbers SET is_default = false, updated_at = now() WHERE is_default;

-- name: SetDefaultPhoneNumber :execrows
UPDATE phone_numbers SET is_default = true, updated_at = now() WHERE id = @id;

-- ── Contacts ────────────────────────────────────────────────────────────────

-- name: UpsertContactByPhone :one
INSERT INTO contacts (phone_e164, wa_id, profile_name)
VALUES (@phone_e164, sqlc.narg(wa_id), @profile_name)
ON CONFLICT (tenant_id, phone_e164) DO UPDATE SET
    wa_id        = coalesce(excluded.wa_id, contacts.wa_id),
    profile_name = CASE WHEN excluded.profile_name <> '' THEN excluded.profile_name ELSE contacts.profile_name END,
    updated_at   = now()
RETURNING *;

-- name: TouchContactInbound :exec
UPDATE contacts SET last_inbound_at = greatest(coalesce(last_inbound_at, @at), @at), updated_at = now()
WHERE id = @id;

-- name: GetContact :one
SELECT * FROM contacts WHERE id = @id;

-- ── Messages ────────────────────────────────────────────────────────────────

-- name: InsertOutboundMessage :one
INSERT INTO messages (id, phone_number_id, contact_id, direction, type, body, payload, status, status_rank,
                      reply_to_wamid, source, created_by, consent_category, sent_by, ai_usage_id)
VALUES (@id, @phone_number_id, @contact_id, 'outbound', @type, @body, @payload, 'pending', 0,
        sqlc.narg(reply_to_wamid), @source, sqlc.narg(created_by), sqlc.narg(consent_category),
        sqlc.narg(sent_by), sqlc.narg(ai_usage_id))
RETURNING *;

-- name: InsertInboundMessage :one
-- Duplicate deliveries of the same wamid insert nothing (no row returned).
INSERT INTO messages (phone_number_id, contact_id, direction, wamid, type, body, payload, status, status_rank,
                      reply_to_wamid, source, occurred_at)
VALUES (@phone_number_id, @contact_id, 'inbound', @wamid, @type, @body, @payload, 'received', 0,
        sqlc.narg(reply_to_wamid), 'webhook', @occurred_at)
ON CONFLICT (tenant_id, wamid) DO NOTHING
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = @id;

-- name: GetMessageByWAMID :one
SELECT * FROM messages WHERE wamid = @wamid;

-- name: ClaimMessageForSend :one
-- Only a pending message, or a deferred one whose time has come, can be
-- sent. A message left in "sending" means a previous attempt died
-- mid-request; it must not be sent again.
UPDATE messages SET status = 'sending', status_rank = 1, attempts = attempts + 1, next_attempt_at = NULL, updated_at = now()
WHERE id = @id AND (status = 'pending' OR (status = 'deferred' AND next_attempt_at <= now()))
RETURNING *;

-- name: DeferMessage :exec
-- Meta withheld the message for ecosystem health (131049): it waits outside
-- any campaign window until next_attempt_at.
UPDATE messages SET status = 'deferred', status_rank = 0, error_code = sqlc.narg(error_code),
    error_title = sqlc.narg(error_title),
    next_attempt_at = now() + make_interval(secs => @after_seconds::double precision), updated_at = now()
WHERE id = @id AND status = 'sending';

-- name: MarkMessageSkipped :exec
-- Consent blocked the message right before it was sent.
UPDATE messages SET status = 'skipped', status_rank = 6, failure_class = 'blocked_by_consent',
    error_code = @error_code, error_title = sqlc.narg(error_title), next_attempt_at = NULL, updated_at = now()
WHERE id = @id AND status IN ('pending', 'sending', 'deferred');

-- name: MarkMessageAccepted :exec
UPDATE messages SET wamid = coalesce(wamid, @wamid),
    status = CASE WHEN status_rank < 3 THEN 'sent' ELSE status END,
    status_rank = greatest(status_rank, 3),
    sent_at = coalesce(sent_at, now()), updated_at = now()
WHERE id = @id;

-- name: ReleaseMessageForRetry :exec
UPDATE messages SET status = 'pending', status_rank = 0, error_code = sqlc.narg(error_code),
    error_title = sqlc.narg(error_title), updated_at = now()
WHERE id = @id AND status = 'sending';

-- name: MarkMessageUnknown :exec
UPDATE messages SET status = 'unknown', status_rank = 2, error_title = sqlc.narg(error_title), updated_at = now()
WHERE id = @id AND status IN ('pending', 'sending');

-- name: MarkMessageFailed :exec
UPDATE messages SET status = 'failed', status_rank = 6, error_code = sqlc.narg(error_code),
    error_title = sqlc.narg(error_title), failure_class = sqlc.narg(failure_class),
    failed_at = coalesce(failed_at, now()), updated_at = now()
WHERE id = @id AND status IN ('pending', 'sending', 'unknown');

-- name: ApplyMessageStatus :one
-- Monotonic status transition from a webhook; see the messages table comment.
UPDATE messages SET
    wamid        = coalesce(wamid, @wamid),
    status       = CASE WHEN @rank::smallint > status_rank AND NOT (@status::text = 'failed' AND status_rank >= 4)
                        THEN @status::text ELSE status END,
    status_rank  = CASE WHEN @rank::smallint > status_rank AND NOT (@status::text = 'failed' AND status_rank >= 4)
                        THEN @rank::smallint ELSE status_rank END,
    sent_at      = CASE WHEN @status::text IN ('sent', 'delivered', 'read') THEN coalesce(sent_at, @at) ELSE sent_at END,
    delivered_at = CASE WHEN @status::text IN ('delivered', 'read') THEN coalesce(delivered_at, @at) ELSE delivered_at END,
    read_at      = CASE WHEN @status::text = 'read' THEN coalesce(read_at, @at) ELSE read_at END,
    failed_at    = CASE WHEN @status::text = 'failed' AND status_rank < 4 THEN coalesce(failed_at, @at) ELSE failed_at END,
    error_code   = CASE WHEN @status::text = 'failed' AND status_rank < 4 THEN sqlc.narg(error_code) ELSE error_code END,
    error_title  = CASE WHEN @status::text = 'failed' AND status_rank < 4 THEN sqlc.narg(error_title) ELSE error_title END,
    failure_class = CASE WHEN @status::text = 'failed' AND status_rank < 4 THEN sqlc.narg(failure_class) ELSE failure_class END,
    updated_at   = now()
WHERE id = @id AND direction = 'outbound'
RETURNING *;

-- name: ListMessagesByContact :many
SELECT * FROM messages
WHERE contact_id = @contact_id AND (sqlc.narg(before_id)::uuid IS NULL OR id < sqlc.narg(before_id)::uuid)
ORDER BY id DESC
LIMIT @lim;

-- ── Media ───────────────────────────────────────────────────────────────────

-- name: InsertMediaFile :one
INSERT INTO media_files (message_id, meta_media_id, mime_type, filename, sha256)
VALUES (sqlc.narg(message_id), sqlc.narg(meta_media_id), @mime_type, @filename, @sha256)
RETURNING *;

-- name: GetMediaFile :one
SELECT * FROM media_files WHERE id = @id;

-- name: MarkMediaStored :exec
UPDATE media_files SET status = 'stored', storage_key = @storage_key, size_bytes = @size_bytes,
    last_error = NULL, updated_at = now()
WHERE id = @id;

-- name: MarkMediaFailed :exec
UPDATE media_files SET status = 'failed', last_error = @last_error, updated_at = now() WHERE id = @id;

-- name: ListMediaForMessage :many
SELECT * FROM media_files WHERE message_id = @message_id ORDER BY created_at;

-- ── Webhook events ──────────────────────────────────────────────────────────

-- name: InsertWebhookEvent :one
-- Returns no row when the identical body was already received.
INSERT INTO webhook_events (payload_sha256, body) VALUES (@payload_sha256, @body)
ON CONFLICT (tenant_id, payload_sha256) DO NOTHING
RETURNING id;

-- name: GetWebhookEvent :one
SELECT * FROM webhook_events WHERE id = @id;

-- name: MarkWebhookProcessed :exec
UPDATE webhook_events SET status = 'processed', processed_at = now(), attempts = attempts + 1, last_error = NULL
WHERE id = @id;

-- name: MarkWebhookAttempt :exec
UPDATE webhook_events SET attempts = attempts + 1, last_error = @last_error,
    status = CASE WHEN @final::boolean THEN 'failed' ELSE status END
WHERE id = @id;

-- name: DeleteOldWebhookEvents :execrows
DELETE FROM webhook_events WHERE status = 'processed' AND received_at < @before;

-- ── Templates ───────────────────────────────────────────────────────────────

-- name: UpsertTemplate :one
INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, parameter_format,
                               components, rejected_reason, quality_score, synced_at)
VALUES (@account_id, @meta_template_id, @name, @language, @category, @status, @parameter_format,
        @components, @rejected_reason, @quality_score, now())
ON CONFLICT (account_id, name, language) DO UPDATE SET
    meta_template_id = excluded.meta_template_id,
    category         = excluded.category,
    status           = excluded.status,
    parameter_format = excluded.parameter_format,
    components       = excluded.components,
    rejected_reason  = excluded.rejected_reason,
    quality_score    = excluded.quality_score,
    synced_at        = now(),
    updated_at       = now()
RETURNING *;

-- name: DeleteTemplatesNotSynced :execrows
DELETE FROM message_templates WHERE account_id = @account_id AND synced_at < @since;

-- name: ListTemplates :many
SELECT t.*, a.name AS account_name FROM message_templates t JOIN whatsapp_accounts a ON a.id = t.account_id
WHERE (sqlc.narg(account_id)::uuid IS NULL OR t.account_id = sqlc.narg(account_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text)
ORDER BY t.name, t.language;

-- name: GetTemplate :one
SELECT * FROM message_templates WHERE id = @id;

-- name: FindApprovedTemplate :one
SELECT * FROM message_templates
WHERE account_id = @account_id AND name = @name AND language = @language;

-- name: UpdateTemplateStatusByMetaID :execrows
UPDATE message_templates SET status = @status,
    rejected_reason = CASE WHEN @status::text = 'REJECTED' THEN @reason ELSE rejected_reason END,
    updated_at = now()
WHERE meta_template_id = @meta_template_id;

-- name: DeleteTemplate :execrows
DELETE FROM message_templates WHERE id = @id;

-- name: SetMessageMetaStatus :exec
-- Meta's own state for paced templates (accepted, held_for_quality_assessment, paused).
UPDATE messages SET meta_status = @meta_status, updated_at = now() WHERE id = @id;

-- name: ContactHasMessages :one
SELECT EXISTS (SELECT 1 FROM messages WHERE contact_id = @contact_id);

-- name: CountCampaignsForAccount :one
SELECT count(*) FROM campaigns c JOIN phone_numbers p ON p.id = c.phone_number_id WHERE p.account_id = @account_id;
