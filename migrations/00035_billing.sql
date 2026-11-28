-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP9: subscriptions and billing (docs/BILLING.md, docs/PHASE-17.md
-- §1.4 and §5.13). Payment is by bank transfer (havale/EFT) only: a company
-- gets a proforma with Messora's IBANs and a unique payment reference, a
-- platform admin records the incoming transfer and confirms the match, the
-- subscription is extended (or AI credits / message balance added) and an
-- invoice record is created for the official e-Fatura/e-Arşiv invoice.
--
-- Money is integer kuruş (1 TL = 100 kuruş) everywhere except the message
-- balance, which is micro-lira (1 TL = 1,000,000) because one message costs
-- a fraction of a kuruş.

-- ── Subscription status ────────────────────────────────────────────────────
-- read_only: the trial ended or the grace period after an unpaid renewal
-- passed; the panel reads and exports, nothing is sent or created.
-- squawk-ignore ban-drop-constraint
ALTER TABLE tenants DROP CONSTRAINT tenants_subscription_status_check;
ALTER TABLE tenants ADD CONSTRAINT tenants_subscription_status_check
    CHECK (subscription_status IN ('trial', 'active', 'past_due', 'read_only', 'canceled')) NOT VALID;
-- The tenant registry is small: validated at once.
-- squawk-ignore constraint-missing-not-valid
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_subscription_status_check;

-- ── Subscriptions ──────────────────────────────────────────────────────────
-- One row per company (created on first use). The status itself stays in
-- tenants.subscription_status (read by plans, the platform panel and the
-- API guard); this row holds the billing period and what belongs to it.
-- A company without a period (active before billing, or a trial) is never
-- renewed automatically.
CREATE TABLE subscriptions (
    tenant_id            uuid PRIMARY KEY DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    -- The company's code in payment references (Crockford base32).
    code                 text NOT NULL UNIQUE CHECK (code ~ '^[0-9A-HJKMNP-TV-Z]{4}$'),
    billing_cycle        text NOT NULL DEFAULT 'monthly' CHECK (billing_cycle IN ('monthly', 'yearly')),
    period_start         timestamptz,
    period_end           timestamptz,
    -- A lower plan or another cycle chosen mid-period: the next renewal
    -- uses them.
    pending_plan_key     text REFERENCES plans (key) ON UPDATE CASCADE,
    pending_cycle        text CHECK (pending_cycle IN ('monthly', 'yearly')),
    -- A company-specific price per cycle, VAT excluded (custom-priced
    -- plans such as enterprise); NULL = the plan's list price.
    custom_price_kurus   bigint CHECK (custom_price_kurus >= 0),
    grace_ends_at        timestamptz,
    read_only_since      timestamptz,
    status_reason        text NOT NULL DEFAULT '' CHECK (length(status_reason) <= 500),
    -- Money received but not owed yet (overpayments, unmatched transfers):
    -- applied to the next proformas.
    account_credit_kurus bigint NOT NULL DEFAULT 0 CHECK (account_credit_kurus >= 0),
    -- The company's proforma counter (the second part of a reference).
    proforma_seq         bigint NOT NULL DEFAULT 0 CHECK (proforma_seq >= 0),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT subscriptions_period CHECK ((period_start IS NULL) = (period_end IS NULL) AND (period_end IS NULL OR period_end > period_start))
);

-- The buyer on proformas and invoices: required before the first proforma.
CREATE TABLE billing_profiles (
    tenant_id   uuid PRIMARY KEY DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('company', 'individual')),
    -- Company title (ticari unvan) or the person's full name.
    title       text NOT NULL CHECK (length(title) BETWEEN 2 AND 200),
    tax_office  text NOT NULL DEFAULT '' CHECK (length(tax_office) <= 100),
    -- VKN (10 digits, companies) or TCKN (11 digits, individuals).
    tax_number  text NOT NULL CHECK (tax_number ~ '^[0-9]{10,11}$'),
    address     text NOT NULL CHECK (length(address) BETWEEN 5 AND 500),
    city        text NOT NULL DEFAULT '' CHECK (length(city) <= 100),
    country     text NOT NULL DEFAULT 'TR' CHECK (country ~ '^[A-Z]{2}$'),
    email       text NOT NULL CHECK (length(email) BETWEEN 3 AND 254),
    phone       text NOT NULL DEFAULT '' CHECK (length(phone) <= 40),
    updated_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- ── Proformas ──────────────────────────────────────────────────────────────
