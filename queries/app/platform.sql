-- Platform queries (Phase 17 WP2). Unless noted, they run as
-- messora_platform (tenant.Platform context): the tenant registry, global
-- accounts, memberships of every tenant and the admin panel's own tables.

-- ── Tenants ─────────────────────────────────────────────────────────────────

-- name: CreateTenant :one
INSERT INTO tenants (slug, name, subscription_status, trial_ends_at, plan_key, plan_changed_at)
VALUES (@slug, @name, @subscription_status, sqlc.narg(trial_ends_at), sqlc.narg(plan_key), now())
RETURNING *;

-- name: GetTenant :one
SELECT * FROM tenants WHERE id = @id;

-- name: GetTenantForUpdate :one
SELECT * FROM tenants WHERE id = @id FOR UPDATE;

-- name: SlugInUse :one
-- Deleted tenants keep their slug: an address is never handed to someone else.
SELECT EXISTS (SELECT 1 FROM tenants WHERE slug = @slug
               UNION ALL SELECT 1 FROM tenant_domains WHERE hostname = @hostname);

-- name: ListTenants :many
SELECT t.*,
       (SELECT count(*) FROM tenant_memberships m WHERE m.tenant_id = t.id AND m.status = 'active') AS member_count,
       coalesce((SELECT d.hostname FROM tenant_domains d WHERE d.tenant_id = t.id AND d.is_primary), '')::text AS primary_host
FROM tenants t
WHERE (sqlc.narg(query)::text IS NULL
       OR t.slug ILIKE '%' || sqlc.narg(query)::text || '%'
       OR t.name ILIKE '%' || sqlc.narg(query)::text || '%'
       OR EXISTS (SELECT 1 FROM tenant_domains d WHERE d.tenant_id = t.id AND d.hostname ILIKE '%' || sqlc.narg(query)::text || '%'))
  AND (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text)
  AND (sqlc.narg(after)::uuid IS NULL OR t.id < sqlc.narg(after)::uuid)
ORDER BY t.id DESC
LIMIT @lim;

-- name: CountTenants :one
SELECT count(*) FROM tenants t
WHERE (sqlc.narg(query)::text IS NULL
       OR t.slug ILIKE '%' || sqlc.narg(query)::text || '%'
       OR t.name ILIKE '%' || sqlc.narg(query)::text || '%'
       OR EXISTS (SELECT 1 FROM tenant_domains d WHERE d.tenant_id = t.id AND d.hostname ILIKE '%' || sqlc.narg(query)::text || '%'))
  AND (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text);

-- name: UpdateTenant :one
UPDATE tenants SET
    name = coalesce(sqlc.narg(name), name),
    trial_ends_at = CASE WHEN @set_trial::bool THEN sqlc.narg(trial_ends_at) ELSE trial_ends_at END,
    subscription_status = coalesce(sqlc.narg(subscription_status), subscription_status),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SuspendTenant :one
UPDATE tenants SET status = 'suspended', suspended_at = now(), suspended_reason = @reason, updated_at = now()
WHERE id = @id AND status = 'active'
RETURNING *;

-- name: UnsuspendTenant :one
UPDATE tenants SET status = 'active', suspended_at = NULL, suspended_reason = NULL, updated_at = now()
WHERE id = @id AND status = 'suspended'
RETURNING *;

-- name: CountTenantMembers :one
SELECT count(*) FROM tenant_memberships WHERE tenant_id = @tenant_id AND status = 'active';

-- name: PlatformCounts :one
SELECT
    (SELECT count(*) FROM tenants WHERE status <> 'deleted') AS tenants,
    (SELECT count(*) FROM tenants WHERE status = 'active') AS active_tenants,
    (SELECT count(*) FROM tenants WHERE status = 'suspended') AS suspended_tenants,
    (SELECT count(*) FROM tenants WHERE status = 'active' AND subscription_status = 'trial') AS trial_tenants,
    (SELECT count(*) FROM users) AS users,
    (SELECT count(*) FROM tenants WHERE created_at > now() - interval '7 days') AS signups_7d,
    (SELECT count(*) FROM support_access_grants WHERE ended_at IS NULL AND expires_at > now()) AS active_support_sessions;

-- name: TenantOwners :many
SELECT u.id, u.email, u.name
FROM tenant_memberships m JOIN users u ON u.id = m.user_id JOIN roles r ON r.id = m.role_id
WHERE m.tenant_id = @tenant_id AND r.key = 'owner' AND m.status = 'active'
ORDER BY m.created_at;

-- ── Domains ─────────────────────────────────────────────────────────────────

-- name: CreateTenantDomain :one
INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary, verification_token)
VALUES (@tenant_id, @hostname, @kind, sqlc.narg(verified_at), @is_primary, sqlc.narg(verification_token))
RETURNING *;

-- name: ListTenantDomains :many
SELECT * FROM tenant_domains WHERE tenant_id = @tenant_id ORDER BY is_primary DESC, created_at;

