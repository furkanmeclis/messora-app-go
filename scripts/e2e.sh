#!/usr/bin/env bash
# Browser end-to-end tests on a throw-away stack.
#
# Starts a separate Compose project (so a local demo installation is never
# touched) with a mail catcher (docker-compose.e2e.yml), creates the platform
# admin, runs Playwright and tears the stack down again (keep it with
# KEEP_STACK=1).
#
# Host routing: tenant #1 answers at http://localhost:$PORT, the admin panel
# at http://admin.e2e.localhost:$PORT and new workspaces at
# http://<slug>.e2e.localhost:$PORT (Chromium resolves *.localhost to the
# loopback address; Caddy, EDGE_MODE=proxy, serves every host). Tenant #1
# is also at http://default.e2e.localhost:$PORT, and a second company
# ("beta") at http://beta.e2e.localhost:$PORT for tenant-hosts.spec.ts.
#
#   scripts/e2e.sh            # port 8090
#   E2E_PORT=9000 scripts/e2e.sh
set -euo pipefail

cd "$(dirname "$0")/.."
PORT="${E2E_PORT:-8090}"
PROJECT="${E2E_PROJECT:-messora-e2e}"
export HTTP_PORT="$PORT" HTTPS_PORT="$((PORT + 363))" PUBLIC_APP_URL="http://localhost:$PORT" SITE_ADDRESS=":80"
export TENANT_BASE_DOMAIN="${E2E_BASE_DOMAIN:-e2e.localhost}" MAILPIT_HTTP_PORT="${E2E_MAILPIT_PORT:-$((PORT + 7))}" EDGE_MODE=proxy
ADMIN_EMAIL="pat@example.com"
ADMIN_PASSWORD="Platform-keeper-$PROJECT-x" # gitleaks:allow (throw-away test stack)
# Separate image tags per project, so parallel runs never test each other's build.
export MESSORA_VERSION="${MESSORA_VERSION:-$PROJECT}"
# The AI specs use the scriptable fake provider, which only runs outside
# production (see docs/AI.md).
export APP_ENV="${E2E_APP_ENV:-development}" AI_FAKE_PROVIDER=true
# docker-compose.e2e.yml adds the fake Meta Graph API (Embedded Signup specs).
compose() { docker compose -p "$PROJECT" -f docker-compose.yml -f docker-compose.e2e.yml "$@"; }

cleanup() {
  if [ "${KEEP_STACK:-0}" != 1 ]; then compose down -v --remove-orphans >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

[ -f .env ] || cp .env.example .env
compose down -v --remove-orphans >/dev/null 2>&1 || true
compose up -d --build --wait --wait-timeout 300

printf '%s\n' "$ADMIN_PASSWORD" |
  compose exec -T messora-api /usr/local/bin/messora admin create --email "$ADMIN_EMAIL" --name "Pat Platform" --password-stdin
echo "platform admin created"

# The second company of tenant-hosts.spec.ts, as a fixed fixture written
# straight into the platform tables (as the owner): it needs no people, only
# an address. Opening companies through the admin panel and sign-up is
# covered by admin.spec.ts and signup.spec.ts.
# shellcheck disable=SC2016 # expanded inside the container
compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -q -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<SQL
INSERT INTO tenants (slug, name) VALUES ('beta', 'Beta E2E') ON CONFLICT (slug) DO NOTHING;
INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary)
SELECT id, 'beta.$TENANT_BASE_DOMAIN', 'subdomain', now(), true FROM tenants WHERE slug = 'beta'
ON CONFLICT (hostname) DO NOTHING;
SQL
echo "second company opened at beta.$TENANT_BASE_DOMAIN"

cd frontend
export E2E_BASE_URL="http://localhost:$PORT" E2E_PROJECT="$PROJECT" E2E_BASE_DOMAIN="$TENANT_BASE_DOMAIN" \
  E2E_ADMIN_URL="http://admin.$TENANT_BASE_DOMAIN:$PORT" E2E_PLATFORM_URL="http://app.$TENANT_BASE_DOMAIN:$PORT" \
  E2E_MAILPIT_URL="http://127.0.0.1:$MAILPIT_HTTP_PORT" E2E_ADMIN_EMAIL="$ADMIN_EMAIL" E2E_ADMIN_PASSWORD="$ADMIN_PASSWORD"
npx playwright test "$@"
