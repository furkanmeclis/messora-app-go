# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

Phase 17 WP0: Messora becomes a hosted, multi-tenant SaaS only
(docs/PHASE-17.md). Features of the self-hosted product are removed.

### Added

- **Object storage** (`internal/platform/storage`, docs/STORAGE.md): media,
  contact imports and error reports, campaign exports and the logo go
  through one store with a `local` driver (the media volume, HMAC-signed
  `/media/s/` links) and an `s3` driver (AWS S3, Cloudflare R2, MinIO;
  SSE-S3, multipart uploads, presigned URLs, EC2 instance role). Private and
  public buckets; public assets via `PUBLIC_ASSET_BASE_URL` (CDN) or
  `/media/p/`. Tenant-scoped keys `t/<tenant>/<kind>/<yyyy>/<mm>/<uuid>`.
  New settings `STORAGE_DRIVER`, `STORAGE_LOCAL_DIR` (was `MEDIA_DIR`, still
  read), `MEDIA_SIGNING_KEY`, `S3_*`, `PUBLIC_ASSET_BASE_URL`; `/readyz`
  checks storage.

### Removed

- **Licensing:** licence verification, restricted mode, the
  `license:check_in` job, `GET/PUT /license`, Settings → License and the
  licence banner, the `licence` field of the appearance and the operations
  center alerts, the `messora-license` CLI, the Cloudflare licence gate
  (`deployments/license-gate`), the customer install file, the licensed R2
  release job and `LICENSE_GATE_URL`. Campaigns paused by the former check
  keep `license_restricted` as their pause reason.
- **Theme engine:** presets, colour editor, typefaces, density and
  wallpapers, the `theme` settings field, the wizard's theme step and
  Settings → Appearance. Every workspace uses Messora's single look (static
  CSS); the company name and logo stay (the logo moved to
  Settings → General) and people still choose light or dark.
- **PWA:** manifest, offline page, install prompt, Settings → PWA and the
  `pwa` settings field. For one release `sw.js` is a worker that removes
  itself and every page unregisters an old service worker.
- **Desktop mode:** the windowed mode with its embedded page rendering.
  Nothing frames the panel any more: `frame-ancestors 'none'` and
  `X-Frame-Options: DENY`.
- Migration 00023 deletes the stored licence, theme and PWA settings.
- Self-hosting documentation (docs/LICENSING.md; docs/INSTALL.md now
  describes running the hosted service).

### Added (WP2 — identity and the platform admin panel)

- **Global accounts:** one account per person (e-mail unique across the
  platform) with a membership and role in each company
  (`tenant_memberships`); existing users become members of tenant #1
  (migration 00030). Sessions and API keys stay bound to one workspace; a
  password change or reset ends the sessions in every workspace.
- **Self sign-up** (`POST /auth/signup`, slug availability, e-mail
  verification, resend with a one-per-minute limit): creates the company at
  `<slug>.<TENANT_BASE_DOMAIN>`, its owner and a 14-day trial
  (`subscription_status`, `trial_ends_at`). Platform setting
  `signup_enabled`.
- **Workspace switcher:** `GET /me/memberships`, `POST /auth/switch-workspace`
  and `POST /auth/handoff` (one-time, 60-second token valid only at the
  target workspace).
- **Invitations of existing accounts:** the preview says whether the address
  has an account, who invited and to which workspace; accepting asks for the
  existing password instead of creating an account.
- **E-mail address change** (`POST /me/email`, `POST /auth/email-change/confirm`).
- **Platform admin panel API** on `admin.<TENANT_BASE_DOMAIN>`
  (`/api/v1/platform/*`, never in the public API): its own sessions and
  cookie, mandatory TOTP, companies (create with owner invitation, rename,
  trial end, suspend/unsuspend with reason), custom domains with TXT
  verification, accounts (search, memberships, disable, forced password
  reset), time-boxed **support access** (read-only by default, recorded in
  the company's audit log, visible to the company at `GET /support-access`,
  banner in its panel), platform settings and the platform audit log.
- **Platform bootstrap:** `messora migrate` prints a one-time admin link while
  no platform admin exists; `messora admin create --email …` creates or
  promotes one.
- Branded identity e-mails for verification, welcome, password reset and
  change, invitations, new sign-in alerts and address changes.
- Database role `messora_platform` (no login, no BYPASSRLS) that the API
  switches to for platform work only.

### Changed (WP2)

- The Meta rate card and the AI price table are managed in the platform
  panel (`/platform/meta-rates/import`, `/platform/ai-prices*`); workspaces
  read them only.
- Disabling a user in a workspace disables the membership, not the account.
- `GET /auth/capabilities` reports `signup_enabled` instead of
  `setup_required`; `GET /me` includes the workspace and, during support
  access, `support`.
- `APP_*` workspace variables apply to tenant #1 only.
- Reserved workspace addresses are the sign-up form's list; new slugs are
  3–30 characters.
