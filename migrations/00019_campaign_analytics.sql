-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- Phase 16C P3: campaign analytics, forecasts, reports and spam protection.
-- See docs/CAMPAIGN_ANALYTICS.md.
--
-- Recipients keep their outcome queryable without joining messages: the
-- status of their current message, when it was attempted, sent, delivered,
-- read or failed (mirrored from messages by a trigger in the same
-- transaction), its error, its cost, and what the customer did afterwards
-- within the attribution window: replied (first inbound message), reacted
-- (positive / negative / neutral, from a quick reply mapping, an AI
-- classification or by hand) and opted out of marketing.
ALTER TABLE campaign_recipients ADD COLUMN msg_status text;
ALTER TABLE campaign_recipients ADD COLUMN attempted_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN sent_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN delivered_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN read_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN failed_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN error_code text;
ALTER TABLE campaign_recipients ADD COLUMN failure_class text;
ALTER TABLE campaign_recipients ADD COLUMN cost_amount numeric(12, 6);
ALTER TABLE campaign_recipients ADD COLUMN cost_currency text;
ALTER TABLE campaign_recipients ADD COLUMN replied_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN reply_message_id uuid;
-- The quick reply button the customer tapped (payload, else its text).
ALTER TABLE campaign_recipients ADD COLUMN reply_payload text;
ALTER TABLE campaign_recipients ADD COLUMN reaction text;
ALTER TABLE campaign_recipients ADD COLUMN reaction_source text;
ALTER TABLE campaign_recipients ADD COLUMN reaction_at timestamptz;
ALTER TABLE campaign_recipients ADD COLUMN opted_out_at timestamptz;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_reaction_check
    CHECK (reaction IS NULL OR reaction IN ('positive', 'negative', 'neutral')) NOT VALID;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_reaction_source_check
    CHECK (reaction_source IS NULL OR reaction_source IN ('quick_reply', 'ai', 'manual')) NOT VALID;

-- Quick reply payloads (or button texts) → positive | negative | neutral.
-- squawk-ignore adding-field-with-default
ALTER TABLE campaigns ADD COLUMN reaction_map jsonb NOT NULL DEFAULT '{}'::jsonb;

-- What kind of customer a contact is (free text, e.g. "bireysel",
-- "kurumsal"), for breakdowns.
ALTER TABLE contacts ADD COLUMN contact_type text;
ALTER TABLE contacts ADD CONSTRAINT contacts_contact_type_check
    CHECK (contact_type IS NULL OR length(contact_type) BETWEEN 1 AND 60) NOT VALID;

-- The status-webhook path finds a recipient by its message; replies find
-- the contact's latest reached recipient; spam protection reads the
-- campaign's latest attempts.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX campaign_recipients_message_idx ON campaign_recipients (message_id) WHERE message_id IS NOT NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX campaign_recipients_contact_sent_idx ON campaign_recipients (contact_id, sent_at DESC) WHERE sent_at IS NOT NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX campaign_recipients_attempted_idx ON campaign_recipients (campaign_id, attempted_at DESC) WHERE attempted_at IS NOT NULL;

-- One word per recipient for counting: pending (not yet attempted or in
-- flight), sent, delivered, read, failed, unknown, blocked (skipped by
-- consent or rendering), cancelled.
-- +goose StatementBegin
CREATE FUNCTION campaign_recipient_outcome(state text, msg_status text)
RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE
        WHEN state = 'skipped' THEN 'blocked'
        WHEN state = 'cancelled' THEN 'cancelled'
        WHEN state IN ('pending', 'retry') OR msg_status IS NULL THEN 'pending'
        WHEN msg_status IN ('pending', 'sending', 'held', 'deferred') THEN 'pending'
        WHEN msg_status = 'skipped' THEN 'blocked'
        WHEN msg_status = 'cancelled' THEN 'cancelled'
        WHEN msg_status IN ('sent', 'delivered', 'read', 'failed', 'unknown') THEN msg_status
        ELSE 'pending'
    END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION campaign_ctr_d(now_cond boolean, before_cond boolean)
RETURNS bigint LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT coalesce(now_cond::int, 0)::bigint - coalesce(before_cond::int, 0)::bigint
$$;
-- +goose StatementEnd

