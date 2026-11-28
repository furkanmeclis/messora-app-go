#!/usr/bin/env bash
# Browser end-to-end tests on a throw-away stack.
#
# Starts a separate Compose project (so a local demo installation is never
# touched), obtains the one-time setup link, runs Playwright and tears the
# stack down again (keep it with KEEP_STACK=1).
#
# Host routing: tenant #1 answers at http://localhost:$PORT and at
# http://default.e2e.localhost:$PORT; a second company ("beta") is opened
# at http://beta.e2e.localhost:$PORT for the cross-host isolation spec
# (tenant-hosts.spec.ts). Chromium resolves *.localhost to the loopback
# address; Caddy (EDGE_MODE=proxy) serves every host.
#
#   scripts/e2e.sh            # port 8090
#   E2E_PORT=9000 scripts/e2e.sh
set -euo pipefail

cd "$(dirname "$0")/.."
PORT="${E2E_PORT:-8090}"
PROJECT="${E2E_PROJECT:-messora-e2e}"
export HTTP_PORT="$PORT" HTTPS_PORT="$((PORT + 363))" PUBLIC_APP_URL="http://localhost:$PORT" SITE_ADDRESS=":80"
export TENANT_BASE_DOMAIN="${E2E_BASE_DOMAIN:-e2e.localhost}" EDGE_MODE=proxy
# Separate image tags per project, so parallel runs never test each other's build.
export MESSORA_VERSION="${MESSORA_VERSION:-$PROJECT}"
# The AI specs use the scriptable fake provider, which only runs outside
# production (see docs/AI.md).
export APP_ENV="${E2E_APP_ENV:-development}" AI_FAKE_PROVIDER=true
compose() { docker compose -p "$PROJECT" "$@"; }

cleanup() {
  if [ "${KEEP_STACK:-0}" != 1 ]; then compose down -v --remove-orphans >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

[ -f .env ] || cp .env.example .env
compose down -v --remove-orphans >/dev/null 2>&1 || true
compose up -d --build --wait --wait-timeout 300

SETUP_URL=$(compose exec -T messora-api /usr/local/bin/messora setup token | grep -o 'http[^ ]*setup?token=[^ ]*')
echo "setup link obtained"

# The second company. Opening companies belongs to the platform admin panel
# (Phase 17 WP2); until that is on this branch the spec's company is written
# straight into the platform tables, as the owner.
# shellcheck disable=SC2016 # expanded inside the container
compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -q -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<SQL
INSERT INTO tenants (slug, name) VALUES ('beta', 'Beta E2E') ON CONFLICT (slug) DO NOTHING;
INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary)
SELECT id, 'beta.$TENANT_BASE_DOMAIN', 'subdomain', now(), true FROM tenants WHERE slug = 'beta'
ON CONFLICT (hostname) DO NOTHING;
SQL
echo "second company opened at beta.$TENANT_BASE_DOMAIN"

cd frontend
E2E_BASE_URL="http://localhost:$PORT" E2E_SETUP_URL="$SETUP_URL" E2E_PROJECT="$PROJECT" E2E_BASE_DOMAIN="$TENANT_BASE_DOMAIN" \
  npx playwright test "$@"