- Rate-limit budgets of sessions and API keys are kept per workspace; MFA
  sign-in challenges are bound to the workspace (or the admin panel) they
  started on.

### Removed (WP2)

- The first-run setup flow: `GET /setup/status`, `POST /setup` and
  `messora setup token`.
- `POST /settings/meta-rates/import` and the write operations of
  `/ai/prices` in the workspace API (moved to the platform panel).

### Added (WP1 — tenant foundation)

- **Tenants:** `tenants`, `tenant_domains` and `platform_settings`. Every
  company's data carries `tenant_id` (57 tables); existing data moves to
  tenant #1, whose slug comes from the organisation name (migrations
  00024–00028).
- **Row-level security:** forced on every tenant table. The services
  connect as `messora_app` (no BYPASSRLS, owns nothing) and every
  connection carries the tenant of its request or job; without one the
  pool refuses the connection. The outbox relay connects as
  `messora_system`. New secrets `POSTGRES_APP_PASSWORD` and
  `POSTGRES_SYSTEM_PASSWORD` (generated by `messora init`, applied by
  `messora migrate`).
- **Tenant from the host:** the API serves the tenant of the request's
  host (the panel forwards it in `X-Messora-Tenant-Host`). Unknown hosts
  get `404 TENANT_NOT_FOUND`, suspended tenants `403 TENANT_SUSPENDED`;
  a session or API key works only at its own tenant's address.
  `TENANT_BASE_DOMAIN` gives tenant #1 its `<slug>.<domain>` address.
- Names, phone numbers, Meta message IDs, webhook deduplication and
  idempotency keys are unique per tenant; child rows reference parents by
  `(tenant_id, id)`.
- Queue tasks made from the outbox carry their tenant and run as it.
- `messora migrate` seeds the platform AI price table (the API no longer
  does at start-up); its marker moves to `platform_settings` (migration
  00029).

### Fixed

- The panel's right-click menu no longer closes at once when the browser
  scrolls the clicked row into view (tables wider than the window).

## [0.12.0] - 2026-11-26

Phase 16C: advanced campaign management, AI automation and conversation
intelligence. Plan, verified Meta and AI-provider facts and decisions:
docs/PHASE-16C.md.

### Added — Phase 16C P0: real-time, queues, Meta cost and signals