-- Campaign-level counters, maintained incrementally by triggers on
-- campaign_recipients and messages in the transaction that changes them (no
-- count(*) per status webhook). campaign_counters_recompute repairs drift;
-- the reconciler job runs it for active and recent campaigns.
--   recipients, r_*   recipients by state
--   o_*               recipients by outcome (campaign_recipient_outcome)
--   sent/delivered/read  recipients that ever reached the status
--   rejected          recipients whose message failed with a Meta error code
--   replied, positive/negative/neutral, opted_out   within the attribution window
--   m_*               campaign messages by status (resends included)
--   err_*             campaign messages whose error is 131048/131049/131050
--   cost, priced, unpriced  sum of message costs; messages with a cost;
--                     billable messages without a matching rate
CREATE TABLE campaign_counters (
    campaign_id   uuid PRIMARY KEY REFERENCES campaigns (id) ON DELETE CASCADE,
    recipients    bigint NOT NULL DEFAULT 0,
    r_pending     bigint NOT NULL DEFAULT 0,
    r_queued      bigint NOT NULL DEFAULT 0,
    r_skipped     bigint NOT NULL DEFAULT 0,
    r_cancelled   bigint NOT NULL DEFAULT 0,
    r_retry       bigint NOT NULL DEFAULT 0,
    o_pending     bigint NOT NULL DEFAULT 0,
    o_sent        bigint NOT NULL DEFAULT 0,
    o_delivered   bigint NOT NULL DEFAULT 0,
    o_read        bigint NOT NULL DEFAULT 0,
    o_failed      bigint NOT NULL DEFAULT 0,
    o_unknown     bigint NOT NULL DEFAULT 0,
    o_blocked     bigint NOT NULL DEFAULT 0,
    o_cancelled   bigint NOT NULL DEFAULT 0,
    sent          bigint NOT NULL DEFAULT 0,
    delivered     bigint NOT NULL DEFAULT 0,
    read          bigint NOT NULL DEFAULT 0,
    rejected      bigint NOT NULL DEFAULT 0,
    replied       bigint NOT NULL DEFAULT 0,
    positive      bigint NOT NULL DEFAULT 0,
    negative      bigint NOT NULL DEFAULT 0,
    neutral       bigint NOT NULL DEFAULT 0,
    opted_out     bigint NOT NULL DEFAULT 0,
    m_pending     bigint NOT NULL DEFAULT 0,
    m_sending     bigint NOT NULL DEFAULT 0,
    m_unknown     bigint NOT NULL DEFAULT 0,
    m_sent        bigint NOT NULL DEFAULT 0,
    m_delivered   bigint NOT NULL DEFAULT 0,
    m_read        bigint NOT NULL DEFAULT 0,
    m_failed      bigint NOT NULL DEFAULT 0,
    m_held        bigint NOT NULL DEFAULT 0,
    m_cancelled   bigint NOT NULL DEFAULT 0,
    m_deferred    bigint NOT NULL DEFAULT 0,
    m_skipped     bigint NOT NULL DEFAULT 0,
    err_131048    bigint NOT NULL DEFAULT 0,
    err_131049    bigint NOT NULL DEFAULT 0,
    err_131050    bigint NOT NULL DEFAULT 0,
    priced        bigint NOT NULL DEFAULT 0,
    unpriced      bigint NOT NULL DEFAULT 0,
    cost          numeric(16, 6) NOT NULL DEFAULT 0,
    reconciled_at timestamptz,
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementBegin
CREATE FUNCTION campaign_recipient_counters() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    n_out text := campaign_recipient_outcome(NEW.state, NEW.msg_status);
    o_out text;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        o_out := campaign_recipient_outcome(OLD.state, OLD.msg_status);
    END IF;
    UPDATE campaign_counters SET
        recipients = recipients + (TG_OP = 'INSERT')::int,
        r_pending = r_pending + campaign_ctr_d(NEW.state = 'pending', OLD.state = 'pending'),
        r_queued = r_queued + campaign_ctr_d(NEW.state = 'queued', OLD.state = 'queued'),
        r_skipped = r_skipped + campaign_ctr_d(NEW.state = 'skipped', OLD.state = 'skipped'),
        r_cancelled = r_cancelled + campaign_ctr_d(NEW.state = 'cancelled', OLD.state = 'cancelled'),
        r_retry = r_retry + campaign_ctr_d(NEW.state = 'retry', OLD.state = 'retry'),
        o_pending = o_pending + campaign_ctr_d(n_out = 'pending', o_out = 'pending'),
        o_sent = o_sent + campaign_ctr_d(n_out = 'sent', o_out = 'sent'),
        o_delivered = o_delivered + campaign_ctr_d(n_out = 'delivered', o_out = 'delivered'),
        o_read = o_read + campaign_ctr_d(n_out = 'read', o_out = 'read'),
        o_failed = o_failed + campaign_ctr_d(n_out = 'failed', o_out = 'failed'),
        o_unknown = o_unknown + campaign_ctr_d(n_out = 'unknown', o_out = 'unknown'),
        o_blocked = o_blocked + campaign_ctr_d(n_out = 'blocked', o_out = 'blocked'),
        o_cancelled = o_cancelled + campaign_ctr_d(n_out = 'cancelled', o_out = 'cancelled'),
        sent = sent + campaign_ctr_d(NEW.sent_at IS NOT NULL, OLD.sent_at IS NOT NULL),
        delivered = delivered + campaign_ctr_d(NEW.delivered_at IS NOT NULL, OLD.delivered_at IS NOT NULL),
        read = read + campaign_ctr_d(NEW.read_at IS NOT NULL, OLD.read_at IS NOT NULL),
        rejected = rejected + campaign_ctr_d(NEW.msg_status = 'failed' AND NEW.error_code ~ '^[0-9]+$', OLD.msg_status = 'failed' AND OLD.error_code ~ '^[0-9]+$'),
        replied = replied + campaign_ctr_d(NEW.replied_at IS NOT NULL, OLD.replied_at IS NOT NULL),
        positive = positive + campaign_ctr_d(NEW.reaction = 'positive', OLD.reaction = 'positive'),
        negative = negative + campaign_ctr_d(NEW.reaction = 'negative', OLD.reaction = 'negative'),
        neutral = neutral + campaign_ctr_d(NEW.reaction = 'neutral', OLD.reaction = 'neutral'),
        opted_out = opted_out + campaign_ctr_d(NEW.opted_out_at IS NOT NULL, OLD.opted_out_at IS NOT NULL),
        updated_at = now()
    WHERE campaign_id = NEW.campaign_id;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION campaign_message_counters() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        -- The recipient row mirrors its current message.
        UPDATE campaign_recipients SET
            msg_status    = NEW.status,
            attempted_at  = coalesce(attempted_at, CASE WHEN NEW.status IN ('sent', 'delivered', 'read', 'failed', 'deferred', 'unknown') THEN now() END),
            sent_at       = coalesce(sent_at, NEW.sent_at),
            delivered_at  = coalesce(delivered_at, NEW.delivered_at),
            read_at       = coalesce(read_at, NEW.read_at),
            failed_at     = CASE WHEN NEW.status = 'failed' THEN coalesce(NEW.failed_at, now()) END,
            error_code    = NEW.error_code,
            failure_class = NEW.failure_class,
            cost_amount   = CASE WHEN NEW.cost_amount IS NULL AND OLD.cost_amount IS NULL THEN cost_amount
                                 ELSE coalesce(cost_amount, 0) + coalesce(NEW.cost_amount, 0) - coalesce(OLD.cost_amount, 0) END,
            cost_currency = coalesce(NEW.cost_currency, cost_currency)
        WHERE message_id = NEW.id AND campaign_id = NEW.campaign_id;
    END IF;
    UPDATE campaign_counters SET
        m_pending = m_pending + campaign_ctr_d(NEW.status = 'pending', OLD.status = 'pending'),
        m_sending = m_sending + campaign_ctr_d(NEW.status = 'sending', OLD.status = 'sending'),
        m_unknown = m_unknown + campaign_ctr_d(NEW.status = 'unknown', OLD.status = 'unknown'),
        m_sent = m_sent + campaign_ctr_d(NEW.status = 'sent', OLD.status = 'sent'),
        m_delivered = m_delivered + campaign_ctr_d(NEW.status = 'delivered', OLD.status = 'delivered'),
        m_read = m_read + campaign_ctr_d(NEW.status = 'read', OLD.status = 'read'),
        m_failed = m_failed + campaign_ctr_d(NEW.status = 'failed', OLD.status = 'failed'),
        m_held = m_held + campaign_ctr_d(NEW.status = 'held', OLD.status = 'held'),
        m_cancelled = m_cancelled + campaign_ctr_d(NEW.status = 'cancelled', OLD.status = 'cancelled'),
        m_deferred = m_deferred + campaign_ctr_d(NEW.status = 'deferred', OLD.status = 'deferred'),
        m_skipped = m_skipped + campaign_ctr_d(NEW.status = 'skipped', OLD.status = 'skipped'),
        err_131048 = err_131048 + campaign_ctr_d(NEW.error_code = '131048', OLD.error_code = '131048'),
        err_131049 = err_131049 + campaign_ctr_d(NEW.error_code = '131049', OLD.error_code = '131049'),
        err_131050 = err_131050 + campaign_ctr_d(NEW.error_code = '131050', OLD.error_code = '131050'),
        priced = priced + campaign_ctr_d(NEW.cost_amount IS NOT NULL, OLD.cost_amount IS NOT NULL),
        unpriced = unpriced + campaign_ctr_d(NEW.pricing_type = 'regular' AND NEW.cost_amount IS NULL, OLD.pricing_type = 'regular' AND OLD.cost_amount IS NULL),
        cost = cost + coalesce(NEW.cost_amount, 0) - coalesce(OLD.cost_amount, 0),
        updated_at = now()
    WHERE campaign_id = NEW.campaign_id;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- The counters of one campaign recomputed from its recipient and message
-- rows (the reconciler; also the reference the tests compare with). It
-- locks the counters row first, so a transaction changing recipients
-- concurrently either committed before (and is counted) or applies its
-- delta afterwards. Returns whether anything had drifted.
-- +goose StatementBegin
CREATE FUNCTION campaign_counters_recompute(cid uuid) RETURNS boolean
LANGUAGE plpgsql AS $$
DECLARE
    drifted boolean;
BEGIN
    INSERT INTO campaign_counters (campaign_id) VALUES (cid) ON CONFLICT DO NOTHING;
    PERFORM 1 FROM campaign_counters WHERE campaign_id = cid FOR UPDATE;
    WITH r AS (
        SELECT count(*) AS recipients,
            count(*) FILTER (WHERE state = 'pending') AS r_pending,
            count(*) FILTER (WHERE state = 'queued') AS r_queued,
            count(*) FILTER (WHERE state = 'skipped') AS r_skipped,
            count(*) FILTER (WHERE state = 'cancelled') AS r_cancelled,
            count(*) FILTER (WHERE state = 'retry') AS r_retry,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'pending') AS o_pending,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'sent') AS o_sent,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'delivered') AS o_delivered,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'read') AS o_read,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'failed') AS o_failed,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'unknown') AS o_unknown,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'blocked') AS o_blocked,
            count(*) FILTER (WHERE campaign_recipient_outcome(state, msg_status) = 'cancelled') AS o_cancelled,
            count(sent_at) AS sent,
            count(delivered_at) AS delivered,
            count(read_at) AS read,
            count(*) FILTER (WHERE msg_status = 'failed' AND error_code ~ '^[0-9]+$') AS rejected,
            count(replied_at) AS replied,
            count(*) FILTER (WHERE reaction = 'positive') AS positive,
            count(*) FILTER (WHERE reaction = 'negative') AS negative,
            count(*) FILTER (WHERE reaction = 'neutral') AS neutral,
            count(opted_out_at) AS opted_out
        FROM campaign_recipients WHERE campaign_id = cid
    ), m AS (
        SELECT count(*) FILTER (WHERE status = 'pending') AS m_pending,
            count(*) FILTER (WHERE status = 'sending') AS m_sending,
            count(*) FILTER (WHERE status = 'unknown') AS m_unknown,
            count(*) FILTER (WHERE status = 'sent') AS m_sent,
            count(*) FILTER (WHERE status = 'delivered') AS m_delivered,
            count(*) FILTER (WHERE status = 'read') AS m_read,
            count(*) FILTER (WHERE status = 'failed') AS m_failed,
            count(*) FILTER (WHERE status = 'held') AS m_held,
            count(*) FILTER (WHERE status = 'cancelled') AS m_cancelled,
            count(*) FILTER (WHERE status = 'deferred') AS m_deferred,
            count(*) FILTER (WHERE status = 'skipped') AS m_skipped,
            count(*) FILTER (WHERE error_code = '131048') AS err_131048,
            count(*) FILTER (WHERE error_code = '131049') AS err_131049,
            count(*) FILTER (WHERE error_code = '131050') AS err_131050,
            count(cost_amount) AS priced,
            count(*) FILTER (WHERE pricing_type = 'regular' AND cost_amount IS NULL) AS unpriced,
            coalesce(sum(cost_amount), 0) AS cost
        FROM messages WHERE campaign_id = cid
    ), x AS (SELECT * FROM r, m),
    old AS (SELECT * FROM campaign_counters WHERE campaign_id = cid),
    u AS (
        UPDATE campaign_counters k SET
            recipients = x.recipients,
            r_pending = x.r_pending,
            r_queued = x.r_queued,
            r_skipped = x.r_skipped,
            r_cancelled = x.r_cancelled,
            r_retry = x.r_retry,
            o_pending = x.o_pending,
            o_sent = x.o_sent,
            o_delivered = x.o_delivered,
            o_read = x.o_read,
            o_failed = x.o_failed,
            o_unknown = x.o_unknown,
            o_blocked = x.o_blocked,
            o_cancelled = x.o_cancelled,
            sent = x.sent,
            delivered = x.delivered,
            read = x.read,
            rejected = x.rejected,
            replied = x.replied,
            positive = x.positive,
            negative = x.negative,
            neutral = x.neutral,
            opted_out = x.opted_out,
            m_pending = x.m_pending,
            m_sending = x.m_sending,
            m_unknown = x.m_unknown,
            m_sent = x.m_sent,
            m_delivered = x.m_delivered,
            m_read = x.m_read,
            m_failed = x.m_failed,
            m_held = x.m_held,
            m_cancelled = x.m_cancelled,
            m_deferred = x.m_deferred,
            m_skipped = x.m_skipped,
            err_131048 = x.err_131048,
            err_131049 = x.err_131049,
            err_131050 = x.err_131050,
            priced = x.priced,
            unpriced = x.unpriced,
            cost = x.cost,
            reconciled_at = now(), updated_at = now()
        FROM x, old
        WHERE k.campaign_id = cid
        RETURNING (old.recipients IS DISTINCT FROM x.recipients
            OR old.r_pending IS DISTINCT FROM x.r_pending
            OR old.r_queued IS DISTINCT FROM x.r_queued
            OR old.r_skipped IS DISTINCT FROM x.r_skipped
            OR old.r_cancelled IS DISTINCT FROM x.r_cancelled
            OR old.r_retry IS DISTINCT FROM x.r_retry
            OR old.o_pending IS DISTINCT FROM x.o_pending
            OR old.o_sent IS DISTINCT FROM x.o_sent
            OR old.o_delivered IS DISTINCT FROM x.o_delivered
            OR old.o_read IS DISTINCT FROM x.o_read
            OR old.o_failed IS DISTINCT FROM x.o_failed
            OR old.o_unknown IS DISTINCT FROM x.o_unknown
            OR old.o_blocked IS DISTINCT FROM x.o_blocked
            OR old.o_cancelled IS DISTINCT FROM x.o_cancelled
            OR old.sent IS DISTINCT FROM x.sent
            OR old.delivered IS DISTINCT FROM x.delivered
            OR old.read IS DISTINCT FROM x.read
            OR old.rejected IS DISTINCT FROM x.rejected
            OR old.replied IS DISTINCT FROM x.replied
            OR old.positive IS DISTINCT FROM x.positive
            OR old.negative IS DISTINCT FROM x.negative
            OR old.neutral IS DISTINCT FROM x.neutral
            OR old.opted_out IS DISTINCT FROM x.opted_out
            OR old.m_pending IS DISTINCT FROM x.m_pending
            OR old.m_sending IS DISTINCT FROM x.m_sending
            OR old.m_unknown IS DISTINCT FROM x.m_unknown
            OR old.m_sent IS DISTINCT FROM x.m_sent
            OR old.m_delivered IS DISTINCT FROM x.m_delivered
            OR old.m_read IS DISTINCT FROM x.m_read
            OR old.m_failed IS DISTINCT FROM x.m_failed
            OR old.m_held IS DISTINCT FROM x.m_held
            OR old.m_cancelled IS DISTINCT FROM x.m_cancelled
            OR old.m_deferred IS DISTINCT FROM x.m_deferred
            OR old.m_skipped IS DISTINCT FROM x.m_skipped
            OR old.err_131048 IS DISTINCT FROM x.err_131048
            OR old.err_131049 IS DISTINCT FROM x.err_131049
            OR old.err_131050 IS DISTINCT FROM x.err_131050
            OR old.priced IS DISTINCT FROM x.priced
            OR old.unpriced IS DISTINCT FROM x.unpriced
            OR old.cost IS DISTINCT FROM x.cost) AS d
    )
    SELECT coalesce(bool_or(d), false) INTO drifted FROM u;
    RETURN drifted;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION campaign_counters_init() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO campaign_counters (campaign_id) VALUES (NEW.id) ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- Backfill: existing recipients take their message's state, then every
