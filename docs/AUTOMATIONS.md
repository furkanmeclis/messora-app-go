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
- **Exactly-once database steps.** Messages, tags, fields, assignment and
  closing commit in the same transaction as the run's progress.
  `http_request` and `send_email` are delivered at least once.
- **Editing is safe.** Changing an automation's trigger or steps cancels its
  runs in progress (they would follow the old program).