-- What a company is asked to pay: a plan period (renewal or first
-- purchase), a prorated upgrade, an AI credit pack or a message balance
-- top-up. number is the document number (PF-<year>-<n>, one sequence for
-- the platform); reference is what the company writes in the transfer's
-- description (MSR + company code + proforma counter + check character).
-- buyer and seller are snapshots taken when the proforma is created.
CREATE SEQUENCE proforma_number_seq;
CREATE TABLE proformas (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id             uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    number                text NOT NULL UNIQUE,
    reference             text NOT NULL UNIQUE CHECK (reference ~ '^MSR[0-9A-HJKMNP-TV-Z]{9}$'),
    kind                  text NOT NULL CHECK (kind IN ('subscription', 'upgrade', 'ai_credits', 'message_balance')),
    status                text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'partially_paid', 'paid', 'void')),
    -- A renewal of the period ending at period_start (created by the
    -- billing job).
    renewal               boolean NOT NULL DEFAULT false,
    -- lines: [{"description_tr", "description_en", "quantity", "unit_kurus", "net_kurus"}].
    lines                 jsonb NOT NULL CHECK (jsonb_typeof(lines) = 'array'),
    net_kurus             bigint NOT NULL CHECK (net_kurus >= 0),
    -- Basis points: 2000 = 20 %.
    vat_rate_bp           bigint NOT NULL CHECK (vat_rate_bp BETWEEN 0 AND 10000),
    vat_kurus             bigint NOT NULL CHECK (vat_kurus >= 0),
    total_kurus           bigint NOT NULL CHECK (total_kurus > 0),
    paid_kurus            bigint NOT NULL DEFAULT 0 CHECK (paid_kurus >= 0),
    currency              text NOT NULL DEFAULT 'TRY' CHECK (currency = 'TRY'),
    plan_key              text REFERENCES plans (key) ON UPDATE CASCADE,
    billing_cycle         text CHECK (billing_cycle IN ('monthly', 'yearly')),
    period_start          timestamptz,
    period_end            timestamptz,
    ai_credits            bigint CHECK (ai_credits > 0),
    pack_key              text,
    -- The message balance a top-up adds (VAT excluded), kuruş.
    message_amount_kurus  bigint CHECK (message_amount_kurus > 0),
    buyer                 jsonb NOT NULL,
    seller                jsonb NOT NULL,
    due_at                timestamptz NOT NULL,
    paid_at               timestamptz,
    fulfilled_at          timestamptz,
    voided_at             timestamptz,
    void_reason           text NOT NULL DEFAULT '' CHECK (length(void_reason) <= 500),
    created_by            uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT proformas_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT proformas_total CHECK (total_kurus = net_kurus + vat_kurus),
    CONSTRAINT proformas_paid CHECK (paid_kurus <= total_kurus)
);
-- One open plan proforma (period or upgrade) per company at a time.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX proformas_one_open_plan ON proformas (tenant_id) WHERE kind IN ('subscription', 'upgrade') AND status IN ('open', 'partially_paid');
-- One live renewal per period (a plan change in the renewal window
-- replaces it; the billing job issues a period's renewal once, see
-- billing_events).
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX proformas_one_renewal ON proformas (tenant_id, period_start) WHERE renewal AND status <> 'void';
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX proformas_status_idx ON proformas (status, due_at);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX proformas_tenant_idx ON proformas (tenant_id, created_at DESC);

