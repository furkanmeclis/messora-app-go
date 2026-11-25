-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 16C P4: time-limited offers. See docs/WHATSAPP.md ("Offers").
--
-- An offer is what a campaign promotes until it ends. Meta's limited-time
-- offer (LTO) templates show a countdown to expiration_time_ms, which
-- campaigns fill from ends_at, and the copy code button takes coupon_code.
-- What Meta has no field for (start, product, description) stays here.
--   status  draft     not published; the scheduler ignores it
--           scheduled published, starts_at in the future
--           active    between starts_at and ends_at
--           expired   ends_at passed (terminal; campaigns using it stop)
-- The scheduler moves scheduled → active → expired every minute.
CREATE TABLE offers (
    id             uuid PRIMARY KEY DEFAULT uuidv7(),
    title          text NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    description    text NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    discount_text  text NOT NULL DEFAULT '' CHECK (length(discount_text) <= 60),
    product        text NOT NULL DEFAULT '' CHECK (length(product) <= 200),
    starts_at      timestamptz NOT NULL,
    ends_at        timestamptz NOT NULL,
    -- Public https image (shown in the panel and used as a template's
    -- image header) and/or a WhatsApp media ID of the same image.
    image_url      text CHECK (image_url IS NULL OR (image_url ~ '^https://' AND length(image_url) <= 2000)),
    image_media_id text CHECK (image_media_id IS NULL OR length(image_media_id) <= 200),
    cta_url        text CHECK (cta_url IS NULL OR (cta_url ~ '^https?://' AND length(cta_url) <= 2000)),
    coupon_code    text CHECK (coupon_code IS NULL OR length(coupon_code) BETWEEN 1 AND 20),
    template_id    uuid REFERENCES message_templates (id) ON DELETE SET NULL,
    status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'scheduled', 'active', 'expired')),
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX offers_due_idx ON offers (status, starts_at, ends_at) WHERE status IN ('scheduled', 'active');

-- The offer a campaign promotes. New column, so every row is NULL: the
-- constraint holds without a validating scan.
ALTER TABLE campaigns ADD COLUMN offer_id uuid;
-- squawk-ignore adding-foreign-key-constraint
ALTER TABLE campaigns ADD CONSTRAINT campaigns_offer_id_fkey FOREIGN KEY (offer_id) REFERENCES offers (id) ON DELETE SET NULL NOT VALID;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS offer_id;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS offers;
