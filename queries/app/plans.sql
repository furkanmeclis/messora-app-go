-- Plans, usage meters, AI credits and plan change requests (Phase 17 WP8,
-- docs/ENTITLEMENTS.md). The catalogue (plans) and the platform's AI keys
-- are platform tables: the application reads them, the platform role
-- (tenant.Platform) writes them. Usage, credits and requests are tenant
-- rows under row-level security.

-- ── Plans ───────────────────────────────────────────────────────────────────

-- name: GetTenantPlan :one
-- A tenant's plan as the entitlements provider reads it (readable without
-- a tenant: tenants and plans are platform tables).
SELECT t.id, t.plan_key, t.plan_overrides, t.subscription_status, t.trial_ends_at,
       p.limits AS plan_limits, p.features AS plan_features, p.name_tr AS plan_name_tr, p.name_en AS plan_name_en
FROM tenants t LEFT JOIN plans p ON p.key = t.plan_key
WHERE t.id = @id;

-- name: ListPlans :many
SELECT p.*, (SELECT count(*) FROM tenants t WHERE t.plan_key = p.key AND t.status <> 'deleted') AS tenant_count
FROM plans p
ORDER BY p.sort_order, p.key;

-- name: ListPublicPlans :many
SELECT * FROM plans WHERE public AND active ORDER BY sort_order, key;

-- name: GetPlan :one
SELECT * FROM plans WHERE key = @key;

-- name: CreatePlan :one
INSERT INTO plans (key, name_tr, name_en, description_tr, description_en, sort_order, limits, features, currency,
                   price_monthly, price_yearly, custom_pricing, active, public)
VALUES (@key, @name_tr, @name_en, @description_tr, @description_en, @sort_order, @limits, @features, @currency,
        (sqlc.narg(price_monthly)::text)::numeric(12, 2), (sqlc.narg(price_yearly)::text)::numeric(12, 2),
        @custom_pricing, @active, @public)
RETURNING *;

-- name: UpdatePlan :one
UPDATE plans SET
    name_tr = @name_tr, name_en = @name_en, description_tr = @description_tr, description_en = @description_en,
    sort_order = @sort_order, limits = @limits, features = @features, currency = @currency,
    price_monthly = (sqlc.narg(price_monthly)::text)::numeric(12, 2), price_yearly = (sqlc.narg(price_yearly)::text)::numeric(12, 2),
    custom_pricing = @custom_pricing, active = @active, public = @public, updated_at = now()
WHERE key = @key
RETURNING *;

-- name: SetTenantPlan :one
UPDATE tenants SET plan_key = sqlc.narg(plan_key), plan_overrides = @plan_overrides, plan_changed_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- ── Usage meters (tenant context) ──────────────────────────────────────────

-- name: PlanUsageCounts :one
SELECT
    (SELECT count(*) FROM tenant_memberships m WHERE m.status = 'active') AS users,
    (SELECT count(*) FROM user_invitations i WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()) AS pending_invitations,
    (SELECT count(*) FROM phone_numbers n JOIN whatsapp_accounts a ON a.id = n.account_id WHERE a.status <> 'disabled') AS phone_numbers,
    (SELECT count(*) FROM ai_profiles) AS ai_agents,
    (SELECT count(*) FROM automations WHERE active) AS automation_flows,
    (SELECT count(*) FROM webhook_endpoints) AS webhook_endpoints;

-- name: CountContactsForPlan :one
SELECT count(*) FROM contacts;

-- name: ExistingContactPhones :many
SELECT phone_e164 FROM contacts WHERE phone_e164 = ANY (@phones::text[]);

-- name: CountSeatsForPlan :one
-- Seats taken: active members and open invitations (an invitation holds a
-- seat until it is accepted, revoked or expires).
SELECT ((SELECT count(*) FROM tenant_memberships m WHERE m.status = 'active')
     + (SELECT count(*) FROM user_invitations i WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()))::bigint;

-- name: CountActiveMembersForPlan :one
SELECT count(*) FROM tenant_memberships WHERE status = 'active';

