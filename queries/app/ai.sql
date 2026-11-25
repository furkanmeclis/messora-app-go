-- name: ListAIProviders :many
SELECT * FROM ai_providers ORDER BY created_at;

-- name: GetAIProvider :one
SELECT * FROM ai_providers WHERE id = @id;

-- name: InsertAIProvider :one
INSERT INTO ai_providers (id, kind, name, api_key_enc, key_last4, base_url, enabled, created_by)
VALUES (@id, @kind, @name, @api_key_enc, @key_last4, sqlc.narg(base_url), @enabled, sqlc.narg(created_by))
RETURNING *;

-- name: UpdateAIProvider :one
UPDATE ai_providers SET
    name = coalesce(sqlc.narg(name), name),
    api_key_enc = coalesce(sqlc.narg(api_key_enc), api_key_enc),
    key_last4 = coalesce(sqlc.narg(key_last4), key_last4),
    base_url = CASE WHEN @set_base_url::boolean THEN sqlc.narg(base_url) ELSE base_url END,
    enabled = coalesce(sqlc.narg(enabled), enabled),
    -- A new key or endpoint has not been checked yet.
    status = CASE WHEN sqlc.narg(api_key_enc)::bytea IS NOT NULL OR @set_base_url::boolean THEN 'unknown' ELSE status END,
    status_detail = CASE WHEN sqlc.narg(api_key_enc)::bytea IS NOT NULL OR @set_base_url::boolean THEN '' ELSE status_detail END,
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetAIProviderStatus :one
UPDATE ai_providers SET status = @status, status_detail = @status_detail, checked_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DeleteAIProvider :execrows
DELETE FROM ai_providers WHERE id = @id;

-- name: ListAIModels :many
SELECT m.*, p.kind AS provider_kind, p.name AS provider_name
FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id
ORDER BY p.name, m.model_id;

-- name: GetAIModel :one
SELECT * FROM ai_models WHERE provider_id = @provider_id AND model_id = @model_id;

-- name: UpsertAIModel :exec
INSERT INTO ai_models (provider_id, model_id, display_name, context_window, max_output, capabilities, lifecycle, synced_at)
VALUES (@provider_id, @model_id, @display_name, sqlc.narg(context_window), sqlc.narg(max_output), @capabilities, sqlc.narg(lifecycle), now())
ON CONFLICT (provider_id, model_id) DO UPDATE SET
    display_name = excluded.display_name, context_window = excluded.context_window, max_output = excluded.max_output,
    capabilities = excluded.capabilities, lifecycle = excluded.lifecycle, synced_at = now();

-- name: DeleteStaleAIModels :execrows
DELETE FROM ai_models WHERE provider_id = @provider_id AND NOT (model_id = ANY (@keep::text[]));

-- name: MarkAIModelsSynced :exec
UPDATE ai_providers SET models_synced_at = now() WHERE id = @id;

-- name: ListAIPrices :many
SELECT id, provider_kind, model_pattern, currency, input::text AS input, coalesce(cached_input::text, '')::text AS cached_input, coalesce(cache_write_5m::text, '')::text AS cache_write_5m, coalesce(cache_write_1h::text, '')::text AS cache_write_1h, output::text AS output, rules, effective_from, effective_to, source, source_url, notes, confirmed_by, confirmed_at, created_at, updated_at FROM ai_model_prices ORDER BY provider_kind, model_pattern, effective_from DESC;

-- name: ListAIPricesForKind :many
SELECT id, provider_kind, model_pattern, currency, input::text AS input, coalesce(cached_input::text, '')::text AS cached_input, coalesce(cache_write_5m::text, '')::text AS cache_write_5m, coalesce(cache_write_1h::text, '')::text AS cache_write_1h, output::text AS output, rules, effective_from, effective_to, source, source_url, notes, confirmed_by, confirmed_at, created_at, updated_at FROM ai_model_prices WHERE provider_kind = @provider_kind;