-- campaign gets its counters, before the triggers start counting.
UPDATE campaign_recipients r SET msg_status = m.status, sent_at = m.sent_at, delivered_at = m.delivered_at, read_at = m.read_at,
    failed_at = CASE WHEN m.status = 'failed' THEN coalesce(m.failed_at, m.updated_at) END,
    attempted_at = CASE WHEN m.status IN ('sent', 'delivered', 'read', 'failed', 'deferred', 'unknown')
                        THEN coalesce(m.sent_at, m.failed_at, m.updated_at) END,
    error_code = m.error_code, failure_class = m.failure_class, cost_amount = m.cost_amount, cost_currency = m.cost_currency
FROM messages m WHERE m.id = r.message_id;
INSERT INTO campaign_counters (campaign_id) SELECT id FROM campaigns;
SELECT campaign_counters_recompute(id) FROM campaigns;

CREATE TRIGGER campaigns_counters_init AFTER INSERT ON campaigns
    FOR EACH ROW EXECUTE FUNCTION campaign_counters_init();
CREATE TRIGGER campaign_recipients_counters_ins AFTER INSERT ON campaign_recipients
    FOR EACH ROW EXECUTE FUNCTION campaign_recipient_counters();
CREATE TRIGGER campaign_recipients_counters_upd AFTER UPDATE ON campaign_recipients
    FOR EACH ROW WHEN (OLD.state IS DISTINCT FROM NEW.state OR OLD.msg_status IS DISTINCT FROM NEW.msg_status
        OR OLD.sent_at IS DISTINCT FROM NEW.sent_at OR OLD.delivered_at IS DISTINCT FROM NEW.delivered_at
        OR OLD.read_at IS DISTINCT FROM NEW.read_at OR OLD.error_code IS DISTINCT FROM NEW.error_code
        OR OLD.replied_at IS DISTINCT FROM NEW.replied_at OR OLD.reaction IS DISTINCT FROM NEW.reaction
        OR OLD.opted_out_at IS DISTINCT FROM NEW.opted_out_at)
    EXECUTE FUNCTION campaign_recipient_counters();
