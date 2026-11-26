-- Campaign analytics (Phase 16C P3, docs/CAMPAIGN_ANALYTICS.md).

-- name: GetCampaignCounters :one
SELECT campaign_id, recipients, r_pending, r_queued, r_skipped, r_cancelled, r_retry,
    o_pending, o_sent, o_delivered, o_read, o_failed, o_unknown, o_blocked, o_cancelled,
    sent, delivered, read, rejected, replied, positive, negative, neutral, opted_out,
    m_pending, m_sending, m_unknown, m_sent, m_delivered, m_read, m_failed, m_held, m_cancelled, m_deferred, m_skipped,
    err_131048, err_131049, err_131050, priced, unpriced, cost::text AS cost, reconciled_at, updated_at
FROM campaign_counters WHERE campaign_id = @campaign_id;

-- name: RecomputeCampaignCounters :one
SELECT campaign_counters_recompute(@campaign_id::uuid)::boolean AS drifted;

-- name: CampaignsToReconcile :many
-- Active campaigns and those finished within the attribution horizon
-- (replies and opt-outs still arrive).
SELECT id FROM campaigns
WHERE status IN ('preparing', 'running') OR status LIKE 'paused%'
   OR (finished_at IS NOT NULL AND finished_at > now() - make_interval(days => @horizon_days::integer))
ORDER BY id LIMIT 500;

-- name: LatestReachedRecipient :one
-- The contact's most recent campaign recipient reached (sent) at or before
-- at, with its campaign's number and reaction map: the only campaign an
-- inbound message or opt-out at that moment may be attributed to.
SELECT r.id, r.campaign_id, r.sent_at, r.replied_at, r.reaction_source, r.opted_out_at,
    c.phone_number_id, c.reaction_map
FROM campaign_recipients r JOIN campaigns c ON c.id = r.campaign_id
WHERE r.contact_id = @contact_id AND r.sent_at IS NOT NULL AND r.sent_at <= @at
ORDER BY r.sent_at DESC, r.id DESC LIMIT 1;

-- name: SetRecipientReplied :execrows
UPDATE campaign_recipients SET replied_at = @at, reply_message_id = @message_id, updated_at = now()
WHERE id = @id AND replied_at IS NULL;

-- name: SetRecipientReplyPayload :exec
UPDATE campaign_recipients SET reply_payload = @payload, updated_at = now() WHERE id = @id;

-- name: SetRecipientReaction :execrows
-- A manual label is never overwritten by the system (quick reply mapping
-- or AI); a manual label replaces anything. An AI label never replaces a
-- tapped quick reply (the customer's own choice; an AI result may finish
-- after a later tap). reaction NULL clears.
UPDATE campaign_recipients SET reaction = sqlc.narg(reaction),
    reaction_source = CASE WHEN sqlc.narg(reaction)::text IS NULL THEN NULL ELSE @source::text END,
    reaction_at = CASE WHEN sqlc.narg(reaction)::text IS NULL THEN NULL ELSE now() END, updated_at = now()
WHERE id = @id AND campaign_id = @campaign_id AND (@source::text = 'manual' OR reaction_source IS DISTINCT FROM 'manual')
    AND (@source::text <> 'ai' OR reaction_source IS DISTINCT FROM 'quick_reply');

-- name: SetRecipientOptedOut :execrows
UPDATE campaign_recipients SET opted_out_at = @at, updated_at = now() WHERE id = @id AND opted_out_at IS NULL;

-- name: SetCampaignReactionMap :one
UPDATE campaigns SET reaction_map = @reaction_map, updated_at = now() WHERE id = @id RETURNING *;

-- name: RemapQuickReplyReactions :execrows
-- After the mapping changed: recipients who tapped a quick reply take its
-- label (none when unmapped); AI labels give way, manual ones stay.
UPDATE campaign_recipients SET
    reaction = CASE WHEN (@reaction_map::jsonb ->> reply_payload) IN ('positive', 'negative', 'neutral') THEN @reaction_map::jsonb ->> reply_payload END,
    reaction_source = CASE WHEN (@reaction_map::jsonb ->> reply_payload) IN ('positive', 'negative', 'neutral') THEN 'quick_reply' END,
    reaction_at = CASE WHEN (@reaction_map::jsonb ->> reply_payload) IN ('positive', 'negative', 'neutral') THEN coalesce(reaction_at, replied_at, now()) END,
    updated_at = now()
WHERE campaign_id = @campaign_id AND reply_payload IS NOT NULL AND reaction_source IS DISTINCT FROM 'manual'
  AND reaction IS DISTINCT FROM CASE WHEN (@reaction_map::jsonb ->> reply_payload) IN ('positive', 'negative', 'neutral') THEN @reaction_map::jsonb ->> reply_payload END;

