#!/usr/bin/env bash
# Backup → destroy → fresh install → restore rehearsal on a throw-away
# Compose project. Proves that a backup alone brings back the data, the
# media and the encryption key.
#
#   scripts/backup-restore-test.sh        # port 8095
set -euo pipefail

cd "$(dirname "$0")/.."
PORT="${BR_PORT:-8095}"
export COMPOSE_PROJECT="${BR_PROJECT:-messora-br}"
export HTTP_PORT="$PORT" HTTPS_PORT="$((PORT + 363))" PUBLIC_APP_URL="http://localhost:$PORT" SITE_ADDRESS=":80"
compose() { docker compose -p "$COMPOSE_PROJECT" "$@"; }
fail() { echo "rehearsal: FAILED: $*" >&2; exit 1; }
WORK="$(mktemp -d "${TMPDIR:-/tmp}/messora-br.XXXXXX")"

cleanup() {
  if [ "${KEEP_STACK:-0}" != 1 ]; then compose down -v --remove-orphans >/dev/null 2>&1 || true; fi
  [ -d "$WORK" ] && find "$WORK" -mindepth 1 -delete && rmdir "$WORK"
}
trap cleanup EXIT

[ -f .env ] || cp .env.example .env
psql_() { compose exec -T postgres sh -c "psql -tA -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"$1\""; }
sum256() { if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi | cut -d' ' -f1; }
vol() { docker volume ls -q --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" --filter "label=com.docker.compose.volume=$1" | head -n1; }
media_vol() { docker volume ls -q --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" --filter "label=com.docker.compose.volume=messora-media" | head -n1; }

echo "== 1. install and write marker data"
compose down -v --remove-orphans >/dev/null 2>&1 || true
compose up -d --build --wait --wait-timeout 300
psql_ "INSERT INTO settings (tenant_id, key, value) VALUES ('00000000-0000-0000-0000-000000000001', 'restore_rehearsal', to_jsonb('before-backup'::text))" >/dev/null
docker run --rm -v "$(media_vol)":/m alpine:3 sh -c 'mkdir -p /m/rehearsal && echo marker > /m/rehearsal/file.txt'
SUM_BEFORE="$(docker run --rm -v "$(vol messora-secrets)":/s:ro alpine:3 cat /s/.state/secrets.env | sum256)"

echo "== 2. back up"
scripts/backup.sh "$WORK"
BACKUP="$(find "$WORK" -mindepth 1 -maxdepth 1 -type d -name 'messora-*' | head -n1)"
[ -n "$BACKUP" ] || fail "no backup directory"

echo "== 3. destroy everything and install fresh (new secrets, empty database)"
compose down -v --remove-orphans >/dev/null
compose up -d --wait --wait-timeout 300
[ "$(psql_ "SELECT count(*) FROM settings WHERE key = 'restore_rehearsal'")" = "0" ] || fail "fresh install is not empty"

echo "== 4. restore"
scripts/restore.sh "$BACKUP" --yes

echo "== 5. verify"
[ "$(psql_ "SELECT value #>> '{}' FROM settings WHERE key = 'restore_rehearsal'")" = "before-backup" ] || fail "database marker missing"
[ "$(docker run --rm -v "$(media_vol)":/m:ro alpine:3 cat /m/rehearsal/file.txt)" = "marker" ] || fail "media file missing"
SUM_AFTER="$(docker run --rm -v "$(vol messora-secrets)":/s:ro alpine:3 cat /s/.state/secrets.env | sum256)"
[ "$SUM_BEFORE" = "$SUM_AFTER" ] || fail "secrets (encryption key) differ after restore"
compose exec -T messora-api /usr/local/bin/messora healthcheck http://127.0.0.1:9100/status >/dev/null || fail "stack unhealthy after restore"
# No owner exists in the rehearsal, so the panel answers with its setup
# page or a redirect to it; anything but an error means it is up.
code=000
for _ in $(seq 1 15); do
  code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$PORT/login" || true)"
  case "$code" in 200|30[0-9]) break ;; esac
  sleep 2
done
case "$code" in 200|30[0-9]) ;; *) fail "panel not reachable (HTTP $code)" ;; esac

echo "rehearsal: OK — database, media and secrets restored; stack healthy"
