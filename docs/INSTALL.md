# Installing Messora

You need a Linux server with Docker (Compose v2.23 or newer), a domain
pointing at it, and your Messora licence key.

```sh
mkdir messora && cd messora
GATE=messora-license-gate.messora-license-gate.workers.dev

# 1. Sign in to the Messora registry. Any username works; the password is
#    your licence key.
docker login "$GATE" -u customer

# 2. Download the install file (one file, nothing else is needed).
curl -fsSL -u customer -o compose.yml "https://$GATE/install/latest/compose.yml"

# 3. Your domain. Caddy fetches the HTTPS certificate itself.
cat > .env <<'ENV'
PUBLIC_APP_URL=https://messora.example.com
SITE_ADDRESS=messora.example.com
ENV

# 4. Start.
docker compose up -d
docker compose logs messora-api | grep setup_url
```

Open the setup link from the last command. The web wizard covers the rest:
owner account, team, e-mail, logo, theme, language and WhatsApp account.
Enter the licence key under **Settings → License**. Until then Messora runs
a 14-day trial.

All secrets (database, Redis, encryption keys) are generated on the first
start and kept in a Docker volume. `.env` only holds what you choose to
override; see the comments in `compose.yml`.

## Updating

```sh
curl -fsSL -u customer -o compose.yml "https://$GATE/install/latest/compose.yml"
docker compose up -d
```

Use `/install/<version>/compose.yml` (e.g. `/install/0.10.0/compose.yml`) to
pin a version. Versions released after your licence's update period are
refused. Settings → License shows the newest release and whether your
licence includes it.

## Behind Traefik or Dokploy

Keep TLS at the existing proxy and give Caddy loopback ports only:

```sh
HTTP_PORT=127.0.0.1:18088
HTTPS_PORT=127.0.0.1:18443
TRUSTED_PROXIES=private_ranges
```

**Dokploy:**
1. Create a *Compose* service with the provider **Raw** and paste
   `compose.yml` into it.
2. Put the settings above (plus `PUBLIC_APP_URL`) in its Environment tab.
3. Add the registry under *Settings → Registry*, using the gate host and
   your licence key as the password.
4. Add a domain for service `caddy`, port 80, with HTTPS.

To update, paste the new `compose.yml` and redeploy.

Backups, monitoring and troubleshooting: [OPERATIONS.md](OPERATIONS.md).
