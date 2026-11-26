# Campaign analytics, forecasts, reports and spam protection

> Phase 16C P3. Code: `internal/modules/campaigns` (`analytics.go`,
> `forecast.go`, `protection.go`, `recipients.go`, `exports.go`,
> `report.go`, `analysis.go`), `internal/modules/metacost/rules.go`
> (charging rules), migration `00019_campaign_analytics.sql`. UI: campaign
> detail → **Analysis center**, wizard review step → **Pre-launch analysis**,
> **Settings → Campaign protection**.

## Where the numbers come from

Every figure is tagged with its source:

| Source | Figures |
|---|---|
| `meta` | what Meta reported: sent (accepted), delivered, read, failed and the error codes (131048, 131049, 131050 …), each message's pricing category and type, number quality, template status and quality, Meta's own daily cost (`pricing_analytics`) |
| `system` ("Sistem hesaplaması") | what Messora calculated: audience, consent blocks, pending, replies, positive/negative/neutral reactions, opt-outs, rates, forecasts, costs priced from the imported rate card |

Meta does **not** report which customer blocked or reported the business
(docs/PHASE-16C.md §1.3). The spam and quality section therefore shows only
aggregate signals.

## Recipient outcome and counters (R9)

Each `campaign_recipients` row mirrors its current message — `msg_status`,
`attempted_at` (first time the message reached Meta: sent, delivered, read,
failed, deferred or unknown), `sent_at`, `delivered_at`, `read_at`,
`failed_at`, `error_code`, `failure_class`, `cost_amount` — through a
trigger on `messages`, in the same transaction as the status change (status
webhooks, send outcomes, pauses, cancels, resends). On top of that:

- `replied_at`, `reply_message_id`: the first inbound message attributed to
  the recipient (see below);
- `reply_payload`, `reaction`, `reaction_source`, `reaction_at`: the tapped
  quick reply and the label (`positive | negative | neutral`, source
  `quick_reply | ai | manual`);
- `opted_out_at`: the contact stopped marketing within the attribution
  window.

`campaign_counters` holds one row per campaign, maintained by triggers on
`campaign_recipients` and `messages` (the delta of each row change). Reading
a campaign's numbers is one row; nothing is counted per webhook.
`campaign_counters_recompute(campaign)` recomputes the row from the rows
themselves (it locks the counters row first, so concurrent changes are
either counted or applied afterwards); the job `campaign:analytics_reconcile`
(every 5 minutes) runs it for active campaigns and campaigns finished within
the attribution window + 1 day, and logs any drift it repairs.

The API's `counts` keep their former keys (recipients by state, messages by
status) and add the analytics counters (`a_*`), so the realtime events
`campaign.progress`/`campaign.updated` carry them: the detail page updates
without polling.

### Definitions

| Metric | Definition |
|---|---|
| audience (`total`) | recipient rows of the campaign |
| blocked | recipients skipped by consent or rendering (`state = skipped`) or whose message was skipped at send time |
| eligible | audience − blocked |
| pending | not attempted yet or in flight (pending, retry, queued with a pending/sending/held/deferred message) |
| sent | recipients whose message Meta accepted (`sent_at` set; also counts if it later failed) |
| delivered / read | recipients with `delivered_at` / `read_at` |
| failed | recipients whose current message failed |
| rejected by Meta | failed with a Meta error code |
| replied | recipients with an attributed inbound message |
| positive / negative / neutral | recipients with that reaction label |
| opted out | recipients who stopped marketing within the window |
| cost | sum of the campaign messages' `cost_amount` (rate card price of Meta's pricing category, resends included) |
| sent rate | sent / eligible |
| delivery rate | delivered / sent |
| read rate | read / delivered |
| reply, positive, negative, opt-out rate | replied, positive, negative, opted out / delivered |
| duration | `finished_at − started_at` (live: now − started_at) |

The forecast and forecast vs actual use exactly these definitions.

## Attribution

A reply or an opt-out is credited to **the campaign that reached the
contact last** (the recipient with the latest `sent_at` not after the
event), and only while

- it is at most **attribution window** days after that send (setting,
  default 7, 1–30), and
- for replies, the message came in on the number the campaign sent from.

A newer campaign reaching the contact ends the earlier one's window by
construction (it becomes the latest). Meta's timestamps have whole seconds;
send times are compared at that precision. Only the first inbound message
sets `replied_at`. Opt-outs are the marketing consent becoming `opted_out`
(keywords, the API, manual changes) and the customer's **Stop** in WhatsApp
(`user_preferences`); a 131050 error only reveals an earlier Stop and is
counted separately (`err_131050`). AI usage is attributed with the same
window (`ai_usage.campaign_id`, docs/AI.md "Campaign attribution"); the
analysis center's `ai_cost` (requests, tokens, conversations, cost) sums
those rows, the same ones the AI usage page groups by campaign, and the
campaign's operating cost is Meta + AI.

## Positive / negative reactions

