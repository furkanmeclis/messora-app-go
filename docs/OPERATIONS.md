# Operating Messora

Everything an operator needs after `docker compose up -d --build`: going
live on a domain, backups, upgrades, monitoring and troubleshooting.

## 1. Going live

The multi-tenant installation (platform domain, wildcard and custom-domain
certificates, Dokploy) is described in [INSTALL.md](INSTALL.md); staging on
AWS in [STAGING_AWS.md](STAGING_AWS.md). The steps below are the minimal
single-domain path.

1. A Linux server with Docker Engine 26+ and Compose 2.24+, 2 vCPU / 4 GB RAM
   at least (the default container limits add up to about 3.7 GB), and a
   DNS name pointing at it.
2. In `.env`:

   ```dotenv
   SITE_ADDRESS=messora.example.com
   PUBLIC_APP_URL=https://messora.example.com
   APP_ENV=production
   ```

   Caddy obtains and renews a Let's Encrypt certificate automatically
   (ports 80 and 443 must be reachable from the internet).
3. `docker compose up -d --build`, then open the platform admin bootstrap
   link printed by `docker compose logs messora-migrate`
   (`https://admin.<TENANT_BASE_DOMAIN>/bootstrap?token=…`, valid 24 hours) or
   run `docker compose exec -T messora-api messora admin create --email you@example.com --password-stdin`.
   Platform admins must set up two-factor authentication at their first
   sign-in. Companies are created in the admin panel (with an owner
   invitation) or by self sign-up on `app.<TENANT_BASE_DOMAIN>`.
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

PostgreSQL's memory follows `PG_SHARED_BUFFERS` (default 128MB),
`PG_EFFECTIVE_CACHE_SIZE` (512MB), `PG_WORK_MEM` (8MB) and
`PG_MAX_CONNECTIONS` (100); keep `shared_buffers` at about a quarter of
`MEM_POSTGRES`. Each API and worker process opens at most `DB_MAX_CONNS`
(20) connections per pool.

**8 GB / 2 vCPU host shared with Dokploy** (staging; Dokploy, its database
and Traefik take about 1.5 GB):

| Service | `MEM_*` | Notes |
|---|---|---|
| postgres | `2g` | `PG_SHARED_BUFFERS=512MB`, `PG_EFFECTIVE_CACHE_SIZE=1536MB` |
| redis | `512m` | queues and caches; `noeviction` |
| messora-api | `768m` | |
| messora-worker | `1g` | `WORKER_CONCURRENCY=20` |
| messora-scheduler | `128m` | |
| messora-web | `768m` | |
| centrifugo | `256m` | |
| caddy | `128m` | |
| backup | `256m` | only busy while dumping |

That is about 5.8 GB of limits; actual use at rest is far lower. CPU
limits are per container (at most 2), so the host's two cores are shared
by the kernel's scheduler.

## 2. Backups

### Nightly database dump to S3 (multi-tenant / Dokploy)

The `backup` service (`deployments/backup`, Compose profile `backup`) dumps
the database every night and uploads it to S3. Enable it in `.env`:

```dotenv
COMPOSE_PROFILES=backup
BACKUP_S3_BUCKET=messora-media-575500997783
BACKUP_S3_PREFIX=ops/backups/postgres     # default
BACKUP_AT=02:30                           # UTC, default
BACKUP_RUN_ON_START=true                  # one dump right after each deploy
# S3_REGION (or AWS_REGION) is reused; BACKUP_S3_ENDPOINT for R2 / MinIO;
# BACKUP_AWS_ACCESS_KEY_ID / BACKUP_AWS_SECRET_ACCESS_KEY only without an
# instance role. BACKUP_S3_SSE=AES256 (default) or none.
```

Each run: `pg_dump --format=custom` into the `backup-work` volume,
`pg_restore --list` to prove the archive is readable, upload to
`s3://<bucket>/<prefix>/messora-<UTC stamp>.dump` with SSE and a `sha256`
metadata entry, then the local file is removed. A failed night is retried
the next day; the container turns **unhealthy** after 26 hours without a
successful dump (alert on it). One dump now:
`docker compose run --rm backup once`.

Retention is an S3 lifecycle rule on the prefix, e.g. expire after 30 days:

```bash
aws s3api put-bucket-lifecycle-configuration --bucket messora-media-575500997783 \
  --lifecycle-configuration '{"Rules":[{"ID":"messora-db-backups","Status":"Enabled",
  "Filter":{"Prefix":"ops/backups/postgres/"},"Expiration":{"Days":30}}]}'
```

(This replaces the bucket's lifecycle configuration: merge it with rules
the bucket already has.) The instance role needs `s3:PutObject` on the
prefix; restores need `s3:GetObject`. A dedicated backup bucket (another
account or region, object lock) is better for production than a prefix of
the media bucket.

**Not in the dump:** the secrets (`APP_ENCRYPTION_KEY` decrypts the stored
credentials, so it must not sit next to the dumps; keep it in the Dokploy
environment and a password manager), media (in S3 already: enable bucket
versioning) and Redis.

