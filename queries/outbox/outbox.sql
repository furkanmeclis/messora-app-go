-- name: InsertOutboxEvent :one
-- The database clock is the only clock: delays are relative to now() so
-- skew between application hosts and PostgreSQL cannot hide events.
-- tenant_id is the writer's tenant; row-level security rejects any other.
INSERT INTO outbox_events (tenant_id, topic, payload, available_at)
VALUES (sqlc.narg(tenant_id)::uuid, @topic, @payload, now() + make_interval(secs => @delay_seconds::double precision))
RETURNING id;

-- name: ClaimOutboxBatch :many
-- Rows stay locked until the surrounding transaction ends, so concurrent
-- relays never publish the same event twice in parallel.
SELECT id, tenant_id, topic, payload, attempts
FROM outbox_events
WHERE published_at IS NULL
  AND available_at <= now()
ORDER BY id
LIMIT @batch_size
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE outbox_events
SET published_at = now(), attempts = attempts + 1, last_error = NULL
WHERE id = ANY(@ids::bigint[]);

-- name: MarkOutboxFailed :exec
UPDATE outbox_events
SET attempts = attempts + 1,
    last_error = @last_error,
    available_at = now() + make_interval(secs => @retry_in_seconds::double precision)
WHERE id = @id;

-- name: DeletePublishedOutboxBefore :execrows
DELETE FROM outbox_events
WHERE id IN (
    SELECT o.id FROM outbox_events o
    WHERE o.published_at < @cutoff
    ORDER BY o.id
    LIMIT @batch_size
);

-- name: CountUnpublishedOutbox :one
SELECT count(*) FROM outbox_events WHERE published_at IS NULL;
