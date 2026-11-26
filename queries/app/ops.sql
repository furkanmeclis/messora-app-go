-- name: OpsCampaignStatusCounts :one
-- Operations center (GET /ops/overview): campaigns by state; completed and
-- failed since day_start (the viewer's midnight). Every ops list is bounded;
-- the conversation queries apply the visibility rule of ListConversations.
SELECT
    count(*) FILTER (WHERE status = 'running')::bigint AS running,
    count(*) FILTER (WHERE status = 'preparing')::bigint AS preparing,
    count(*) FILTER (WHERE status = 'scheduled')::bigint AS scheduled,
    count(*) FILTER (WHERE status IN ('paused', 'paused_limit'))::bigint AS paused,
    count(*) FILTER (WHERE status = 'paused_limit')::bigint AS paused_limit,
    count(*) FILTER (WHERE status = 'paused' AND paused_reason = 'spam_protection')::bigint AS paused_spam,
    count(*) FILTER (WHERE status = 'paused' AND paused_reason = 'offer_expired')::bigint AS paused_offer,
    count(*) FILTER (WHERE status = 'completed' AND finished_at >= @day_start::timestamptz)::bigint AS completed_today,
    count(*) FILTER (WHERE status = 'failed' AND coalesce(finished_at, updated_at) >= @day_start::timestamptz)::bigint AS failed_today
FROM campaigns
WHERE status IN ('running', 'preparing', 'scheduled', 'paused', 'paused_limit')
   OR (status IN ('completed', 'failed') AND coalesce(finished_at, updated_at) >= @day_start::timestamptz);

-- name: OpsActiveCampaigns :many
-- Campaigns sending, waiting or scheduled: sending first, then paused,
-- then the next scheduled; with the sender's currency for the cost.
SELECT c.*, coalesce(wa.currency, '')::text AS currency, pn.display_phone_number
FROM campaigns c
JOIN phone_numbers pn ON pn.id = c.phone_number_id
JOIN whatsapp_accounts wa ON wa.id = pn.account_id
WHERE c.status IN ('running', 'preparing', 'paused', 'paused_limit', 'scheduled')
ORDER BY CASE c.status WHEN 'running' THEN 0 WHEN 'preparing' THEN 1 WHEN 'paused_limit' THEN 2 WHEN 'paused' THEN 3 ELSE 4 END,
    c.scheduled_at NULLS LAST, c.id
LIMIT @lim;

-- name: OpsAwaitingHuman :many
-- Conversations whose escalation no person accepted yet, oldest first.
SELECT c.id, c.contact_id, c.assignee_id, c.owner_type, c.ai_mode, c.last_message_at,
    h.id AS handoff_id, h.reason, h.reason_detail, h.created_at AS escalated_at, h.to_user_id,
    ct.phone_e164, coalesce(nullif(ct.name, ''), ct.profile_name, '')::text AS contact_name,
    count(*) OVER ()::bigint AS total
FROM conversations c
JOIN conversation_handoffs h ON h.id = c.pending_handoff_id
JOIN contacts ct ON ct.id = c.contact_id
WHERE c.pending_handoff_id IS NOT NULL
  AND (@see_all::bool OR c.assignee_id IS NULL OR c.assignee_id = @viewer_id::uuid)
ORDER BY h.created_at, c.id
LIMIT @lim;

-- name: OpsUnassignedWaiting :many
-- Open conversations nobody is assigned to, not answered by the AI, whose
-- last message is the customer's; longest waiting first.
SELECT c.id, c.contact_id, c.last_inbound_at, c.unread_count, c.owner_type,
    ct.phone_e164, coalesce(nullif(ct.name, ''), ct.profile_name, '')::text AS contact_name,
    count(*) OVER ()::bigint AS total
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
WHERE c.status = 'open' AND c.assignee_id IS NULL AND c.owner_type <> 'ai' AND c.pending_handoff_id IS NULL
  AND c.last_inbound_at IS NOT NULL AND c.last_inbound_at >= c.last_message_at
ORDER BY c.last_inbound_at, c.id
LIMIT @lim;

-- name: OpsWaitingReply :many
-- Open conversations visible to the viewer whose last message is the
-- customer's (waiting for an answer), longest waiting first.
SELECT c.id, c.contact_id, c.assignee_id, c.last_inbound_at, c.unread_count, c.owner_type,
    ct.phone_e164, coalesce(nullif(ct.name, ''), ct.profile_name, '')::text AS contact_name,
    u.name AS assignee_name,
    count(*) OVER ()::bigint AS total
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN users u ON u.id = c.assignee_id
WHERE c.status = 'open' AND c.last_inbound_at IS NOT NULL AND c.last_inbound_at >= c.last_message_at
  AND (@see_all::bool OR c.assignee_id IS NULL OR c.assignee_id = @viewer_id::uuid)
ORDER BY c.last_inbound_at, c.id
LIMIT @lim;

-- name: OpsAIActiveCount :one
-- Open conversations the AI answers, visible to the viewer.
SELECT count(*)::bigint FROM conversations c
WHERE c.owner_type = 'ai' AND c.status = 'open'
  AND (@see_all::bool OR c.assignee_id IS NULL OR c.assignee_id = @viewer_id::uuid);

-- name: OpsRecentMessages :many
-- The newest messages of one direction since since_id (a UUIDv7 bound,
-- so the primary key limits the scan), visible to the viewer: messages
-- without a conversation only with see_all, as their realtime events.
SELECT m.id, m.contact_id, m.direction, m.type, m.status, m.source, m.created_at, m.error_code,
    left(m.body, 120)::text AS preview, c.id AS conversation_id,
    ct.phone_e164, coalesce(nullif(ct.name, ''), ct.profile_name, '')::text AS contact_name
FROM messages m
JOIN contacts ct ON ct.id = m.contact_id
LEFT JOIN conversations c ON c.contact_id = m.contact_id AND c.phone_number_id = m.phone_number_id
WHERE m.direction = @direction AND m.id >= @since_id::uuid
  AND (@see_all::bool OR (c.id IS NOT NULL AND (c.assignee_id IS NULL OR c.assignee_id = @viewer_id::uuid)))
ORDER BY m.id DESC
LIMIT @lim;

-- name: OpsMessageHours :many
-- Messages per hour since since_id (sparkline of the last 24 hours).
SELECT date_trunc('hour', created_at)::timestamptz AS hour,
    count(*) FILTER (WHERE direction = 'outbound')::bigint AS outbound,
    count(*) FILTER (WHERE direction = 'inbound')::bigint AS inbound
FROM messages
WHERE id >= @since_id::uuid
GROUP BY 1
ORDER BY 1;

-- name: OpsActiveRuns :many
-- Automation runs in progress, newest first.
SELECT r.id, r.automation_id, a.name AS automation_name, r.status, r.started_by, r.steps_executed, r.created_at,
    r.conversation_id, c.assignee_id AS conversation_assignee_id,
    count(*) OVER ()::bigint AS total
FROM automation_runs r
JOIN automations a ON a.id = r.automation_id
LEFT JOIN conversations c ON c.id = r.conversation_id
WHERE r.status IN ('queued', 'running', 'waiting')
ORDER BY r.created_at DESC
LIMIT @lim;

-- name: OpsFailedRuns :many
-- Automation runs that failed since since, newest first.
SELECT r.id, r.automation_id, a.name AS automation_name, coalesce(r.error, '')::text AS error, r.error_code, r.attempt, r.finished_at, r.created_at,
    r.conversation_id, c.assignee_id AS conversation_assignee_id,
    count(*) OVER ()::bigint AS total
FROM automation_runs r
JOIN automations a ON a.id = r.automation_id
LEFT JOIN conversations c ON c.id = r.conversation_id
WHERE r.status = 'failed' AND r.finished_at >= @since::timestamptz
ORDER BY r.finished_at DESC
LIMIT @lim;

-- name: OpsAccountProblems :many
-- Connected accounts in error, disabled, or with a token warning.
SELECT id, name, status, last_error, token_warning, token_expires_at
FROM whatsapp_accounts
WHERE status <> 'active' OR token_warning IS NOT NULL
   OR (token_expires_at IS NOT NULL AND token_expires_at < now() + interval '14 days')
ORDER BY name, id
LIMIT 20;

-- name: OpsLowQualityNumbers :many
-- Numbers whose quality Meta rates medium (YELLOW) or low (RED).
SELECT id, account_id, display_phone_number, verified_name, quality_rating
FROM phone_numbers
WHERE quality_rating IN ('RED', 'YELLOW')
ORDER BY quality_rating, display_phone_number
LIMIT 20;

-- name: OpsAccountEvents :many
-- Meta's account alerts since since (restrictions, quality changes, …),
-- newest first.
SELECT e.id, e.account_id, wa.name AS account_name, e.phone_number_id, pn.display_phone_number, e.kind, e.occurred_at
FROM whatsapp_account_events e
JOIN whatsapp_accounts wa ON wa.id = e.account_id
LEFT JOIN phone_numbers pn ON pn.id = e.phone_number_id
WHERE e.occurred_at >= @since
ORDER BY e.occurred_at DESC
LIMIT @lim;
