# AI providers, prices and AI profiles

Messora connects to large language model providers through one internal
interface (`internal/modules/ai`). Every AI feature — replies and
suggestions in the inbox, classification, data extraction, automation
steps — goes through `ai.Service.Generate`, which:

- refuses every call while the company switch **AI is on** is off
  (`AI_DISABLED`);
- picks the provider and model: the call's, then the AI profile's override,
  then the company default (Settings → AI → General);
- sends the request with the provider's no-retention options where they
  exist (below);
- maps provider failures to typed errors and never retries on its own;
- records **exactly one** row in `ai_usage` per call — successful, failed
  or refused — with the token counts, the cost at the price valid at that
  moment, the latency and the purpose.

Configuration lives in Settings → AI (providers, models and prices,
company settings) and in **AI profiles** (one per WhatsApp number). The
permissions are `ai.read` (owners, admins, managers) and `ai.manage`
(owners and admins). Every change is written to the audit log.

## Providers

| Provider | SDK | Generation | Structured output | Key check (free) |
|---|---|---|---|---|
| OpenAI | `openai-go/v3` v3.107.0 | Responses API | `text.format` `json_schema`, strict | `GET /v1/models` |
| Anthropic | `anthropic-sdk-go/v2` v2.15.0 | Messages API | `output_config.format` `json_schema` | `GET /v1/models` |
| Google Gemini | `google.golang.org/genai` v1.79.1 | `generateContent` (v1beta) | `responseMimeType: application/json` + `responseJsonSchema` | `GET /v1beta/models` |
| DeepSeek | `openai-go/v3` with base URL `https://api.deepseek.com` | Chat Completions | `response_format: json_object` only — the schema is given in the instructions, the answer is validated by Messora and one repair request is made | `GET /user/balance` (also reports the balance) |

- Several connections may exist, of any kind. API keys are sealed with
  `APP_ENCRYPTION_KEY` (associated data `ai_provider.key:<id>`), decrypted
  only for a call, never returned by the API (only `key_last4`), never
  written to logs or the audit log, and scrubbed from provider error
  messages before they are stored or shown.
- A custom `base_url` is accepted only for DeepSeek and OpenAI-compatible
  endpoints and must be `https`. All provider calls use Messora's SSRF-safe
  HTTP client: private and loopback addresses are refused.
- For Gemini a successful model list does not prove that generation is
  enabled for the key (billing may still be missing).
- Tool calls use `tool_choice: auto` everywhere: Claude Opus 5.5 and
  Sonnet 5.5 reject a forced tool choice.
- Internal notes of the team (`operator_note` messages) are given to the
  model as instructions — a developer message (OpenAI), a system message
  (DeepSeek) or part of the system prompt (Anthropic, Gemini) — and never as
  customer content.

### Data retention and training

Meta forbids using WhatsApp platform data to train any model (WhatsApp
Business Terms 4.7). Messora never sends conversations to anything but the
configured provider, and:

- **OpenAI:** every request is sent with `store: false`, so the response is
  not kept for later retrieval. OpenAI does not train on API data by
  default.
- **Anthropic:** the Messages API has no per-request retention flag;
  Anthropic does not train on API data under its commercial terms.
- **Gemini:** there is no per-request flag. Google does not use prompts of
  **paid-tier** (billing-enabled) projects to improve its models; free-tier
  keys may be used for that, so connect a key of a project with billing.
- **DeepSeek:** there is no per-request flag; review DeepSeek's terms
  before connecting it.

The company's data sharing notice (Settings → AI) tells customers that
their messages go to the AI provider and are not used for training.

### Errors

| Kind | Meaning | Retry |
|---|---|---|
| `rate_limited` | 429 rate limit; `RetryAfter` from `Retry-After` when sent (OpenAI, Anthropic); Gemini and DeepSeek send none — back off with jitter | yes |
| `quota_exceeded` | OpenAI `insufficient_quota`, Anthropic `enforced_spend_limit_reached` / `billing_error`, DeepSeek 402 | **no** |
| `auth` | invalid or unauthorised key | no |
| `overloaded` | Anthropic 529, 503 | yes, with backoff |
| `unavailable` | network failure, timeout, other 5xx | yes, with backoff |
| `bad_request` | the provider rejected the request | no |
| `refusal` | the model or its safety filter declined | no |
| `invalid_output` | the structured answer did not match the schema (DeepSeek after one repair) | no |

