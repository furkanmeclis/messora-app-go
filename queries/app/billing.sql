-- Subscriptions and billing (Phase 17 WP9, docs/BILLING.md). Billing rows
-- are tenant rows under row-level security, written by platform
-- transactions scoped to the company (SetTenantLocal); the platform panel
-- lists them across companies (platform_access policies). Transfers are a
-- platform table only the platform role sees.

-- ── Subscriptions ───────────────────────────────────────────────────────────

-- name: GetSubscription :one
SELECT * FROM subscriptions WHERE tenant_id = app_tenant_id();

-- name: GetSubscriptionForUpdate :one
SELECT * FROM subscriptions WHERE tenant_id = app_tenant_id() FOR UPDATE;

-- name: InsertSubscription :one
-- A taken code (or an existing row) inserts nothing; the caller retries
-- with another code.
INSERT INTO subscriptions (code) VALUES (@code)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: UpdateSubscription :one
UPDATE subscriptions SET
    billing_cycle = @billing_cycle, period_start = sqlc.narg(period_start), period_end = sqlc.narg(period_end),
    pending_plan_key = sqlc.narg(pending_plan_key), pending_cycle = sqlc.narg(pending_cycle),
    custom_price_kurus = sqlc.narg(custom_price_kurus), grace_ends_at = sqlc.narg(grace_ends_at),
    read_only_since = sqlc.narg(read_only_since), status_reason = @status_reason,
    account_credit_kurus = @account_credit_kurus, updated_at = now()
WHERE tenant_id = app_tenant_id()
RETURNING *;

-- name: NextProformaSeq :one
UPDATE subscriptions SET proforma_seq = proforma_seq + 1, updated_at = now()
WHERE tenant_id = app_tenant_id()
RETURNING code, proforma_seq;

-- name: SetTenantSubscriptionStatus :one
-- The platform role (the tenant registry).
UPDATE tenants SET subscription_status = @subscription_status, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetTenantPlanKey :one
UPDATE tenants SET plan_key = @plan_key, plan_changed_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: GetTenantBilling :one
-- The tenant registry row with its plan (readable without a tenant).
SELECT t.id, t.slug, t.name, t.status, t.subscription_status, t.trial_ends_at, t.plan_key,
       coalesce(p.name_tr, '')::text AS plan_name_tr, coalesce(p.name_en, '')::text AS plan_name_en,
       coalesce(p.price_monthly::text, '')::text AS plan_price_monthly, coalesce(p.price_yearly::text, '')::text AS plan_price_yearly,
       coalesce(p.custom_pricing, false)::boolean AS plan_custom_pricing
FROM tenants t LEFT JOIN plans p ON p.key = t.plan_key
WHERE t.id = @id;

-- name: ListBillingSubscriptions :many
-- Every company with its subscription (platform panel), optionally one
-- status.
SELECT t.id, t.slug, t.name, t.status, t.subscription_status, t.trial_ends_at, t.plan_key,
       coalesce(p.name_tr, '')::text AS plan_name_tr, coalesce(p.name_en, '')::text AS plan_name_en,
       coalesce(p.price_monthly::text, '')::text AS plan_price_monthly, coalesce(p.price_yearly::text, '')::text AS plan_price_yearly,
       s.billing_cycle, s.period_start, s.period_end, s.grace_ends_at, s.read_only_since, s.status_reason,
       s.custom_price_kurus, s.account_credit_kurus, s.pending_plan_key, s.pending_cycle,
       (SELECT coalesce(sum(f.total_kurus - f.paid_kurus), 0) FROM proformas f
        WHERE f.tenant_id = t.id AND f.status IN ('open', 'partially_paid'))::bigint AS open_kurus
FROM tenants t
LEFT JOIN subscriptions s ON s.tenant_id = t.id
LEFT JOIN plans p ON p.key = t.plan_key
WHERE t.status <> 'deleted' AND (sqlc.narg(status)::text IS NULL OR t.subscription_status = sqlc.narg(status)::text)
ORDER BY t.created_at
LIMIT @lim;

-- ── Billing profile ─────────────────────────────────────────────────────────

-- name: GetBillingProfile :one
SELECT * FROM billing_profiles WHERE tenant_id = app_tenant_id();

