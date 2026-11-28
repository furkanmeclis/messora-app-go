# Installing Messora

Messora is now a hosted, multi-tenant service (Messora Cloud). Customer
self-hosting has been discontinued: there are no licence keys, no licence
registry and no customer install file any more. Companies get an account on
the hosted service instead (see [PHASE-17.md](PHASE-17.md)).

The rest of this page is for the people who run the hosted service. The
staging installation on AWS follows it step by step in
[STAGING_AWS.md](STAGING_AWS.md); day-2 work (backups, upgrades,
monitoring) is in [OPERATIONS.md](OPERATIONS.md).

## 1. Host layout

One platform domain (`TENANT_BASE_DOMAIN`, here `messora.tech`) carries
everything; DNS needs only `messora.tech` and `*.messora.tech` pointing at
the server.

| Host | Serves |
|---|---|
| `<slug>.messora.tech` | a company's panel, its public API (`/api/v1`), its API docs (`/docs/api`), its domain-bound webhooks and realtime |
| `admin.messora.tech` | the platform admin panel (Phase 17 WP2) |
| `app.messora.tech`, `www.`, the bare `messora.tech` | the platform address: sign-up and e-mail verification (WP2). The bare domain is served like `app.`; a marketing site can take it over later with one router in front |
| `hooks.messora.tech` | incoming Meta webhooks only: `/meta/<key>` (a company's keyed address) and `/webhooks/*`; everything else is 404 |
| a company's custom domain | the same as its sub-domain, once verified in the platform panel (TXT record, WP2). The company points it at `custom.messora.tech` (CNAME, covered by the wildcard record) or at the server's address (A) |
| `cdn.messora.tech` | public files (logos, template images) from the public bucket via CloudFront (`PUBLIC_ASSET_BASE_URL`), not this stack |

**No `api.` host.** The public API lives on each company's own address
(`https://<slug>.messora.tech/api/v1`, PHASE-17 §2): the tenant is the
host's and the key's, and both must agree. `api.messora.tech` is not
routed and gets no certificate from the on-demand check.

The labels above are reserved slugs (`tenant.ReservedSlugs`), so no company
can take them. `custom` is not reserved yet; it only has to resolve (the
wildcard record does that), but it should join the list with the next
identity migration so no company is called that.

## 2. Edge and TLS

Caddy is always in the stack (routing, header hygiene, the realtime
origin check). `EDGE_MODE` says who terminates TLS:

| | `EDGE_MODE=proxy` (default) | `EDGE_MODE=caddy` |
|---|---|---|
| Where | behind Dokploy's Traefik (staging), a load balancer, or locally | own server, Caddy owns ports 80/443 |
| Caddy listens | plain HTTP `:80` for every host, no host ports with `docker-compose.proxy.yml` | `:80` + `:443` (HTTP/3 too) |
| `messora.tech` + `*.messora.tech` | Traefik: one wildcard certificate from a **DNS-01** resolver (`namedotcom`) | Caddy: one wildcard certificate by **DNS-01** through the name.com module of the edge image |
| Custom domains | Traefik **HTTP provider**: one HTTP-01 router per verified domain, read from `GET /internal/edge/traefik` | Caddy **on-demand TLS**; the `ask` endpoint `GET /internal/tls/allow?domain=` says yes only for platform hosts and verified domains of active companies |
| Forwarded headers trusted from | `TRUSTED_PROXIES` (the proxy network's subnet) | nobody (`TRUSTED_PROXIES` for a TCP load balancer) |

Both internal endpoints are on the API's internal listener (`:9100`),
which the edge never routes. The ask endpoint is rate limited (10/s, burst
50; platform hosts are never limited) and answers 403 for unknown,
unverified and suspended hosts, so DNS pointed at the server by a stranger
cannot make Caddy request certificates.

**Forwarded host.** Caddy sets `X-Forwarded-Host` to the `Host` it
received, always (a value sent by a client is replaced; Traefik in front
passes the browser's `Host` through). The panel (BFF) reads it, so the
company is the one the browser opened, also on Next.js' internal
re-requests; the BFF forwards it to the API in `X-Messora-Tenant-Host`,
which the API believes only together with the BFF secret. Caddy strips both
`X-Messora-*` headers from incoming requests.

**Realtime.** The panel connects to `/connection/websocket` on its own
host. Caddy refuses a WebSocket whose `Origin` is not that same host (403),
for sub-domains and custom domains alike. Centrifugo checks the origin
again against `PUBLIC_APP_URL`, `https://messora.tech` and
`https://*.messora.tech` (`messora init` writes them); for custom domains,
which cannot be listed up front, Caddy drops the already-checked `Origin`
header (Centrifugo accepts a connection without one, and every connection
still needs a token from the API).

The edge image (`deployments/caddy/Dockerfile`) is Caddy 2.11.9 built
with `caddy-dns/namedotcom` v0.1.2 on `libdns/namedotcom` v0.9.0. It runs
as UID 1000; the container may bind 80/443 through
`net.ipv4.ip_unprivileged_port_start=0`. Its configuration
(`deployments/caddy/`, `Caddyfile` → `modes/<EDGE_MODE>.caddy` →
`routes.caddy`) is mounted read-only, so a change needs only a restart:

```sh
docker compose exec caddy caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
```

## 3. Multi-tenant on Dokploy (Traefik in front)

This is how staging runs ([STAGING_AWS.md](STAGING_AWS.md) has the exact
values and API calls).

1. **DNS** (at name.com): `messora.tech` A → server, `*.messora.tech` A →
   server. Nothing else is needed for companies; `custom.messora.tech` is
   covered by the wildcard.
2. **name.com API token** for DNS-01: name.com → Account → Security →
   API token. You get a user name and a token (production API,
   `https://api.name.com`). The token can change every record of the
   account: store it only in the proxy's environment.
3. **Traefik (once per server).** In Dokploy → Settings → Traefik:
   - environment: `NAMECOM_USERNAME=<user>`, `NAMECOM_API_TOKEN=<token>`,
     `NAMECOM_SERVER=https://api.name.com`;
   - `traefik.yml`, add a DNS-01 resolver and the HTTP provider (keep
     Dokploy's existing `letsencrypt` HTTP-01 resolver, the custom domains
     use it):

     ```yaml
     certificatesResolvers:
       letsencrypt-dns:
         acme:
           email: ops@messora.tech
           storage: /etc/dokploy/traefik/dynamic/acme-dns.json
           dnsChallenge:
             provider: namedotcom
             resolvers: ["1.1.1.1:53", "8.8.8.8:53"]
     providers:
       http:
         # PROXY_ROUTER + "-edge": Caddy's alias on dokploy-network.
         endpoint: "http://messora-edge:8081/internal/edge/traefik"
         pollInterval: "30s"
     ```

   - restart Traefik. Its log shows the wildcard certificate being
     obtained once the stack below is up.
4. **Compose application:** Dokploy → project → *Create service* →
   *Compose*, source this repository (branch of the release), **Compose
   path `docker-compose.dokploy.yml`** (it includes `docker-compose.yml`
   and `docker-compose.proxy.yml`). Do not add domains in Dokploy's UI:
   the routers come from the labels of the `caddy` service.
5. **Environment:** paste
   [`deployments/dokploy/messora.env.example`](../deployments/dokploy/messora.env.example)
   and fill every `<…>`: the secrets, `TRUSTED_PROXIES` (the subnet of
   `dokploy-network`: `docker network inspect dokploy-network -f
   '{{range .IPAM.Config}}{{.Subnet}} {{end}}'`), the S3 buckets, the
   Resend API key. `STORAGE_DRIVER=s3` uses the EC2 instance role; see
   [STORAGE.md](STORAGE.md).
6. **Deploy.** `messora-init` writes the secrets (values from the
   environment win), `messora-migrate` applies the schema, then the
   services start; `docker compose ps` shows every one healthy.
7. **First platform admin.** Open the bootstrap link that
   `messora-migrate` logs (`https://admin.messora.tech/bootstrap?token=…`,
   valid 24 h), or in Dokploy's terminal of `messora-api`:

   ```sh
   printf '%s\n' '<password>' | messora admin create --email you@messora.tech --name "You" --password-stdin
   ```

   Two-factor authentication is set up at the first sign-in on
   `admin.messora.tech`. (Until WP2 is merged the branch still uses the
   setup link of tenant #1: `messora setup token`.)
8. **Companies** are opened in the admin panel or by sign-up on
   `app.messora.tech`; each is at `<slug>.messora.tech` at once. A custom
   domain is added and verified in the admin panel; within one provider
   poll (30 s) Traefik routes it and obtains its certificate by HTTP-01.
9. **Meta:** each company enters its own app in Settings → WhatsApp / Meta
   and gets `https://hooks.messora.tech/meta/<key>` as its callback URL
   ([META_SETUP.md](META_SETUP.md)).

`PUBLIC_APP_URL` is the platform address (`https://app.messora.tech`): the
origin of platform e-mails and the first realtime origin. Tenant #1 (the
data from before tenants) answers at `<slug>.<base>` (`default` on a fresh
database) and at the
`PUBLIC_APP_URL` host unless that is a platform host.

## 4. Caddy as the edge (own server)

```dotenv
EDGE_MODE=caddy
TENANT_BASE_DOMAIN=messora.tech
PUBLIC_APP_URL=https://app.messora.tech
NAMECOM_USERNAME=<user>
NAMECOM_API_TOKEN=<token>
# TRUSTED_PROXIES=<load balancer addresses>   only with a proxy in front
```

`docker compose up -d --build` publishes 80/443 (`HTTP_PORT`,
`HTTPS_PORT`). Caddy obtains `messora.tech` + `*.messora.tech` by DNS-01 and
each verified custom domain at its first visit. Certificates live in the
`caddy-data` volume (back it up or accept re-issuance; Let's Encrypt rate
limits allow 50 certificates per registered domain per week).
`EDGE_ON_DEMAND_ISSUER=internal` switches on-demand certificates to Caddy's
local CA, for tests only.

## 5. Published images

The release workflow publishes signed images to GHCR:
`ghcr.io/<owner>/messora`, `messora-web`, `messora-caddy` (the edge) and
`messora-backup`. To run them instead of building:

```sh
MESSORA_IMAGE=ghcr.io/<owner>/messora
MESSORA_WEB_IMAGE=ghcr.io/<owner>/messora-web
MESSORA_CADDY_IMAGE=ghcr.io/<owner>/messora-caddy
MESSORA_BACKUP_IMAGE=ghcr.io/<owner>/messora-backup
MESSORA_VERSION=0.12.0
MESSORA_PULL=missing
MESSORA_INIT_PULL=missing
```

All secrets (database, Redis, encryption keys) are generated on the first
start and kept in a Docker volume unless set in the environment.

## 6. Single domain, locally

Without `TENANT_BASE_DOMAIN` the stack serves one address
(`PUBLIC_APP_URL`, tenant #1). `SITE_ADDRESS=messora.example.com` (with
`EDGE_MODE=proxy`) gets one automatic certificate for it. Locally, a
sub-domain of `localhost` needs no DNS: `TENANT_BASE_DOMAIN=messora.localhost`
puts companies at `http://<slug>.messora.localhost` (Chromium and Firefox
resolve `*.localhost` to the loopback address); `scripts/e2e.sh` runs that
way with `e2e.localhost`.
