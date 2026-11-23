# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

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