-- name: CountPhoneNumbersForPlan :one
SELECT count(*) FROM phone_numbers n JOIN whatsapp_accounts a ON a.id = n.account_id WHERE a.status <> 'disabled';

-- name: CountAIProfilesForPlan :one
SELECT count(*) FROM ai_profiles;

-- name: CountActiveAutomationsForPlan :one
SELECT count(*) FROM automations WHERE active AND (sqlc.narg(except_id)::uuid IS NULL OR id <> sqlc.narg(except_id)::uuid);

-- name: StorageBytesUsed :one
-- Bytes of the files the tenant keeps: received media, contact import
-- files (kept 30 days) and finished campaign exports. Logos and template
-- samples are small and not counted.
SELECT (coalesce((SELECT sum(size_bytes) FROM media_files WHERE status = 'stored' AND storage_key IS NOT NULL), 0)
      + coalesce((SELECT sum(size_bytes) FROM contact_imports WHERE storage_key <> ''), 0)
      + coalesce((SELECT sum(size_bytes) FROM campaign_exports WHERE status = 'done'), 0))::bigint;

-- name: LockTenantPlanCounter :exec
-- Serialises the creating transactions of one counted resource of the
-- tenant (count-then-insert under the same lock).
SELECT pg_advisory_xact_lock(hashtextextended('messora.plan:' || app_tenant_id()::text || ':' || @resource::text, 0));

-- ── AI credits (tenant context; amounts in millicredits) ───────────────────

-- name: EnsurePlanCreditLot :exec
INSERT INTO ai_credit_lots (source, period_start, granted, starts_at, expires_at, note)
VALUES ('plan', @period_start, @granted, @starts_at, @expires_at, 'plan')
ON CONFLICT (tenant_id, period_start) WHERE source = 'plan' DO NOTHING;

-- name: SetPlanCreditLotGranted :execrows
UPDATE ai_credit_lots SET granted = @granted, updated_at = now()
WHERE source = 'plan' AND period_start = @period_start;

-- name: LockActiveCreditLots :many
-- The lots usable now, in consumption order (the plan's month first, then
-- what expires first), locked against concurrent consumption.
SELECT * FROM ai_credit_lots
WHERE starts_at <= @now AND (expires_at IS NULL OR expires_at > @now)
ORDER BY (source = 'plan') DESC, expires_at NULLS LAST, created_at, id
FOR UPDATE;

-- name: ActiveCreditTotals :one
SELECT coalesce(sum(granted), 0)::bigint AS granted, coalesce(sum(used), 0)::bigint AS used,
       coalesce(sum(greatest(granted - used, 0)), 0)::bigint AS remaining
FROM ai_credit_lots
WHERE starts_at <= @now AND (expires_at IS NULL OR expires_at > @now);

-- name: AddCreditLotUsage :exec
UPDATE ai_credit_lots SET used = used + @amount, updated_at = now() WHERE id = @id;

-- name: InsertCreditLot :one
INSERT INTO ai_credit_lots (source, granted, starts_at, expires_at, note, reference, created_by)
VALUES (@source, @granted, @starts_at, sqlc.narg(expires_at), @note, sqlc.narg(reference), sqlc.narg(created_by))
RETURNING *;

-- name: ListCreditLots :many
SELECT * FROM ai_credit_lots ORDER BY created_at DESC, id DESC LIMIT @lim;

-- name: CreditsUsedSince :one
-- Millicredits charged to AI calls with the platform's keys since @since.
SELECT coalesce(sum(credits), 0)::bigint FROM ai_usage WHERE created_at >= @since AND key_source = 'platform';

-- ── Plan change requests ───────────────────────────────────────────────────

-- name: InsertPlanChangeRequest :one
INSERT INTO plan_change_requests (current_plan, requested_plan, billing_cycle, message, requested_by)
VALUES (sqlc.narg(current_plan), @requested_plan, @billing_cycle, @message, sqlc.narg(requested_by))
RETURNING *;

-- name: GetOpenPlanChangeRequest :one
SELECT * FROM plan_change_requests WHERE status = 'open' ORDER BY created_at DESC LIMIT 1;