-- name: GetTenantDomain :one
SELECT * FROM tenant_domains WHERE id = @id;

-- name: PrimaryHost :one
-- The address a tenant's links point at: its primary domain, or its
-- oldest verified one.
SELECT hostname FROM tenant_domains
WHERE tenant_id = @tenant_id AND verified_at IS NOT NULL
ORDER BY is_primary DESC, created_at
LIMIT 1;

-- name: RecordDomainCheck :one
UPDATE tenant_domains SET
    last_checked_at = now(),
    last_check_error = sqlc.narg(error),
    verified_at = CASE WHEN @verified::bool THEN coalesce(verified_at, now()) ELSE verified_at END
WHERE id = @id
RETURNING *;

-- name: DeleteTenantDomain :execrows
DELETE FROM tenant_domains WHERE id = @id AND kind = 'custom';

-- name: SetPrimaryDomain :exec
UPDATE tenant_domains SET is_primary = (id = @id) WHERE tenant_id = @tenant_id;

-- ── Accounts ────────────────────────────────────────────────────────────────

-- name: SearchUsers :many
SELECT u.*,
       (SELECT count(*) FROM tenant_memberships m WHERE m.user_id = u.id) AS membership_count
FROM users u
WHERE (sqlc.narg(query)::text IS NULL
       OR u.email ILIKE '%' || sqlc.narg(query)::text || '%'
       OR u.name ILIKE '%' || sqlc.narg(query)::text || '%')
  AND (sqlc.narg(after)::uuid IS NULL OR u.id < sqlc.narg(after)::uuid)
ORDER BY u.id DESC
LIMIT @lim;

-- name: ListMembershipsOfUser :many
-- Every company the person belongs to, with the address of each.
SELECT t.id AS tenant_id, t.slug, t.name AS tenant_name, t.status AS tenant_status,
       m.status, m.created_at, r.key AS role_key, r.name AS role_name,
       coalesce((SELECT d.hostname FROM tenant_domains d
                 WHERE d.tenant_id = t.id AND d.verified_at IS NOT NULL
                 ORDER BY d.is_primary DESC, d.created_at LIMIT 1), '')::text AS host
FROM tenant_memberships m
JOIN tenants t ON t.id = m.tenant_id
JOIN roles r ON r.id = m.role_id
WHERE m.user_id = @user_id AND t.status <> 'deleted'
ORDER BY t.name, t.id;

-- name: GetMembershipIn :one
SELECT m.status, r.key AS role_key, t.status AS tenant_status
FROM tenant_memberships m JOIN roles r ON r.id = m.role_id JOIN tenants t ON t.id = m.tenant_id
WHERE m.tenant_id = @tenant_id AND m.user_id = @user_id;

-- name: SetUserStatus :exec
UPDATE users SET status = @status, updated_at = now() WHERE id = @id;

-- name: RequirePasswordReset :exec
UPDATE users SET password_reset_required = true, updated_at = now() WHERE id = @id;

-- name: SetPlatformAdmin :exec
UPDATE users SET is_platform_admin = @is_platform_admin, updated_at = now() WHERE id = @id;

-- name: CountPlatformAdmins :one
SELECT count(*) FROM users WHERE is_platform_admin AND status = 'active';

-- name: RevokeUserSessionsEverywhere :execrows
-- Every tenant's sessions of the user (platform role policy).
UPDATE user_sessions SET revoked_at = now()
WHERE user_id = @user_id AND revoked_at IS NULL AND id <> coalesce(sqlc.narg(except_id), '00000000-0000-0000-0000-000000000000'::uuid);

-- name: RevokePlatformSessionsOf :execrows
UPDATE platform_sessions SET revoked_at = now()
WHERE user_id = @user_id AND revoked_at IS NULL AND id <> coalesce(sqlc.narg(except_id), '00000000-0000-0000-0000-000000000000'::uuid);

-- ── E-mail verification ─────────────────────────────────────────────────────

-- name: CreateEmailVerification :exec
INSERT INTO email_verification_tokens (user_id, tenant_id, token_hash, expires_at)
VALUES (@user_id, @tenant_id, @token_hash, @expires_at);

-- name: GetEmailVerification :one
SELECT v.*, u.email, u.email_verified_at, u.name AS user_name, u.locale, t.slug, t.name AS tenant_name
FROM email_verification_tokens v
JOIN users u ON u.id = v.user_id
JOIN tenants t ON t.id = v.tenant_id
WHERE v.token_hash = @token_hash
FOR UPDATE OF v;

-- name: UseEmailVerification :exec
UPDATE email_verification_tokens SET used_at = now() WHERE user_id = @user_id AND used_at IS NULL;

-- name: LatestVerificationTenant :one
SELECT tenant_id FROM email_verification_tokens WHERE user_id = @user_id ORDER BY created_at DESC LIMIT 1;

-- ── Platform sessions ───────────────────────────────────────────────────────

