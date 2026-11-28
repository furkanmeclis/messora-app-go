# Subscriptions and billing

Phase 17 WP9. Messora sells plans, AI credit packs and message balance to
its companies in TL, VAT (KDV, 20 %) shown apart, paid **by bank transfer
(havale/EFT) only** at launch. Card payments (PayTR, iyzico Abonelik) come
later behind the same `payments.Provider` interface. Code:
`internal/modules/billing` (`invoicing/`, `payments/`), migration `00035`,
API tags `billing` (workspace) and `platform` (`/platform/billing/*`).

## Who is who

- **Seller.** The legal entity that sells and invoices is not settled yet
  (a partner company will likely issue the invoices), so the seller is a
  platform setting: company name, tax office and number, MERSİS, address,
  e-mail, a note printed on proformas and up to five TL bank accounts
  (IBAN checksum verified). Platform panel → Billing settings. Without a
  seller and a bank account no proforma is issued (`409
  BILLING_NOT_CONFIGURED`) and renewals wait (logged).
- **Buyer.** Each workspace enters its invoice details once (Settings →
  Billing): company (title, tax office, 10-digit VKN) or individual (full
  name, 11-digit TCKN, check digits verified), address, billing e-mail.
  Required before the first proforma (`409 BILLING_PROFILE_REQUIRED`).
  Buyer and seller are **snapshots** on every proforma and invoice record.
- **Official invoice.** `invoicing.Issuer` issues the e-Fatura / e-Arşiv
  invoice of a paid proforma. The launch issuer is `manual`: Messora keeps
  an invoice record (status `pending`), a platform admin records the
  official number and date and uploads the PDF issued elsewhere (status
  `issued`); the workspace downloads it. An integrator (Paraşüt, Sovos, …)
  replaces `manual` without touching billing.

## Subscription states

`tenants.subscription_status` (the registry, read by plans, the API guard
and the platform panel) with the period in `subscriptions` (one row per
company, created on first use):

| Status | Meaning | Panel |
| --- | --- | --- |
| `trial` | 14 days with the growth plan's limits (WP8). The stored value is `trial` (the brief's "trialing"; it predates billing). | info banner in the last 3 days |
| `active` | a paid period (`period_start`–`period_end`), or a company without a period (active before billing, comped by hand) that is never renewed automatically | — |
| `past_due` | the renewal was not paid by the period's end: **7 days of grace** (`grace_ends_at`), everything still works | amber banner |
| `read_only` | the trial ended unpaid, or the grace ended: read and export only | red banner |
| `canceled` | set by a platform admin: no renewal; read-only once the period ends | red banner after the end |

**Read-only** blocks, with `402 SUBSCRIPTION_READ_ONLY`:

- every `POST`, `PUT` and `PATCH` of a workspace session or API key except
  the operations marked `x-messora-read-only: allow` in the spec: signing
  in and out, the account and its second factor, accepting invitations, realtime
  tokens, billing (details, plan choice, proformas), exports and previews
  (campaign export, segment preview, forecast, preflight, recipient
  search), stopping work (pause and cancel a campaign, hand a conversation
  over, close it, mark it read), member roles and the workspace settings.
  `DELETE` is always allowed (removing data is never blocked). The guard
  checks the flag at `apiguard` (`Options.ReadOnly`), the spec is
  validated at start-up (`x-messora-read-only` only takes `allow`).
- engines, whatever calls them: `messaging.Queue` (API, inbox, AI replies,
  automation steps), campaign start/resume (`402`) and the campaign
  dispatcher (pauses a running campaign with reason
  `subscription_read_only`), automations (no new runs; a run that
  continues fails with `subscription_read_only`), the AI reply loop (does
  not answer).
- **Inbound is never lost**: Meta's webhooks are stored and processed;
  messages, statuses and conversations keep arriving.

Payment brings a read-only workspace back to `active` at once (the
replica that confirms drops its caches; the API and workers re-read the
state within 5 seconds, `billing.GateTTL`).

## Timeline (the billing job)

`billing:tick` runs every 15 minutes per active tenant (the WP3 fan-out).
Every transition checks the current status, and every e-mail or issued
renewal is recorded once in `billing_events` (unique per company, kind and
key), so a retried or repeated run does nothing again. All transitions
are written to the company's and the platform's audit logs.