-- name: GetAIPrice :one
SELECT id, provider_kind, model_pattern, currency, input::text AS input, coalesce(cached_input::text, '')::text AS cached_input, coalesce(cache_write_5m::text, '')::text AS cache_write_5m, coalesce(cache_write_1h::text, '')::text AS cache_write_1h, output::text AS output, rules, effective_from, effective_to, source, source_url, notes, confirmed_by, confirmed_at, created_at, updated_at FROM ai_model_prices WHERE id = @id;

-- name: UpsertAIPrice :one
INSERT INTO ai_model_prices (provider_kind, model_pattern, currency, input, cached_input, cache_write_5m, cache_write_1h, output,
                             rules, effective_from, effective_to, source, source_url, notes)
VALUES (@provider_kind, @model_pattern, @currency, (@input::text)::numeric(12, 6), (sqlc.narg(cached_input)::text)::numeric(12, 6), (sqlc.narg(cache_write_5m)::text)::numeric(12, 6), (sqlc.narg(cache_write_1h)::text)::numeric(12, 6),
        (@output::text)::numeric(12, 6), @rules, @effective_from, sqlc.narg(effective_to), @source, @source_url, @notes)
ON CONFLICT (provider_kind, model_pattern, effective_from) DO UPDATE SET
    currency = excluded.currency, input = excluded.input, cached_input = excluded.cached_input,
    cache_write_5m = excluded.cache_write_5m, cache_write_1h = excluded.cache_write_1h, output = excluded.output,
    rules = excluded.rules, effective_to = excluded.effective_to, source = excluded.source, source_url = excluded.source_url,
    notes = excluded.notes,
    -- Changed figures need a fresh confirmation.
    confirmed_by = NULL, confirmed_at = NULL, updated_at = now()
RETURNING id, provider_kind, model_pattern, currency, input::text AS input, coalesce(cached_input::text, '')::text AS cached_input, coalesce(cache_write_5m::text, '')::text AS cache_write_5m, coalesce(cache_write_1h::text, '')::text AS cache_write_1h, output::text AS output, rules, effective_from, effective_to, source, source_url, notes, confirmed_by, confirmed_at, created_at, updated_at;

-- name: InsertAIPriceIfAbsent :execrows
INSERT INTO ai_model_prices (provider_kind, model_pattern, currency, input, cached_input, cache_write_5m, cache_write_1h, output,
                             rules, effective_from, effective_to, source, source_url, notes)
VALUES (@provider_kind, @model_pattern, @currency, (@input::text)::numeric(12, 6), (sqlc.narg(cached_input)::text)::numeric(12, 6), (sqlc.narg(cache_write_5m)::text)::numeric(12, 6), (sqlc.narg(cache_write_1h)::text)::numeric(12, 6),
        (@output::text)::numeric(12, 6), @rules, @effective_from, sqlc.narg(effective_to), @source, @source_url, @notes)
ON CONFLICT (provider_kind, model_pattern, effective_from) DO NOTHING;

-- name: InsertAIPrice :one
INSERT INTO ai_model_prices (provider_kind, model_pattern, currency, input, cached_input, cache_write_5m, cache_write_1h, output,
                             rules, effective_from, effective_to, source, source_url, notes)
VALUES (@provider_kind, @model_pattern, @currency, (@input::text)::numeric(12, 6), (sqlc.narg(cached_input)::text)::numeric(12, 6), (sqlc.narg(cache_write_5m)::text)::numeric(12, 6), (sqlc.narg(cache_write_1h)::text)::numeric(12, 6),
        (@output::text)::numeric(12, 6), @rules, @effective_from, sqlc.narg(effective_to), @source, @source_url, @notes)
RETURNING id, provider_kind, model_pattern, currency, input::text AS input, coalesce(cached_input::text, '')::text AS cached_input, coalesce(cache_write_5m::text, '')::text AS cache_write_5m, coalesce(cache_write_1h::text, '')::text AS cache_write_1h, output::text AS output, rules, effective_from, effective_to, source, source_url, notes, confirmed_by, confirmed_at, created_at, updated_at;

