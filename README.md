# Messora

**Self-hosted WhatsApp Business Cloud API platform** — customer records,
segmentation, campaigns, a shared inbox, automations and a developer API, run
on your own server for a single business.

> **Status: release candidate.** All product features are implemented and
> covered by integration, load and browser tests. Before messaging customers,
> run the manual smoke test with a WhatsApp test number described in
> [docs/WHATSAPP.md](docs/WHATSAPP.md).

## Features

- **WhatsApp Cloud API** — connect accounts and numbers, sync and create
  templates, every message type, signed webhooks, media download.
- **Contacts** — tags, groups, custom fields, consent history, CSV/Excel
  import (100k rows in ~20 s) and export, dynamic segments.
- **Campaigns** — scheduled template sends with per-contact variables,
  pause/resume/cancel, per-number pacing that adapts to Meta's limits,
  live statistics. Tested with 100,000 recipients: no duplicates, no losses.
- **Shared inbox** — conversations with assignment, visibility rules, the
  24-hour window, live updates.
- **Automations** — triggers (keywords, new contacts, tags, delivery
  statuses, closed conversations) and steps (replies, templates, tags,
  conditions, delays, HTTP, e-mail …).
- **Developer API and outgoing webhooks** — OpenAPI 3.1 spec, API keys with
  scopes, idempotency keys, signed webhooks with retries.
- **Security** — two-factor authentication, roles and permissions, audit log,
  encrypted secrets, SSRF-protected outbound HTTP.

Messora uses only Meta's **official WhatsApp Cloud API** (no WhatsApp Web or
QR-code sessions). Reliability is the first design goal: no lost messages, no
duplicate sends, and explicit handling of "did it go out?" uncertainty — see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Quick start

Requirements: a Linux or macOS host with **Docker Engine 26+ and Docker
Compose 2.24+**. You do not need Go, Node.js, PostgreSQL, Redis or Centrifugo
installed.

```bash
git clone https://github.com/furkanmeclis/messora-app-go.git messora
cd messora
cp .env.example .env
docker compose up -d --build
```

Open <http://localhost>. The first start takes a few minutes (images are
built locally). Create the owner account with the one-time setup link:

```bash
docker compose exec messora-api messora setup token
```

What happens on first start:

1. `messora-init` generates every secret you left empty in `.env` (database
   and Redis passwords, encryption key, Centrifugo keys …) into the
   `messora-secrets` volume — one file per service, readable only by it.
2. `messora-migrate` creates the database schema.
3. The API, worker, scheduler, panel, Centrifugo and Caddy start; each has a
   health check and `docker compose ps` shows them as `healthy`.

### Ports and domains

| Setting (`.env`) | Default | Purpose |
|---|---|---|
| `HTTP_PORT` / `HTTPS_PORT` | `80` / `443` | Host ports published by Caddy |
| `SITE_ADDRESS` | `:80` | `:80` = plain HTTP for local use. Set your domain (e.g. `messora.example.com`) for automatic Let's Encrypt HTTPS — **required for WhatsApp webhooks** |
| `PUBLIC_APP_URL` | `http://localhost` | Exact URL users open (scheme, host and non-default port); used for realtime origin checks |

If ports 80/443 are taken locally, set e.g. `HTTP_PORT=8088` and
`PUBLIC_APP_URL=http://localhost:8088`.

## Architecture in one picture

```text
          ┌──────── caddy (TLS, routing) ────────┐
          │                │                     │
   /api/v1, /webhooks   panel (/)          /connection
          │                │                     │
     messora-api ◄── messora-web (Next.js BFF)  centrifugo
          │
   PostgreSQL 18 ── Redis 8 ── messora-worker ── messora-scheduler
```

One Go binary serves every backend role. PostgreSQL is the source of truth;
Redis carries the job queue (Asynq), locks and rate limits; Centrifugo pushes
realtime updates. Details: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Operations

```bash
docker compose ps                  # status and health
docker compose logs -f messora-api # logs (JSON, secrets redacted)
scripts/backup.sh /srv/backups     # database + secrets + media
docker compose up -d --build       # update after git pull (migrations run automatically)
```

Going live on a domain, backups and restore, upgrades, monitoring and
troubleshooting: [docs/OPERATIONS.md](docs/OPERATIONS.md).

## Development

```bash
make help              # all tasks
make test              # unit tests
make test-integration  # + real PostgreSQL/Redis via testcontainers (Docker)
make lint web-lint     # linters
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Documentation

| Document | Contents |
|---|---|
| [docs/FEATURE_MATRIX.md](docs/FEATURE_MATRIX.md) | Feature scope and current status |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Architecture, reliability design, pinned versions |
| [docs/DATABASE_DESIGN.md](docs/DATABASE_DESIGN.md) | Target schema |
| [docs/SECURITY.md](docs/SECURITY.md) | Threat model and controls |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Deployment, backup/restore, upgrades, monitoring |
| [docs/WHATSAPP.md](docs/WHATSAPP.md) | WhatsApp setup, delivery guarantees, inbox rules |
| [docs/AUTOMATIONS.md](docs/AUTOMATIONS.md) | Automation triggers, steps and guarantees |
| [docs/WEBHOOKS.md](docs/WEBHOOKS.md) | Outgoing webhooks for integrators |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Phases and exit criteria |
| `api/openapi.yaml` | Developer API (OpenAPI 3.1) |

The design documents are currently written in Turkish.

## License

Messora is commercial software: using it requires a licence key (see
[LICENSE](LICENSE) and [docs/LICENSING.md](docs/LICENSING.md)). Third-party
notices are in [NOTICE](NOTICE).

Security issues: please follow [SECURITY.md](SECURITY.md).
