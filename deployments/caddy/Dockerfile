# syntax=docker/dockerfile:1
# Messora edge: Caddy with the name.com DNS provider, for the wildcard
# certificate of *.<TENANT_BASE_DOMAIN> by ACME DNS-01 (EDGE_MODE=caddy).
# The configuration is mounted from deployments/caddy (docker-compose.yml)
# and also baked in, so `caddy validate` works on the image alone.
#
# Versions are pinned: Caddy v2.11.9, caddy-dns/namedotcom v0.1.2 (the
# Caddy module, unchanged since 2021) on libdns/namedotcom v0.9.0 (the
# provider, libdns v1 like Caddy 2.11).

FROM caddy:2.11.9-builder@sha256:cda07fe9d82728c7ac44d844306d435e98131423dc7b114f05a8a8ecfff94707 AS build
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    xcaddy build v2.11.9 \
      --with github.com/caddy-dns/namedotcom@v0.1.2 \
      --with github.com/libdns/namedotcom@v0.9.0 \
      --output /usr/bin/caddy

FROM caddy:2.11.9@sha256:4851e54a46b206517c574821bb5cc67f170d2156041e9fbc4c9193b45e84a75b
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
COPY --from=build /usr/bin/caddy /usr/bin/caddy
COPY Caddyfile routes.caddy /etc/caddy/
COPY modes /etc/caddy/modes
# Fails the build when the module is missing. Certificates and state live
# in /data and /config (named volumes inherit this ownership).
# The builder gives the binary a file capability (cap_net_bind_service);
# with no-new-privileges and every capability dropped such a binary may
# not run at all, so it is removed: the sysctl below is what allows the
# low ports.
RUN setcap -r /usr/bin/caddy \
 && caddy list-modules | grep -qx dns.providers.namedotcom \
 && mkdir -p /data/caddy /config/caddy \
 && chown -R 1000:1000 /data /config
# Non-root: binding ports 80 and 443 is allowed by the container's
# net.ipv4.ip_unprivileged_port_start=0 (docker-compose.yml).
USER 1000:1000
