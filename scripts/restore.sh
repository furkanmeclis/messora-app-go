#!/usr/bin/env bash
# Restore a backup made by scripts/backup.sh. DESTRUCTIVE: the database,
# secrets and media of the target installation are replaced.
#
#   scripts/restore.sh backups/messora-20261123-101500 --yes
#   COMPOSE_PROJECT=messora-e2e scripts/restore.sh <dir> --yes
#
# Works on the same server or a fresh one: run `docker compose up -d` once
# on the new server first (so volumes exist), then restore.
set -euo pipefail

cd "$(dirname "$0")/.."
PROJECT="${COMPOSE_PROJECT:-messora}"
SRC="${1:-}"
CONFIRM="${2:-}"
HELPER_IMAGE="${BACKUP_HELPER_IMAGE:-alpine:3}"

compose() { docker compose -p "$PROJECT" "$@"; }
fail() { echo "restore: $*" >&2; exit 1; }
volume() {
  docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT" --filter "label=com.docker.compose.volume=$1" | head -n1
}

[ -n "$SRC" ] && [ -d "$SRC" ] || fail "usage: scripts/restore.sh <backup-dir> --yes"
[ "$CONFIRM" = "--yes" ] || fail "this replaces the database, secrets and media of project '$PROJECT'; add --yes to continue"
for f in database.dump secrets.tar.gz SHA256SUMS; do [ -f "$SRC/$f" ] || fail "$SRC/$f is missing"; done
SRC_ABS="$(cd "$SRC" && pwd)"

echo "→ verifying checksums"
(cd "$SRC" && { sha256sum -c --quiet SHA256SUMS 2>/dev/null || shasum -a 256 -c --quiet SHA256SUMS; }) || fail "checksum mismatch"

SECRETS_VOL="$(volume messora-secrets)"
MEDIA_VOL="$(volume messora-media)"
[ -n "$SECRETS_VOL" ] || fail "volumes of project $PROJECT not found; run 'docker compose up -d' once first"

echo "→ stopping the stack"
# Everything stops, including PostgreSQL: containers that mount a subpath of
# the secrets volume must be recreated after it is replaced.
compose stop >/dev/null

echo "→ secrets"
docker run --rm --network none -v "$SECRETS_VOL":/dst -v "$SRC_ABS":/src:ro "$HELPER_IMAGE" \
  sh -c 'find /dst -mindepth 1 -delete && tar -C /dst --numeric-owner -xzpf /src/secrets.tar.gz'

if [ -f "$SRC/media.tar.gz" ] && [ -n "$MEDIA_VOL" ]; then
  echo "→ media"
  docker run --rm --network none -v "$MEDIA_VOL":/dst -v "$SRC_ABS":/src:ro "$HELPER_IMAGE" \
    sh -c 'find /dst -mindepth 1 -delete && tar -C /dst --numeric-owner -xzpf /src/media.tar.gz'
fi

echo "→ database"
compose up -d --wait --force-recreate messora-init postgres >/dev/null
# PostgreSQL keeps the password it was initialised with; align it with the
# restored secrets (local socket connections need no password). Refuse to
# continue if the restored password cannot be read.
compose exec -T postgres sh -c 'pw="$(cat /run/messora-secrets/password)" && [ -n "$pw" ] || { echo "restored database password missing" >&2; exit 1; }
  psql -v ON_ERROR_STOP=1 -q -U "$POSTGRES_USER" -d postgres -c "ALTER ROLE \"$POSTGRES_USER\" PASSWORD '"'"'$pw'"'"'"' >/dev/null
compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -q -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"' >/dev/null
compose exec -T postgres sh -c 'pg_restore --no-owner --exit-on-error -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <"$SRC/database.dump"

echo "→ starting Messora (migrations bring older backups up to date)"
compose up -d --wait --force-recreate

echo "Restored $SRC into project $PROJECT."
