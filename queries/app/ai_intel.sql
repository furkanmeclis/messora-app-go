-- Phase 16C P7: lead capture, field provenance, campaign attribution of AI
-- usage, usage metrics, the AI budget and campaign reply classification.

-- ── Lead capture ────────────────────────────────────────────────────────────

-- name: UpsertInboundContact :one
-- Like UpsertContactByPhone, for the sender of an inbound message: a
-- contact created here is an unknown number (created_by_inbound_at, profile
-- status new). An existing contact keeps its marks.
INSERT INTO contacts (phone_e164, wa_id, profile_name, created_by_inbound_at, profile_status)
VALUES (@phone_e164, sqlc.narg(wa_id), @profile_name, now(), 'new')
ON CONFLICT (tenant_id, phone_e164) DO UPDATE SET
    wa_id        = coalesce(excluded.wa_id, contacts.wa_id),
    profile_name = CASE WHEN excluded.profile_name <> '' THEN excluded.profile_name ELSE contacts.profile_name END,
    updated_at   = now()
RETURNING *;

-- name: SetContactProfile :exec
UPDATE contacts SET profile_status = @profile_status, profile_progress = @profile_progress, updated_at = now()
WHERE id = @id;

-- name: ListContactFieldSources :many
SELECT * FROM contact_field_sources WHERE contact_id = @contact_id ORDER BY field;

-- name: UpsertContactFieldSource :exec
INSERT INTO contact_field_sources (contact_id, field, source, verified, collected_at, ai_usage_id, conversation_id, verified_by, verified_at)
VALUES (@contact_id, @field, @source, @verified, now(), sqlc.narg(ai_usage_id), sqlc.narg(conversation_id), sqlc.narg(verified_by),
        CASE WHEN @verified::boolean THEN now() END)
ON CONFLICT (contact_id, field) DO UPDATE SET
    source = excluded.source, verified = excluded.verified, collected_at = excluded.collected_at,
    ai_usage_id = excluded.ai_usage_id, conversation_id = excluded.conversation_id,
    verified_by = excluded.verified_by, verified_at = excluded.verified_at;

-- name: DeleteContactFieldSource :exec
DELETE FROM contact_field_sources WHERE contact_id = @contact_id AND field = @field;

-- name: VerifyContactFieldSources :many
UPDATE contact_field_sources SET verified = true, verified_by = sqlc.narg(verified_by), verified_at = now()
WHERE contact_id = @contact_id AND field = ANY(@fields::text[]) AND NOT verified
RETURNING field;

-- ── Campaign attribution ────────────────────────────────────────────────────

-- name: LatestCampaignSend :one
-- The latest campaign message that reached the contact on the number at or
-- before @at (accepted by Meta, not failed). Attribution then applies the
-- window (internal/modules/ai/attribution.go).
SELECT m.campaign_id::uuid AS campaign_id, m.sent_at::timestamptz AS sent_at FROM messages m
WHERE m.contact_id = @contact_id AND m.phone_number_id = @phone_number_id AND m.direction = 'outbound'
  AND m.campaign_id IS NOT NULL AND m.sent_at IS NOT NULL AND m.sent_at <= @at
  AND m.status IN ('sent', 'delivered', 'read')
ORDER BY m.sent_at DESC, m.id DESC
LIMIT 1;

-- name: ConversationThreadOf :one
SELECT contact_id, phone_number_id FROM conversations WHERE id = @id;

-- name: SetAIUsageCampaign :execrows
-- Fills the campaign of a usage row once; a row already attributed keeps
-- its campaign (a request is never counted for two campaigns).
UPDATE ai_usage SET campaign_id = @campaign_id WHERE id = @id AND campaign_id IS NULL;

-- name: ListUnattributedAIUsage :many
-- Usage rows of conversations without a campaign since @since (the
-- reconcile job retries them for late-arriving campaign data).
SELECT id, conversation_id::uuid AS conversation_id, created_at FROM ai_usage
WHERE campaign_id IS NULL AND conversation_id IS NOT NULL AND created_at >= @since AND id > @after
ORDER BY id
LIMIT @lim;

-- ── Usage metrics ───────────────────────────────────────────────────────────

