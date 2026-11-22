# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

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
