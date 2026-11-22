# Messora

**Self-hosted WhatsApp Business Cloud API platform** — customer records,
segmentation, campaigns, a shared inbox, automations and a developer API, run
on your own server for a single business.

> **Status: early development (Phase 1 of 10).** The infrastructure is in
> place and runs with one command; product features arrive phase by phase —
> see the [roadmap](docs/ROADMAP.md). Not ready for production messaging yet.

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
built locally). You will see the installation status page; every component
should report **Active**.

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
docker compose down                # stop, keep data
docker compose up -d --build       # update after git pull (migrations run automatically)
```

**Back up** the `postgres-data` volume (or `pg_dump`) **and** the
`messora-secrets` volume: it holds `APP_ENCRYPTION_KEY`, without which stored
WhatsApp credentials cannot be decrypted. Full backup/restore guidance comes
with Phase 10.

Prometheus metrics and readiness are served on port 9100 inside each backend
container (not published through Caddy).

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
| [docs/ANALYSIS.md](docs/ANALYSIS.md) | Verified analysis of ZeroMsg, the project Messora derives from |
| [docs/FEATURE_MATRIX.md](docs/FEATURE_MATRIX.md) | Feature-by-feature status and plan |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Architecture, reliability design, pinned versions |
| [docs/DATABASE_DESIGN.md](docs/DATABASE_DESIGN.md) | Target schema |
| [docs/SECURITY.md](docs/SECURITY.md) | Threat model and controls |
| [docs/MIGRATION_PLAN.md](docs/MIGRATION_PLAN.md) | How ZeroMsg features move to Messora |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Phases and exit criteria |

The design documents are currently written in Turkish.

## Acknowledgements and license

Messora is MIT licensed — see [LICENSE](LICENSE). Parts of the user interface
are derived from [ZeroMsg](https://github.com/megoxv/zeromsg) by megoxv (MIT);
see [NOTICE](NOTICE).

Security issues: please follow [SECURITY.md](SECURITY.md).