-- name: AIUsageSummary :many
-- Usage between @from (inclusive) and @to (exclusive) grouped by @group_by
-- (model | provider | purpose | campaign | day | total). Costs are summed
-- per currency.
SELECT (CASE @group_by::text
            WHEN 'model' THEN u.provider_kind || '/' || u.model
            WHEN 'provider' THEN coalesce(u.provider_id::text, u.provider_kind)
            WHEN 'purpose' THEN u.purpose
            WHEN 'campaign' THEN coalesce(u.campaign_id::text, '')
            WHEN 'day' THEN to_char(u.created_at AT TIME ZONE @time_zone::text, 'YYYY-MM-DD')
            ELSE '' END)::text AS key,
       count(*)::bigint AS requests,
       count(*) FILTER (WHERE u.purpose IN ('reply', 'suggest') AND u.status = 'ok')::bigint AS replies,
       count(*) FILTER (WHERE u.status = 'error')::bigint AS failed,
       coalesce(sum(u.input_tokens), 0)::bigint AS input_tokens,
       coalesce(sum(u.output_tokens), 0)::bigint AS output_tokens,
       coalesce(sum(u.cached_tokens), 0)::bigint AS cached_tokens,
       coalesce(sum(u.cache_write_tokens + u.cache_write_1h_tokens), 0)::bigint AS cache_write_tokens,
       coalesce(sum(u.cost) FILTER (WHERE coalesce(u.currency, 'USD') = 'USD'), 0)::text AS cost,
       count(*) FILTER (WHERE u.cost IS NOT NULL AND u.currency <> 'USD')::bigint AS other_currency,
       coalesce(avg(u.latency_ms) FILTER (WHERE u.status = 'ok'), 0)::float8 AS avg_latency_ms,
       count(DISTINCT u.conversation_id) FILTER (WHERE u.purpose = 'reply')::bigint AS ai_conversations
FROM ai_usage u
WHERE u.created_at >= @from_at AND u.created_at < @to_at
GROUP BY 1
ORDER BY 1;

-- name: CountAIEscalations :one
-- Handoffs from the AI to people between @from_at and @to_at.
SELECT count(*)::bigint FROM conversation_handoffs WHERE from_type = 'ai' AND created_at >= @from_at AND created_at < @to_at;

-- name: CampaignNames :many
SELECT id, name FROM campaigns WHERE id = ANY(@ids::uuid[]);

-- ── Budget ──────────────────────────────────────────────────────────────────

-- name: AICostSince :one
-- USD cost of AI requests since @since (the month's budget).
SELECT coalesce(sum(cost), 0)::text FROM ai_usage WHERE created_at >= @since AND coalesce(currency, 'USD') = 'USD';

-- name: ListAIManagers :many
-- Active users who manage AI (budget notices).
SELECT DISTINCT u.id FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE u.status = 'active' AND rp.permission_key = 'ai.manage';

-- ── Campaign reply classification ───────────────────────────────────────────

-- name: InsertReplyClassification :one
-- The first reply of a contact to a campaign; later replies insert nothing.
INSERT INTO campaign_reply_classifications (campaign_id, contact_id, recipient_id, conversation_id, message_id)
VALUES (@campaign_id, @contact_id, sqlc.narg(recipient_id), sqlc.narg(conversation_id), sqlc.narg(message_id))
ON CONFLICT (campaign_id, contact_id) DO NOTHING
RETURNING id;

-- name: GetReplyClassification :one
SELECT * FROM campaign_reply_classifications WHERE id = @id;

-- name: FinishReplyClassification :exec
UPDATE campaign_reply_classifications SET status = @status, sentiment = sqlc.narg(sentiment), confidence = sqlc.narg(confidence),
    reason = @reason, error = @error, ai_usage_id = sqlc.narg(ai_usage_id), classified_at = now()
WHERE id = @id AND status = 'pending';

-- name: CountConversationClassifications :one
-- Classification requests of a conversation in the last hour.
SELECT count(*) FROM ai_usage
WHERE conversation_id = @conversation_id AND purpose = 'classify' AND created_at > now() - interval '1 hour';