A campaign has a `reaction_map`: quick reply payload or button text →
`positive | negative | neutral` (set in the wizard's review step or on the
detail page; the template's quick reply buttons are offered). When an
attributed inbound message is a quick reply, its payload (else its text) is
looked up and the recipient is labelled with source `quick_reply`.
Changing the mapping relabels earlier taps. Operators can label a recipient
by hand (`manual`), which the system never overrides. Free-text first
replies are handed to the `ReplyClassifier`: with **Settings → AI →
Classify replies to campaigns** on, the AI classifier enqueues work inside
the inbound transaction and stores the result with source `ai`
(docs/AI.md "Campaign reply classification"). An AI label never replaces
a tapped quick reply; a quick reply replaces an AI label. All system labels are shown as "Sistem hesaplaması".

## Pre-launch analysis (forecast)

`POST /campaigns/forecast` (wizard review step) and, unchangeable, stored
in `campaign_forecasts` when the campaign starts materialising:

- **who**: total, eligible, blocked by reason (the consent preflight);
- **duration and finish**: the campaign's weighted fair share of the
  number's campaign lane with the campaigns running on it now (P2 scheduler:
  Meta throughput, reserved share, priority, rate cap). With a limited
  portfolio messaging tier, the eligible customers that would count against
  it (service window closed and not already counted in the last 24 hours)
  are compared with what is left of the limit: over it, sending needs
  several 24-hour windows (`limit.windows`) and the finish moves out
  accordingly;
- **Meta cost**: per recipient country × the template's category × the
  rate card valid at the start, by `metacost.Charged` (2026-10-01 rules:
  marketing and authentication always charged; utility inside an open
  service window free until 2026-09-30 and charged since 2026-10-01; service
  messages charged after the number's 1,000 free per month). Tiered rates
  use the month's first tier (an upper bound). A country without a rate is
  counted in `not_in_rate_card` ("fiyat kartında yok") and never guessed;
- **performance**: only from **completed** campaigns whose template has the
  same category, pooled (Σ numerator / Σ denominator), when there are at
  least **3 campaigns and 500 recipients**; otherwise `rates.status =
  insufficient_history` with what was found and what is needed. Nothing is
  invented.

`GET /campaigns/{id}/forecast` returns the stored forecast next to the
actual figures (live while running, `final` once ended) with differences
(`delta = actual − forecast`, `delta_pct` relative to the forecast).

## Recipients table, bulk actions, exports

`POST /campaigns/{id}/recipients/search`: server-side pages with total,
search (name, phone), sort (created, name, phone, sent/delivered/read/
replied time, cost) and filters — statuses ORed (sent, delivered, read,
failed, pending, unknown, blocked, cancelled, rejected, replied, positive,
negative, neutral, opted_out, and every unreached class of P2), tag, group,
segment, contact type, custom attribute = value, explicit ids.

`POST /campaigns/{id}/recipients/bulk`: add a tag, add to a group, or
**follow up** — a draft campaign with the same number, template, parameters
and reaction mapping whose audience is the selected contacts (audience type
`contact_ids`, at most 50,000). Audited (`campaign.recipients_bulk`).

Exports honour the filters. Up to 20,000 rows stream directly
(`GET …/recipients/export?format=csv|xlsx`, CSV with formula injection
neutralised, XLSX through excelize's stream writer); larger ones, page
selections and reports run as a background job (`campaign:export`, bulk
queue) whose file is kept for 7 days and downloaded from the Reports tab.
Requests and downloads are audited (`campaign.export`,
`campaign.export_requested`, `campaign.export_download`).

## Breakdowns

`GET /campaigns/{id}/breakdown?dimension=group|segment|tag|contact_type|attribute&attr_key=`
computes recipients, sent, delivered, read, replied, positive, negative,
opted out and cost per bucket in one query over the recipient rows (a
contact in two tags counts in both; segments are evaluated as `FILTER`
clauses of one query, at most 30). Contacts have an optional free-text
`contact_type` (contact detail page).

## Report

The report export: XLSX with the sheets **Özet, Maliyet, Teslimat,
Alıcılar, Segment, Etiket, Tahmin vs Gerçek**, or a one-page PDF summary
(go-pdf/fpdf, MIT; Go fonts, BSD, embedded for Turkish characters).

## Spam protection

Settings → **Campaign protection** (`/settings/campaign-protection`,
key `campaign_protection`):

| Setting | Default |
|---|---|
| enabled | on |
| stop at once on 131048 | on |
| stop on account restriction (`account_update` ACCOUNT_RESTRICTION, ACCOUNT_VIOLATION, DISABLED_UPDATE, ACCOUNT_DELETED, ACCOUNT_OFFBOARDED) | on |
| window / minimum attempts | 200 / 50 |
| 131049 + 131050 above | 10 % of the window's attempts |
| opt-outs above | 2 % of the window's attempts |
| quality stop (number, and template quality) | red (yellow: yellow or red; off) |
| stop when Meta pauses or disables the template | on |
| attribution window | 7 days |

Evaluation is incremental: on campaign messages failing or deferred with
131048/131049/131050 (send path and status webhooks), on attributed replies
and opt-outs, on account webhooks (`account_update`, a FLAGGED
`phone_number_quality_update`, `message_template_status_update`,
`message_template_quality_update`), after the hourly signal sync, and every
5 minutes by the reconciler. A tripped rule pauses the campaign through
P2's pause transition with reason `spam_protection` and the detail (`rule`,
values, thresholds) in `campaign_pauses`. After an operator resumes such a
campaign, the rates count only attempts after the resume, and the state
rules (quality, template) trip again only on a new event, not on the
unchanged state. The UI explains the rule and the resume dialog warns.

Number quality and template status/quality changes are also recorded as
account events (`quality_rating_change`, `template_change`), the campaign's
quality timeline.
