# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.10.0] - 2026-11-23

First licensed release. Images are also served from the licence registry
(log in with the licence key; see docs/LICENSING.md).

### Added — Phase 16: licensing and distribution

- Ed25519-signed licence keys with optional expiry, update window, user and
  number limits, and online or offline verification. Without a key Messora
  runs a 14-day trial. Expired or revoked licences get a grace period, then
  **restricted mode** pauses sending, campaigns and automations; data stays
  readable and a valid key lifts it at once.
- Settings → License (owner enters the key; status, limits, installation ID,
  newest release and whether it is included) and a banner when the licence
  needs attention.
- Check-in with the licence gate every 6 hours; signed answers carry
  revocations and the newest release. `LICENSE_GATE_URL` overrides the gate.
- `messora-license`, the issuer's CLI: Keychain-held signing keys, a ledger,
  revocation, encrypted iCloud Drive backup and restore, publishing to the
  registry. The Cloudflare Worker gate lives in `deployments/license-gate`.
- Releases are copied into the private R2 registry by the release workflow.
- Deploying with Dokploy (docs/OPERATIONS.md).

### Fixed

- Every Messora process allocated 64 MiB at start-up for the login
  timing-equalizer hash; it is now computed on first use. The scheduler
  idles at about 13 MiB instead of 78 MiB, and no longer gets OOM-killed
  under its 128 MiB limit.

## [0.9.0] - 2026-11-23

First published images (ghcr.io/furkanmeclis/messora, ghcr.io/furkanmeclis/messora-web). Feature-complete for v1; not yet validated against the live WhatsApp Cloud API.

### Added — Phase 9 and 10: panel screens and production readiness

- Panel screens for contacts (list, detail, import wizard, groups, tags,
  segment builder), channels, templates, campaigns (wizard and live
  dashboard), automations (visual editor and run log) and outgoing
  webhooks; a Playwright smoke test opens every route.
- File attachments in the inbox and `POST /api/v1/media` (streamed to Meta,
  type and size checked).
- Backups, restore and a CI-tested restore rehearsal; pinned image digests,
  resource limits and an operations guide.

### Fixed

- Dates and numbers render the same on the server and in the browser, in
  the user's time zone and language (hydration failed outside UTC).
- Idempotency keys are released when a request is rejected.

### Added — Phase 8: automations

- Automations react to new messages (with keyword matching), new contacts,
  added tags, delivery statuses and closed conversations with replies,
  templates, tags, groups, field updates, conditions, HTTP requests, e-mail,
  delays, assignment, closing and muting. See docs/AUTOMATIONS.md.
- A run is unique per triggering event, effects of automations never trigger
  automations, delays are durable across restarts, and every executed step
  is logged.

### Added — Phase 8: outgoing webhooks

- Webhook endpoints (`/api/v1/webhook-endpoints`, panel page under
  settings, permission `webhooks.manage`): HTTPS URL, description, event
  subscriptions, active flag, a server-generated signing secret shown once
  and rotatable, test events.
- Events: `message.received` (with the message text), `message.status`,
  `conversation.assigned` / `.closed` / `.reopened`, `campaign.completed` /
  `.failed` / `.cancelled`, `contact.created` / `.updated` / `.opted_out` /
  `.opted_in`, recorded in the same transaction as the change through the
  outbox. Versioned envelope `{"id", "type", "version", "created_at",
  "data"}`; see `docs/WEBHOOKS.md`.
- Delivery in the worker: one delivery per event and endpoint (unique, so
  repeated relays never duplicate), `Messora-Signature: t=…,v1=…`
  (HMAC-SHA256 over `timestamp.body`) and a stable `Messora-Event-Id` for
  receiver-side dedupe. 2xx succeeds, 410 disables the endpoint, timeouts,
  408, 429 (with `Retry-After`) and 5xx retry 10 times over about 24 hours,
  other answers fail at once. Delivery log with status, attempts, last
  response code, duration and response excerpt; manual redelivery; 30-day
  retention.
- `internal/platform/safehttp`: an SSRF-safe HTTP client for user-supplied
  URLs (also for the automation HTTP action). It resolves names itself,
  refuses loopback, private, link-local/metadata, CGNAT, multicast and
  reserved addresses (also IPv4-mapped, NAT64 and numeric shortcuts),
  re-checks the socket address at connect time against DNS rebinding,
  ignores proxy variables, does not follow redirects by default, bounds
  response bodies and keeps credentials out of errors.

