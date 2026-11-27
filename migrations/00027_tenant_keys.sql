-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30min';

-- Phase 17 WP1 step 3 of 4: keys that were unique across the database are
-- unique per tenant, so one company can never block or collide with
-- another's names, phone numbers or Meta IDs. Routing keys stay global:
-- whatsapp_accounts.waba_id, phone_numbers.phone_number_id and every token
-- and key hash (users.email too, until global users and memberships in WP2).
-- The tables are small or freshly backfilled; plain index builds match the
-- rest of the migrations.

-- settings: one document per key and tenant.
-- squawk-ignore ban-drop-constraint
ALTER TABLE settings DROP CONSTRAINT settings_pkey;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX settings_pkey ON settings (tenant_id, key);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE settings ADD CONSTRAINT settings_pkey PRIMARY KEY USING INDEX settings_pkey;

-- contacts: a phone number once per tenant.
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT contacts_phone_e164_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX contacts_phone_e164_key ON contacts (tenant_id, phone_e164);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE contacts ADD CONSTRAINT contacts_phone_e164_key UNIQUE USING INDEX contacts_phone_e164_key;

-- Tag, group and segment names (case-insensitive).
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX tags_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX tags_name_key ON tags (tenant_id, lower(name));
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX contact_groups_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX contact_groups_name_key ON contact_groups (tenant_id, lower(name));
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX segments_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX segments_name_key ON segments (tenant_id, lower(name));

-- One default number per tenant.
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX phone_numbers_one_default;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX phone_numbers_one_default ON phone_numbers (tenant_id) WHERE is_default;

-- AI provider names.
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_providers DROP CONSTRAINT ai_providers_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX ai_providers_name_key ON ai_providers (tenant_id, name);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_name_key UNIQUE USING INDEX ai_providers_name_key;

-- Meta message IDs: a tenant can never claim (or collide with) another
-- tenant's wamid.
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_wamid_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX messages_wamid_key ON messages (tenant_id, wamid);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE messages ADD CONSTRAINT messages_wamid_key UNIQUE USING INDEX messages_wamid_key;

-- Inbound webhook deduplication.
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_payload_sha256_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX webhook_events_payload_sha256_key ON webhook_events (tenant_id, payload_sha256);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_payload_sha256_key UNIQUE USING INDEX webhook_events_payload_sha256_key;

-- Meta user-preference events.
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX meta_user_preferences_once;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX meta_user_preferences_once ON meta_user_preferences (tenant_id, wa_id, user_id, category, value, occurred_at) NULLS NOT DISTINCT;

-- One open invitation per e-mail address and tenant.
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX user_invitations_open_email_idx;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX user_invitations_open_email_idx ON user_invitations (tenant_id, email) WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- Idempotency keys are scoped to the tenant as well as the caller.
-- squawk-ignore ban-drop-constraint
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX idempotency_keys_pkey ON idempotency_keys (tenant_id, principal, key);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_pkey PRIMARY KEY USING INDEX idempotency_keys_pkey;

-- ── Triggers that write rows copy the tenant of the row that fired them ──
-- (they run as the caller, so row-level security applies to them too).

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION campaign_counters_init() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO campaign_counters (tenant_id, campaign_id) VALUES (NEW.tenant_id, NEW.id) ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION campaign_counters_recompute(cid uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    drifted boolean;
BEGIN
    INSERT INTO campaign_counters (tenant_id, campaign_id)
    SELECT c.tenant_id, c.id FROM campaigns c WHERE c.id = cid
    ON CONFLICT DO NOTHING;
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
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION phone_number_quality_event() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO whatsapp_account_events (tenant_id, account_id, phone_number_id, kind, payload, occurred_at)
    VALUES (NEW.tenant_id, NEW.account_id, NEW.id, 'quality_rating_change',
            jsonb_build_object('from', OLD.quality_rating, 'to', NEW.quality_rating), now())
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION message_template_change_event() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO whatsapp_account_events (tenant_id, account_id, kind, payload, occurred_at)
    VALUES (NEW.tenant_id, NEW.account_id, 'template_change', jsonb_build_object('template', NEW.name, 'language', NEW.language,
            'status_from', OLD.status, 'status_to', NEW.status, 'quality_from', OLD.quality_score, 'quality_to', NEW.quality_score), now())
    ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- The ops hint is an outbox event of the tenant whose row changed.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ops_hint() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO outbox_events (tenant_id, topic, payload)
    VALUES (NEW.tenant_id, 'rt:system:ops', jsonb_build_object('type', 'ops.updated', 'v', 1, 'id', uuidv7(),
        'ts', to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), 'section', TG_ARGV[0]));
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Restores the single-tenant keys and triggers. It fails, as it should,
-- once two tenants share a key.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ops_hint() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO outbox_events (topic, payload)
    VALUES ('rt:system:ops', jsonb_build_object('type', 'ops.updated', 'v', 1, 'id', uuidv7(),
        'ts', to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), 'section', TG_ARGV[0]));
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION message_template_change_event() RETURNS trigger
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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION phone_number_quality_event() RETURNS trigger
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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION campaign_counters_recompute(cid uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
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
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION campaign_counters_init() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO campaign_counters (campaign_id) VALUES (NEW.id) ON CONFLICT DO NOTHING;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- squawk-ignore ban-drop-constraint
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX idempotency_keys_pkey ON idempotency_keys (principal, key);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_pkey PRIMARY KEY USING INDEX idempotency_keys_pkey;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX user_invitations_open_email_idx;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX user_invitations_open_email_idx ON user_invitations (email) WHERE accepted_at IS NULL AND revoked_at IS NULL;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX meta_user_preferences_once;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX meta_user_preferences_once ON meta_user_preferences (wa_id, user_id, category, value, occurred_at) NULLS NOT DISTINCT;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_payload_sha256_key;
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_payload_sha256_key UNIQUE (payload_sha256);
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_wamid_key;
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE messages ADD CONSTRAINT messages_wamid_key UNIQUE (wamid);
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_providers DROP CONSTRAINT ai_providers_name_key;
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_name_key UNIQUE (name);
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX phone_numbers_one_default;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX phone_numbers_one_default ON phone_numbers (is_default) WHERE is_default;
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX segments_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX segments_name_key ON segments (lower(name));
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX contact_groups_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX contact_groups_name_key ON contact_groups (lower(name));
-- squawk-ignore require-concurrent-index-deletion
DROP INDEX tags_name_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX tags_name_key ON tags (lower(name));
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT contacts_phone_e164_key;
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE contacts ADD CONSTRAINT contacts_phone_e164_key UNIQUE (phone_e164);
-- squawk-ignore ban-drop-constraint
ALTER TABLE settings DROP CONSTRAINT settings_pkey;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX settings_pkey ON settings (key);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE settings ADD CONSTRAINT settings_pkey PRIMARY KEY USING INDEX settings_pkey;
