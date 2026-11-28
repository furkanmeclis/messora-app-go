# Operating Messora

Everything an operator needs after `docker compose up -d --build`: going
live on a domain, backups, upgrades, monitoring and troubleshooting.

## 1. Going live

1. A Linux server with Docker Engine 26+ and Compose 2.24+, 2 vCPU / 4 GB RAM
   for a typical single business (the default container limits add up to
   about 3.6 GB), and a DNS name pointing at it.
2. In `.env`:

   ```dotenv
   SITE_ADDRESS=messora.example.com
   PUBLIC_APP_URL=https://messora.example.com
   APP_ENV=production
   ```

   Caddy obtains and renews a Let's Encrypt certificate automatically
   (ports 80 and 443 must be reachable from the internet).
3. `docker compose up -d --build`, then follow the setup link printed in
   `docker compose logs messora-api` (or run
   `docker compose exec messora-api messora setup token`) to create the owner
   account. Two-factor authentication is mandatory for owners and admins.
4. Connect WhatsApp: Channels → connect account, then enter the webhook values
   from Channels → Webhook in the Meta App dashboard. See
   [WHATSAPP.md](WHATSAPP.md) — including the manual smoke test with a test
   number before messaging customers.
5. Optional: SMTP (`SMTP_*`) for invitations and password reset.

Secrets you leave empty in `.env` are generated once by `messora-init` and
kept in the `messora-secrets` volume. Values you set in `.env` win.

### Resource limits

Each service has memory and CPU limits suitable for a small server. Raise
memory with `MEM_<SERVICE>` in `.env` (e.g. `MEM_POSTGRES=4g`,
`MEM_MESSORA_WORKER=1g`) or override anything in a
`docker-compose.override.yml`. Logs rotate at 20 MB × 5 files per container.

## 2. Backups

```bash
scripts/backup.sh /srv/backups          # database + secrets + media
```

A backup directory contains:

| File | Contents |
|---|---|
| `database.dump` | `pg_dump` (custom format) taken while the stack runs |
| `secrets.tar.gz` | the `messora-secrets` volume — **includes `APP_ENCRYPTION_KEY`** |
| `media.tar.gz` | files of the local storage driver (media, imports, exports, logos); with `STORAGE_DRIVER=s3` back up the buckets instead (versioning or replication, see [STORAGE.md](STORAGE.md)) |
| `manifest.txt`, `SHA256SUMS` | version, schema version, checksums |

Without the secrets archive, encrypted data (WhatsApp tokens, TOTP secrets,
webhook signing secrets) is lost; with it, anyone can read that data. Encrypt
backups and keep a copy off the server, e.g.:

```bash
scripts/backup.sh /srv/backups && \
  tar -C /srv/backups -cz "$(ls -t /srv/backups | head -1)" | age -r <recipient> > backup.tgz.age
```

Schedule it with cron (daily) and prune old backups. Redis is not backed up:
it only holds queues, caches and rate-limit state; the reapers recreate
queued work from PostgreSQL after a restore.

### Restore

```bash
scripts/restore.sh /srv/backups/messora-20261123-101500 --yes
```

The script verifies checksums, stops the application services, replaces the
secrets and media volumes, aligns the database role's password with the
restored secrets, replaces the database, and starts everything (migrations
bring an older backup up to the running version). On a new server: clone the
repository, copy your `.env`, run `docker compose up -d --build` once, then
restore.

**Rehearsal.** `scripts/backup-restore-test.sh` installs a throw-away stack,
writes marker data, backs up, destroys every volume, installs fresh (new
secrets, empty database), restores and verifies the data, the media and the
encryption key. CI runs it on every change; run it yourself after changing
anything in the deployment.

## 3. Upgrades

```bash
scripts/backup.sh /srv/backups     # always back up first
git pull
docker compose up -d --build
docker compose ps                  # every service healthy
```

`messora-migrate` applies database migrations before the API, worker and
scheduler start. Migrations are forward-only in production and written to
avoid long locks; read [CHANGELOG.md](../CHANGELOG.md) before upgrading
across several releases. To roll back, restore the backup taken before the
upgrade and check out the previous release.

