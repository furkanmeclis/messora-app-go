#!/usr/bin/env bash
# Browser end-to-end tests on a throw-away stack.
#
# Starts a separate Compose project (so a local demo installation is never
# touched), obtains the one-time setup link, runs Playwright and tears the
# stack down again (keep it with KEEP_STACK=1).
#
#   scripts/e2e.sh            # port 8090
#   E2E_PORT=9000 scripts/e2e.sh
set -euo pipefail

cd "$(dirname "$0")/.."
PORT="${E2E_PORT:-8090}"
PROJECT="${E2E_PROJECT:-messora-e2e}"
export HTTP_PORT="$PORT" HTTPS_PORT="$((PORT + 363))" PUBLIC_APP_URL="http://localhost:$PORT" SITE_ADDRESS=":80"
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

cd frontend
E2E_BASE_URL="http://localhost:$PORT" E2E_SETUP_URL="$SETUP_URL" npx playwright test "$@"