### Added — Phase 5: campaigns

- Campaigns send an approved template to a segment, group, tag or every
  contact, now or at a scheduled time, with per-contact variables
  (`{{name}}`, `{{var1}}`, `{{attr:…}}` …) and fallbacks; recipients
  without a value, opted out or deleted are skipped with a reason.
- Resumable materialisation and a windowed dispatcher keep the queue small
  (at most the window in flight per campaign); pause holds unsent messages,
  resume re-queues them, cancel drops them.
- Per-number GCRA pacing with AIMD: Meta's 130429 halves the rate, success
  grows it back; deferrals do not count as failed attempts.
- A reaper repairs work left by a crashed worker: messages stuck in
  `sending` become `unknown` (never re-sent), lost tasks are re-created and
  stalled dispatchers restarted. Live counts, an event log and realtime
  progress on `campaign:<id>`.

### Added — Phase 4: contacts

- Contacts API with E.164 normalisation, personalisation fields (`var1`–`var5`
  and free attributes), tags, groups, internal notes and an append-only
  opt-in/opt-out history; templates are refused for opted-out contacts.
- Dynamic segments: a JSON rule tree compiled to parameterised SQL from fixed
  field and operator tables, with live previews and counts.
- CSV and Excel (.xlsx) import: streamed upload (50 MB), delimiter and BOM detection, column
  mapping, skip or update for duplicates, per-row error report, progress,
  cancellation and resume after restarts. Streamed CSV and XLSX export with spreadsheet
  formula neutralisation.

### Added — Phase 7: inbox

- Conversations: one per customer and business number, derived from
  messages and updated in the transaction of every message write (backfilled
  on upgrade). Status open/closed (a customer message reopens), assignee,
  team-wide unread count, last activity and the 24-hour window
  (`window_open`, `window_expires_at`).
- `/api/v1/conversations` API: list with status, assignee (`me`,
  `unassigned`, user), unread, number and search filters (keyset paging,
  10k conversations well under 200 ms), detail with contact summary,
  message history (newest first, cursor), replies (text, media, template
  through `messaging.Queue`, same window and opt-out rules), assignment,
  close/reopen, mark read, assignment and status history, and an
  authenticated media download (safe Content-Type/Disposition, nosniff,
  sandbox CSP).
- Visibility: `conversations.read` alone sees conversations assigned to the
  user or to nobody, on every endpoint including history and media;
  invisible conversations answer 404. Agents may subscribe to
  `conversation:<id>` for conversations visible to them.
- Realtime `conversation.updated` events on `inbox:all` and
  `conversation:<id>`; message events also go to the conversation channel
  and carry `conversation_id`.
- Panel: Inbox page with filters, conversation list, thread with delivery
  ticks and media, reply box with the "outside the 24-hour window — send a
  template" state, assignment, close/reopen, customer panel (consent, tags,
  notes, history), live updates without polling.

### Changed — Phase 7

- The 24-hour window is checked per conversation (customer and sending
  number) instead of per contact, also for `POST /api/v1/messages`.

### Added — Phase 6: real-time

- `POST /api/v1/realtime/connection-token` and
  `POST /api/v1/realtime/subscription-token` for signed-in panel users:
  short-lived HS256 tokens for Centrifugo, issued only for channels the
  user's role may read (`inbox:all`, `conversation:<id>`, `campaign:all`,
  `campaign:<id>`); anything else answers `403 CHANNEL_FORBIDDEN`. The
  user's personal channel and system notices are subscribed server-side.
- Realtime events through the transactional outbox (`rt:<channel>` topics,
  relayed by the worker with the outbox ID as Centrifugo idempotency key):
  `message.created` for inbound and queued outbound messages and
  `message.status` for send outcomes and delivery updates. Events carry IDs
  and states, never message content.
- One Centrifuge connection in the panel with automatic reconnects and
  history recovery; when missed events cannot be recovered (for example
  after a Centrifugo restart) the page is re-fetched. The overview shows
  live message activity. The CSP allows only the same-origin WebSocket.
- The API now requires `CENTRIFUGO_HMAC_SECRET` and the worker
  `CENTRIFUGO_API_KEY` (both generated by `messora init`).

### Added — Phase 3: WhatsApp Cloud API