The SDKs' own retries are switched off; queue workers decide (see
`ai.IsRetryable`, `ai.RetryAfterOf`). The API answers `AI_RATE_LIMITED`
(429 with `Retry-After`), `AI_QUOTA_EXCEEDED`, `AI_AUTH_FAILED`,
`AI_PROVIDER_UNAVAILABLE`, `AI_BAD_REQUEST`, `AI_REFUSED`,
`AI_INVALID_OUTPUT` (502), `AI_DISABLED` and `AI_NOT_CONFIGURED` (409).

## Model catalogue

`ai_models` is refreshed daily (`ai:model_sync`, queue `ai`) and from the
panel. Each field is taken from the provider's list endpoint; what a
provider does not report stays `null` and is shown as "unknown":

- **OpenAI** reports only ids (and `shutdown_date`, shown as deprecated).
- **Anthropic** reports `max_input_tokens`, `max_tokens` and capabilities
  (image input, structured outputs, thinking). Tool use has no flag because
  every Claude model supports it.
- **Gemini** reports `inputTokenLimit`, `outputTokenLimit` and `thinking`;
  only models supporting `generateContent` are kept.
- **DeepSeek** reports `context_window`, `max_output_tokens` and
  `input_modalities` (vision when `image` is listed).

## Prices and cost

No provider publishes prices through an API, so Messora keeps
`ai_model_prices` (per 1M tokens): input, cached input (cache read), cache
write (5 minutes) and cache write (1 hour, Anthropic), output, a currency,
validity dates (`effective_from` inclusive, `effective_to` exclusive), the
source and who confirmed the row. A row's `model_pattern` is an exact
model id or a prefix ending in `*`; the exact id, then the longest prefix,
then the newest valid row wins.

Rules (`rules` JSON):

```json
{
  "thresholds": [{ "input_tokens_over": 200000, "input": "4", "output": "15" }],
  "windows": [{ "label": "peak", "weekdays": [1, 2, 3, 4, 5], "start": "01:00", "end": "04:00", "multiplier": "2" }]
}
```

- A **threshold** replaces rates when the total input (cached and cache
  writes included) exceeds the limit; the highest exceeded one applies
  (Claude Haiku 5.5 above 100K, OpenAI above 272K).
- A **window** applies on ISO weekdays between `start` and `end` (UTC unless
  `time_zone` is set; it may wrap past midnight): its rates replace the
  row's, then `multiplier` multiplies every rate (DeepSeek's 2x peak hours,
  01:00–04:00 and 06:00–10:00 UTC Monday to Friday).
- Announced price changes (e.g. Gemini on 2026-12-01 and 2027-01-01) are new
  rows with their `effective_from`.

Cost = input × input price + cached × cached price + cache writes × their
prices + output × output price (reasoning tokens are part of output). A
missing cached or cache-write price uses the input price and marks the
cost as estimated. Usage is normalised per provider: OpenAI `input_tokens`
includes cached reads and cache writes (subtracted); Anthropic
`input_tokens` excludes them; Gemini `promptTokenCount` includes
`cachedContentTokenCount` and thinking tokens are billed as output;
DeepSeek splits `prompt_cache_hit_tokens` / `prompt_cache_miss_tokens`.

The cost is computed once, at request time, and stored in `ai_usage`;
editing or deleting a price never changes recorded costs.

Where prices come from:

1. **Seed** — the list prices read from the providers' pricing pages on
   2026-11-25 (`internal/modules/ai/seed_prices.json`), stored once per
   installation, **unconfirmed**. Rates the pages did not state (cache
   writes, long-context prices) are not seeded; the notes say what to add.
2. **LiteLLM import** — an admin uploads LiteLLM's
   `model_prices_and_context_window.json` (MIT licence; per-token prices
   are converted to per 1M, including `*_above_<N>k_tokens` thresholds and
   cache prices). Messora never downloads it. Imported rows replace rows of
   the same model and date and are unconfirmed.
3. **Manual** — an admin adds or edits a row; adding counts as confirmed.
   Confirming records who checked the row against its source; changing a
   figure without confirming clears the confirmation.

## Company AI settings

`GET/PUT /api/v1/ai/settings`: the master switch (off by default), the
default provider and model, the AI disclosure text and the data sharing
notice (Turkish and English).

## AI profiles

A profile decides how the AI behaves on one WhatsApp number (a number has
at most one profile): the instructions (system prompt, up to 100,000
characters), an optional provider/model override, auto reply or
suggestions only, handoff rules, information collection, allowed actions,
the disclosure mode and active/paused.

