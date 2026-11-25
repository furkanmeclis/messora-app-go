-- name: InsertCampaign :one
INSERT INTO campaigns (name, phone_number_id, template_name, template_language, template_params, audience, window_size, created_by, offer_id, priority, max_mps)
VALUES (@name, @phone_number_id, @template_name, @template_language, @template_params, @audience, @window_size, sqlc.narg(created_by), sqlc.narg(offer_id), @priority, sqlc.narg(max_mps))
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
    priority          = coalesce(sqlc.narg(priority), priority),
    max_mps           = CASE WHEN @clear_max_mps::boolean THEN NULL ELSE coalesce(sqlc.narg(max_mps), max_mps) END,
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
SELECT r.id, r.contact_id, r.phone_e164, r.message_id, r.state FROM campaign_recipients r
WHERE r.campaign_id = @campaign_id AND r.state = 'pending'
ORDER BY r.id LIMIT @lim
FOR UPDATE SKIP LOCKED;

-- name: ClaimDueRetries :many
-- Recipients whose withheld (131049) message may be sent again: a new
-- message replaces message_id.
SELECT r.id, r.contact_id, r.phone_e164, r.message_id, r.state FROM campaign_recipients r
WHERE r.campaign_id = @campaign_id AND r.state = 'retry' AND r.next_attempt_at <= now()
ORDER BY r.next_attempt_at, r.id LIMIT @lim
FOR UPDATE SKIP LOCKED;

-- name: MarkRecipientQueued :exec
UPDATE campaign_recipients SET state = 'queued', message_id = @message_id,
    resends = resends + CASE WHEN state = 'retry' THEN 1 ELSE 0 END, next_attempt_at = NULL, updated_at = now()
WHERE id = @id;

-- name: ScheduleRecipientResend :one
-- Meta withheld an accepted campaign message (131049 in the status
-- webhook): its recipient gets a new message no earlier than after_seconds,
-- once, while the campaign is still running or paused and the recipient has
-- resends left.
UPDATE campaign_recipients r SET state = 'retry',
    next_attempt_at = now() + make_interval(secs => @after_seconds::double precision), updated_at = now()
FROM campaigns c
WHERE r.message_id = @message_id AND r.state = 'queued' AND r.resends < @max_resends::integer
  AND c.id = r.campaign_id AND (c.status = 'running' OR c.status LIKE 'paused%')
RETURNING r.id, r.campaign_id, r.next_attempt_at;

-- name: CountRetries :one
SELECT count(*) FILTER (WHERE next_attempt_at <= now()) AS due, count(*) AS total
FROM campaign_recipients WHERE campaign_id = @campaign_id AND state = 'retry';

-- name: MarkRecipientSkipped :exec
UPDATE campaign_recipients SET state = 'skipped', skip_reason = @skip_reason, updated_at = now() WHERE id = @id;

-- name: CancelPendingRecipients :execrows
UPDATE campaign_recipients SET state = 'cancelled', next_attempt_at = NULL, updated_at = now()
WHERE campaign_id = @campaign_id AND state IN ('pending', 'retry');

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
UPDATE messages SET status = 'cancelled', failure_class = 'cancelled', next_attempt_at = NULL, updated_at = now()
WHERE campaign_id = @campaign_id AND status IN ('pending', 'held', 'deferred');

-- name: CountDeferred :one
SELECT count(*) FROM messages WHERE campaign_id = @campaign_id AND status = 'deferred';

-- name: DueDeferredMessages :many
-- Deferred messages whose retry time passed a while ago: their send task
-- may have been lost.
SELECT id, campaign_id FROM messages
WHERE status = 'deferred' AND next_attempt_at < now() - make_interval(secs => @older_than_seconds::double precision)
ORDER BY next_attempt_at LIMIT 1000;

-- name: HoldIfCampaignStopped :one
-- Called before a campaign message is claimed: a paused campaign holds it,
-- a cancelled or failed one cancels it.
-- Every pause holds (paused, and later pause kinds such as an expired
-- offer) except paused_limit: messages queued before the portfolio limit
-- was reached were within it and still go out.
UPDATE messages m SET status = CASE WHEN c.status LIKE 'paused%' THEN 'held' ELSE 'cancelled' END,
    failure_class = CASE WHEN c.status LIKE 'paused%' THEN m.failure_class ELSE 'cancelled' END,
    next_attempt_at = NULL, updated_at = now()
FROM campaigns c
WHERE m.id = @id AND m.campaign_id = c.id
  AND (m.status = 'pending' OR (m.status = 'deferred' AND m.next_attempt_at <= now()))
  AND ((c.status LIKE 'paused%' AND c.status <> 'paused_limit') OR c.status IN ('cancelled', 'failed'))
RETURNING m.status;

-- name: CampaignRecipientStats :many
SELECT state, count(*) AS n FROM campaign_recipients WHERE campaign_id = @campaign_id GROUP BY state;

-- name: CampaignMessageStats :many
SELECT status, count(*) AS n FROM messages WHERE campaign_id = @campaign_id GROUP BY status;