- Graph API client (configurable version, default v26.0) with send-outcome
  classification: messages are retried only when Meta certainly did not
  accept them; ambiguous outcomes become `unknown` and are reconciled from the
  status webhook via `biz_opaque_callback_data`.
- Connect WhatsApp Business Accounts with encrypted tokens and app secrets;
  phone number and template sync; template creation and deletion.
- `POST /api/v1/messages` for text, template, media, location, reaction and
  interactive messages, with the 24-hour customer service window enforced.
- Signed webhook ingestion that stores events before acknowledging them,
  deduplicates Meta's retries and applies delivery statuses monotonically;
  template review and number limit updates; inbound media download to a
  dedicated volume.
- A programmable Meta mock (`whatsapptest`) and `META_DRY_RUN`, forced on in
  tests. See docs/WHATSAPP.md.

### Added — Phase 2: panel

- Sign-in flows (setup, sign-in, two-factor verification, invitation
  acceptance, password reset) and administration pages (overview, profile,
  security with TOTP enrolment and sessions, users and invitations, API keys,
  audit log, settings) on a Next.js BFF with HttpOnly session cookies and a
  nonce-based Content Security Policy.
- Playwright end-to-end journeys on a throw-away stack (`scripts/e2e.sh`),
  run in CI.

### Added — Phase 2: identity and administration (API)

- Spec-first REST API (`api/openapi.yaml`, OpenAPI 3.1) with generated Go
  types and router; authorization, request validation, per-principal rate
  limits and `Idempotency-Key` replays enforced from the spec.
- First-owner setup with a one-time token (logged on first start,
  `messora setup token` CLI), sign-in with Argon2id, account lockout,
  TOTP two-factor authentication with replay protection and recovery codes,
  mandatory 2FA per role, re-authentication for sensitive actions, session
  management, password change and e-mailed password reset.
- Users, e-mail invitations, roles (Owner, Admin, Manager, Agent) and 30
  module permissions with privilege-escalation and last-owner safeguards.
- Scoped API keys, workspace settings and an append-only audit log with
  cursor pagination.
- SMTP mailer; hourly cleanup of expired sessions and idempotency records.

### Changed

- Panel UI rebuilt on shadcn/ui (radix-nova) with Messora's own design tokens.
- Web image runs on distroless Node 26 (no shell or npm; UID 65532).

### Added — CI hardening

- All GitHub Actions pinned to commit SHAs; least-privilege permissions.
- Coverage gate, `go mod tidy` check, license checks (Go and npm), squawk
  migration lint, hadolint, actionlint, Trivy image and config scans,
  gitleaks over the full history, CodeQL (Go, TypeScript, Actions),
  OpenSSF Scorecard, dependency review and Conventional-Commit PR titles.
- Compose end-to-end job: smoke test, restart idempotency, worker crash
  recovery.
- Dependabot for Go, npm, Docker, Compose and Actions.
- Tag-triggered release of multi-arch images to GHCR with SBOM, provenance
  and keyless cosign signatures.

### Added — Phase 1: foundation

- Single `messora` binary with `api`, `worker`, `scheduler`, `migrate`,
  `init`, `healthcheck` and `version` commands.
- Configuration from environment plus an auto-generated secrets file, with
  per-role validation and a `Secret` type that never prints its value.
- Structured JSON logging with redaction of credentials, message content and
  phone numbers; request IDs propagated from the panel to the API and jobs.
- PostgreSQL 18 (pgx 5.13) with goose migrations (advisory-locked, up/down
  tested) and sqlc-generated queries.
- Redis 8 with AOF persistence and `noeviction`; Asynq worker with priority
  queues, exponential backoff with jitter and task metrics.
- Transactional outbox with a LISTEN/NOTIFY relay and idempotent task
  publication.
- Leader-elected scheduler (Redis lease lock + slot-based task IDs).
- Health (`/healthz`, `/readyz`) and Prometheus metrics on an internal port
  for every service; panel-only `GET /api/v1/system/status`.
- Next.js 16 panel/BFF built on shadcn/ui with the Messora theme and an
  installation status page.
- Docker Compose stack (Caddy, panel, API, worker, scheduler, init, migrate,
  PostgreSQL, Redis, Centrifugo) that starts with one command and generates
  its own secrets.
- CI: unit + integration tests, golangci-lint, govulncheck, sqlc drift check,
  frontend lint/type/build, secret scan and a Compose smoke test.
- Design documents in `docs/`.