-- name: UpsertBillingProfile :one
INSERT INTO billing_profiles (kind, title, tax_office, tax_number, address, city, country, email, phone, updated_by)
VALUES (@kind, @title, @tax_office, @tax_number, @address, @city, @country, @email, @phone, sqlc.narg(updated_by))
ON CONFLICT (tenant_id) DO UPDATE SET
    kind = EXCLUDED.kind, title = EXCLUDED.title, tax_office = EXCLUDED.tax_office, tax_number = EXCLUDED.tax_number,
    address = EXCLUDED.address, city = EXCLUDED.city, country = EXCLUDED.country, email = EXCLUDED.email,
    phone = EXCLUDED.phone, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- ── Proformas ───────────────────────────────────────────────────────────────

-- name: NextProformaNumber :one
SELECT nextval('proforma_number_seq')::bigint;

-- name: InsertProforma :one
INSERT INTO proformas (number, reference, kind, renewal, lines, net_kurus, vat_rate_bp, vat_kurus, total_kurus,
                       plan_key, billing_cycle, period_start, period_end, ai_credits, pack_key, message_amount_kurus,
                       buyer, seller, due_at, created_by)
VALUES (@number, @reference, @kind, @renewal, @lines, @net_kurus, @vat_rate_bp, @vat_kurus, @total_kurus,
        sqlc.narg(plan_key), sqlc.narg(billing_cycle), sqlc.narg(period_start), sqlc.narg(period_end),
        sqlc.narg(ai_credits), sqlc.narg(pack_key), sqlc.narg(message_amount_kurus),
        @buyer, @seller, @due_at, sqlc.narg(created_by))
RETURNING *;

-- name: GetProforma :one
-- Tenant-scoped queries name the tenant explicitly: the platform role also
-- sees other companies' rows (platform_access).
SELECT * FROM proformas WHERE id = @id AND tenant_id = app_tenant_id();

-- name: GetProformaForUpdate :one
SELECT * FROM proformas WHERE id = @id AND tenant_id = app_tenant_id() FOR UPDATE;

-- name: GetPlatformProforma :one
-- Any company's proforma (platform panel).
SELECT * FROM proformas WHERE id = @id;

-- name: GetProformaByReference :one
SELECT * FROM proformas WHERE reference = @reference;

-- name: ListProformas :many
SELECT * FROM proformas WHERE tenant_id = app_tenant_id() ORDER BY created_at DESC LIMIT @lim;

-- name: ListPlatformProformas :many
SELECT f.*, t.slug AS tenant_slug, t.name AS tenant_name
FROM proformas f JOIN tenants t ON t.id = f.tenant_id
WHERE (sqlc.narg(status)::text IS NULL OR f.status = sqlc.narg(status)::text)
  AND (sqlc.narg(tenant_id)::uuid IS NULL OR f.tenant_id = sqlc.narg(tenant_id)::uuid)
ORDER BY f.created_at DESC
LIMIT @lim;

-- name: OpenProformasByAmount :many
-- Open proformas of every company still owing exactly amount (matching a
-- transfer without a reference).
SELECT * FROM proformas
WHERE status IN ('open', 'partially_paid') AND total_kurus - paid_kurus = @amount
ORDER BY due_at
LIMIT 10;

-- name: LockOpenProformas :many
-- The company's open proformas in the order money is applied to them.
SELECT * FROM proformas
WHERE tenant_id = app_tenant_id() AND status IN ('open', 'partially_paid')
ORDER BY due_at, created_at
FOR UPDATE;

-- name: GetOpenPlanProforma :one
SELECT * FROM proformas WHERE tenant_id = app_tenant_id() AND kind IN ('subscription', 'upgrade') AND status IN ('open', 'partially_paid');

-- name: GetRenewalProforma :one
SELECT * FROM proformas WHERE tenant_id = app_tenant_id() AND renewal AND period_start = @period_start AND status <> 'void';

-- name: AddProformaPayment :one
UPDATE proformas SET paid_kurus = paid_kurus + @amount,
    status = CASE WHEN paid_kurus + @amount >= total_kurus THEN 'paid' ELSE 'partially_paid' END,
    paid_at = CASE WHEN paid_kurus + @amount >= total_kurus THEN now() ELSE paid_at END,
    updated_at = now()
WHERE id = @id AND tenant_id = app_tenant_id() AND status IN ('open', 'partially_paid') AND paid_kurus + @amount <= total_kurus
RETURNING *;

-- name: MarkProformaFulfilled :exec
UPDATE proformas SET fulfilled_at = now(), updated_at = now() WHERE id = @id AND tenant_id = app_tenant_id() AND fulfilled_at IS NULL;