```
trial ──(trial_ends_at, unpaid)──────────────────────────────▶ read_only
active ─(period_end − 10 d)─ renewal proforma (invoice_issued e-mail)
       ─(due − 7 d)─ payment_reminder ─(due − 3 d)─ payment_reminder ─(due day)─ payment_reminder
       ─(period_end, unpaid)─▶ past_due (payment_reminder "overdue", grace 7 d)
past_due ─(grace_ends_at)──────────────────────────────────────▶ read_only (subscription_suspended)
any ──(proforma paid)──▶ active
```

- **Renewal**: `renewal_lead_days` (default 10, at least 7) before the
  period ends, a proforma for the next period with the plan and cycle in
  effect (and a scheduled change, see below); due when the period ends.
  Issued once per period: a renewal a platform admin voids is not issued
  again (extend or comp the period instead).
- **Reminders**: 7 and 3 days before the due date and on the due day
  (the last 24 hours), each once; a run that comes late sends only the
  latest stage. They stop once the proforma is paid.
- **Paying the renewal** continues the periods (the next period starts
  where the last ended) while the company is active or past due. A
  read-only company that pays starts a new period on the payment day.
- **First purchase** (trial or a company without a period): the period
  starts on payment; paid during the trial, it ends one cycle after the
  trial's end (the remaining trial days are kept).

E-mails (docs/EMAIL.md) go to the owners and to the billing e-mail of the
invoice details, branded, in each person's language.

## Plan changes

`POST /billing/plan-change {plan_key, billing_cycle}`:

- **No paid period** (trial, read-only, canceled, never paid): a proforma
  for one full period of the plan; an unpaid one from before is replaced
  (one open plan proforma per company).
- **Upgrade** (a higher price in the same cycle, during a paid period):
  a prorated proforma; the plan changes when it is paid, the period stays.
  `prorated = (new price − current price) × days left ÷ days of the
  period`, whole days (the rest of today counts as a day), half up to a
  kuruş; due in `due_days` but never after the period's end; an unpaid
  upgrade lapses at its due date (its price was for the days left then).
- **Downgrade or another cycle**: scheduled (`pending_plan_key`,
  `pending_cycle`); the next renewal uses them, the plan changes when that
  period starts. `DELETE /billing/plan-change` drops the change.
- **In the renewal window** (an unpaid renewal is open) every change is
  scheduled and the renewal is re-issued with the new terms.
- Custom-priced plans (enterprise) are not self-serve (`409
  PLAN_NOT_SELF_SERVE`): the platform admin sets the company's price per
  cycle (`set_custom_price`) and the renewals use it.
- Past due: pay the renewal first (`409 RENEWAL_DUE`).

Yearly costs ten monthly prices (12 months for 10) unless the plan has
its own yearly list price.

## Money

- **Units**: kuruş (`int64`, 1 TL = 100) for documents and payments;
  micro-lira (1 TL = 1,000,000) for the message balance, because one
  message costs a fraction of a kuruş. API amounts are decimal strings
  (`"3588.00"`; the balance also with six decimals).
- **Rounding**: every division rounds **half up** (away from zero), the
  way Turkish invoices round: 0.5 kuruş becomes 1 kuruş. Not banker's
  rounding: `10.5 → 11`, `12.5 → 13`.
- **VAT** is computed once on a proforma's net total
  (`vat = round(net × rate)`), never per line; `total = net + vat`.
- **FX**: the USD → TL rate is a platform setting the admin enters daily
  (`pricing.fx_usd_try`, `fx_updated_at`). `billing.FXSource` and
  `Service.SetFXRate` are the hook for an automatic TCMB (Central Bank)
  rate later.

## Payment references

`MSR` + the company's code (4 characters) + its proforma counter (4) + a
check character, shown as `MSR-4F8K-002C-7`. Characters are Crockford
base32 (no I, L, O, U); input is upper-cased, `O` read as `0`, `I`/`L` as
`1`, spaces, dashes and dots ignored, and the prefix may be left out. The
check character is Luhn mod 32: it catches every wrong character and
almost every swap of neighbours (unit tests). References are unique
(database constraint); proforma document numbers are `PF-<year>-<n>`.

## Matching transfers (admin runbook)

Platform panel → **Payments**.

1. **Record** each incoming transfer (amount, date, sender, description)
   or **import the bank statement** (CSV, below). Automatic matching
   suggests a proforma:
   - `reference_amount`: a valid reference in the description and the
     amount is what the proforma still owes: confirm.
   - `reference`: the reference matches but the amount differs: a partial
     payment (the proforma stays `partially_paid` until the rest comes) or
     an overpayment (the rest is kept as account credit): confirm.
   - `amount`: no reference, and exactly one open proforma of all
     companies owes this amount: check the sender, then confirm.
   - none, with **candidates**: several proformas owe this amount; pick the
     right one by the sender's name.
