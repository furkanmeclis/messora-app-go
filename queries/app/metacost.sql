-- ── Rate card ───────────────────────────────────────────────────────────────

-- name: UpsertMetaRateMarket :exec
INSERT INTO meta_rate_markets (country, market) VALUES (@country, @market)
ON CONFLICT (country) DO UPDATE SET market = excluded.market, imported_at = now();

-- name: UpsertMetaRate :exec
INSERT INTO meta_rates (currency, market, category, price, tier_lower, tier_upper, effective_from, source, imported_by)
VALUES (@currency, @market, @category, (@price::text)::numeric(12, 6), sqlc.narg(tier_lower), sqlc.narg(tier_upper),
        @effective_from, @source, sqlc.narg(imported_by))
ON CONFLICT (currency, market, category, tier_lower, effective_from) DO UPDATE SET
    price       = excluded.price,
    tier_upper  = excluded.tier_upper,
    source      = excluded.source,
    imported_at = now(),
    imported_by = excluded.imported_by;

-- name: CloseSupersededMetaRates :exec
-- A rate is valid until the next effective date of the same currency,
-- market and category.
UPDATE meta_rates r SET effective_to = (
    SELECT min(x.effective_from) FROM meta_rates x
    WHERE x.currency = r.currency AND x.market = r.market AND x.category = r.category AND x.effective_from > r.effective_from)
WHERE r.effective_to IS DISTINCT FROM (
    SELECT min(x.effective_from) FROM meta_rates x
    WHERE x.currency = r.currency AND x.market = r.market AND x.category = r.category AND x.effective_from > r.effective_from);

-- name: ListMetaRates :many
SELECT r.id, r.currency, r.market, r.category, r.price::text AS price, r.tier_lower, r.tier_upper,
       r.effective_from, r.effective_to, r.source, r.imported_at,
       coalesce((SELECT string_agg(m.country, ' ' ORDER BY m.country) FROM meta_rate_markets m WHERE m.market = r.market), '')::text AS countries
FROM meta_rates r
ORDER BY r.market, r.category, r.currency, r.tier_lower NULLS FIRST, r.effective_from DESC;

-- name: MetaRateCandidates :many
-- The rates that can price a message to country in category: its own
-- market's and the "Other" market's.
SELECT r.currency, r.market, r.category, r.price::text AS price, r.tier_lower, r.tier_upper, r.effective_from, r.effective_to
FROM meta_rates r
WHERE r.category = @category
  AND (r.market = 'Other' OR r.market = (SELECT m.market FROM meta_rate_markets m WHERE m.country = @country));

-- name: GetMetaRateMarket :one
SELECT market FROM meta_rate_markets WHERE country = @country;

-- name: CountBillableInMonth :one
-- Billable messages of the same account, category and market sent earlier
-- in the calendar month: the volume that selects a rate tier.
SELECT count(*) FROM messages m
JOIN phone_numbers p ON p.id = m.phone_number_id
WHERE p.account_id = @account_id
  AND m.id <> @message_id
  AND m.pricing_type = 'regular'
  AND m.pricing_category = @category
  AND m.recipient_country = ANY (@countries::text[])
  AND m.sent_at >= @month_start AND m.sent_at < @at;

-- name: ListMarketCountries :many
SELECT country FROM meta_rate_markets WHERE market = @market;

-- name: SetMessagePricing :exec
-- Pricing arrives on "sent" or "delivered" (never both); a later webhook
-- without it never clears what was stored.
UPDATE messages SET
    pricing_type      = coalesce(sqlc.narg(pricing_type), pricing_type),
    pricing_category  = coalesce(sqlc.narg(pricing_category), pricing_category),
    pricing_model     = coalesce(sqlc.narg(pricing_model), pricing_model),
    billable          = coalesce(sqlc.narg(billable), billable),
    recipient_country = coalesce(sqlc.narg(recipient_country), recipient_country),
    cost_amount       = coalesce((sqlc.narg(cost_amount)::text)::numeric(12, 6), cost_amount),
    cost_currency     = coalesce(sqlc.narg(cost_currency), cost_currency),
    cost_source       = coalesce(sqlc.narg(cost_source), cost_source),
    updated_at        = now()
