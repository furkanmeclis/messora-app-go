-- ── Users ───────────────────────────────────────────────────────────────────

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, locale, timezone)
VALUES (@email, @name, @password_hash, @locale, @timezone)
RETURNING *;

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_id)
SELECT @user_id, r.id FROM roles r WHERE r.key = @role_key;

-- name: ReplaceRole :exec
WITH removed AS (DELETE FROM user_roles WHERE user_id = @user_id)
INSERT INTO user_roles (user_id, role_id)
SELECT @user_id, r.id FROM roles r WHERE r.key = @role_key;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = @email;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserForUpdate :one
SELECT * FROM users WHERE id = @id FOR UPDATE;

-- name: ListUsersWithRole :many
SELECT u.*, r.key AS role_key, r.name AS role_name, r.rank AS role_rank
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
ORDER BY u.created_at;

-- name: GetUserRole :one
SELECT r.key AS role_key, r.name AS role_name, r.rank AS role_rank
FROM user_roles ur JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = @user_id
ORDER BY r.rank
LIMIT 1;

-- name: CountActiveOwners :one
SELECT count(*) FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE r.key = 'owner' AND u.status = 'active';

-- name: UserPermissions :many
SELECT DISTINCT rp.permission_key
FROM user_roles ur JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE ur.user_id = @user_id
ORDER BY rp.permission_key;

-- name: RecordLoginFailure :one
-- Counts consecutive failures and locks the account for lock_minutes once
-- max_failures is reached. An expired lock starts a fresh count.
WITH cur AS (
    SELECT CASE WHEN locked_until IS NOT NULL AND locked_until <= now() THEN 1 ELSE failed_logins + 1 END AS n
    FROM users WHERE id = @id FOR UPDATE
)
UPDATE users SET
    failed_logins = cur.n,
    locked_until = CASE WHEN cur.n >= @max_failures::int THEN now() + make_interval(mins => @lock_minutes::int)
                        WHEN users.locked_until <= now() THEN NULL
                        ELSE users.locked_until END,
    updated_at = now()
FROM cur
WHERE users.id = @id
RETURNING users.failed_logins, users.locked_until;

-- name: RecordLoginSuccess :exec
UPDATE users SET failed_logins = 0, locked_until = NULL, last_login_at = now(), updated_at = now()
WHERE id = @id;

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = @password_hash, updated_at = now() WHERE id = @id;

-- name: UpdateProfile :one
UPDATE users SET
    name = coalesce(sqlc.narg(name), name),
    locale = coalesce(sqlc.narg(locale), locale),
    timezone = coalesce(sqlc.narg(timezone), timezone),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetUserStatus :exec
UPDATE users SET status = @status, updated_at = now() WHERE id = @id;

-- name: SetTotpSecret :exec
UPDATE users SET totp_secret_enc = @totp_secret_enc, totp_confirmed_at = sqlc.narg(totp_confirmed_at), updated_at = now()
WHERE id = @id;

-- ── Roles ───────────────────────────────────────────────────────────────────

-- name: ListRoles :many
SELECT * FROM roles ORDER BY rank;

-- name: ListRolePermissions :many
SELECT r.key AS role_key, rp.permission_key
FROM role_permissions rp JOIN roles r ON r.id = rp.role_id
ORDER BY r.rank, rp.permission_key;

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY module, key;

-- name: GetRoleByKey :one
SELECT * FROM roles WHERE key = @key;

-- ── Sessions ────────────────────────────────────────────────────────────────

-- name: CreateSession :one
INSERT INTO user_sessions (user_id, token_hash, ip, user_agent, reauth_at, expires_at)
VALUES (@user_id, @token_hash, sqlc.narg(ip), sqlc.narg(user_agent), now(), @expires_at)
RETURNING *;

-- name: GetActiveSessionByTokenHash :one
SELECT s.*, u.status AS user_status
FROM user_sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash AND s.revoked_at IS NULL AND s.expires_at > now();

-- name: TouchSession :exec
UPDATE user_sessions SET last_seen_at = now() WHERE id = @id;

-- name: MarkSessionReauthenticated :one
UPDATE user_sessions SET reauth_at = now() WHERE id = @id RETURNING reauth_at;

-- name: ListUserSessions :many
SELECT * FROM user_sessions
WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > now()
ORDER BY last_seen_at DESC;

-- name: RevokeSession :execrows
UPDATE user_sessions SET revoked_at = now()
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeUserSessions :execrows
UPDATE user_sessions SET revoked_at = now()
WHERE user_id = @user_id AND revoked_at IS NULL AND id <> coalesce(sqlc.narg(except_id), '00000000-0000-0000-0000-000000000000'::uuid);

-- name: DeleteExpiredSessions :execrows
DELETE FROM user_sessions WHERE expires_at < now() - interval '7 days' OR revoked_at < now() - interval '7 days';

-- ── Recovery codes ──────────────────────────────────────────────────────────