- **Handoff on request is always on and cannot be changed**: Meta's
  Business Messaging Policy requires a quick, clear path to a person.
  Other triggers: confidence below a threshold, no answer, critical intents,
  an action the profile may not take.
- **Information collection** lists contact fields (`name`, `email`,
  `var1`…`var5`) or custom attributes (`attr:<key>`); nothing else is asked
  for. Creating contacts and updating fields are separate permissions.
- **Allowed actions**: `create_contact`, `update_contact_fields`,
  `add_tag`, `assign_conversation`, `add_internal_note` and automations as
  `automation:<id>`.
- **Disclosure**: in the first message (default), when asked, or never
  (the panel warns).

The instructions sent to the model are the company's prompt followed by
the rules Messora enforces (`ai.Instructions`), and the answer follows
`ai.ReplySchema`: `reply`, `confidence`, `intent`, `escalate`,
`escalate_reason` and `extracted_fields`.

The **test console** (`POST /api/v1/ai/profiles/{id}/test`) runs a sample
conversation through the saved profile exactly like a live turn and shows
the structured reply, tokens, cost and the full instructions. Nothing is
sent to a customer; the request is recorded with purpose `test`.

## Sohbet devri (conversation intelligence and AI handoff)

> Phase 16C P6. Code: `internal/modules/conversations` (`ownership.go`
> state machine, `escalation.go` handoff rules, `aicontext.go` context
> builder, `aireply.go` reply loop, `handoff.go` agent actions),
> `internal/modules/activity` (timeline). Migration `00018_conversation_ai.sql`.

### Who answers a conversation

Every conversation has an owner (`owner_type`) and an AI mode (`ai_mode`):

| Owner | Modes | Meaning |
|---|---|---|
| `none` / `human` | `off` | No AI. |
| `none` / `human` | `suggest` | The AI drafts a reply for each customer message; an agent accepts, edits or rejects it. |
| `ai` | `auto_reply` | The AI answers the customer. |
| `ai` | `awaiting_approval` | The AI writes the reply; an agent approves it before it is sent. |

The transitions are one pure function (`conversations.Next`), unit-tested
case by case:

- **First customer message:** a conversation nobody owns takes the mode of
  the number's AI profile — `auto_reply` (owner `ai`) when the profile is
  active and auto replies, `suggest` when it is active and only suggests,
  otherwise nothing (and it is decided again on the next message). AI must
  be on for the company and allowed for the contact.
- **Hand to the AI** (`PUT /conversations/{id}/ai-mode`, `POST …/handoff`
  with `to: ai`): needs AI on, an active profile on the number and AI
  allowed for the contact, else `409 AI_UNAVAILABLE`. An unanswered last
  customer message is answered right away.
- **Hand to a person** (`POST …/handoff` with `to: human`): to a user
  (another person needs `conversations.assign`) or to the pool
  (unassigned). The conversation waits for a person (`pending_handoff_id`).
- **Take over** (`POST …/takeover`, or any agent reply while the AI owns
  the conversation or it waits for a person): the agent becomes owner and
  assignee, a pending escalation is accepted. The AI then keeps the mode the
  profile sets in `handoff.on_human_takeover`: `off` (default) or
  `suggest`.
- **Escalation:** the AI hands over (below); the conversation waits for a
  person.
- **AI disabled for the contact** (`PUT /contacts/{id}/ai`) takes every
  conversation of the contact from the AI (to the pool, or its assignee)
  and stops suggestions. **AI switched off for the company** is noticed on
  the next customer message: a conversation the AI owns is escalated with
  reason `ai_error` (`disabled`) — never dropped silently.

Every change writes a handoff row when the owner changes
(`conversation_handoffs`), a timeline item, and an audit entry
(`conversation.ai_mode`, `conversation.handoff`, `conversation.takeover`,
`conversation.escalate`, `contact.ai`). They need `conversations.reply`
(contacts: `contacts.update`) and follow the visibility rules: a
conversation the caller cannot see answers 404.

### The AI reply loop

1. A customer message (not a reaction or a consent keyword) in a
   conversation the AI owns or drafts in schedules `ai:reply` (queue `ai`)
   **3 seconds** later. When the task runs and a newer customer message
   exists, it does nothing: the newer message's task answers the whole
   burst once.