-- name: ListPlanChangeRequests :many
-- Every tenant's requests (platform role), newest first.
SELECT r.*, t.slug AS tenant_slug, t.name AS tenant_name, u.email AS requested_by_email, u.name AS requested_by_name
FROM plan_change_requests r
JOIN tenants t ON t.id = r.tenant_id
LEFT JOIN users u ON u.id = r.requested_by
WHERE (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status)::text)
ORDER BY r.created_at DESC, r.id DESC
LIMIT @lim;

-- name: ResolvePlanChangeRequest :one
UPDATE plan_change_requests SET status = @status, resolution_note = @resolution_note, resolved_by = sqlc.narg(resolved_by), resolved_at = now()
WHERE id = @id AND status = 'open'
RETURNING *;

-- name: GetPlanChangeRequest :one
SELECT * FROM plan_change_requests WHERE id = @id;

-- name: ListPlatformAdmins :many
-- Platform admins to tell about a plan change request.
SELECT id, email, name, locale FROM users WHERE is_platform_admin AND status = 'active' ORDER BY created_at;

-- ── The platform's AI keys ─────────────────────────────────────────────────

-- name: ListPlatformAIProviders :many
SELECT * FROM platform_ai_providers ORDER BY kind;

-- name: GetPlatformAIProvider :one
SELECT * FROM platform_ai_providers WHERE id = @id;

-- name: GetPlatformAIProviderByKind :one
SELECT * FROM platform_ai_providers WHERE kind = @kind;

-- name: InsertPlatformAIProvider :one
INSERT INTO platform_ai_providers (id, kind, name, api_key_enc, key_last4, base_url, enabled)
VALUES (@id, @kind, @name, @api_key_enc, @key_last4, sqlc.narg(base_url), @enabled)
RETURNING *;

-- name: UpdatePlatformAIProvider :one
UPDATE platform_ai_providers SET
    name = coalesce(sqlc.narg(name), name),
    api_key_enc = coalesce(sqlc.narg(api_key_enc), api_key_enc),
    key_last4 = coalesce(sqlc.narg(key_last4), key_last4),
    base_url = CASE WHEN @set_base_url::boolean THEN sqlc.narg(base_url) ELSE base_url END,
    enabled = coalesce(sqlc.narg(enabled), enabled),
    status = CASE WHEN sqlc.narg(api_key_enc)::bytea IS NOT NULL OR @set_base_url::boolean THEN 'unknown' ELSE status END,
    status_detail = CASE WHEN sqlc.narg(api_key_enc)::bytea IS NOT NULL OR @set_base_url::boolean THEN '' ELSE status_detail END,
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetPlatformAIProviderStatus :one
UPDATE platform_ai_providers SET status = @status, status_detail = @status_detail, checked_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DeletePlatformAIProvider :execrows
DELETE FROM platform_ai_providers WHERE id = @id;

-- name: ListPlatformAIModels :many
SELECT m.*, p.kind AS provider_kind, p.name AS provider_name
FROM platform_ai_models m JOIN platform_ai_providers p ON p.id = m.provider_id
WHERE p.enabled OR @include_disabled::boolean
ORDER BY p.kind, m.model_id;

-- name: UpsertPlatformAIModel :exec
INSERT INTO platform_ai_models (provider_id, model_id, display_name, context_window, max_output, capabilities, lifecycle, synced_at)
VALUES (@provider_id, @model_id, @display_name, sqlc.narg(context_window), sqlc.narg(max_output), @capabilities, sqlc.narg(lifecycle), now())
ON CONFLICT (provider_id, model_id) DO UPDATE SET
    display_name = excluded.display_name, context_window = excluded.context_window, max_output = excluded.max_output,
    capabilities = excluded.capabilities, lifecycle = excluded.lifecycle, synced_at = now();

-- name: DeleteStalePlatformAIModels :execrows
DELETE FROM platform_ai_models WHERE provider_id = @provider_id AND NOT (model_id = ANY (@keep::text[]));

-- name: MarkPlatformAIModelsSynced :exec
UPDATE platform_ai_providers SET models_synced_at = now() WHERE id = @id;
