# Automations

An automation reacts to one **trigger** with a list of **steps**, edited on a
canvas (see below). Automations run in the worker; every run and every
executed step is logged (`GET /api/v1/automations/{id}/runs`,
`GET /api/v1/automation-runs/{runId}`).

## Triggers

| Type | Fires when | Options |
|---|---|---|
| `message_received` | a customer message arrives | `match`: `any` (default), `contains`, `exact`, `starts_with` with `keywords` (case-insensitive); `phone_number_id` |
| `new_contact` | the first message ever arrives from a contact | `phone_number_id` |
| `tag_added` | a person or integration adds the tag to a contact | `tag` |
| `message_status` | an outbound message becomes `delivered`, `read` or `failed` | `status`, optional `campaign_id` |
| `conversation_closed` | a person or integration closes a conversation | — |
| `subscription_request` | an inbound message matches a consent keyword (stop word, opt-out request phrase, resubscribe word; [CONSENT.md](CONSENT.md)); the consent change is already applied | optional `subscription`: `stop`, `request` or `resume` (default: any) |
| `window_closing` | the 24-hour customer service window of a conversation closes within `minutes`, the window is still open and the customer's message is unanswered (the thread's last message is theirs) | `minutes` (5 – 1380, default 60); `include_answered` (also when the team answered last); `phone_number_id` |
| `no_reply` | nobody answered for `minutes`: `no_reply_from: customer` — our message is the thread's last one; `agent` — the customer's is | `no_reply_from` and `minutes` (5 min – 30 days) required; `include_campaigns` (customer: campaign messages count as ours); `phone_number_id` |
| `campaign_reply` | a customer's first reply to a campaign, as the campaign analytics attribute it ([CAMPAIGN_ANALYTICS.md](CAMPAIGN_ANALYTICS.md)): the campaign that reached them last, on its number, within `attribution_days`; once per recipient (reactions and system notices are not replies) | optional `campaign_id` |
| `reaction` | a customer's response turns positive or negative: a WhatsApp emoji reaction (👍 ❤️ 🙏 … / 👎 😡 😢 …) to a message, or a campaign recipient's reaction label — a mapped quick-reply button, the AI's classification of the reply or a person's label; neutral, cleared and removed reactions start nothing | optional `reaction` (`positive`/`negative`), `campaign_id` (of the message reacted to / the recipient's campaign) |
| `ai_escalation` | the AI hands a conversation to a person ([AI.md](AI.md), "Sohbet devri") | optional `reasons`: `customer_request`, `no_answer`, `low_confidence`, `critical_intent`, `unauthorized_action`, `window_closed`, `ai_error` |

Every trigger accepts `cooldown_seconds`: at most one run per contact within
that time. Contacts muted by a `mute` step start no automation until the mute
ends. CSV imports do not fire `tag_added`.

### Time-based triggers

`window_closing` and `no_reply` are found by a scanner (task
`automation:scan`, every minute from the scheduler), not by an event.
Reactions, system notices and failed or cancelled sends do not count as
"the last message". The event key makes each occasion fire once:
`window:<conversation>:<customer message that opened the window>` and
`noreply:<from>:<conversation>:<unanswered message>`. A window therefore fires
once however often the scanner looks at it; when the customer writes again
the new window fires again. `no_reply` looks back at most 24 hours beyond the
waiting time, so enabling it does not wake up old silent conversations;
`no_reply` from the customer only considers conversations the customer has
written in.

**Meta's window rule.** Only a message from the customer reopens the 24-hour
window. `window_closing` is for giving the customer a genuinely useful
answer while it is still possible (or alerting the team) — never for sending
filler to "keep the window open". The trigger's help text in the editor
says so, and no preset sends a keep-alive.

### Reactions

`trigger.EmitReaction(ctx, tx, q, trigger.ReactionEvent{…})` is the contract
for whoever records reactions, in the transaction that stores them: key,
contact, conversation, number, the reaction message, the campaign, the
polarity (`positive`/`negative`) and the source. It is raised by

- the webhook for WhatsApp emoji reactions (source `emoji`, polarity from
  `trigger.Polarity`);
- the campaigns module whenever a recipient's reaction label is set or
  changed to positive or negative (`campaigns.SetReaction`,
  `RecordInbound`): source `quick_reply` (a tapped button the campaign's
  reaction map labels), `ai` (the AI classification of a free-text reply,
  [AI.md](AI.md)) or `manual` (a person's label). Remapping the buttons
  afterwards (`PUT /campaigns/{id}/reaction-map`) relabels past replies
  without starting automations.

A campaign recipient's reactions share one key per polarity
(`trigger.RecipientReactionKey`: `recipient:<id>:<polarity>`; an emoji
reaction to the campaign message uses it too), so the same recipient starts
the "Olumlu/Olumsuz müşteri dönüşü" presets at most once per polarity —
whether the customer taps the button, the AI labels the reply, a person
confirms it or Meta redelivers the webhook. Correcting a label to the other
polarity starts the other automation once.

### Campaign replies

`campaign_reply` uses the campaign analytics' attribution — one rule
everywhere: the campaigns module raises it in the inbound transaction when
it marks the recipient as replied (the first message on the campaign's
number after the campaign reached the contact, before another campaign did
and within the attribution window `attribution_days`, Campaign protection
settings). Its key is `campaign_reply:<recipient>`.

## Steps

| Step | Fields |
|---|---|
| `reply` | `text` — free-form message; only inside the 24-hour window |
| `send_template` | `template`: `name`, `language`, `body`/`header` variables (as in campaigns) |
| `add_tag`, `remove_tag` | `tag` |
| `add_to_group` | `group_id` |
| `set_field` | `field` (`name`, `email`, `var1`…`var5`), `value` |
| `condition` | `if`: `field`, `operator`, `value`; `then` and/or `else` step lists (up to 4 levels) |
| `http_request` | `url` (https), `method`, `headers`, `body` — SSRF-protected, 15 s timeout |
| `send_email` | `to`, `subject`, `body` — needs SMTP |
| `delay` | `seconds` (1 s – 30 days) |
| `assign` | `assignee_id` (omit to unassign) |
| `close_conversation` | — |
| `mute` | `seconds` (1 minute – 90 days) |
| `handoff_to_ai` | `mode`: `auto_reply` (default) or `awaiting_approval` — the number's AI profile takes the conversation (skipped when the AI is unavailable) |
| `handoff_to_human` | optional `assignee_id` (else the team's pool), `text` (note for the person, ≤ 500) — the conversation waits to be taken; the assignee or the pool is notified |
| `add_internal_note` | `text` (≤ 4000), `for_ai` (the AI reads it as an instruction) — never sent to the customer |
| `ai_generate_message` | `prompt` (with placeholders), `output`: `variable` (default) or `send` (also send it as a reply), `variable` (default `ai_message`) |
| `ai_analyze_message` | — writes `sentiment` (`positive`/`neutral`/`negative`) and `summary` |
| `ai_detect_intent` | `intents` (1–20, `[a-z0-9_]`; `other` is implied), `variable` (default `intent`) — branch with a following condition on `var.intent` |
| `ai_extract_fields` | `fields`: `[{key, label}]` with `name`, `email`, `var1`…`var5` or `attr:<key>` — writes what the customer stated, provenance `ai_unverified` |
| `ai_classify_segment` | `tags` **or** `group_ids` to choose one from (or none), `variable` (default `segment`) — adds the tag / group |
| `ai_auto_reply` | — one reply of the number's AI profile (its prompt, disclosure and handoff rules); skipped when the AI already owns the conversation |
| `ai_route_to_agent` | optional `assignee_id`, `text` — hands to a person with the AI's handoff summary |
| `ai_build_profile` | optional `attribute` (default `ai_profile`) — writes a short customer profile and `<attribute>_interests` |

HTTP, e-mail, message and AI steps accept `retry`: `{max, backoff}` (see
below).

Text fields accept placeholders: `{{name}}`, `{{first_name}}`, `{{phone}}`,
`{{email}}`, `{{var1}}`…`{{var5}}`, `{{attr:<key>}}`, `{{message}}` (the
triggering message) and `{{var:<name>}}` (a run variable an AI step wrote).

Condition fields: `message.body`, `message.type`, `contact.name`,
`contact.email`, `contact.phone`, `contact.var1`…`var5`, `contact.tag`,
`contact.opted_out`, `conversation.window_open`, `event.tag`, `event.status`,
`event.subscription`, `event.reaction`, `event.reason` and `var.<name>`.
Operators: `contains`, `not_contains`, `equals`, `not_equals`, `starts_with`,
`is_set`, `is_not_set`, `is_true`, `is_false`.

A step that cannot apply (window closed, contact opted out, missing template
variable, no conversation, AI disabled for the contact) is logged as
`skipped` and the run continues. A step that errors (HTTP 4xx/5xx, e-mail
failure, AI configuration) stops the run as `failed`; the run's `error_code`
says why (`ai_disabled`, `ai_not_configured`, `ai_not_allowed`,
`ai_no_profile`, `ai_error`, `retries_exhausted`, `step_failed`).

## AI steps

Before an AI step runs, the engine checks that AI is on for the company
(`ai.Enabled`); when it is off the run fails at once with `error_code`
`ai_disabled` and a message pointing to Settings → AI (a configuration error
is never retried). Steps that store contact fields (`ai_extract_fields`,
`ai_build_profile`) need an active AI profile on the conversation's number
(`ai_no_profile`) whose data collection **may update fields**
(`may_update_fields`, otherwise `ai_not_allowed`) — the same permission the
AI's own turns need; the `update_contact_fields` action is not consulted.
For other steps, when the number has an AI profile its `allowed_actions`
must include what the step does: `add_tag` for `ai_classify_segment`
(otherwise `ai_not_allowed`); its prompt is the
business context and its provider/model override applies. `ai_auto_reply`
needs an active profile (`ai_no_profile`). Handing over to a person is
always allowed. Every model call is recorded in `ai_usage` with purpose
`automation` and the run's id (`automation_run_id`); the step log carries
`ai_usage_id` and the model. The canvas marks AI nodes with a warning badge
while AI is off.

AI steps read the last 20 messages of the thread. Their results are run
variables (`automation_runs.vars`, shown on the run). Fields AI steps write
(`ai_extract_fields`, `ai_build_profile`, the fields an `ai_auto_reply`
collects) go through `automation.FieldWriter`, which the conversations
module implements on the provenance store with the same rules as the AI's
own data collection ([AI.md](AI.md) "What is stored"): a value a person, an
import or the API set, or one a person verified, is never overwritten
(`ai.MayAIWrite`; the step logs it under `kept` and the timeline as
`kept_existing`); stored values are marked source `ai`, not verified, in
`contact_field_sources` with the AI request (`ai_usage_id`) and the
conversation; values that do not fit the field (an e-mail that is not one)
are left out; fields naming special categories of personal data or secrets
are rejected when the automation is saved. The run log and the timeline
show them as `ai_unverified`.

## Retries and idempotency

`retry: {max, backoff}` (max 0–10, backoff 1–3600 seconds) repeats a failed
HTTP, e-mail, message or AI step: the attempt is logged as `retrying` (with
`attempt`, `max_retries`, `retry_in_seconds`), the run waits — durably, like
a delay — `backoff` seconds doubled for each earlier retry (at most an
hour) and runs the same step again. Only transient failures are retried:
network errors, HTTP 5xx/408/429, provider overload and rate limits;
configuration errors and HTTP 4xx fail at once. When the last retry fails
the run is `failed` with `error_code` `retries_exhausted`, and the worker
archives the task (`asynq.SkipRetry`), so it appears on the failed jobs page
(dead-letter queue).

Sends are idempotent: a message step queues at most one message per run and
step (`automation_sends`, key run id + instruction index), so a retried or
recovered step never sends twice — the log shows `deduplicated`. Model calls
and HTTP/e-mail steps happen at least once; their effects (messages, tags,
fields, handoffs) commit exactly once, together with the run's progress.

## Preset library

`GET /api/v1/automations/presets` lists the built-in presets
(`internal/modules/automation/presets/*.json`, embedded and versioned);
`POST /api/v1/automations/presets/{key}/install?lang=tr|en` creates a normal,
freely editable automation from one — **disabled**, named in the chosen
language, remembering `preset: {key, version}`. Both are panel-only
(session). Values a preset cannot know are its `setup` items
(`[{step_id, field, label}]`, e.g. which template): the automation saves
with them empty (those steps compile leniently), the editor marks the nodes,
and it cannot be enabled until they are filled (422 on `active`); the list
empties as the steps are filled. Editing a preset in the library never
changes installed copies.

| Key | Title | Trigger → steps | Setup |
|---|---|---|---|
| `welcome` | Yeni müşteri karşılama | new_contact → reply, tag `yeni-musteri` | — |
| `unsubscribe_request` | Abonelikten çıkma talebi | subscription_request → request: ask to confirm with DUR / stop: thank → tag `abonelik-talebi` → internal note | — |
| `collect_customer_info` | Müşteri bilgisi toplama | message_received (daily cooldown) → ai_extract_fields (name, email, city) → tag | — (needs AI) |
| `handoff_to_agent` | Temsilciye devir | message_received with "temsilci", "yetkili", "canlı destek" … → handoff_to_human → reply → note | — |
| `ai_auto_reply` | AI ile otomatik cevap | message_received → ai_auto_reply | — (needs AI) |
| `window_reminder` | 24 saatlik pencere hatırlatması | window_closing 60 min → internal note → handoff_to_human → tag (no message to the customer) | — |
| `follow_up_unanswered` | Cevapsız müşteriyi takip etme | no_reply (customer, 24 h) → send_template → tag | the template |
| `campaign_follow_up` | Kampanya sonrası takip | campaign_reply → tag → note → handoff_to_human | — |
| `positive_feedback` | Olumlu müşteri dönüşü | reaction positive → tag `olumlu-donus` → assign | the sales agent |
| `negative_feedback` | Olumsuz müşteri dönüşü | reaction negative → tag `olumsuz-donus` → note → handoff_to_human | — |
| `detect_silent_customers` | Belirli süre cevap vermeyen müşterileri tespit etme | no_reply (customer, 3 days) → tag `sessiz` | — |

The panel shows them under Automations → "Hazır otomasyonlar"; installing
opens the new automation on the canvas.

## The canvas editor

The panel edits an automation as a graph of nodes on a canvas (React Flow,
`frontend/src/app/(panel)/automations/`): a trigger node, one node per step
(coloured by category: message, contact & inbox, flow, integration) and
edges drawn by dragging from a node's bottom handle to another node's top
handle. A palette adds nodes (drag onto the canvas, or click to add after
the selected node — or at the end of the flow when nothing is selected);
the inspector on the right edits the selected node, the trigger, or the
automation's name and status. Zoom controls, a minimap, a "tidy up"
automatic layout, undo/redo (⌘/Ctrl+Z, ⇧⌘/Ctrl+Shift+Z or Ctrl+Y) and
Backspace/Delete are available; deleting a step between two nodes connects
them. Problems show on the nodes (red outline and message) while editing:
unconnected nodes, missing values, too deep nesting; the API stays the
authority and its field errors land on the node they concern.

The steps remain the source of truth for the engine, so the canvas graph is
always a tree that converts to and from them losslessly:

- every node except the trigger has exactly one incoming edge;
- a plain step has one output (the next step);
- a condition has three outputs: **yes** (`then`), **no** (`else`) and
  **then** — the steps that run after either branch (the steps that follow
  the condition in its list);
- each output holds at most one edge, and an edge never closes a cycle.

`[A, if C then [B] else [D], E]` is therefore trigger→A, A→C, C-yes→B,
C-no→D, C-then→E. The editor refuses connections that break these rules.

## Step ids and layout

Every step has an `id` (1–64 letters, digits, `-` or `_`, unique within the
automation; `trigger` is reserved). It is optional in the API: steps sent
without one are named by the server (`n` + 12 base-36 characters, the shape
the canvas generates). Automations saved before ids existed read with ids
derived from each step's position — `s0`, `s1`, `s1t0` (first then-step of
the second step), `s1e0` (else) — so they open on the canvas with an
automatic layout and keep those ids when saved.

`layout` maps node ids (`trigger` for the trigger node) to `{x, y}` canvas
positions (at most 200, |x|, |y| ≤ 10⁶). Positions of ids that are not
steps of the automation are dropped on save. Changing only the layout does
not create a new version and does not cancel runs in progress.

## Run trace

Each compiled instruction keeps the id of the step it comes from
(`step_id`; the condition for a branch, none for jumps). Whenever a run
executes an instruction, the engine appends that id to the run's `visited`
list in the same transaction as the step log entry, so `visited[i]`
belongs to `steps[i]` of the run detail. Runs of programs compiled before
ids existed are traced with the positional ids above.

`GET /api/v1/automations/{id}/runs` and `GET /api/v1/automation-runs/{runId}`
return `visited`. The run history links each traced run to
`/automations/{id}/runs/{runId}`: a read-only canvas that numbers the nodes
in execution order, animates the path taken, dims the nodes the run did not
reach and marks the node where it failed or waits. Runs started before the
trace existed have an empty `visited` list.

## Guarantees

- **Once per event.** A run is unique per (automation, triggering event):
  a webhook Meta delivers twice starts one run.
- **No chains.** Anything an automation does — messages it sends and their
  delivery statuses, tags it adds, conversations it closes — never triggers
  automations, so two automations cannot trigger each other (or themselves).
- **Durable delays.** A run's position is a single instruction index stored
  in the database; steps are compiled into a flat program with forward jumps
  only, so every run terminates. A delay schedules its resume through the
  transactional outbox, and a reaper restarts runs whose task was lost, so
  delays survive worker restarts and deploys.
- **Exactly-once database steps.** Messages, tags, fields, assignment,
  handoffs, notes and closing commit in the same transaction as the run's
  progress; a message step sends at most once per run (`automation_sends`).
  `http_request`, `send_email` and AI calls happen at least once.
- **Editing is safe.** Changing an automation's trigger or steps cancels its
  runs in progress (they would follow the old program).