CREATE TRIGGER messages_campaign_counters_ins AFTER INSERT ON messages
    FOR EACH ROW WHEN (NEW.campaign_id IS NOT NULL) EXECUTE FUNCTION campaign_message_counters();
CREATE TRIGGER messages_campaign_counters_upd AFTER UPDATE ON messages
    FOR EACH ROW WHEN (NEW.campaign_id IS NOT NULL AND (OLD.status IS DISTINCT FROM NEW.status
        OR OLD.error_code IS DISTINCT FROM NEW.error_code OR OLD.failure_class IS DISTINCT FROM NEW.failure_class
        OR OLD.cost_amount IS DISTINCT FROM NEW.cost_amount OR OLD.pricing_type IS DISTINCT FROM NEW.pricing_type
        OR OLD.sent_at IS DISTINCT FROM NEW.sent_at OR OLD.delivered_at IS DISTINCT FROM NEW.delivered_at
        OR OLD.read_at IS DISTINCT FROM NEW.read_at))
    EXECUTE FUNCTION campaign_message_counters();

-- The pre-launch forecast of a campaign, fixed when it starts (never
-- updated): what was expected, with its inputs, for forecast vs actual.
CREATE TABLE campaign_forecasts (
    campaign_id uuid PRIMARY KEY REFERENCES campaigns (id) ON DELETE CASCADE,
    forecast    jsonb NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
-- +goose StatementBegin
CREATE FUNCTION campaign_forecasts_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'campaign_forecasts rows are immutable';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER campaign_forecasts_no_update BEFORE UPDATE ON campaign_forecasts
    FOR EACH ROW EXECUTE FUNCTION campaign_forecasts_immutable();

-- Exports of a campaign's recipients and its report, produced by a
-- background job (bulk queue) for large campaigns; the file is kept until
-- expires_at.
CREATE TABLE campaign_exports (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    campaign_id  uuid NOT NULL REFERENCES campaigns (id) ON DELETE CASCADE,
    kind         text NOT NULL CHECK (kind IN ('recipients', 'report')),
    format       text NOT NULL CHECK (format IN ('csv', 'xlsx', 'pdf')),
    filters      jsonb NOT NULL DEFAULT '{}'::jsonb,
    status       text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed', 'expired')),
    file_key     text,
    filename     text,
    size_bytes   bigint,
    rows_written bigint,
    error        text,
    requested_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    expires_at   timestamptz
);
CREATE INDEX campaign_exports_campaign_idx ON campaign_exports (campaign_id, id DESC);
CREATE INDEX campaign_exports_expiry_idx ON campaign_exports (expires_at) WHERE status = 'done';

-- Number quality and template status/quality changes become account
-- events, the timeline of the campaign's spam and quality section.
-- +goose StatementBegin
CREATE FUNCTION phone_number_quality_event() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO whatsapp_account_events (account_id, phone_number_id, kind, payload, occurred_at)
    VALUES (NEW.account_id, NEW.id, 'quality_rating_change',
            jsonb_build_object('from', OLD.quality_rating, 'to', NEW.quality_rating), now())
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER phone_numbers_quality_event AFTER UPDATE OF quality_rating ON phone_numbers
    FOR EACH ROW WHEN (OLD.quality_rating IS DISTINCT FROM NEW.quality_rating)
    EXECUTE FUNCTION phone_number_quality_event();

-- +goose StatementBegin
CREATE FUNCTION message_template_change_event() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO whatsapp_account_events (account_id, kind, payload, occurred_at)
    VALUES (NEW.account_id, 'template_change', jsonb_build_object('template', NEW.name, 'language', NEW.language,
            'status_from', OLD.status, 'status_to', NEW.status, 'quality_from', OLD.quality_score, 'quality_to', NEW.quality_score), now())
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER message_templates_change_event AFTER UPDATE OF status, quality_score ON message_templates
    FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status OR OLD.quality_score IS DISTINCT FROM NEW.quality_score)
    EXECUTE FUNCTION message_template_change_event();

