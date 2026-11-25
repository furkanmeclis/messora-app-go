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

## Development and tests

`AI_FAKE_PROVIDER=true` (refused with `APP_ENV=production`) enables the
provider kind `fake`: it answers deterministically ("Fake reply to: …",
structured output built from the schema), fails on demand when the last
customer message is `fake:error:<kind>`, and can be scripted in Go tests
with `Service.SetFakeScript`. `internal/modules/ai/aitest` emulates the
four providers' HTTP APIs with their documented JSON shapes for tests.
`scripts/e2e.sh` runs its stack with `APP_ENV=development` and the fake
provider.