2. Guards, each handing the conversation to a person when the AI owns it
   (a suggestion turn only shows `ai_status: error`): AI off / contact
   disabled / no active profile (`ai_error`), more AI requests in the last
   hour than `max_replies_per_hour` (company AI settings, default 20;
   `ai_error`, `rate_limited`), the monthly AI budget used up
   (`ai_budget`, see "Maliyet ve kullanım"), the 24-hour window closed in
   auto reply (`window_closed`; free text cannot be sent).
3. `ai.status: thinking` is announced; the request is built
   (`BuildTurn`): the profile's instructions and the platform rules, what
   the business knows about the customer (name, e-mail, var1–5, custom
   attributes), the last `context_messages` messages (default 20; customer
   messages as user content, the business's as assistant content) and the
   **notes for the AI** as operator notes in time order. The answer is the
   structured reply (`reply`, `confidence`, `intent`, `escalate`,
   `escalate_reason`, `extracted_fields`, `actions`).
4. Provider errors (any `ai.Error` kind) and unreadable answers hand over
   with `ai_error` and the error kind.
5. The handoff rules (`conversations.Decide`), in this order: the customer
   asked for a person — the model's intent `human_request` **or** a
   deterministic Turkish/English check (`WantsHuman`: *temsilci…*, *insan*,
   *gerçek biri*, *canlı destek*, *yetkili*, *human*, *agent*, *real
   person* …, with Turkish case folding from the consent keywords) — then a
   critical intent, the model's `escalate`, an action the profile may not
   take (when `on_unauthorized_action`), confidence below the threshold, an
   empty reply.
6. Otherwise: **auto reply** sends the text through the normal send path
   (consent and window rules; a refusal hands over), as
   `messages.source = ai`, `sent_by = ai` with the `ai_usage_id`;
   **awaiting approval** stores a draft of kind `approval`; **suggest**
   stores a draft of kind `suggestion` (a newer draft or an agent's reply
   supersedes it). Drafts are accepted (`…/ai-drafts/{id}/accept`, with an
   optional edited `body`) or rejected.
7. What the reply asked for and the profile allows runs in the same
   transaction: collected fields are written to the contact (data
   collection with *may update fields*; types, provenance and the rules
   in "Müşteri bilgisi toplama"), automations allowed as
   `automation:<id>` start for the conversation (`started_by: ai`).
   Refused actions are recorded.

The turn claims the customer message (`ai_handled_message_id`) in the
transaction that sends, drafts or escalates, so a retried or redelivered
task never sends twice.

**Disclosure.** With the profile's disclosure mode *first message*,
Messora puts the company's disclosure text (Settings → AI) in front of the
first AI message of each conversation (`ai_disclosed_at`); the model is
told not to repeat it and to answer honestly when asked. *When asked*
leaves it to the model; *off* is warned about in the panel.

### Escalation

An escalation creates a handoff from `ai` to the pool (or the assignee)
with the reason (`customer_request`, `no_answer`, `low_confidence`,
`critical_intent`, `unauthorized_action`, `window_closed`, `ai_error`,
`manual`), a **summary**, the customer's need, a suggested next action
and the fields the AI collected. The summary is one extra request
(purpose `summarize`, structured: `summary`, `detected_need`,
`suggested_next_action`); when the AI failed or cannot write it, a
deterministic summary of the customer's last messages is used
(`summary_source: fallback`). People who may take it are told
(`notification.created`, kind `conversation.escalated`): the assignee, or
everyone who can read conversations. The inbox shows the banner and lists
the conversation under **İnsan bekleyen** (`?ai=waiting_human`) until
someone takes it over.

### Internal notes

`conversation_notes` are internal: never sent to the customer and never
given to the model as customer content. Notes marked **AI'a not**
(`for_ai`) are operator instructions in the AI's next turns (the latest 20);
the AI is told never to quote them. Tests assert that note text appears
neither in user content sent to the model nor in any payload sent to Meta.

### Activity timeline

`conversation_activity` is one typed stream written next to every change:
`handed_to_ai`, `handed_to_human`, `ai_mode_changed`, `assignee_changed`,
`closed`, `reopened`, `note_added`, `ai_action`, `ai_reply_sent`,
`ai_draft_created`, `ai_draft_decided`, `ai_error`, `escalation`,
`handoff_accepted`, `automation_started` / `automation_completed` /
`automation_failed` (name, started by, steps, error), `message_sent`,
`message_failed` (campaign messages are left out), `contact_ai_disabled` /
`contact_ai_enabled`. `GET /conversations/{id}/activity` pages it (latest
items, oldest first; the cursor goes back), each new item is announced as
`conversation.activity` on `conversation:<id>`. Automation runs tied to a
conversation (`automation_runs.conversation_id`, set from the triggering
event) are listed by `GET /conversations/{id}/automation-runs` and
announced as `automation.run.updated` on every start, step and end.

## Müşteri bilgisi toplama (customer information collection)

> Phase 16C P7. Code: `internal/modules/conversations/leadcapture.go`
> (storing and profile progress), `internal/modules/ai/fields.go` (types,
> provenance rule, sensitive categories), `internal/modules/contacts/provenance.go`
> (edits by people, verification). Migration `00020_ai_customer_intelligence.sql`.

### Unknown numbers

A message from a number nobody knew creates the contact (as before) and
marks it: `created_by_inbound_at` and `profile_status = new`. Contacts
created any other way (panel, import, API, an outbound send) have no
status and are never treated as unknown numbers.

When the number's AI profile has **data collection** on, lists fields and
**may create contacts** (`may_create_contact`), each AI turn with such a
contact carries extra rules (`ai.LeadCaptureInstructions`): greet, find
out what the customer needs and help, and ask for the still missing
required fields **one or two at a time**, after answering, never as a
form. The status becomes `collecting`; when every required field is
filled it becomes `complete`, the timeline gets `profile_completed`, and
with **hand over when complete** (`handoff_when_complete`) the
conversation goes to the team right after the AI's message (escalation
reason `profile_complete`, with the collected fields), e.g. to sales.
`contacts.profile_progress` keeps `{required, filled, missing}`. Consent
and opt-out rules (docs/CONSENT.md) apply unchanged: the AI only answers
inside the service window and a consent keyword is never answered by it.

