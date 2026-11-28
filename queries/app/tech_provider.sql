-- ── Tech Provider / Embedded Signup (Phase 17 WP6) ─────────────────────────

-- name: InsertTechProviderAccount :one
-- The account of an Embedded Signup; disabled until the onboarding finishes.
INSERT INTO whatsapp_accounts (id, name, waba_id, app_id, access_token_enc, connection_method, status, last_error, created_by)
VALUES (@id, @name, @waba_id, @app_id, @access_token_enc, 'tech_provider', 'disabled', @last_error, sqlc.narg(created_by))
RETURNING *;

-- name: ConvertAccountToTechProvider :one
-- A disabled own-app account (its history is kept) switches to Messora's
-- app: Messora's app id, no app secret, the business token.
UPDATE whatsapp_accounts SET connection_method = 'tech_provider', app_id = @app_id, app_secret_enc = NULL,
    access_token_enc = @access_token_enc, status = 'disabled', last_error = @last_error, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: ConvertAccountToOwnApp :one
-- A disconnected Tech Provider account switches back to the company's own app.
UPDATE whatsapp_accounts SET connection_method = 'own_app', name = @name, app_id = @app_id, app_secret_enc = @app_secret_enc,
    access_token_enc = @access_token_enc, status = 'active', last_error = NULL, token_notified_at = NULL, updated_at = now()
WHERE id = @id AND connection_method = 'tech_provider'
RETURNING *;

-- name: SetTechProviderAccountToken :one
UPDATE whatsapp_accounts SET access_token_enc = @access_token_enc, app_id = @app_id, updated_at = now()
WHERE id = @id AND connection_method = 'tech_provider'
RETURNING *;

-- name: ActivateWhatsAppAccount :exec
UPDATE whatsapp_accounts SET status = 'active', last_error = NULL, token_notified_at = NULL, updated_at = now()
WHERE id = @id;

-- name: DisableWhatsAppAccount :one
UPDATE whatsapp_accounts SET status = 'disabled', last_error = @reason, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: CountLiveTechProviderAccounts :one
SELECT count(*) FROM whatsapp_accounts WHERE connection_method = 'tech_provider' AND status <> 'disabled';

-- name: InsertMetaOnboarding :one
INSERT INTO meta_onboardings (id, waba_id, phone_number_id, business_id, name, business_token_enc, pin_enc, pin_generated, created_by)
VALUES (@id, @waba_id, @phone_number_id, sqlc.narg(business_id), @name, sqlc.narg(business_token_enc), @pin_enc, @pin_generated, sqlc.narg(created_by))
RETURNING *;

-- name: GetMetaOnboarding :one
SELECT * FROM meta_onboardings WHERE id = @id;

-- name: GetOpenMetaOnboardingByWABA :one
SELECT * FROM meta_onboardings WHERE waba_id = @waba_id AND status IN ('running', 'failed');

-- name: ListMetaOnboardings :many
-- The unfinished onboardings and the most recent finished ones.
SELECT * FROM meta_onboardings
WHERE status IN ('running', 'failed') OR created_at > now() - interval '30 days'
ORDER BY created_at DESC
LIMIT 20;

-- name: ClaimMetaOnboarding :one
-- Starts a retry: a failed onboarding, or a running one whose request died
-- (no progress for two minutes). Concurrent retries get no row.
UPDATE meta_onboardings SET status = 'running', attempts = attempts + 1, updated_at = now()
WHERE id = @id AND (status = 'failed' OR (status = 'running' AND updated_at < now() - interval '2 minutes'))
RETURNING *;

-- name: SetMetaOnboardingInput :one
-- A retry may bring a new code (a new business token) or a new PIN.
UPDATE meta_onboardings SET
    business_token_enc = CASE WHEN @reset_token::boolean THEN NULL ELSE business_token_enc END,
    step               = CASE WHEN @reset_token::boolean THEN 'exchange' ELSE step END,
    pin_enc            = coalesce(sqlc.narg(pin_enc), pin_enc),
    pin_generated      = CASE WHEN sqlc.narg(pin_enc)::bytea IS NULL THEN pin_generated ELSE false END,
    name               = coalesce(sqlc.narg(name), name),
    updated_at         = now()
WHERE id = @id
RETURNING *;

-- name: AdvanceMetaOnboarding :one
UPDATE meta_onboardings SET step = @step,
    business_token_enc = coalesce(sqlc.narg(business_token_enc), business_token_enc),
    account_id         = coalesce(sqlc.narg(account_id), account_id),
    name               = coalesce(sqlc.narg(name), name),
    error_code = NULL, error_message = NULL, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: FailMetaOnboarding :one
UPDATE meta_onboardings SET status = 'failed', error_code = @error_code, error_message = @error_message, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: CompleteMetaOnboarding :one
-- Done: the business token now lives in the account. The PIN (the
-- number's two-step verification PIN) stays sealed for the owner to read.
UPDATE meta_onboardings SET status = 'completed', step = 'done', business_token_enc = NULL,
    error_code = NULL, error_message = NULL, completed_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: CancelMetaOnboarding :one
UPDATE meta_onboardings SET status = 'cancelled', business_token_enc = NULL, updated_at = now()
WHERE id = @id AND status IN ('running', 'failed')
RETURNING *;