-- name: SpamWindow :one
-- The campaign's last n attempts (recipients whose message reached Meta)
-- and how many of them hit 131049/131050 or opted out.
SELECT count(*)::bigint AS attempts,
    count(*) FILTER (WHERE w.error_code IN ('131049', '131050'))::bigint AS ecosystem,
    count(*) FILTER (WHERE w.opted_out_at IS NOT NULL)::bigint AS opted_out
FROM (SELECT r.error_code, r.opted_out_at FROM campaign_recipients r
      WHERE r.campaign_id = @campaign_id AND r.attempted_at IS NOT NULL
        AND (sqlc.narg(since)::timestamptz IS NULL OR r.attempted_at > sqlc.narg(since)::timestamptz)
      ORDER BY r.attempted_at DESC LIMIT @n) w;

-- name: ActiveCampaignsOfAccount :many
-- Campaigns that are sending (or about to) through the account's numbers.
SELECT c.id, c.phone_number_id, c.template_name, c.template_language, c.status FROM campaigns c
JOIN phone_numbers p ON p.id = c.phone_number_id
WHERE p.account_id = @account_id AND c.status IN ('preparing', 'running', 'paused_limit');

-- name: ActiveCampaignIDs :many
SELECT id FROM campaigns WHERE status IN ('preparing', 'running', 'paused_limit') ORDER BY id;

-- name: CampaignTemplate :one
-- The campaign's template on its sender's account (category, status,
-- quality, buttons).
SELECT t.* FROM campaigns c JOIN phone_numbers p ON p.id = c.phone_number_id
JOIN message_templates t ON t.account_id = p.account_id AND t.name = c.template_name AND t.language = c.template_language
WHERE c.id = @campaign_id;

-- name: CampaignAccountEvents :many
-- Account events of the campaign's account in a period: quality and
-- template changes, restrictions (the spam and quality section).
SELECT e.* FROM whatsapp_account_events e
WHERE e.account_id = @account_id AND e.occurred_at >= @from_at AND e.occurred_at <= @to_at
  AND (e.phone_number_id IS NULL OR e.phone_number_id = @phone_number_id)
ORDER BY e.occurred_at, e.id LIMIT 500;

-- name: CampaignHourly :many
-- Sends, deliveries, reads and replies per hour, from the recipient rows.
SELECT date_trunc('hour', t.at)::timestamptz AS hour, t.kind::text AS kind, count(*)::bigint AS n
FROM (
    SELECT r.sent_at AS at, 'sent' AS kind FROM campaign_recipients r WHERE r.campaign_id = @campaign_id AND r.sent_at IS NOT NULL
    UNION ALL SELECT r.delivered_at, 'delivered' FROM campaign_recipients r WHERE r.campaign_id = @campaign_id AND r.delivered_at IS NOT NULL
    UNION ALL SELECT r.read_at, 'read' FROM campaign_recipients r WHERE r.campaign_id = @campaign_id AND r.read_at IS NOT NULL
    UNION ALL SELECT r.replied_at, 'replied' FROM campaign_recipients r WHERE r.campaign_id = @campaign_id AND r.replied_at IS NOT NULL
    UNION ALL SELECT r.failed_at, 'failed' FROM campaign_recipients r WHERE r.campaign_id = @campaign_id AND r.failed_at IS NOT NULL
) t
GROUP BY 1, 2 ORDER BY 1, 2;

-- name: InsertCampaignForecast :execrows
INSERT INTO campaign_forecasts (campaign_id, forecast) VALUES (@campaign_id, @forecast) ON CONFLICT (campaign_id) DO NOTHING;

-- name: GetCampaignForecast :one
SELECT * FROM campaign_forecasts WHERE campaign_id = @campaign_id;

-- name: ForecastHistory :many
-- Completed campaigns whose template has the given category, newest first,
-- with their counters: the basis of forecast rates.
SELECT c.id, c.finished_at, k.recipients, k.o_blocked, k.sent, k.delivered, k.read, k.replied, k.positive, k.negative, k.opted_out
FROM campaigns c
JOIN campaign_counters k ON k.campaign_id = c.id
JOIN phone_numbers p ON p.id = c.phone_number_id
JOIN message_templates t ON t.account_id = p.account_id AND t.name = c.template_name AND t.language = c.template_language
WHERE c.status = 'completed' AND upper(t.category) = upper(@category::text)
  AND (sqlc.narg(exclude_id)::uuid IS NULL OR c.id <> sqlc.narg(exclude_id)::uuid)