-- name: DeleteRecoveryCodes :exec
DELETE FROM user_recovery_codes WHERE user_id = @user_id;

-- name: InsertRecoveryCode :exec
INSERT INTO user_recovery_codes (user_id, code_hash) VALUES (@user_id, @code_hash);

-- name: UseRecoveryCode :execrows
UPDATE user_recovery_codes SET used_at = now()
WHERE user_id = @user_id AND code_hash = @code_hash AND used_at IS NULL;

-- ── Invitations ─────────────────────────────────────────────────────────────

-- name: CreateInvitation :one
INSERT INTO user_invitations (email, role_id, token_hash, invited_by, expires_at)
SELECT @email, r.id, @token_hash, @invited_by, @expires_at FROM roles r WHERE r.key = @role_key
RETURNING id, email, created_at, expires_at;

-- name: ExpireStaleInvitation :exec
-- Frees the address of an expired, unaccepted invitation so it can be re-invited.
UPDATE user_invitations SET revoked_at = now()
WHERE email = @email AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at <= now();

-- name: ListOpenInvitations :many
SELECT i.id, i.email, i.created_at, i.expires_at, r.key AS role_key, r.name AS role_name, u.name AS invited_by_name
FROM user_invitations i
JOIN roles r ON r.id = i.role_id
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL
ORDER BY i.created_at DESC;

-- name: GetOpenInvitationByTokenHash :one
SELECT i.id, i.email, i.expires_at, r.key AS role_key, r.name AS role_name
FROM user_invitations i JOIN roles r ON r.id = i.role_id
WHERE i.token_hash = @token_hash AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now();

-- name: AcceptInvitation :execrows
UPDATE user_invitations SET accepted_at = now()
WHERE id = @id AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeInvitation :execrows
UPDATE user_invitations SET revoked_at = now()
WHERE id = @id AND accepted_at IS NULL AND revoked_at IS NULL;

-- ── Password reset ──────────────────────────────────────────────────────────

-- name: CreatePasswordReset :exec
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (@user_id, @token_hash, @expires_at);

-- name: InvalidatePasswordResets :exec
UPDATE password_reset_tokens SET used_at = now() WHERE user_id = @user_id AND used_at IS NULL;

-- name: ConsumePasswordReset :one
UPDATE password_reset_tokens SET used_at = now()
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: CountRecentPasswordResets :one
SELECT count(*) FROM password_reset_tokens WHERE user_id = @user_id AND created_at > now() - interval '1 hour';

-- ── API keys ────────────────────────────────────────────────────────────────

-- name: CreateApiKey :one
INSERT INTO api_keys (name, prefix, key_hash, scopes, created_by, expires_at)
VALUES (@name, @prefix, @key_hash, @scopes, @created_by, sqlc.narg(expires_at))
RETURNING *;

-- name: ListApiKeys :many
SELECT k.*, u.name AS created_by_name
FROM api_keys k LEFT JOIN users u ON u.id = k.created_by
WHERE k.revoked_at IS NULL
ORDER BY k.created_at DESC;

-- name: GetActiveApiKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = @key_hash AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now());

-- name: TouchApiKey :exec
-- Updates at most once a minute to avoid write amplification under load.
UPDATE api_keys SET last_used_at = now()
WHERE id = @id AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');

-- name: RevokeApiKey :one
UPDATE api_keys SET revoked_at = now() WHERE id = @id AND revoked_at IS NULL RETURNING name;

-- ── Idempotency ─────────────────────────────────────────────────────────────

-- name: ClaimIdempotencyKey :one
-- Returns inserted=true for a new key; otherwise the stored row.
WITH ins AS (
    INSERT INTO idempotency_keys (principal, key, request_hash, status)
    VALUES (@principal, @key, @request_hash, 'in_progress')
    ON CONFLICT (tenant_id, principal, key) DO NOTHING
    RETURNING true AS inserted, request_hash, status, response_status, response_body
)
SELECT inserted, request_hash, status, response_status, response_body FROM ins
UNION ALL
SELECT false, request_hash, status, response_status, response_body
FROM idempotency_keys WHERE principal = @principal AND key = @key AND NOT EXISTS (SELECT 1 FROM ins);

-- name: CompleteIdempotencyKey :exec
UPDATE idempotency_keys SET status = 'done', response_status = @response_status, response_body = @response_body
WHERE principal = @principal AND key = @key;

-- name: ReleaseIdempotencyKey :exec
DELETE FROM idempotency_keys WHERE principal = @principal AND key = @key AND status = 'in_progress';

-- name: DeleteOldIdempotencyKeys :execrows
DELETE FROM idempotency_keys WHERE created_at < now() - interval '24 hours';

-- name: LockOwnerChanges :exec
-- Serializes changes that could remove an owner, so two concurrent
-- demotions cannot both pass the "at least one owner" check.
SELECT pg_advisory_xact_lock(hashtextextended(concat('messora.owner_changes:', app_tenant_id()), 0));