2. **Confirm** (`match`): the amount is applied to the proforma; what it
   does not need becomes the company's **account credit**, which pays the
   company's other open proformas (due first, first) and every new one at
   once. A fully paid proforma is fulfilled immediately: the period is
   extended / plan changed / AI credits granted / message balance added,
   the invoice record created, `payment_received` sent.
3. A transfer for a company without a fitting proforma: match it to the
   company (`tenant_id`): all of it becomes account credit.
4. Not a customer payment (interest, refunds): **ignore** with a reason.
5. **Invoices** → record the official invoice number and date, upload the
   PDF. The workspace sees "issued" and downloads it.

Nothing is applied before an admin confirms. A transfer is matched once
(`409 TRANSFER_NOT_UNMATCHED`). Refunds are made outside Messora; record
them with an `adjust_account_credit` action.

### Statement CSV

UTF-8, a header row, comma or semicolon separated. Columns (Turkish names
accepted): `date`/`tarih` (`YYYY-MM-DD` or `DD.MM.YYYY`), `amount`/`tutar`
(`1234.56`, `1.234,56`, `1,234.56`, `1234,56`), `description`/`açıklama`,
optional `sender`/`gönderen` and `sender_iban`/`iban`. Only incoming
(positive) lines are recorded. A line is imported once: its fingerprint
covers the date, amount, sender, description and how many identical lines
came before it in the file, so re-importing a statement adds nothing.

```csv
Tarih;Tutar;Açıklama;Gönderen
28.11.2026;3.588,00;MSR-4F8K-002C-7 Aralık aboneliği;ACME GIDA LTD
```

## Admin actions

Tenant detail → Billing (every action needs a reason; both audit logs):
`extend_days`, `comp_months` (move the period end; a company without a
live period gets one from now, and becomes active), `set_cycle`,
`set_status` (`active`, `read_only`, `canceled`), `set_custom_price`
(per cycle, VAT excluded; `null` returns to the list price),
`adjust_message_balance` and `adjust_account_credit` (signed TL).
Lists: subscriptions (filter by status), proformas (void an unpaid one),
invoices, and an overview (MRR = paying companies' monthly value, yearly
divided by 12; ARR; counts by status; collections this and last month;
open amount; unmatched transfers; pending invoices).

## AI credit packs

A catalogue in the pricing setting (default 1,000 / 5,000 / 20,000 credits
for 590 / 2,490 / 7,990 TL + KDV). A paid pack is a `purchase` lot of the
WP8 AI credit ledger (`credits.Grant`, `reference` = the proforma's
reference), valid 12 months (`ai_credit_validity_months`).

## Message balance

Applies **only to companies sending through Messora's Tech Provider
connection**: Messora's Meta account pays Meta, and the fee is passed on.
**Companies with their own Meta app are billed by Meta directly**: nothing
is debited, nothing is checked, the balance is not shown as used.

- A TL wallet (`message_wallets`), topped up by paid proformas (the net
  amount; VAT is on the proforma), at least `min_message_topup` (1,000 TL).
- Every message Meta charges (pricing type `regular` in its status
  webhook) is debited **once** in the transaction that stores the pricing:
  Meta's USD price on the platform rate card × the FX rate, with the
  margin: **marketing × 1.20**, every other category **+ $0.0006** per
  message (platform settings). Free messages (service window, free entry
  points) cost nothing; failed messages carry no pricing and are never
  charged. Each charge keeps Meta's price, the rate and the balance after
  it (`GET /billing/message-balance`).
- Race-safe: one charge row per message (unique), the wallet row updated in
  place; concurrent webhooks queue on it and never lose a debit.
- Templates through a Tech Provider number need a balance above zero:
  single sends answer `402 MESSAGE_BALANCE_EXHAUSTED`, campaigns refuse to
  start or resume and a running campaign pauses with reason
  `message_balance`. Replies in the customer service window are free and
  keep working. The balance may go below zero by the messages already in
  flight when it reached zero; the next top-up covers it.
- Below `low_balance_threshold` (200 TL) one `credit_low` e-mail (an
  outbox task after the transaction); a top-up back above the threshold
  re-arms it.

## Not yet

- e-Fatura / e-Arşiv integration (an integrator `invoicing.Issuer`).
- Card payments (PayTR, iyzico Abonelik) and automatic top-ups.
- The TCMB daily rate fetcher (`FXSource`).
- Bank statement APIs (open banking) instead of CSV.
- Proration credits on downgrade (a downgrade waits for the renewal).