WHERE id = @id;

-- name: GetMessageCost :one
SELECT pricing_type, pricing_category, cost_amount::text AS cost_amount, cost_currency FROM messages WHERE id = @id;

-- ── Account signals ─────────────────────────────────────────────────────────

-- name: SetWhatsAppAccountCurrency :exec
UPDATE whatsapp_accounts SET currency = @currency, updated_at = now() WHERE id = @id;

-- name: ListActiveWhatsAppAccounts :many
SELECT * FROM whatsapp_accounts WHERE status = 'active' ORDER BY created_at;

-- name: InsertWhatsAppAccountEvent :exec
INSERT INTO whatsapp_account_events (account_id, phone_number_id, kind, payload, occurred_at)
VALUES (@account_id, sqlc.narg(phone_number_id), @kind, @payload, @occurred_at)
ON CONFLICT (account_id, kind, occurred_at, md5(payload::text)) DO NOTHING;

-- name: FindPhoneNumberByDisplay :one
SELECT * FROM phone_numbers
WHERE account_id = @account_id
  AND regexp_replace(display_phone_number, '[^0-9]', '', 'g') = regexp_replace(@display_phone_number::text, '[^0-9]', '', 'g')
LIMIT 1;

-- name: SetPhoneNumberSignals :exec
-- Values from a webhook; NULL keeps what is stored.
UPDATE phone_numbers SET
    messaging_limit = coalesce(sqlc.narg(messaging_limit), messaging_limit),
    throughput_mps  = coalesce(sqlc.narg(throughput_mps), throughput_mps),
    quality_rating  = coalesce(sqlc.narg(quality_rating), quality_rating),
    updated_at      = now()
WHERE id = @id;

-- name: SetAccountMessagingLimit :exec
-- The messaging limit is the business portfolio's: every number shares it.
UPDATE phone_numbers SET messaging_limit = @messaging_limit, updated_at = now() WHERE account_id = @account_id;

-- name: SetTemplateQualityByMetaID :execrows
UPDATE message_templates SET quality_score = @quality_score, updated_at = now()
WHERE meta_template_id = @meta_template_id;

-- name: GetPhoneNumberThroughput :one
SELECT throughput_mps FROM phone_numbers WHERE id = @id;

-- ── User preferences ────────────────────────────────────────────────────────

-- name: InsertMetaUserPreference :one
INSERT INTO meta_user_preferences (phone_number_id, wa_id, user_id, phone_e164, category, value, detail, occurred_at)
VALUES (sqlc.narg(phone_number_id), sqlc.narg(wa_id), sqlc.narg(user_id), sqlc.narg(phone_e164), @category, @value, @detail, @occurred_at)
ON CONFLICT (tenant_id, wa_id, user_id, category, value, occurred_at) DO NOTHING
RETURNING *;

-- name: LatestMetaUserPreference :one
-- The newest recorded preference of a number for a category: webhooks may
-- arrive out of order, only the latest choice counts.
SELECT * FROM meta_user_preferences
WHERE phone_e164 = @phone_e164 AND category = @category
ORDER BY occurred_at DESC, id DESC LIMIT 1;

-- ── Daily cost (pricing_analytics) ──────────────────────────────────────────

-- name: UpsertMetaCostDaily :exec
INSERT INTO meta_cost_daily (account_id, day, phone_number, country, pricing_category, pricing_type, volume, cost, currency)
VALUES (@account_id, @day, @phone_number, @country, @pricing_category, @pricing_type, @volume,
        (sqlc.narg(cost)::text)::numeric(14, 6), sqlc.narg(currency))
ON CONFLICT (account_id, day, phone_number, country, pricing_category, pricing_type) DO UPDATE SET
    volume     = excluded.volume,
    cost       = excluded.cost,
    currency   = excluded.currency,
    fetched_at = now();