-- name: ListRecipients :many
SELECT r.*, m.status AS message_status, m.error_code, m.error_title, m.failure_class,
    coalesce(campaign_recipient_class(r.state, r.skip_reason, m.status, m.failure_class), '')::text AS unreached_class
FROM campaign_recipients r LEFT JOIN messages m ON m.id = r.message_id
WHERE r.campaign_id = @campaign_id
  AND (sqlc.narg(state)::text IS NULL OR r.state = sqlc.narg(state)::text)
  AND (sqlc.narg(class)::text IS NULL OR campaign_recipient_class(r.state, r.skip_reason, m.status, m.failure_class) = sqlc.narg(class)::text)
  AND (sqlc.narg(before_id)::uuid IS NULL OR r.id < sqlc.narg(before_id)::uuid)
ORDER BY r.id DESC LIMIT @lim;

-- name: StuckSendingMessages :many
-- Claimed more than the given age ago and never settled: the worker died.
SELECT id FROM messages WHERE status = 'sending' AND updated_at < now() - make_interval(secs => @older_than_seconds::double precision)
LIMIT 500;

-- name: StalePendingMessages :many
-- Pending for longer than expected: its send task may have been lost.
SELECT id, campaign_id FROM messages WHERE status = 'pending' AND updated_at < now() - make_interval(secs => @older_than_seconds::double precision)
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
INSERT INTO messages (id, phone_number_id, contact_id, direction, type, body, payload, status, status_rank, source, campaign_id, consent_category, retry_of)
VALUES (@id, @phone_number_id, @contact_id, 'outbound', 'template', @body, @payload, 'pending', 0, 'campaign', @campaign_id, @consent_category, sqlc.narg(retry_of))
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
WHERE (updated_at >= @since OR status IN ('preparing', 'running', 'paused_limit'))
  AND (sqlc.narg(id)::uuid IS NULL OR id = sqlc.narg(id)::uuid)
ORDER BY updated_at, id
LIMIT @lim;

-- name: DueResumes :many
-- Paused campaigns whose resume time has come: paused_limit when the
-- portfolio limit frees up, any pause with a resume scheduled by an
-- operator.
SELECT id FROM campaigns WHERE resume_at <= now() AND status LIKE 'paused%' ORDER BY resume_at LIMIT 100;

-- name: SetCampaignPauseInfo :one
UPDATE campaigns SET paused_reason = @reason, paused_at = now(), paused_by = sqlc.narg(paused_by), resume_at = sqlc.narg(resume_at), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: ClearCampaignPauseInfo :one
UPDATE campaigns SET paused_reason = NULL, paused_at = NULL, paused_by = NULL, resume_at = NULL, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetCampaignResumeAt :one
UPDATE campaigns SET resume_at = sqlc.narg(resume_at), updated_at = now()
WHERE id = @id AND status LIKE 'paused%'
RETURNING *;

-- name: InsertCampaignPause :one
INSERT INTO campaign_pauses (campaign_id, status, reason, detail, paused_by, resume_at, snapshot)
VALUES (@campaign_id, @status, @reason, @detail, sqlc.narg(paused_by), sqlc.narg(resume_at), @snapshot)
RETURNING *;

-- name: CloseCampaignPause :exec
-- The open pause of a campaign ends with its resume.
UPDATE campaign_pauses SET resumed_at = now(), resumed_by = sqlc.narg(resumed_by), changes = sqlc.narg(changes)
WHERE id = (SELECT p.id FROM campaign_pauses p WHERE p.campaign_id = @campaign_id AND p.resumed_at IS NULL ORDER BY p.id DESC LIMIT 1);

-- name: ListCampaignPauses :many
SELECT * FROM campaign_pauses WHERE campaign_id = @campaign_id ORDER BY id DESC LIMIT 100;

-- name: UpdateCampaignSettings :one
-- What may change while a campaign is in progress (and when resuming).
UPDATE campaigns SET
    name     = coalesce(sqlc.narg(name), name),
    priority = coalesce(sqlc.narg(priority), priority),
    max_mps  = CASE WHEN @clear_max_mps::boolean THEN NULL ELSE coalesce(sqlc.narg(max_mps), max_mps) END,
    updated_at = now()
WHERE id = @id AND (status IN ('draft', 'scheduled', 'preparing', 'running') OR status LIKE 'paused%')
RETURNING *;

-- name: ContendingCampaigns :many
-- Running campaigns on a number that still have recipients to send to: the
-- contenders for the number's campaign capacity.
SELECT c.id, c.priority, c.max_mps FROM campaigns c
WHERE c.phone_number_id = @phone_number_id AND c.status = 'running'
  AND (EXISTS (SELECT 1 FROM campaign_recipients r WHERE r.campaign_id = c.id AND r.state = 'pending')
       OR EXISTS (SELECT 1 FROM campaign_recipients r WHERE r.campaign_id = c.id AND r.state = 'retry' AND r.next_attempt_at <= now()))
ORDER BY c.id;