### What is stored

The answer's `extracted_fields` (one key per configured field) are stored
when the profile **may update fields** (`may_update_fields`; P6 also
required the `update_contact_fields` action — no longer):

- **Data minimisation:** only fields the profile lists are read; anything
  else the model returns is ignored. Profiles cannot list special
  categories of personal data or secrets (health, beliefs, ethnic origin,
  political opinions, sex life, union membership, criminal records,
  biometric/genetic data, identity and card numbers, passwords — Turkish
  and English words, `ai.SensitiveField`), and the instructions tell the
  model never to ask for or record them.
- **Types:** a field has a `type` — `text` (default), `email` (always for
  the e-mail field), `phone` (normalised to E.164), `select` (one of its
  `options`, stored as configured), `number`, `date` (YYYY-MM-DD). A value
  that does not fit is dropped and the timeline says so.
- **Provenance:** `contact_field_sources` keeps per field `source`
  (`ai | user | import | api`), `verified`, `collected_at`, the
  `ai_usage_id` and the conversation. AI values are stored as **"AI
  tarafından toplandı, doğrulanmadı"** (`source ai`, `verified false`).
- **Never over people's data:** the AI fills empty fields and replaces only
  its own unverified values (`ai.MayAIWrite`). A value a person, an import
  or the API set — a field without a provenance row counts as such — or a
  value a person verified is kept (timeline: `kept_existing`).
- A person editing a field (panel or API) makes it theirs (`user`/`api`,
  verified); clearing it removes the record. **Doğrula**
  (`POST /contacts/{id}/verify-fields`, `contacts.update`, audited
  `contact.verify_fields`) marks AI values as checked.