-- name: UpdateAIPrice :one
UPDATE ai_model_prices SET
    model_pattern = @model_pattern, currency = @currency, input = (@input::text)::numeric(12, 6), cached_input = (sqlc.narg(cached_input)::text)::numeric(12, 6),
    cache_write_5m = (sqlc.narg(cache_write_5m)::text)::numeric(12, 6), cache_write_1h = (sqlc.narg(cache_write_1h)::text)::numeric(12, 6), output = (@output::text)::numeric(12, 6),
    rules = @rules, effective_from = @effective_from, effective_to = sqlc.narg(effective_to), source_url = @source_url, notes = @notes,
    confirmed_by = sqlc.narg(confirmed_by), confirmed_at = sqlc.narg(confirmed_at), updated_at = now()
WHERE id = @id
RETURNING id, provider_kind, model_pattern, currency, input::text AS input, coalesce(cached_input::text, '')::text AS cached_input, coalesce(cache_write_5m::text, '')::text AS cache_write_5m, coalesce(cache_write_1h::text, '')::text AS cache_write_1h, output::text AS output, rules, effective_from, effective_to, source, source_url, notes, confirmed_by, confirmed_at, created_at, updated_at;

-- name: DeleteAIPrice :execrows
DELETE FROM ai_model_prices WHERE id = @id;

-- name: InsertAIUsage :one
INSERT INTO ai_usage (provider_id, provider_kind, model, purpose, profile_id, conversation_id, contact_id, campaign_id, automation_run_id,
                      input_tokens, cached_tokens, cache_write_tokens, cache_write_1h_tokens, output_tokens, reasoning_tokens,
                      cost, currency, price_id, latency_ms, status, error_kind)
VALUES (sqlc.narg(provider_id), @provider_kind, @model, @purpose, sqlc.narg(profile_id), sqlc.narg(conversation_id), sqlc.narg(contact_id),
        sqlc.narg(campaign_id), sqlc.narg(automation_run_id), @input_tokens, @cached_tokens, @cache_write_tokens, @cache_write_1h_tokens,
        @output_tokens, @reasoning_tokens, (sqlc.narg(cost)::text)::numeric(12, 6), sqlc.narg(currency), sqlc.narg(price_id), @latency_ms, @status, sqlc.narg(error_kind))
RETURNING id;

-- name: ListAIUsage :many
SELECT * FROM ai_usage ORDER BY created_at DESC, id DESC LIMIT @lim;

-- name: ListAIProfiles :many
SELECT pr.*, pn.display_phone_number AS phone_display
FROM ai_profiles pr LEFT JOIN phone_numbers pn ON pn.id = pr.phone_number_id
ORDER BY pr.created_at;

-- name: GetAIProfile :one
SELECT * FROM ai_profiles WHERE id = @id;

-- name: GetAIProfileForNumber :one
SELECT * FROM ai_profiles WHERE phone_number_id = @phone_number_id;

-- name: InsertAIProfile :one
INSERT INTO ai_profiles (name, phone_number_id, system_prompt, provider_id, model, auto_reply, handoff, data_collection,
                         allowed_actions, disclosure_mode, status, created_by)
VALUES (@name, sqlc.narg(phone_number_id), @system_prompt, sqlc.narg(provider_id), sqlc.narg(model), @auto_reply, @handoff,
        @data_collection, @allowed_actions, @disclosure_mode, @status, sqlc.narg(created_by))
RETURNING *;

-- name: UpdateAIProfile :one
UPDATE ai_profiles SET
    name = @name, phone_number_id = sqlc.narg(phone_number_id), system_prompt = @system_prompt, provider_id = sqlc.narg(provider_id),
    model = sqlc.narg(model), auto_reply = @auto_reply, handoff = @handoff, data_collection = @data_collection,
    allowed_actions = @allowed_actions, disclosure_mode = @disclosure_mode, status = @status, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: DeleteAIProfile :execrows
DELETE FROM ai_profiles WHERE id = @id;

-- name: PhoneNumberExists :one
SELECT EXISTS (SELECT 1 FROM phone_numbers WHERE id = @id);

-- name: AutomationExists :one
SELECT EXISTS (SELECT 1 FROM automations WHERE id = @id);