-- name: RecordDispatch :one
-- Counts messages a dispatcher tick queued and, every two seconds, turns
-- them into the measured rate; rolled = true when a new rate was computed
-- (live progress is published then).
WITH cur AS (
    SELECT k.id, (k.rate_window_start IS NOT NULL AND now() - k.rate_window_start >= interval '2 seconds') AS roll,
           k.rate_window_start, k.rate_window_queued
    FROM campaigns k WHERE k.id = @id
)
UPDATE campaigns c SET
    send_rate = CASE WHEN cur.roll
                     THEN (cur.rate_window_queued + @queued::bigint)::double precision / extract(epoch FROM now() - cur.rate_window_start)::double precision
                     ELSE c.send_rate END,
    rate_window_queued = CASE WHEN cur.roll OR cur.rate_window_start IS NULL THEN 0 ELSE cur.rate_window_queued + @queued::bigint END,
    rate_window_start  = CASE WHEN cur.roll OR cur.rate_window_start IS NULL THEN now() ELSE cur.rate_window_start END,
    updated_at = now()
FROM cur WHERE c.id = cur.id
RETURNING c.send_rate, cur.roll AS rolled;

-- name: CampaignQueue :many
-- Campaigns that are sending or waiting for the portfolio limit, with what
-- is left (the queue view per number).
SELECT c.id, c.name, c.status, c.phone_number_id, c.priority, c.max_mps, c.send_rate, c.paused_reason, c.resume_at,
    c.total_recipients, c.started_at,
    (SELECT count(*) FROM campaign_recipients r WHERE r.campaign_id = c.id AND r.state IN ('pending', 'retry'))::bigint AS waiting,
    (SELECT count(*) FROM messages m WHERE m.campaign_id = c.id AND m.status IN ('pending', 'sending', 'held'))::bigint AS in_flight
FROM campaigns c
WHERE c.status IN ('preparing', 'running', 'paused_limit')
ORDER BY c.phone_number_id, c.id;

-- name: RemainingConsentGroups :many
-- The recipients a resume would still send to (pending, waiting for a
-- resend, and held messages) grouped by their contacts' consent, for the
-- pre-resume summary.
SELECT c.marketing_consent, c.service_consent, (c.opted_out_at IS NOT NULL)::boolean AS opted_out,
    (c.meta_marketing_stopped_at IS NOT NULL)::boolean AS stopped, (c.undeliverable_at IS NOT NULL)::boolean AS undeliverable,
    count(*) AS n
FROM campaign_recipients r JOIN contacts c ON c.id = r.contact_id
LEFT JOIN messages m ON m.id = r.message_id
WHERE r.campaign_id = @campaign_id
  AND (r.state IN ('pending', 'retry') OR (r.state = 'queued' AND m.status = 'held'))
GROUP BY 1, 2, 3, 4, 5;

-- name: CampaignClassCounts :many
-- Recipients of a campaign by why they were not reached (unique contacts;
-- class NULL = reached).
SELECT coalesce(campaign_recipient_class(r.state, r.skip_reason, m.status, m.failure_class), 'reached')::text AS class,
    count(DISTINCT r.contact_id) AS n
FROM campaign_recipients r LEFT JOIN messages m ON m.id = r.message_id
WHERE r.campaign_id = @campaign_id AND r.contact_id IS NOT NULL
GROUP BY 1;

-- name: RecordPortfolioReach :exec
-- A template reached a customer outside their service window: they count
-- against the portfolio's messaging limit for 24 hours.
INSERT INTO portfolio_reach (account_id, contact_id, last_at)
SELECT p.account_id, @contact_id, now() FROM phone_numbers p WHERE p.id = @phone_number_id
ON CONFLICT (account_id, contact_id) DO UPDATE SET last_at = greatest(portfolio_reach.last_at, excluded.last_at);

-- name: PortfolioUsage :one
-- Unique customers the account messaged outside their window in the last
-- 24 hours, and when the oldest of them stops counting.
SELECT count(*) AS used, coalesce(min(last_at) + interval '24 hours', now())::timestamptz AS frees_at
FROM portfolio_reach WHERE account_id = @account_id AND last_at > now() - interval '24 hours';

-- name: PortfolioInFlight :one
-- Campaign messages of the account that are queued or being sent: they
-- count once accepted.
SELECT count(*) FROM messages m JOIN phone_numbers p ON p.id = m.phone_number_id
WHERE p.account_id = @account_id AND m.campaign_id IS NOT NULL AND m.status IN ('pending', 'sending');

-- name: ReachedRecently :one
-- Whether a customer already counts against the limit (another template to
-- them within the day costs no extra capacity).
SELECT EXISTS (SELECT 1 FROM portfolio_reach WHERE account_id = @account_id AND contact_id = @contact_id
    AND last_at > now() - interval '24 hours');

-- name: LockPortfolio :exec
-- Serialises the limit checks of campaigns sharing an account.
SELECT pg_advisory_xact_lock(hashtextextended(concat('portfolio:', sqlc.arg(account_id)::uuid), 0));
