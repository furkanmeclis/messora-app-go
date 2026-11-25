-- ── Offers (Phase 16C P4) ───────────────────────────────────────────────────

-- name: InsertOffer :one
INSERT INTO offers (title, description, discount_text, product, starts_at, ends_at, image_url, image_media_id,
                    cta_url, coupon_code, template_id, status, created_by)
VALUES (@title, @description, @discount_text, @product, @starts_at, @ends_at, sqlc.narg(image_url), sqlc.narg(image_media_id),
        sqlc.narg(cta_url), sqlc.narg(coupon_code), sqlc.narg(template_id), @status, sqlc.narg(created_by))
RETURNING *;

-- name: GetOffer :one
SELECT * FROM offers WHERE id = @id;

-- name: LockOffer :one
SELECT * FROM offers WHERE id = @id FOR UPDATE;

-- name: ListOffers :many
SELECT * FROM offers
WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
ORDER BY ends_at DESC, id DESC
LIMIT 500;

-- name: UpdateOffer :one
-- Replaces every editable field (the service merges the request first).
UPDATE offers SET
    title          = @title,
    description    = @description,
    discount_text  = @discount_text,
    product        = @product,
    starts_at      = @starts_at,
    ends_at        = @ends_at,
    image_url      = sqlc.narg(image_url),
    image_media_id = sqlc.narg(image_media_id),
    cta_url        = sqlc.narg(cta_url),
    coupon_code    = sqlc.narg(coupon_code),
    template_id    = sqlc.narg(template_id),
    status         = @status,
    updated_at     = now()
WHERE id = @id
RETURNING *;

-- name: DeleteOffer :execrows
DELETE FROM offers WHERE id = @id;

-- name: ActivateDueOffers :many
-- scheduled → active once starts_at passed (and the offer has not ended).
UPDATE offers SET status = 'active', updated_at = now()
WHERE status = 'scheduled' AND starts_at <= now() AND ends_at > now()
RETURNING *;

-- name: ExpireDueOffers :many
-- scheduled or active → expired once ends_at passed.
UPDATE offers SET status = 'expired', updated_at = now()
WHERE status IN ('scheduled', 'active') AND ends_at <= now()
RETURNING *;

-- name: CampaignsUsingOffer :many
SELECT id, status FROM campaigns WHERE offer_id = @offer_id;

-- name: SetCampaignOffer :exec
UPDATE campaigns SET offer_id = sqlc.narg(offer_id), updated_at = now()
WHERE id = @id AND status IN ('draft', 'scheduled');
