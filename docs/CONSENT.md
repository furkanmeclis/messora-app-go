# Subscription, consent and send blocking

> Phase 16C P1. Code: `internal/modules/consent` (rules, keywords, settings),
> `internal/modules/messaging` (send path, keywords on inbound messages,
> Meta signals), `internal/modules/campaigns/consent.go` (materialisation and
> preflight).

## Consent per category

Every contact has a consent status for two categories:

| Category | Covers |
|---|---|
| `marketing` | marketing templates (campaigns, API, automations) |
| `service` | utility and authentication templates, and replies inside the customer service window |

Statuses: `opted_in`, `unknown` (every contact starts here), `opt_out_requested`
(the customer asked to stop in their own words; an agent confirms),
`opted_out`, `resubscribed`. Each category keeps when it last changed, where
the change came from (`source`) and why (`reason`).

Two flags come from Meta and are kept apart from the business's own consent:

- **`meta_marketing_stopped_at`** — the customer chose *Stop* for this
  business's marketing in WhatsApp (`user_preferences` webhook, or error
  `131050` on a marketing message). *Resume* clears it. It never changes the
  contact's own marketing consent: a customer who opted out with the business
  stays opted out after resuming in WhatsApp.
- **`undeliverable_at`** (+ `undeliverable_code`) — Meta could not deliver to
  the number (`131026`). The next window-opening message from the number
  clears it.

**Legacy `opted_out`.** `opted_out_at` / `opted_out` mean "opted out of
everything": they are set exactly when both categories are `opted_out`.
`POST /contacts/{id}/consent` with `kind: opt_out | opt_in` and the bulk
`opt_out` / `opt_in` actions set both categories, as before. Contacts that
were opted out before this release were migrated to both categories
`opted_out`. A row whose `opted_out_at` is set counts as opted out of both
categories whatever its category columns say.

Every change is appended to `consent_events` (kind, category — `NULL` for
both —, resulting status, source, note, actor). Sources: `manual`, `bulk`,
`api`, `import`, `keyword`, `meta_user_preferences`, `meta_error`,
`automation`, `inbound`. Changes made in the panel or through the API are
also written to the audit log (`contact.opt_out`, `contact.opt_in`,
`contact.consent`, `contact.bulk_opt_out` …). History:
`GET /contacts/{id}/consent-events`.

## The rules (`consent.Decide`)

`Decide(contact consent, message category, company policy)` returns
`allow`, `warn(reason)` or `block(reason, hard)`. The message category is the
template's category from the template record (`MARKETING` → marketing,
`UTILITY` → utility, `AUTHENTICATION` → authentication, anything else →
marketing); a non-template message is `service`.

| Message | Block (hard, never overridable) | Block (company / pending) | Warn | Allow |
|---|---|---|---|---|
| any | number undeliverable | | | |
| marketing | marketing `opted_out`; stopped in WhatsApp | `opt_out_requested`; `unknown` when the company blocks unknown consent | `unknown` (default policy) | `opted_in`, `resubscribed` |
| utility, authentication | service `opted_out` | | service `opt_out_requested` | otherwise |
| service (reply in the window) | service `opted_out`, unless the customer wrote again after opting out | | service `opt_out_requested`; replying after an opt-out | otherwise |

- Hard blocks apply to every path: campaigns, `POST /messages`, inbox
  replies and automations. Nothing (no setting, API flag or automation)
  overrides them.
- A marketing opt-out (e.g. the customer wrote "İPTAL") does not stop
  utility templates or replies; a full opt-out stops everything
  business-initiated. A customer who writes again after a full opt-out may
  be answered in that conversation (the message opened a new window); the
  decision is a warning, not a block.
- **Company policy** (Settings → Subscription & consent): marketing to
  contacts with unknown consent is *allowed with a warning* (default) or
  *blocked*. Every existing contact starts as unknown.