ORDER BY c.finished_at DESC NULLS LAST LIMIT 20;

-- name: CampaignCostDaily :many
-- Meta's own (approximate) cost for the campaign's number and days: the
-- reconciled total, which covers every message of the number on those
-- days, not only the campaign's.
SELECT d.day, d.pricing_category, sum(d.volume)::bigint AS volume, sum(d.cost)::text AS cost, max(d.currency)::text AS currency
FROM meta_cost_daily d JOIN phone_numbers p ON p.account_id = d.account_id
WHERE p.id = @phone_number_id AND d.day BETWEEN @from_day AND @to_day
  AND (d.phone_number = '' OR regexp_replace(d.phone_number, '[^0-9]', '', 'g') = regexp_replace(p.display_phone_number, '[^0-9]', '', 'g'))
GROUP BY d.day, d.pricing_category ORDER BY d.day, d.pricing_category;

-- name: CampaignCostByCategory :many
-- The campaign's per-message cost by pricing category and type.
SELECT coalesce(pricing_category, '')::text AS category, coalesce(pricing_type, '')::text AS type,
    count(*)::bigint AS messages, coalesce(sum(cost_amount), 0)::text AS cost,
    count(*) FILTER (WHERE pricing_type = 'regular' AND cost_amount IS NULL)::bigint AS unpriced,
    max(cost_currency)::text AS currency
FROM messages WHERE campaign_id = @campaign_id AND (pricing_type IS NOT NULL OR pricing_category IS NOT NULL)
GROUP BY 1, 2 ORDER BY 1, 2;

-- name: InsertCampaignExport :one
INSERT INTO campaign_exports (campaign_id, kind, format, filters, requested_by)
VALUES (@campaign_id, @kind, @format, @filters, sqlc.narg(requested_by)) RETURNING *;

-- name: GetCampaignExport :one
SELECT * FROM campaign_exports WHERE id = @id;

-- name: ListCampaignExports :many
SELECT * FROM campaign_exports WHERE campaign_id = @campaign_id ORDER BY id DESC LIMIT 20;

-- name: ClaimCampaignExport :one
UPDATE campaign_exports SET status = 'running' WHERE id = @id AND status = 'queued' RETURNING *;

-- name: FinishCampaignExport :exec
UPDATE campaign_exports SET status = 'done', file_key = @file_key, filename = @filename, size_bytes = @size_bytes,
    rows_written = @rows_written, finished_at = now(), expires_at = @expires_at
WHERE id = @id;

-- name: FailCampaignExport :exec
UPDATE campaign_exports SET status = 'failed', error = @error, finished_at = now() WHERE id = @id;

-- name: ExpiredCampaignExports :many
UPDATE campaign_exports SET status = 'expired' WHERE status = 'done' AND expires_at < now() RETURNING id, file_key;

-- name: StaleCampaignExports :many
-- Exports whose job was lost (queued or running for long).
SELECT id FROM campaign_exports WHERE status IN ('queued', 'running') AND created_at < now() - interval '30 minutes';

-- name: SetContactType :exec
UPDATE contacts SET contact_type = sqlc.narg(contact_type), updated_at = now() WHERE id = @id;

-- name: ListContactTypes :many
SELECT contact_type::text AS contact_type, count(*)::bigint AS n FROM contacts WHERE contact_type IS NOT NULL
GROUP BY contact_type ORDER BY n DESC, contact_type LIMIT 100;

-- name: LastSpamResume :one
-- When an operator last resumed the campaign from a spam protection pause.
SELECT coalesce(max(resumed_at), 'epoch')::timestamptz AS resumed_at FROM campaign_pauses WHERE campaign_id = @campaign_id AND reason = 'spam_protection' AND resumed_at IS NOT NULL;

-- name: CampaignAICost :many
-- AI usage attributed to the campaign (ai_usage.campaign_id: each request
-- belongs to one campaign or none, docs/AI.md "Campaign attribution"), by
-- currency. Requests without a price (currency NULL) are counted under
-- USD with no cost, like the AI usage page does.
SELECT coalesce(currency, 'USD')::text AS currency, coalesce(sum(cost), 0)::text AS cost, count(*)::bigint AS requests,
       count(*) FILTER (WHERE status = 'error')::bigint AS failed,
       coalesce(sum(input_tokens), 0)::bigint AS input_tokens,
       coalesce(sum(output_tokens), 0)::bigint AS output_tokens,
       coalesce(sum(cached_tokens), 0)::bigint AS cached_tokens,
       count(DISTINCT conversation_id)::bigint AS conversations
FROM ai_usage WHERE campaign_id = @campaign_id GROUP BY 1 ORDER BY 1;
