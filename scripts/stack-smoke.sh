#!/usr/bin/env bash
# End-to-end smoke test of a running Messora stack.
# Usage: scripts/stack-smoke.sh [base-url]   (default http://localhost)
set -euo pipefail

BASE="${1:-http://localhost}"
fail() { echo "FAIL: $*" >&2; exit 1; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

echo "→ panel responds (anonymous visitors land on setup or sign-in)"
final=$(curl -s -o /dev/null -L -w '%{http_code} %{url_effective}' "$BASE/")
case "$final" in
  "200 "*/login*|"200 "*/setup*) ;;
  *) fail "unexpected landing page: $final" ;;
esac

echo "→ API rejects callers without credentials"
[ "$(code "$BASE/api/v1/system/status")" = 401 ] || fail "system status reachable anonymously"
[ "$(code -X POST -H 'Content-Type: application/json' -d '{"email":"a@b.co","password":"x"}' "$BASE/api/v1/auth/login")" = 403 ] \
  || fail "panel-only login endpoint reachable from outside"
[ "$(code -X POST -H 'X-Messora-BFF: forged' -H 'Content-Type: application/json' -d '{"email":"a@b.co","password":"x"}' "$BASE/api/v1/auth/login")" = 403 ] \
  || fail "forged BFF header accepted"

echo "→ WhatsApp webhooks are public but signed"
[ "$(code -X POST -H 'Content-Type: application/json' -d '{"object":"whatsapp_business_account","entry":[]}' "$BASE/webhooks/whatsapp")" = 401 ] \
  || fail "unsigned webhook accepted"
[ "$(code "$BASE/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=1")" = 403 ] \
  || fail "webhook verification accepted a wrong token"

echo "→ internal endpoints are not routed publicly"
[ "$(code "$BASE/healthz")" != 200 ] || fail "/healthz exposed through the edge"
[ "$(code "$BASE/metrics")" != 200 ] || fail "/metrics exposed through the edge"

echo "→ unknown API routes use the error envelope"
curl -s "$BASE/api/v1/does-not-exist" | grep -q '"code":"NOT_FOUND"' || fail "error envelope missing"

echo "→ security headers on the panel"
headers=$(curl -s -D - -o /dev/null -L "$BASE/login")
echo "$headers" | grep -qi "content-security-policy: .*nonce-" || fail "CSP with nonce missing"
echo "$headers" | grep -qi "x-frame-options: SAMEORIGIN" || fail "X-Frame-Options missing"

echo "→ every component reports healthy (internal status endpoint)"
docker compose exec -T messora-api /usr/local/bin/messora healthcheck http://127.0.0.1:9100/status \
  || { docker compose exec -T messora-api /usr/local/bin/messora healthcheck http://127.0.0.1:9100/readyz; fail "unhealthy components"; }

echo "→ web container sees only its own secret"
keys=$(docker compose exec -T messora-web /nodejs/bin/node -e '
console.log(require("fs").readFileSync("/run/messora-secrets/secrets.env","utf8").split("\n").filter(l=>l&&!l.startsWith("#")).map(l=>l.split("=")[0]).join(","))')
[ "$keys" = "BFF_SHARED_SECRET" ] || fail "web container can read: $keys"

echo "→ Redis requires a password and persists jobs"
docker compose exec -T redis redis-cli ping 2>&1 | grep -q NOAUTH || fail "Redis accepts unauthenticated clients"

echo "OK: stack smoke test passed"