-- +goose Down
DROP TRIGGER IF EXISTS message_templates_change_event ON message_templates;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS message_template_change_event();
DROP TRIGGER IF EXISTS phone_numbers_quality_event ON phone_numbers;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS phone_number_quality_event();
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_exports;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_forecasts;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_forecasts_immutable();
DROP TRIGGER IF EXISTS messages_campaign_counters_upd ON messages;
DROP TRIGGER IF EXISTS messages_campaign_counters_ins ON messages;
DROP TRIGGER IF EXISTS campaign_recipients_counters_upd ON campaign_recipients;
DROP TRIGGER IF EXISTS campaign_recipients_counters_ins ON campaign_recipients;
DROP TRIGGER IF EXISTS campaigns_counters_init ON campaigns;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_counters_init();
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_counters_recompute(uuid);
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_message_counters();
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_recipient_counters();
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign_counters;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_ctr_d(boolean, boolean);
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS campaign_recipient_outcome(text, text);
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS campaign_recipients_attempted_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS campaign_recipients_contact_sent_idx;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX IF EXISTS campaign_recipients_message_idx;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_contact_type_check;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS contact_type;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS reaction_map;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT IF EXISTS campaign_recipients_reaction_source_check;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT IF EXISTS campaign_recipients_reaction_check;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS opted_out_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS reaction_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS reaction_source;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS reaction;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS reply_payload;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS reply_message_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS replied_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS cost_currency;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS cost_amount;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS failure_class;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS error_code;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS failed_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS read_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS delivered_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS sent_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS attempted_at;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS msg_status;