Automation AI steps (`ai_extract_fields`, `ai_build_profile`, the fields an
`ai_auto_reply` collects) store values through the same rules
(`conversations.Service.SetFieldsWithSource`, docs/AUTOMATIONS.md "AI
steps"): they need an AI profile on the number with **may update fields**,
never overwrite a person's, an import's, the API's or a verified value, and
are recorded as source `ai`, not verified, with the automation's AI request.

The contact (`GET /contacts/{id}`) returns `profile_status`,
`profile_progress` and `field_sources`; the panel shows a card with the
completeness and a badge per AI value with **Doğrula**. The contacts list
filters **AI tarafından oluşturulan** (`ai_profile=true`: status
`collecting` or `complete`; also in bulk actions and export).

## Maliyet ve kullanım (cost and usage)

> Phase 16C P7. Code: `internal/modules/ai/attribution.go`, `budget.go`,
> `usage.go`, `classify.go`.

### Campaign attribution

Every `ai_usage` row of a conversation gets a `campaign_id` when it is
recorded (`ai.Attribute`): the campaign whose message reached the
contact **last** on the conversation's number at or before the request
(a message Meta accepted: `sent`, `delivered` or `read`), provided it is
at most **N days** old. N is the campaign attribution window of
**Settings → Campaign protection** (`attribution_days`, default 7, 1–30;
docs/CAMPAIGN_ANALYTICS.md "Attribution"), the one window for replies,
opt-outs and AI usage (`ai.Service.SetAttributionWindow` reads it; there
is no AI setting of its own). A later campaign reaching the same contact
takes over from its send on. Each request belongs to **at most one** campaign (one
column, never rewritten); rows without one are **kampanya dışı**.
`ai:attribution_reconcile` (hourly, queue `ai`) retries rows of the last
window + 1 day that have no campaign (a campaign message recorded late)
and never changes an attributed row. A call may also name its campaign
(classification does).

The campaign's AI figures are in its **analysis center**
(`GET /campaigns/{id}/analytics`, `ai_cost`: requests, failures,
input/output/cached tokens, conversations and cost per currency, and
`total_cost` = Meta estimate + AI when they share a currency; see
docs/CAMPAIGN_ANALYTICS.md). They sum the same `ai_usage` rows as the
`campaign` group of the usage summary below, so the two always agree and
a request is never counted twice.

### Usage metrics

`GET /ai/usage/summary?from&to&group_by=model|provider|purpose|campaign|day&tz`
(session, `ai.read`) returns, for `[from, to)` (at most 400 days):
requests, generated replies (successful `reply` and `suggest` requests),
input/output/cached/cache-write tokens, USD cost (rows priced in another
currency are left out), failed requests, average latency of successful
requests, the conversations the AI answered, escalations (handoffs from
the AI) and the **handoff rate** (escalations ÷ AI conversations), in
total and per group, plus the month's spend against the budget. Settings
→ AI → **Kullanım** shows KPI cards, the daily cost chart and tables by
model, provider, campaign and purpose for a preset period (7/30/90 days,
this month).

### Monthly budget

Settings → AI → **Aylık AI bütçesi** (`monthly_budget`, USD, optional).
Spend is the USD cost recorded since the first day of the month (UTC).
Once it reaches the budget, AI turns stop before calling the provider:
a conversation the AI owns goes to a person (escalation `ai_budget`), a
suggestion turn shows the error, and reply classification is skipped.
The people with `ai.manage` get a notification (`ai.budget_exceeded`)
once per month and the audit log records it. The Usage page shows the
state; raising or removing the budget resumes AI on the next message.

### Campaign reply classification

With **Kampanya cevaplarını sınıflandır** (`classify_campaign_replies`,
off by default) a contact's first **free-text** reply to a campaign is
classified. The campaigns module attributes the reply (the recipient row
of the campaign that reached the contact last on that number, within the
attribution window) and, when it is not a tapped quick reply, hands it to
its `ReplyClassifier` (`conversations.CampaignReplyClassifier`; skipped
for contacts with AI turned off and for consent keywords). That records a
`pending` row in `campaign_reply_classifications` (one per campaign and
contact, linked to the recipient) in the webhook transaction, and
`ai:classify` (queue `ai`) classifies it: purpose `classify`, structured output (`sentiment`
positive | negative | neutral, `confidence`, `reason`), the number's
profile provider/model, usage attributed to that campaign. Results:
`done` with the sentiment, `failed` (provider error kind), `skipped` (AI
off, budget used up, more than 10 classifications in an hour for the
conversation, message deleted). Provider rate limits are retried by the
queue. The reply text is treated as customer content.

A `done` result is written to the recipient row as its **reaction** with
source `ai` (`campaign_recipients.reaction`), so the campaign counters,
the positive/negative/neutral rates, the recipient table filters and
reaction automations see it like any other reaction. Precedence: a label
set by a person (`manual`) is never replaced, and an AI label never
replaces a tapped quick reply (a quick reply tapped later replaces the AI
label). `campaign_reply_classifications` stays as the job log
(confidence, reason, error, usage row); if writing the reaction fails the
job is retried and only the write is repeated.

## Development and tests

`AI_FAKE_PROVIDER=true` (refused with `APP_ENV=production`) enables the
provider kind `fake`: it answers deterministically ("Fake reply to: …",
structured output built from the schema), fails on demand when the last
customer message is `fake:error:<kind>`, and can be scripted in Go tests
with `Service.SetFakeScript`. `internal/modules/ai/aitest` emulates the
four providers' HTTP APIs with their documented JSON shapes for tests.
`scripts/e2e.sh` runs its stack with `APP_ENV=development` and the fake
provider.
