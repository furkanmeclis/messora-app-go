#!/usr/bin/env bash
# Back up a Messora installation: database, secrets and media.
#
#   scripts/backup.sh                 # into ./backups/messora-<timestamp>
#   scripts/backup.sh /srv/backups    # into /srv/backups/messora-<timestamp>
#   COMPOSE_PROJECT=messora-e2e scripts/backup.sh
#
# The database dump is taken while the stack runs (pg_dump is consistent).
# The secrets archive contains APP_ENCRYPTION_KEY: without it encrypted data
# (WhatsApp tokens, TOTP secrets, webhook secrets) cannot be read, and with
# it anyone can. Store backups encrypted and off the server.
#
# Redis is not backed up: it only holds queues and caches. Work that was
# queued at backup time is recreated from PostgreSQL by the reapers.
set -euo pipefail

cd "$(dirname "$0")/.."
PROJECT="${COMPOSE_PROJECT:-messora}"
DEST_ROOT="${1:-backups}"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
DEST="$DEST_ROOT/messora-$STAMP"
HELPER_IMAGE="${BACKUP_HELPER_IMAGE:-alpine:3}"

compose() { docker compose -p "$PROJECT" "$@"; }
fail() { echo "backup: $*" >&2; exit 1; }

volume() { # volume name of a Compose volume in this project
  docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT" --filter "label=com.docker.compose.volume=$1" | head -n1
}

compose ps --status running --services | grep -qx postgres || fail "postgres is not running in project $PROJECT"
SECRETS_VOL="$(volume messora-secrets)"
MEDIA_VOL="$(volume messora-media)"
[ -n "$SECRETS_VOL" ] || fail "secrets volume not found"

mkdir -p "$DEST"
chmod 700 "$DEST"

echo "→ database"
compose exec -T postgres sh -c 'pg_dump --format=custom --no-owner -U "$POSTGRES_USER" -d "$POSTGRES_DB"' >"$DEST/database.dump"
[ -s "$DEST/database.dump" ] || fail "empty database dump"

echo "→ secrets"
# Archives stream to stdout so the files belong to the user running the
# backup (on Linux a container would create them as root).
docker run --rm --network none -v "$SECRETS_VOL":/src:ro "$HELPER_IMAGE" \
  tar -C /src --numeric-owner -czf - . >"$DEST/secrets.tar.gz"

if [ -n "$MEDIA_VOL" ]; then
  echo "→ media"
  docker run --rm --network none -v "$MEDIA_VOL":/src:ro "$HELPER_IMAGE" \
    tar -C /src --numeric-owner -czf - . >"$DEST/media.tar.gz"
fi

VERSION="$(compose exec -T messora-api /usr/local/bin/messora version 2>/dev/null || echo unknown)"
SCHEMA="$(compose exec -T postgres sh -c 'psql -tA -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT max(version_id) FROM goose_db_version WHERE is_applied"' 2>/dev/null || echo unknown)"
{
  echo "created_at=$STAMP"
  echo "project=$PROJECT"
  echo "messora_version=$VERSION"
  echo "schema_version=$SCHEMA"
} >"$DEST/manifest.txt"
(cd "$DEST" && sha256sum ./* >SHA256SUMS 2>/dev/null || shasum -a 256 ./* >SHA256SUMS)
chmod 600 "$DEST"/*

echo "Backup written to $DEST"
ls -lh "$DEST"