Blocked sends answer `422` with a code per reason: `CONTACT_OPTED_OUT`,
`MARKETING_STOPPED`, `CONTACT_UNDELIVERABLE`, `OPT_OUT_REQUESTED`,
`CONSENT_UNKNOWN`. Automation steps that are refused are recorded as
`skipped` with that code.

### Where the rules run

1. **Queueing** (`messaging.QueueTx`): API, inbox and automation messages.
2. **Campaign materialisation and dispatch**: blocked recipients become
   `skipped` with the reason as `skip_reason` (`opted_out`,
   `meta_marketing_stopped`, `undeliverable`, `opt_out_requested`,
   `consent_unknown`).
3. **Right before Meta is called** (`messaging.Send`, after the claim): a
   contact who opted out after the message was queued is not sent to. The
   message becomes `skipped` with `failure_class = blocked_by_consent` and
   the reason in `error_code`.

### Preflight

`POST /campaigns/preflight` (`campaigns.create`) counts an audience for a
template without storing anything: `total`, `eligible` (including warned),
`warned`, `blocked`, `hard_blocked`, and the reasons. The campaign wizard
shows it on the review step, with the current unknown-consent policy.

## Keywords on inbound messages

Text (and quick-reply button) messages are normalised — case folded with
both Turkish i's (`İ I ı i` → `i`), Turkish letters without marks
(`ç ğ ö ş ü` → `c g o s u`), punctuation and emoji removed, spaces collapsed
— and matched against the company's lists:

| List | Default | Match | Effect |
|---|---|---|---|
| Stop words | `İPTAL`, `STOP`, `DUR`, `ABONELİKTEN ÇIK`, `UNSUBSCRIBE` | the whole message | marketing → `opted_out` at once; optional confirmation reply |
| Opt-out request phrases | `mesaj istemiyorum`, `bana tekrar yazmayın`, `artık mesaj almak istemiyorum`, `abonelikten çıkmak istiyorum`, … | anywhere, on word boundaries (messages up to 300 characters) | marketing → `opt_out_requested`; the assignee (or every owner and admin) gets a `notification.created` event on their personal channel |
| Resubscribe words | `BAŞLAT`, `START`, `ABONE OL` | the whole message | marketing (and service, if it was opted out) → `resubscribed`; optional confirmation reply |

Each match is recorded in the conversation's history (`opted_out`,
`opt_out_requested`, `resubscribed`) and raises the automation trigger
`subscription_request` (option `subscription`: `stop`, `request`, `resume`,
or any). The confirmation reply (TR/EN text in the settings, language from
the workspace default) is a service message inside the window the
customer's own message opened; it is never sent when turned off, and a
refusal never undoes the consent change.

Settings: `GET/PUT /settings/consent` (`settings.read` / `settings.update`,
audited as `settings.consent_update`).

## Meta errors

| Code | Meaning | Message | Contact |
|---|---|---|---|
| `131050` | the customer stopped marketing from this business | `failed`, `user_stopped_marketing` | `meta_marketing_stopped_at` (marketing messages only) |
| `131026` | undeliverable | `failed`, `undeliverable` | `undeliverable_at` |
| `131049` | withheld for ecosystem health (per-user marketing limit) | `deferred` until `next_attempt_at` (≥ 24 h), then retried; `failed`, `ecosystem_limit` after three attempts | — |

Both synchronous refusals and failed status webhooks are handled. A
`deferred` message does not count in its campaign's in-flight window (the
dispatcher keeps topping up with other recipients); the campaign completes
only once its deferred messages are sent or given up, ticking once a minute
meanwhile. The campaign reaper re-queues deferred messages whose retry task
was lost. A `131049` reported by the status webhook (after Meta accepted the
request) is recorded as `failed` / `ecosystem_limit`; it is not re-sent
automatically.

`messages.failure_class` says why a message ended failed, skipped or
cancelled: `rejected_by_meta`, `temporary` (gave up after repeated temporary
errors), `permanent` (not a Meta refusal: account, payload),
`user_stopped_marketing`, `undeliverable`, `ecosystem_limit`,
`blocked_by_consent`, `cancelled`.