-- ── Incoming transfers (platform) ──────────────────────────────────────────
-- Bank transfers recorded by a platform admin (by hand or from a bank
-- statement CSV). A transfer is matched to a company (and usually one of
-- its proformas) when the admin confirms it; tenant_id names that company.
-- Only the platform role sees this table.
CREATE TABLE bank_transfers (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    amount_kurus          bigint NOT NULL CHECK (amount_kurus > 0),
    currency              text NOT NULL DEFAULT 'TRY' CHECK (currency = 'TRY'),
    received_on           date NOT NULL,
    sender_name           text NOT NULL DEFAULT '' CHECK (length(sender_name) <= 200),
    sender_iban           text CHECK (sender_iban ~ '^[A-Z]{2}[0-9]{2}[0-9A-Z]{10,30}$'),
    description           text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    -- The payment reference found in (or typed for) the description.
    reference             text CHECK (reference ~ '^MSR[0-9A-HJKMNP-TV-Z]{9}$'),
    source                text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'csv')),
    -- Statement lines are imported once (sha256 of the line's fields).
    fingerprint           text UNIQUE,
    status                text NOT NULL DEFAULT 'unmatched' CHECK (status IN ('unmatched', 'matched', 'ignored')),
    -- What automatic matching suggests (confirmed by an admin).
    suggested_proforma_id uuid,
    suggestion            text CHECK (suggestion IN ('reference_amount', 'reference', 'amount')),
    match_method          text CHECK (match_method IN ('reference_amount', 'reference', 'amount', 'manual')),
    tenant_id             uuid REFERENCES tenants (id) ON DELETE SET NULL,
    proforma_id           uuid,
    note                  text NOT NULL DEFAULT '' CHECK (length(note) <= 500),
    created_by            uuid REFERENCES users (id) ON DELETE SET NULL,
    matched_by            uuid REFERENCES users (id) ON DELETE SET NULL,
    matched_at            timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX bank_transfers_status_idx ON bank_transfers (status, received_on DESC);

-- ── Payments and account credit ────────────────────────────────────────────
-- How a company's money was used: an amount of a transfer (or of its
-- account credit) applied to a proforma.
CREATE TABLE billing_payments (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id     uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    proforma_id   uuid NOT NULL,
    source        text NOT NULL CHECK (source IN ('transfer', 'account_credit')),
    transfer_id   uuid REFERENCES bank_transfers (id) ON DELETE SET NULL,
    amount_kurus  bigint NOT NULL CHECK (amount_kurus > 0),
    received_on   date NOT NULL,
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT billing_payments_proforma_fkey FOREIGN KEY (tenant_id, proforma_id) REFERENCES proformas (tenant_id, id) ON DELETE CASCADE
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX billing_payments_tenant_idx ON billing_payments (tenant_id, created_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX billing_payments_received_idx ON billing_payments (received_on);

-- The account credit's history (subscriptions.account_credit_kurus is its
-- running balance): money in (overpayment, a transfer matched to the
-- company only, an admin adjustment) and out (applied to a proforma).
CREATE TABLE billing_account_entries (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id     uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    amount_kurus  bigint NOT NULL CHECK (amount_kurus <> 0),
    balance_after bigint NOT NULL CHECK (balance_after >= 0),
    reason        text NOT NULL CHECK (reason IN ('overpayment', 'unallocated', 'applied', 'adjustment')),
    transfer_id   uuid REFERENCES bank_transfers (id) ON DELETE SET NULL,
    proforma_id   uuid,
    note          text NOT NULL DEFAULT '' CHECK (length(note) <= 500),
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX billing_account_entries_tenant_idx ON billing_account_entries (tenant_id, created_at DESC);

-- ── Invoices ───────────────────────────────────────────────────────────────
-- The invoice record of a paid proforma. The official e-Fatura/e-Arşiv
-- invoice is issued by the invoicing issuer; the launch issuer is "manual":
-- a platform admin records the official number and uploads the PDF.
CREATE TABLE invoices (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id          uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    proforma_id        uuid NOT NULL,
    issuer             text NOT NULL DEFAULT 'manual' CHECK (length(issuer) BETWEEN 1 AND 40),
    status             text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'issued')),
    official_number    text CHECK (length(official_number) BETWEEN 1 AND 60),
    issued_on          date,
    pdf_key            text,
    net_kurus          bigint NOT NULL CHECK (net_kurus >= 0),
    vat_kurus          bigint NOT NULL CHECK (vat_kurus >= 0),
    total_kurus        bigint NOT NULL CHECK (total_kurus > 0),
    buyer              jsonb NOT NULL,
    seller             jsonb NOT NULL,
    issued_by          uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoices_one_per_proforma UNIQUE (tenant_id, proforma_id),
    CONSTRAINT invoices_proforma_fkey FOREIGN KEY (tenant_id, proforma_id) REFERENCES proformas (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT invoices_issued CHECK ((status = 'issued') = (official_number IS NOT NULL))
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX invoices_status_idx ON invoices (status, created_at DESC);

-- What the billing job did once per company (reminders, transitions): a
-- retried or repeated job finds its key and does nothing again.
CREATE TABLE billing_events (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id   uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (length(kind) BETWEEN 1 AND 60),
    key         text NOT NULL CHECK (length(key) BETWEEN 1 AND 200),
    proforma_id uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT billing_events_once UNIQUE (tenant_id, kind, key)
);

-- ── Message balance ────────────────────────────────────────────────────────
-- Companies on Messora's Tech Provider connection send through Messora's
-- Meta account: every template message Meta charges is debited from this
-- TL wallet at Meta's rate × the platform's FX rate and margin. Companies
-- with their own Meta app are billed by Meta directly and never use it.
-- balance_micros may go below zero by the messages already in flight when
-- it reached zero (new sends are refused from then on).
CREATE TABLE message_wallets (
    tenant_id        uuid PRIMARY KEY DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    balance_micros   bigint NOT NULL DEFAULT 0,
    low_notified_at  timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE message_wallet_entries (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id        uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    kind             text NOT NULL CHECK (kind IN ('top_up', 'charge', 'adjustment')),
    amount_micros    bigint NOT NULL CHECK (amount_micros <> 0),
    balance_after    bigint NOT NULL,
    message_id       uuid,
    proforma_id      uuid,
    category         text,
    -- A charge: Meta's price (USD micros), the FX rate and the margin used.
    meta_usd_micros  bigint,
    fx_rate          numeric(14, 6),
    note             text NOT NULL DEFAULT '' CHECK (length(note) <= 500),
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT message_wallet_entries_charge CHECK ((kind = 'charge') = (message_id IS NOT NULL))
);
-- A message is charged once, whatever webhook repeats its pricing.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX message_wallet_entries_one_charge ON message_wallet_entries (tenant_id, message_id) WHERE kind = 'charge';
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX message_wallet_entries_tenant_idx ON message_wallet_entries (tenant_id, created_at DESC);

-- ── Row-level security ─────────────────────────────────────────────────────
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON subscriptions USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE billing_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON billing_profiles USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE proformas ENABLE ROW LEVEL SECURITY;
ALTER TABLE proformas FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON proformas USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE billing_payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_payments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON billing_payments USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE billing_account_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_account_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON billing_account_entries USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON invoices USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE billing_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON billing_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE message_wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE message_wallets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON message_wallets USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE message_wallet_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE message_wallet_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON message_wallet_entries USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- ── Privileges ─────────────────────────────────────────────────────────────
-- Billing documents and the subscription are written only by platform
-- transactions (the billing service scopes them to the company with
-- app.tenant_id): a company reads its own rows and changes nothing
-- directly, whatever a bug in a tenant code path would try.
REVOKE INSERT, UPDATE, DELETE ON subscriptions, billing_profiles, proformas, billing_payments, billing_account_entries,
    invoices, billing_events FROM messora_app;
-- The message wallet is debited in the transaction that records Meta's
-- pricing of a message (the application role, row-level security).
REVOKE DELETE ON message_wallets, message_wallet_entries FROM messora_app;
-- Transfers are the platform's.
REVOKE ALL ON bank_transfers FROM messora_app;
GRANT SELECT, INSERT, UPDATE ON subscriptions, billing_profiles, proformas, billing_payments, billing_account_entries,
    invoices, billing_events, message_wallets, message_wallet_entries, bank_transfers TO messora_platform;
GRANT USAGE ON SEQUENCE proforma_number_seq TO messora_platform;
-- Lists across companies (subscriptions, proformas, invoices, payments) in
-- the platform panel.
CREATE POLICY platform_access ON subscriptions TO messora_platform USING (true) WITH CHECK (true);
CREATE POLICY platform_access ON proformas TO messora_platform USING (true) WITH CHECK (true);
CREATE POLICY platform_access ON invoices TO messora_platform USING (true) WITH CHECK (true);
CREATE POLICY platform_access ON billing_payments TO messora_platform USING (true) WITH CHECK (true);
CREATE POLICY platform_access ON billing_profiles TO messora_platform USING (true) WITH CHECK (true);

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS message_wallet_entries;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS message_wallets;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS billing_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS invoices;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS billing_account_entries;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS billing_payments;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS bank_transfers;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS proformas;
-- squawk-ignore ban-drop-sequence
DROP SEQUENCE IF EXISTS proforma_number_seq;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS billing_profiles;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS subscriptions;
UPDATE tenants SET subscription_status = 'past_due' WHERE subscription_status = 'read_only';
-- squawk-ignore ban-drop-constraint
ALTER TABLE tenants DROP CONSTRAINT tenants_subscription_status_check;
ALTER TABLE tenants ADD CONSTRAINT tenants_subscription_status_check
    CHECK (subscription_status IN ('trial', 'active', 'past_due', 'canceled')) NOT VALID;
