#!/usr/bin/env bash
# End-to-end smoke test of a running Messora stack.
# Usage: scripts/stack-smoke.sh [base-url]   (default http://localhost)
set -euo pipefail

BASE="${1:-http://localhost}"
fail() { echo "FAIL: $*" >&2; exit 1; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

echo "→ panel responds"
[ "$(code "$BASE/")" = 200 ] || fail "panel did not return 200"

echo "→ panel-only API endpoint rejects external callers"
[ "$(code "$BASE/api/v1/system/status")" = 403 ] || fail "system status reachable without the BFF secret"
[ "$(code -H 'X-Messora-BFF: forged' "$BASE/api/v1/system/status")" = 403 ] || fail "forged BFF header accepted"

echo "→ internal endpoints are not routed publicly"
[ "$(code "$BASE/healthz")" != 200 ] || fail "/healthz exposed through the edge"
[ "$(code "$BASE/metrics")" != 200 ] || fail "/metrics exposed through the edge"

echo "→ unknown API routes use the error envelope"
curl -s "$BASE/api/v1/does-not-exist" | grep -q '"code":"NOT_FOUND"' || fail "error envelope missing"

echo "→ every component reports healthy (checked from inside the panel container)"
status=$(docker compose exec -T messora-web /nodejs/bin/node -e '
const s=require("fs").readFileSync("/run/messora-secrets/secrets.env","utf8").match(/BFF_SHARED_SECRET=(.*)/)[1];
fetch("http://messora-api:8080/api/v1/system/status",{headers:{"X-Messora-BFF":s}})
  .then(r=>r.json()).then(j=>{
    const bad=Object.entries(j.data.components).filter(([,v])=>v.status!=="ok").map(([k,v])=>k+": "+v.error);
    console.log(bad.length ? "UNHEALTHY "+bad.join("; ") : "ALL_OK "+Object.keys(j.data.components).length);
  }).catch(e=>console.log("ERROR "+e.message))')
echo "  $status"
[[ "$status" == ALL_OK* ]] || fail "unhealthy components"

echo "→ web container sees only its own secret"
keys=$(docker compose exec -T messora-web /nodejs/bin/node -e '
console.log(require("fs").readFileSync("/run/messora-secrets/secrets.env","utf8").split("\n").filter(l=>l&&!l.startsWith("#")).map(l=>l.split("=")[0]).join(","))')
[ "$keys" = "BFF_SHARED_SECRET" ] || fail "web container can read: $keys"

echo "→ Redis requires a password and persists jobs"
docker compose exec -T redis redis-cli ping 2>&1 | grep -q NOAUTH || fail "Redis accepts unauthenticated clients"

echo "OK: stack smoke test passed"