-- name: VoidProforma :one
-- Only an unpaid proforma can be voided (money received stays applied).
UPDATE proformas SET status = 'void', voided_at = now(), void_reason = @reason, updated_at = now()
WHERE id = @id AND tenant_id = app_tenant_id() AND status = 'open' AND paid_kurus = 0
RETURNING *;

-- name: ListStaleUpgrades :many
SELECT * FROM proformas WHERE tenant_id = app_tenant_id() AND kind = 'upgrade' AND status = 'open' AND paid_kurus = 0 AND due_at <= @now;

-- ── Payments and account credit ─────────────────────────────────────────────

-- name: InsertBillingPayment :one
INSERT INTO billing_payments (proforma_id, source, transfer_id, amount_kurus, received_on, created_by)
VALUES (@proforma_id, @source, sqlc.narg(transfer_id), @amount_kurus, @received_on, sqlc.narg(created_by))
RETURNING *;

-- name: ListBillingPayments :many
SELECT * FROM billing_payments WHERE tenant_id = app_tenant_id() ORDER BY created_at DESC LIMIT @lim;

-- name: InsertAccountEntry :one
INSERT INTO billing_account_entries (amount_kurus, balance_after, reason, transfer_id, proforma_id, note, created_by)
VALUES (@amount_kurus, @balance_after, @reason, sqlc.narg(transfer_id), sqlc.narg(proforma_id), @note, sqlc.narg(created_by))
RETURNING *;

-- name: ListAccountEntries :many
SELECT * FROM billing_account_entries WHERE tenant_id = app_tenant_id() ORDER BY created_at DESC LIMIT @lim;

-- name: CollectedBetween :one
-- Money received by transfer across companies in [from, to) (platform).
SELECT coalesce(sum(amount_kurus), 0)::bigint FROM bank_transfers
WHERE status = 'matched' AND received_on >= @from_day AND received_on < @to_day;

-- ── Invoices ────────────────────────────────────────────────────────────────

-- name: InsertInvoice :one
INSERT INTO invoices (proforma_id, issuer, status, official_number, issued_on, net_kurus, vat_kurus, total_kurus, buyer, seller)
VALUES (@proforma_id, @issuer, @status, sqlc.narg(official_number), sqlc.narg(issued_on), @net_kurus, @vat_kurus, @total_kurus, @buyer, @seller)
ON CONFLICT (tenant_id, proforma_id) DO NOTHING
RETURNING *;

-- name: GetInvoice :one
SELECT * FROM invoices WHERE id = @id AND tenant_id = app_tenant_id();

-- name: GetPlatformInvoice :one
SELECT * FROM invoices WHERE id = @id;

-- name: ListInvoices :many
SELECT i.*, f.number AS proforma_number, f.kind AS proforma_kind
FROM invoices i JOIN proformas f ON f.tenant_id = i.tenant_id AND f.id = i.proforma_id
WHERE i.tenant_id = app_tenant_id()
ORDER BY i.created_at DESC LIMIT @lim;

-- name: ListPlatformInvoices :many
SELECT i.*, f.number AS proforma_number, f.kind AS proforma_kind, t.slug AS tenant_slug, t.name AS tenant_name
FROM invoices i
JOIN proformas f ON f.tenant_id = i.tenant_id AND f.id = i.proforma_id
JOIN tenants t ON t.id = i.tenant_id
WHERE (sqlc.narg(status)::text IS NULL OR i.status = sqlc.narg(status)::text)
  AND (sqlc.narg(tenant_id)::uuid IS NULL OR i.tenant_id = sqlc.narg(tenant_id)::uuid)
ORDER BY i.created_at DESC
LIMIT @lim;

-- name: SetInvoiceOfficial :one
UPDATE invoices SET status = 'issued', official_number = @official_number, issued_on = @issued_on,
    issued_by = sqlc.narg(issued_by), updated_at = now()
WHERE id = @id AND tenant_id = app_tenant_id()
RETURNING *;

-- name: SetInvoicePDF :one
UPDATE invoices SET pdf_key = @pdf_key, updated_at = now() WHERE id = @id AND tenant_id = app_tenant_id() RETURNING *;

-- ── Events (idempotency of the billing job) ─────────────────────────────────

-- name: InsertBillingEvent :execrows
INSERT INTO billing_events (kind, key, proforma_id) VALUES (@kind, @key, sqlc.narg(proforma_id))
ON CONFLICT (tenant_id, kind, key) DO NOTHING;

-- name: CountBillingEvents :one
SELECT count(*) FROM billing_events WHERE kind = @kind;

-- ── Transfers (platform) ────────────────────────────────────────────────────