The worker finishes in-flight jobs on shutdown (`SHUTDOWN_TIMEOUT`, 30 s;
the container's stop grace period is 45 s). A message that was being sent
when a worker was killed is never sent twice: it becomes `unknown` and is
reconciled from Meta's status webhook.

## 4. Monitoring

- `docker compose ps` — every service has a health check.
- Panel → Overview shows each component (database, schema, Redis, worker,
  scheduler, Centrifugo).
- `docker compose exec messora-api messora healthcheck http://127.0.0.1:9100/status`
  checks the whole stack from inside.
- Prometheus metrics on port 9100 of `messora-api`, `messora-worker` and
  `messora-scheduler` (not published; attach a Prometheus container to the
  `internal` network to scrape them): HTTP latency and status, queue depth,
  task outcomes and durations, outbox lag, database pool, Redis status.
- Logs are JSON with secrets redacted: `docker compose logs -f messora-worker`.
- Tracing (optional): set `OTEL_EXPORTER_OTLP_ENDPOINT` (e.g.
  `http://otel-collector:4318`) and, if needed, `OTEL_EXPORTER_OTLP_HEADERS`
  and `OTEL_TRACES_SAMPLER=parentbased_traceidratio` with
  `OTEL_TRACES_SAMPLER_ARG=0.1`. The API, worker and scheduler then export
  spans for requests (named after the route), queue tasks, database queries
  (operation names only, never SQL text) and calls to Meta; log lines carry
  `trace_id`. The collector must be reachable from the `edge` network.

Alert on: unhealthy containers, growing outbox lag or queue depth, campaigns
with many `unknown` messages, WhatsApp accounts in `error` status (expired or
revoked token), and disk usage of the Docker data directory.

## 5. Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `read-only file system` from Docker | The disk holding Docker's data (or Docker Desktop's virtual disk) is full. Free space, then restart Docker. `docker builder prune` removes build cache only. |
| Webhooks rejected with 401 | The workspace has no account with an app secret, or the app secret belongs to another app: replace it on the account (Channels → Edit). Settings → WhatsApp / Meta counts rejected signatures. |
| Webhooks answered 404 | The webhook key in the callback URL is unknown or its grace period after a rotation is over: copy the current callback URL from Settings → WhatsApp / Meta. |
| Meta webhook verification fails | The callback URL is not public HTTPS (`hooks.<TENANT_BASE_DOMAIN>` needs DNS and TLS, or set `HOOKS_PUBLIC_URL`), or the verify token in the Meta dashboard differs from Settings → WhatsApp / Meta. |
| Account shows `error` | Meta refused the token (expired, revoked or missing permission). Rotate the token in Channels; a successful sync clears the error. |
| Messages stay `unknown` | The send outcome was ambiguous (timeout, dropped connection). They are never re-sent; Meta's status webhook reconciles them. Check that webhooks arrive. |
| Campaign slower than expected | Meta's throughput error (130429) halves the per-number rate; it recovers automatically. |
| Panel shows "Live" but nothing updates | Check `docker compose logs centrifugo`; `PUBLIC_APP_URL` must match the URL in the browser exactly. |
| Lost the owner's second factor | Use a recovery code. If all are lost, another owner can reset it; with a single owner, contact your operator to reset `users.totp_*` in the database. |

## Behind an existing reverse proxy (Traefik, Dokploy)

When another proxy already owns ports 80 and 443 — for example a server
managed with Dokploy — Messora must not bind them. Use the proxy override:

```sh
# .env
PUBLIC_HOST=messora.example.com
PUBLIC_APP_URL=https://messora.example.com
PROXY_ROUTER=messora            # unique name on that proxy
# PROXY_NETWORK=dokploy-network  PROXY_ENTRYPOINT=websecure  PROXY_CERTRESOLVER=letsencrypt

docker compose -f docker-compose.yml -f docker-compose.proxy.yml up -d --build
```

Caddy then publishes no ports, joins the proxy's network and carries Traefik
labels; Traefik terminates TLS and forwards to it. Caddy trusts the proxy's
`X-Forwarded-For` / `X-Forwarded-Proto` only in this mode
(`TRUSTED_PROXIES=private_ranges`), so client addresses in audit logs and rate
limits stay correct and the realtime connection uses `wss://`. Use the same
`-f` pair for every later `docker compose` command (upgrade, backup scripts
take `COMPOSE_FILE`), or set `COMPOSE_FILE=docker-compose.yml:docker-compose.proxy.yml`
in `.env`. Give each Messora service a memory limit (`MEM_*`) on shared servers.

### With Dokploy (its own domains)

See [INSTALL.md](INSTALL.md#behind-traefik-or-dokploy).
