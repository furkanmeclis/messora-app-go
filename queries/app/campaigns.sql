-- name: InsertCampaign :one
INSERT INTO campaigns (name, phone_number_id, template_name, template_language, template_params, audience, window_size, created_by)
VALUES (@name, @phone_number_id, @template_name, @template_language, @template_params, @audience, @window_size, sqlc.narg(created_by))
RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = @id;

-- name: LockCampaign :one
SELECT * FROM campaigns WHERE id = @id FOR UPDATE;

-- name: ListCampaigns :many
SELECT * FROM campaigns
WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(before_id)::uuid IS NULL OR id < sqlc.narg(before_id)::uuid)
ORDER BY id DESC LIMIT @lim;

-- name: UpdateCampaignDraft :one
UPDATE campaigns SET
    name              = coalesce(sqlc.narg(name), name),
    phone_number_id   = coalesce(sqlc.narg(phone_number_id), phone_number_id),
    template_name     = coalesce(sqlc.narg(template_name), template_name),
    template_language = coalesce(sqlc.narg(template_language), template_language),
    template_params   = coalesce(sqlc.narg(template_params), template_params),
    audience          = coalesce(sqlc.narg(audience), audience),
    window_size       = coalesce(sqlc.narg(window_size), window_size),
    updated_at        = now()
WHERE id = @id AND status IN ('draft', 'scheduled')
RETURNING *;

-- name: DeleteCampaign :execrows
DELETE FROM campaigns WHERE id = @id AND status IN ('draft', 'scheduled', 'completed', 'cancelled', 'failed');

-- name: SetCampaignStatus :one
-- Generic transition guarded by the allowed source states.
UPDATE campaigns SET status = @status,
    scheduled_at = CASE WHEN @status::text = 'scheduled' THEN sqlc.narg(scheduled_at) ELSE scheduled_at END,
    started_at   = CASE WHEN @status::text IN ('preparing', 'running') THEN coalesce(started_at, now()) ELSE started_at END,
    finished_at  = CASE WHEN @status::text IN ('completed', 'cancelled', 'failed') THEN now() ELSE finished_at END,
    dispatch_gen = CASE WHEN @status::text = 'running' THEN dispatch_gen + 1 ELSE dispatch_gen END,
    last_error   = CASE WHEN @status::text = 'failed' THEN sqlc.narg(last_error) ELSE last_error END,
    updated_at   = now()
WHERE id = @id AND status = ANY(@from_states::text[])
RETURNING *;

-- name: DueScheduledCampaigns :many
UPDATE campaigns SET status = 'preparing', started_at = coalesce(started_at, now()), updated_at = now()
WHERE status = 'scheduled' AND scheduled_at <= now()
RETURNING id;

-- name: SetMaterializeCursor :exec
UPDATE campaigns SET materialize_cursor = @cursor, total_recipients = total_recipients + @added, updated_at = now()
WHERE id = @id;

-- name: InsertCampaignEvent :exec
INSERT INTO campaign_events (campaign_id, kind, detail, actor_id) VALUES (@campaign_id, @kind, @detail, sqlc.narg(actor_id));

-- name: ListCampaignEvents :many
SELECT * FROM campaign_events WHERE campaign_id = @campaign_id ORDER BY id DESC LIMIT 200;

-- name: InsertRecipient :execrows
INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164, state, skip_reason)
VALUES (@campaign_id, @contact_id, @phone_e164, @state, sqlc.narg(skip_reason))
ON CONFLICT (campaign_id, phone_e164) DO NOTHING;

-- name: ClaimPendingRecipients :many
SELECT r.id, r.contact_id, r.phone_e164 FROM campaign_recipients r
WHERE r.campaign_id = @campaign_id AND r.state = 'pending'
ORDER BY r.id LIMIT @lim
FOR UPDATE SKIP LOCKED;

-- name: MarkRecipientQueued :exec
UPDATE campaign_recipients SET state = 'queued', message_id = @message_id, updated_at = now() WHERE id = @id;

-- name: MarkRecipientSkipped :exec
UPDATE campaign_recipients SET state = 'skipped', skip_reason = @skip_reason, updated_at = now() WHERE id = @id;

-- name: CancelPendingRecipients :execrows
UPDATE campaign_recipients SET state = 'cancelled', updated_at = now() WHERE campaign_id = @campaign_id AND state = 'pending';

-- name: CountInFlight :one
SELECT count(*) FROM messages WHERE campaign_id = @campaign_id AND status IN ('pending', 'sending');

-- name: CountPendingRecipients :one
SELECT count(*) FROM campaign_recipients WHERE campaign_id = @campaign_id AND state = 'pending';