- Inbox events follow conversation visibility (`inbox:all` for read_all,
  `inbox:unassigned`, the assignee's personal channel); versioned event
  envelope with IDs, de-duplication on the client and a targeted
  `GET /realtime/sync` after a missed reconnect; 10-minute history.
- Campaign sends run on their own queue; an `ai` queue; per-task retry
  budgets; System → Failed jobs (list, retry, delete, audited).
- Message pricing from Meta status webhooks (category, type, country) with
  cost from an importable rate card (Settings → Meta rate card) and a daily
  reconciliation against Meta's `pricing_analytics`.
- Number throughput, quality and portfolio messaging limit read from Meta;
  the send ceiling follows the number's real throughput. `user_preferences`,
  quality, account and template-quality webhooks are handled.

### Added — Phase 16C P2: send scheduler and targeting

- One send scheduler per number: campaigns share capacity by priority
  (high/normal/low), 20% stays reserved for inbox, API and automations.
- The portfolio messaging limit (unique users per 24 h) is counted; a
  campaign pauses as `paused_limit` and resumes by itself.
- New audience: customers a previous campaign did not reach, by failure
  class; hard consent blocks are never selectable.
- Pause history with snapshots, a pre-resume summary, edit on resume;
  131049 from the status webhook is resent after at least 24 h.

### Added — Phase 16C P3: campaign analytics, forecast and protection

- Analysis centre: Meta vs system-computed metrics, hourly chart, cost,
  quality and spam section, breakdowns by group, segment, tag, contact type
  and attribute; recipients table with server-side filters, bulk actions,
  CSV/XLSX export; XLSX/PDF report.
- Pre-launch forecast (duration with real capacity, Meta cost from the rate
  card with the 2026-10-01 rules, rates only with enough history) and
  forecast vs actual.
- Positive/negative replies from quick-reply mapping, AI or manual labels.
- Campaign protection: thresholds on 131048/131049/131050, opt-outs,
  quality and template pausing stop a campaign automatically.

### Added — Phase 16C P4: template builder and time-limited offers

- Full-page template builder with Meta's limits enforced server-side:
  location headers, copy code, voice call, OTP, Flow buttons, carousels and
  limited-time offer templates; richer WhatsApp preview.
- Offers with start/end, coupon and image; LTO campaigns send the offer's
  expiry; expired offers block or pause campaigns.

### Added — Phase 16C P5–P7: AI

- AI providers per installation (OpenAI, Anthropic, Gemini, DeepSeek):
  sealed keys, validation, model catalogue sync, a price table with
  effective dates, usage recording; AI profiles per number with a test
  console. Human handoff cannot be disabled (Meta policy).
- Conversation ownership (human / AI), AI modes (off, suggest, auto reply,
  awaiting approval), internal notes for the AI, escalation with summary,
  live activity timeline and automation runs in the inbox.
- Lead capture for unknown numbers with field provenance ("collected by
  the AI, not verified"), AI cost attributed to one campaign, usage page,
  monthly AI budget.

### Added — Phase 16C P8: automations

- Triggers: customer service window closing, no reply, campaign reply,
  reaction, AI escalation; AI steps (generate, analyse, intent, extract
  fields, segment, auto reply, route to agent, build profile), handoff and
  note steps; per-step retries with idempotent sends; 11 one-click presets.

### Added — Phase 16C P9 and P10

- Operations centre on the overview: live campaigns with capacity, ETA and
  cost, escalations, waiting customers, recent messages, automations,
  alerts and quick actions; system health on its own tab.
- Public API reference at `/docs/api` (Scalar, served locally) generated
  from the OpenAPI document; every public operation documented and linted
  in CI; `RateLimit-*` headers; `docs/API.md` inventory.

### Fixed

- Tables page through long lists with their own pager only (no separate
  "load more" button): campaigns, campaign recipients, audit log, webhook
  deliveries and automation runs. A campaign has one recipients table, with
  one status and one reason column.
- The sidebar meets the top bar without a light edge.

### Added — Phase 16C P1: subscription, consent and send blocking

- Consent per category for every contact: marketing and service (utility,
  authentication, replies), each `opted_in`, `unknown`, `opt_out_requested`,
  `opted_out` or `resubscribed` with when, where from and why, plus the
  Meta-sourced flags "stopped marketing in WhatsApp" and "undeliverable".
  Existing opt-outs became opt-outs of both categories; `opted_out` and the
  `opt_in` / `opt_out` API keep working. The full history is kept and
  shown on the contact; panel and API changes are audited.
- One set of rules for campaigns, the API, the inbox and automations, checked
  when a message is queued and again right before it is sent. Opt-outs,
  WhatsApp's Stop and undeliverable numbers are hard blocks no setting can
  override; marketing to contacts with unknown consent is allowed with a
  warning or blocked, as the company chooses.
- Campaign preflight (`POST /campaigns/preflight`) and a review step in the
  campaign wizard showing how many recipients are eligible, warned and
  blocked, and why.
- Keyword opt-out on inbound messages (Turkish-aware matching): stop words
  opt out at once with an optional confirmation, opt-out phrases become a
  request for an agent and notify them, resubscribe words opt back in. Lists
  and texts are editable under Settings → Subscription & consent. New
  automation trigger `subscription_request`.
- WhatsApp's Stop/Resume (`user_preferences`) and errors `131050` / `131026`
  update the contact; `131049` is retried no earlier than 24 hours later and
  no longer occupies a campaign's send window. Messages record why they
  failed (`failure_class`).

### Changed

- A full opt-out (both categories) now also blocks replies in a conversation
  the customer had open when opting out; a customer who writes again may be
  answered.

## [0.11.0] - 2026-11-25

### Added — Phase 16b: Meta connection and messaging screens

- WhatsApp access tokens are checked with Meta when connected, replaced and
  every six hours: validity, permissions, access to the account, type and
  expiry. Channels shows the token's lifetime with **Check connection**, and
  a step-by-step guide to System User tokens (also in the setup wizard).
  Owners and admins are e-mailed before a token expires and when Meta
  refuses it. Short-lived user tokens become long-lived ones when the app
  secret is known.
- Templates with image, video and document headers work in campaigns, inbox
  replies and automations; every variable is mapped to a contact field or
  fixed text, with a value for contacts that lack it, and a WhatsApp-style
  preview shows what a real contact receives. Values are checked against the
  template before anything is sent.
- The inbox is redesigned: a full-height three-pane layout, search as you
  type, an unread filter, day separators, and template replies with
  variables and media, limited to the conversation's account.
- The templates page: search and filters, real sync from Meta, details,
  "use in a campaign", and creating media-header templates (the example file
  is uploaded to Meta).
- Groups, tags and segments have insight pages: reach and activity, the
  message funnel, a 30-day activity chart and every campaign that reached
  their contacts — aimed at them or at an overlapping audience — with
  delivery, read, reply and opt-out figures for those contacts only.
- Every list in the panel uses the same table: search, sortable columns,
  pages, column visibility, density and a row menu; invitations and API keys
  can be revoked in bulk.

### Fixed

- Replacing the token of an account Meta had refused now reactivates it at
  once (it stayed in error until a manual sync), and a token changed in
  `.env` is applied on restart.
- Editing a campaign kept its header media; URL buttons other than the first
  can carry variables.

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