-- name: CreatePlatformSession :one
INSERT INTO platform_sessions (user_id, token_hash, ip, user_agent, reauth_at, expires_at)
VALUES (@user_id, @token_hash, sqlc.narg(ip), sqlc.narg(user_agent), now(), @expires_at)
RETURNING *;

-- name: GetActivePlatformSession :one
SELECT s.*, u.status AS user_status, u.is_platform_admin, u.name AS user_name, u.email, u.totp_confirmed_at
FROM platform_sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash AND s.revoked_at IS NULL AND s.expires_at > now();

-- name: TouchPlatformSession :exec
UPDATE platform_sessions SET last_seen_at = now() WHERE id = @id;

-- name: RevokePlatformSession :execrows
UPDATE platform_sessions SET revoked_at = now() WHERE id = @id AND revoked_at IS NULL;

-- name: MarkPlatformSessionReauthenticated :one
UPDATE platform_sessions SET reauth_at = now() WHERE id = @id RETURNING reauth_at;

-- name: DeleteExpiredPlatformSessions :execrows
DELETE FROM platform_sessions WHERE expires_at < now() - interval '7 days' OR revoked_at < now() - interval '7 days';

-- ── Support access ──────────────────────────────────────────────────────────

-- name: CreateSupportGrant :one
INSERT INTO support_access_grants (tenant_id, admin_id, reason, read_only, expires_at)
VALUES (@tenant_id, @admin_id, @reason, @read_only, @expires_at)
RETURNING *;

-- name: GetSupportGrant :one
SELECT g.*, u.email AS admin_email, u.name AS admin_name, t.name AS tenant_name, t.slug
FROM support_access_grants g JOIN users u ON u.id = g.admin_id JOIN tenants t ON t.id = g.tenant_id
WHERE g.id = @id;

-- name: ListSupportGrants :many
SELECT g.*, u.email AS admin_email, u.name AS admin_name, t.name AS tenant_name, t.slug
FROM support_access_grants g JOIN users u ON u.id = g.admin_id JOIN tenants t ON t.id = g.tenant_id
WHERE (sqlc.narg(tenant_id)::uuid IS NULL OR g.tenant_id = sqlc.narg(tenant_id)::uuid)
ORDER BY g.created_at DESC
LIMIT @lim;

-- name: EndSupportGrant :one
UPDATE support_access_grants SET ended_at = now() WHERE id = @id AND ended_at IS NULL RETURNING *;

-- name: RevokeSupportSessions :execrows
UPDATE user_sessions SET revoked_at = now() WHERE support_grant_id = @grant_id AND revoked_at IS NULL;

-- name: CreateSupportSession :one
-- Inside the tenant (app.tenant_id set for the transaction).
INSERT INTO user_sessions (user_id, token_hash, ip, user_agent, reauth_at, expires_at, support_grant_id, read_only)
VALUES (@user_id, @token_hash, sqlc.narg(ip), sqlc.narg(user_agent), NULL, @expires_at, @support_grant_id, @read_only)
RETURNING *;

-- name: GetSupportGrantForSession :one
-- The grant of a support session, read by the tenant's session lookup.
SELECT g.id, g.reason, g.read_only, g.expires_at, g.ended_at, u.name AS admin_name, u.email AS admin_email
FROM support_access_grants g JOIN users u ON u.id = g.admin_id
WHERE g.id = @id;

-- ── Platform settings and audit ─────────────────────────────────────────────

-- name: GetPlatformSetting :one
SELECT value FROM platform_settings WHERE key = @key;

-- name: UpsertPlatformSetting :exec
INSERT INTO platform_settings (key, value) VALUES (@key, @value)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = now();

-- name: DeletePlatformSetting :exec
DELETE FROM platform_settings WHERE key = @key;

-- name: InsertPlatformAuditLog :exec
INSERT INTO platform_audit_logs (actor_id, actor_email, action, entity_type, entity_id, tenant_id, request_id, ip, user_agent, changes)
VALUES (sqlc.narg(actor_id), sqlc.narg(actor_email), @action, sqlc.narg(entity_type), sqlc.narg(entity_id), sqlc.narg(tenant_id),
        sqlc.narg(request_id), sqlc.narg(ip), sqlc.narg(user_agent), @changes);

-- name: ListPlatformAuditLogs :many
SELECT a.*, t.slug AS tenant_slug
FROM platform_audit_logs a LEFT JOIN tenants t ON t.id = a.tenant_id
WHERE (sqlc.narg(tenant_id)::uuid IS NULL OR a.tenant_id = sqlc.narg(tenant_id)::uuid)
  AND (sqlc.narg(after)::uuid IS NULL OR a.id < sqlc.narg(after)::uuid)
ORDER BY a.id DESC
LIMIT @lim;

-- name: SetTenantLocal :exec
-- Scopes the rest of the transaction to one tenant (row-level security of
-- the tenant tables). Only for platform transactions that create or touch
-- that tenant's rows.
SELECT set_config('app.tenant_id', @tenant_id::text, true);