-- name: InsertBankTransfer :one
INSERT INTO bank_transfers (amount_kurus, received_on, sender_name, sender_iban, description, reference, source, fingerprint,
                            suggested_proforma_id, suggestion, note, created_by)
VALUES (@amount_kurus, @received_on, @sender_name, sqlc.narg(sender_iban), @description, sqlc.narg(reference), @source,
        sqlc.narg(fingerprint), sqlc.narg(suggested_proforma_id), sqlc.narg(suggestion), @note, sqlc.narg(created_by))
ON CONFLICT (fingerprint) DO NOTHING
RETURNING *;

-- name: GetBankTransfer :one
SELECT * FROM bank_transfers WHERE id = @id;

-- name: GetBankTransferForUpdate :one
SELECT * FROM bank_transfers WHERE id = @id FOR UPDATE;

-- name: ListBankTransfers :many
SELECT b.*, t.slug AS tenant_slug, t.name AS tenant_name
FROM bank_transfers b LEFT JOIN tenants t ON t.id = b.tenant_id
WHERE (sqlc.narg(status)::text IS NULL OR b.status = sqlc.narg(status)::text)
ORDER BY b.received_on DESC, b.created_at DESC
LIMIT @lim;

-- name: MatchBankTransfer :one
UPDATE bank_transfers SET status = 'matched', tenant_id = @tenant_id, proforma_id = sqlc.narg(proforma_id),
    match_method = @match_method, matched_by = sqlc.narg(matched_by), matched_at = now(),
    note = @note, updated_at = now()
WHERE id = @id AND status = 'unmatched'
RETURNING *;

-- name: IgnoreBankTransfer :one
UPDATE bank_transfers SET status = 'ignored', note = @note, matched_by = sqlc.narg(matched_by), updated_at = now()
WHERE id = @id AND status = 'unmatched'
RETURNING *;

-- name: CountPendingInvoices :one
SELECT count(*) FROM invoices WHERE status = 'pending';

-- name: CountUnmatchedTransfers :one
SELECT count(*) FROM bank_transfers WHERE status = 'unmatched';

-- ── Message balance ─────────────────────────────────────────────────────────

-- name: EnsureMessageWallet :exec
INSERT INTO message_wallets (tenant_id) VALUES (app_tenant_id()) ON CONFLICT (tenant_id) DO NOTHING;

-- name: GetMessageWallet :one
SELECT * FROM message_wallets WHERE tenant_id = app_tenant_id();

-- name: AddMessageWalletBalance :one
-- Concurrent debits and top-ups queue on the wallet row: no update is lost.
UPDATE message_wallets SET balance_micros = balance_micros + @amount, updated_at = now()
WHERE tenant_id = app_tenant_id()
RETURNING *;

-- name: InsertMessageCharge :one
-- A message's charge, once (a repeated pricing webhook inserts nothing).
INSERT INTO message_wallet_entries (kind, amount_micros, balance_after, message_id, category, meta_usd_micros, fx_rate)
VALUES ('charge', @amount_micros, 0, @message_id, @category, @meta_usd_micros, (@fx_rate::text)::numeric(14, 6))
ON CONFLICT (tenant_id, message_id) WHERE kind = 'charge' DO NOTHING
RETURNING id;

-- name: SetWalletEntryBalance :exec
UPDATE message_wallet_entries SET balance_after = @balance_after WHERE id = @id;

-- name: InsertMessageWalletEntry :one
INSERT INTO message_wallet_entries (kind, amount_micros, balance_after, proforma_id, note, created_by)
VALUES (@kind, @amount_micros, @balance_after, sqlc.narg(proforma_id), @note, sqlc.narg(created_by))
RETURNING *;

-- name: ListMessageWalletEntries :many
SELECT * FROM message_wallet_entries ORDER BY created_at DESC LIMIT @lim;

-- name: MarkWalletLowNotified :execrows
UPDATE message_wallets SET low_notified_at = now() WHERE tenant_id = app_tenant_id() AND low_notified_at IS NULL;

-- name: ClearWalletLowNotified :exec
UPDATE message_wallets SET low_notified_at = NULL WHERE tenant_id = app_tenant_id();

-- name: AccountConnectionMethod :one
SELECT a.connection_method FROM phone_numbers p JOIN whatsapp_accounts a ON a.id = p.account_id WHERE p.id = @phone_number_id;

-- name: UsesTechProvider :one
SELECT EXISTS (SELECT 1 FROM whatsapp_accounts WHERE connection_method = 'tech_provider' AND status <> 'disabled');
