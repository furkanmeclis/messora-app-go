# Installing Messora

Messora is now a hosted, multi-tenant service (Messora Cloud). Customer
self-hosting has been discontinued: there are no licence keys, no licence
registry and no customer install file any more. Companies get an account on
the hosted service instead (see [PHASE-17.md](PHASE-17.md)).

The rest of this page is for the people who run the hosted service.

## Running the stack

The images are published to GHCR by the release workflow
(`ghcr.io/<owner>/messora` and `ghcr.io/<owner>/messora-web`). Run them with
the repository's `docker-compose.yml`:

```sh
MESSORA_IMAGE=ghcr.io/<owner>/messora
MESSORA_WEB_IMAGE=ghcr.io/<owner>/messora-web
MESSORA_VERSION=0.12.0
MESSORA_PULL=missing
MESSORA_INIT_PULL=missing
PUBLIC_APP_URL=https://messora.example.com
SITE_ADDRESS=messora.example.com
```

All secrets (database, Redis, encryption keys) are generated on the first
start and kept in a Docker volume; `.env` only holds what you override.

## Behind Traefik or Dokploy

Keep TLS at the existing proxy and give Caddy loopback ports only:

```sh
HTTP_PORT=127.0.0.1:18088
HTTPS_PORT=127.0.0.1:18443
TRUSTED_PROXIES=private_ranges
```

**Dokploy:** create a *Compose* service from this repository, run
`docker compose up -d --no-build` with the settings above (plus
`PUBLIC_APP_URL`) in its Environment tab, and add a domain for service
`caddy`, port 80, with HTTPS. To keep files in S3 instead of the volume,
add the `STORAGE_DRIVER=s3` and `S3_*` settings of
[STORAGE.md](STORAGE.md) to the same Environment tab.

Backups, monitoring and troubleshooting: [OPERATIONS.md](OPERATIONS.md).
