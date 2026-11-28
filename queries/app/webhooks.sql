-- name: InsertWebhookEndpoint :one
INSERT INTO webhook_endpoints (id, url, description, event_types, active, secret_enc, secret_last4, created_by)
VALUES (@id, @url, @description, @event_types::text[], @active, @secret_enc, @secret_last4, sqlc.narg(created_by))
RETURNING *;

-- name: GetWebhookEndpoint :one
SELECT * FROM webhook_endpoints WHERE id = @id;

-- name: ListWebhookEndpoints :many
SELECT * FROM webhook_endpoints ORDER BY id DESC;

-- name: CountWebhookEndpoints :one
SELECT count(*) FROM webhook_endpoints;

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints SET
    url             = coalesce(sqlc.narg(url), url),
    description     = coalesce(sqlc.narg(description), description),
    event_types     = coalesce(sqlc.narg(event_types)::text[], event_types),
    active          = coalesce(sqlc.narg(active), active),
    -- Switching an endpoint back on starts its health over.
    disabled_reason = CASE WHEN sqlc.narg(active)::boolean THEN NULL ELSE disabled_reason END,
    disabled_at     = CASE WHEN sqlc.narg(active)::boolean THEN NULL ELSE disabled_at END,
    failure_streak  = CASE WHEN sqlc.narg(active)::boolean AND NOT active THEN 0 ELSE failure_streak END,
    failing_since   = CASE WHEN sqlc.narg(active)::boolean AND NOT active THEN NULL ELSE failing_since END,
    updated_at      = now()
WHERE id = @id
RETURNING *;

-- name: SetWebhookEndpointSecret :one
-- Rotation: with a grace period the current secret becomes the previous
-- one (deliveries carry both signatures until it expires); without one the
-- previous secret is dropped at once.
UPDATE webhook_endpoints SET
    previous_secret_enc        = CASE WHEN @grace_seconds::float8 > 0 THEN secret_enc END,
    previous_secret_expires_at = CASE WHEN @grace_seconds::float8 > 0 THEN now() + make_interval(secs => @grace_seconds::float8) END,
    secret_enc = @secret_enc, secret_last4 = @secret_last4, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DisableWebhookEndpoint :execrows
-- Switches an active endpoint off; 0 rows when it already was off (so the
-- admins are told once).
UPDATE webhook_endpoints SET active = false, disabled_reason = @reason, disabled_at = now(), updated_at = now()
WHERE id = @id AND active;

-- name: RecordWebhookSuccess :exec
UPDATE webhook_endpoints SET failure_streak = 0, failing_since = NULL, last_success_at = now()
WHERE id = @id;

-- name: RecordWebhookFailure :one
-- Counts a failed attempt of an active endpoint and returns the run of
-- failures it belongs to.
UPDATE webhook_endpoints SET failure_streak = failure_streak + 1,
    failing_since = coalesce(failing_since, now()), last_failure_at = now()
WHERE id = @id AND active
RETURNING failure_streak, failing_since;

-- name: DeleteWebhookEndpoint :execrows
DELETE FROM webhook_endpoints WHERE id = @id;

-- name: WebhookTypeSubscribed :one
-- Asked by emitters on every domain event; served by a partial GIN index.
SELECT EXISTS (SELECT 1 FROM webhook_endpoints WHERE active AND event_types @> ARRAY[@event_type::text]);

-- name: InsertOutgoingWebhookEvent :exec
INSERT INTO outgoing_webhook_events (id, type, payload) VALUES (@id, @type, @payload);

-- name: GetOutgoingWebhookEvent :one
SELECT * FROM outgoing_webhook_events WHERE id = @id;

-- name: FanOutWebhookEvent :many
-- One delivery per subscribed active endpoint; running it again for the
-- same event inserts nothing.
INSERT INTO webhook_deliveries (event_id, endpoint_id)
SELECT @event_id, ep.id FROM webhook_endpoints ep
WHERE ep.active AND ep.event_types @> ARRAY[@event_type::text]
ON CONFLICT (event_id, endpoint_id) DO NOTHING
RETURNING id;

-- name: InsertWebhookDelivery :one
INSERT INTO webhook_deliveries (event_id, endpoint_id) VALUES (@event_id, @endpoint_id)
RETURNING *;

-- name: GetWebhookDelivery :one
SELECT * FROM webhook_deliveries WHERE id = @id;

-- name: ClaimWebhookDelivery :one
-- Takes a due delivery, or one whose previous worker vanished mid-request.
UPDATE webhook_deliveries SET status = 'delivering', attempts = attempts + 1,
    locked_until = now() + make_interval(secs => @lease_seconds::float8),
    last_attempt_at = now(), updated_at = now()
WHERE id = @id
  AND ((status IN ('pending', 'retrying') AND next_attempt_at <= now() + interval '5 seconds')
       OR (status = 'delivering' AND locked_until < now()))
RETURNING *;

-- name: FinishWebhookDelivery :execrows
UPDATE webhook_deliveries SET status = @status, locked_until = NULL,
    next_attempt_at    = coalesce(sqlc.narg(next_attempt_at), next_attempt_at),
    last_status_code   = sqlc.narg(last_status_code),
    last_duration_ms   = sqlc.narg(last_duration_ms),
    last_error         = sqlc.narg(last_error),
    last_response_body = sqlc.narg(last_response_body),
    delivered_at       = CASE WHEN @status::text = 'succeeded' THEN now() ELSE delivered_at END,
    updated_at         = now()
WHERE id = @id AND status = 'delivering';

-- name: ResetWebhookDelivery :one
-- Manual redelivery: a fresh attempt budget, due now.
UPDATE webhook_deliveries SET status = 'pending', attempts = 0, next_attempt_at = now(), locked_until = NULL, updated_at = now()
WHERE id = @id AND status <> 'delivering'
RETURNING *;

-- name: ListWebhookDeliveries :many
SELECT d.*, e.type AS event_type FROM webhook_deliveries d
JOIN outgoing_webhook_events e ON e.id = d.event_id
WHERE d.endpoint_id = @endpoint_id
  AND (sqlc.narg(status)::text IS NULL OR d.status = sqlc.narg(status)::text)
  AND (sqlc.narg(before_id)::uuid IS NULL OR d.id < sqlc.narg(before_id)::uuid)
ORDER BY d.id DESC LIMIT @lim;

-- name: DeleteOldOutgoingWebhookEvents :execrows
-- Retention; deliveries go with their event.
DELETE FROM outgoing_webhook_events
WHERE id IN (SELECT old.id FROM outgoing_webhook_events old WHERE old.created_at < @cutoff::timestamptz ORDER BY old.created_at LIMIT @batch_size);
