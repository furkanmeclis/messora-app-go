# Automations

An automation reacts to one **trigger** with a list of **steps**. Automations
run in the worker; every run and every executed step is logged
(`GET /api/v1/automations/{id}/runs`, `GET /api/v1/automation-runs/{runId}`).

## Triggers

| Type | Fires when | Options |
|---|---|---|
| `message_received` | a customer message arrives | `match`: `any` (default), `contains`, `exact`, `starts_with` with `keywords` (case-insensitive); `phone_number_id` |
| `new_contact` | the first message ever arrives from a contact | `phone_number_id` |
| `tag_added` | a person or integration adds the tag to a contact | `tag` |
| `message_status` | an outbound message becomes `delivered`, `read` or `failed` | `status`, optional `campaign_id` |
| `conversation_closed` | a person or integration closes a conversation | — |

Every trigger accepts `cooldown_seconds`: at most one run per contact within
that time. Contacts muted by a `mute` step start no automation until the mute
ends. CSV imports do not fire `tag_added`.

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

Text fields accept placeholders: `{{name}}`, `{{first_name}}`, `{{phone}}`,
`{{email}}`, `{{var1}}`…`{{var5}}`, `{{attr:<key>}}` and `{{message}}` (the
triggering message).

Condition fields: `message.body`, `message.type`, `contact.name`,
`contact.email`, `contact.phone`, `contact.var1`…`var5`, `contact.tag`,
`contact.opted_out`, `conversation.window_open`, `event.tag`, `event.status`.
Operators: `contains`, `not_contains`, `equals`, `not_equals`, `starts_with`,
`is_set`, `is_not_set`, `is_true`, `is_false`.

A step that cannot apply (window closed, contact opted out, missing template
variable, no conversation) is logged as `skipped` and the run continues. A
step that errors (HTTP 4xx/5xx, e-mail failure) stops the run as `failed`.

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
- **Exactly-once database steps.** Messages, tags, fields, assignment and
  closing commit in the same transaction as the run's progress.
  `http_request` and `send_email` are delivered at least once.
- **Editing is safe.** Changing an automation's trigger or steps cancels its
  runs in progress (they would follow the old program).