-- name: CountHeld :one
SELECT count(*) FROM messages WHERE campaign_id = @campaign_id AND status = 'held';

-- name: ReleaseHeldMessages :many
UPDATE messages SET status = 'pending', status_rank = 0, updated_at = now()
WHERE campaign_id = @campaign_id AND status = 'held'
RETURNING id;

-- name: CancelUnsentMessages :execrows
UPDATE messages SET status = 'cancelled', updated_at = now()
WHERE campaign_id = @campaign_id AND status IN ('pending', 'held');

-- name: HoldIfCampaignStopped :one
-- Called before a campaign message is claimed: a paused campaign holds it,
-- a cancelled or failed one cancels it.
UPDATE messages m SET status = CASE WHEN c.status = 'paused' THEN 'held' ELSE 'cancelled' END, updated_at = now()
FROM campaigns c
WHERE m.id = @id AND m.campaign_id = c.id AND m.status = 'pending'
  AND c.status IN ('paused', 'cancelled', 'failed')
RETURNING m.status;

-- name: CampaignRecipientStats :many
SELECT state, count(*) AS n FROM campaign_recipients WHERE campaign_id = @campaign_id GROUP BY state;

-- name: CampaignMessageStats :many
SELECT status, count(*) AS n FROM messages WHERE campaign_id = @campaign_id GROUP BY status;

-- name: ListRecipients :many
SELECT r.*, m.status AS message_status, m.error_code, m.error_title
FROM campaign_recipients r LEFT JOIN messages m ON m.id = r.message_id
WHERE r.campaign_id = @campaign_id
  AND (sqlc.narg(state)::text IS NULL OR r.state = sqlc.narg(state)::text)
  AND (sqlc.narg(before_id)::uuid IS NULL OR r.id < sqlc.narg(before_id)::uuid)
ORDER BY r.id DESC LIMIT @lim;

-- name: StuckSendingMessages :many
-- Claimed more than the given age ago and never settled: the worker died.
SELECT id FROM messages WHERE status = 'sending' AND updated_at < now() - make_interval(secs => @older_than_seconds::double precision)
LIMIT 500;

-- name: StalePendingMessages :many
-- Pending for longer than expected: its send task may have been lost.
SELECT id FROM messages WHERE status = 'pending' AND updated_at < now() - make_interval(secs => @older_than_seconds::double precision)
ORDER BY updated_at LIMIT 1000;

-- name: TouchMessage :exec
UPDATE messages SET updated_at = now() WHERE id = @id AND status = 'pending';

-- name: IdleRunningCampaigns :many
-- Running campaigns whose dispatcher has not moved for a while.
SELECT id FROM campaigns WHERE status = 'running' AND updated_at < now() - make_interval(secs => @older_than_seconds::double precision);

-- name: TouchCampaign :exec
UPDATE campaigns SET updated_at = now() WHERE id = @id;

-- name: StalePreparingCampaigns :many
SELECT id FROM campaigns WHERE status = 'preparing' AND updated_at < now() - make_interval(secs => @older_than_seconds::double precision);

-- name: InsertCampaignMessage :one
INSERT INTO messages (id, phone_number_id, contact_id, direction, type, body, payload, status, status_rank, source, campaign_id)
VALUES (@id, @phone_number_id, @contact_id, 'outbound', 'template', @body, @payload, 'pending', 0, 'campaign', @campaign_id)
RETURNING *;

-- name: ClaimCampaignProgress :one
-- Returns the campaign when its progress may be published now; otherwise
-- marks it dirty for the reaper and returns no row.
WITH claim AS (
    UPDATE campaigns c SET progress_published_at = now(), progress_dirty = false
    WHERE c.id = @id AND (c.progress_published_at IS NULL OR c.progress_published_at < now() - interval '2 seconds')
    RETURNING c.*
), dirty AS (
    UPDATE campaigns d SET progress_dirty = true
    WHERE d.id = @id AND NOT EXISTS (SELECT 1 FROM claim)
)
SELECT * FROM claim;

-- name: TakeDirtyCampaigns :many
UPDATE campaigns SET progress_dirty = false, progress_published_at = now()
WHERE progress_dirty
RETURNING *;

-- name: ListCampaignsChangedSince :many
-- Realtime re-sync after missed events: campaigns changed at or after since,
-- and every active one (their counters change without touching the row).
SELECT * FROM campaigns
WHERE (updated_at >= @since OR status IN ('preparing', 'running'))
  AND (sqlc.narg(id)::uuid IS NULL OR id = sqlc.narg(id)::uuid)
ORDER BY updated_at, id
LIMIT @lim;