**Restore from S3** into a running stack:

```bash
aws s3 cp s3://messora-media-575500997783/ops/backups/postgres/messora-<stamp>.dump /tmp/db.dump
docker compose stop messora-api messora-worker messora-scheduler
docker compose exec -T postgres sh -c 'pg_restore --clean --if-exists --no-owner -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < /tmp/db.dump
docker compose up -d     # messora-migrate brings an older dump up to date
```

The secrets of the installation the dump came from must be in place (the
same `APP_ENCRYPTION_KEY`; the role passwords are set by `messora migrate`
on every run).

### Full backup on the host (single server)

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

**On Dokploy** (multi-tenant), per release:

1. Read the release's CHANGELOG entry (new environment variables, manual
   steps) and add new variables to the Environment tab.
2. Take a dump: `docker compose run --rm backup once` (Dokploy terminal on
   the host, in the application's directory, with
   `-f docker-compose.dokploy.yml`).
3. Point the Compose application at the new tag or branch and **Deploy**
   (Dokploy runs `docker compose up -d --build`). The migration job runs
   first: `messora-migrate` must exit 0 before `messora-api`,
   `messora-worker` and `messora-scheduler` are recreated (they wait on
   `service_completed_successfully`); if it fails the deploy stops there.
   Its log: `docker compose logs messora-migrate`.
4. `messora-init` rewrites Centrifugo's configuration on every deploy (new
   origins after a `TENANT_BASE_DOMAIN` change); restart Centrifugo when it
   changed: `docker compose restart centrifugo`.
5. Check: every service healthy, `https://admin.<base>/login` loads, one
   company panel shows "Live", `curl -s -o /dev/null -w '%{http_code}'
   https://hooks.<base>/meta/does-not-exist` answers 404 from the API.

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
| Realtime never connects on one host | Caddy answers 403 when the WebSocket's `Origin` is not the page's own host. For company sub-domains Centrifugo also checks `https://*.<TENANT_BASE_DOMAIN>`: after changing the base domain, redeploy (messora-init rewrites the list) and restart Centrifugo. |
| No certificate for a company sub-domain | Behind Traefik: the DNS-01 resolver (`letsencrypt-dns`) or Traefik's `NAMECOM_*` environment is missing; see Traefik's log. `EDGE_MODE=caddy`: `docker compose logs caddy` (name.com user and token). |
| No certificate for a custom domain | The domain is not verified, its company is suspended, or its DNS does not reach this server yet. `docker compose exec messora-api messora healthcheck "http://127.0.0.1:9100/internal/tls/allow?domain=<host>"` exits 0 when it is allowed; behind Traefik `docker compose exec caddy wget -qO- http://messora-api:9100/internal/edge/traefik` must list it and Traefik's HTTP provider must be configured. |
| Every page says there is no workspace at this address | The host is not a company address: `TENANT_BASE_DOMAIN` differs from the DNS name, or a proxy in front rewrites `Host` (Traefik must keep `passHostHeader`). |
| Lost the owner's second factor | Use a recovery code. If all are lost, another owner can reset it; with a single owner, contact your operator to reset `users.totp_*` in the database. |

## Behind an existing reverse proxy (Traefik, Dokploy)

When another proxy already owns ports 80 and 443 — for example a server
managed with Dokploy — Messora must not bind them. Use the proxy override
(on Dokploy `docker-compose.dokploy.yml`, which includes it):

```sh
# .env
TENANT_BASE_DOMAIN=messora.example.com
PUBLIC_APP_URL=https://app.messora.example.com
TRUSTED_PROXIES=10.0.1.0/24       # the proxy network's subnet
PROXY_ROUTER=messora              # unique name on that proxy
# PROXY_NETWORK=dokploy-network  PROXY_ENTRYPOINT=websecure  PROXY_HTTP_ENTRYPOINT=web
# PROXY_DNS_CERTRESOLVER=letsencrypt-dns

docker compose -f docker-compose.yml -f docker-compose.proxy.yml up -d --build
```

Caddy then publishes no ports, runs `EDGE_MODE=proxy`, joins the proxy's
network (alias `<PROXY_ROUTER>-edge`) and carries Traefik labels for the
base domain and its sub-domains (wildcard certificate from the DNS-01
resolver, HTTP to HTTPS redirect). Custom domains come from Traefik's HTTP
provider ([INSTALL.md](INSTALL.md#3-multi-tenant-on-dokploy-traefik-in-front)).
Caddy believes `X-Forwarded-For` / `X-Forwarded-Proto` only from
`TRUSTED_PROXIES` (default `private_ranges` in this mode; set the subnet,
since other applications on a shared proxy network have private addresses
too), so client addresses in audit logs and rate limits stay correct and
the realtime connection uses `wss://`. Use the same `-f` pair for every
later `docker compose` command, or set
`COMPOSE_FILE=docker-compose.yml:docker-compose.proxy.yml` in `.env`.
