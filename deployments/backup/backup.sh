#!/bin/sh
# Messora database backup to S3.
#
#   messora-backup run     every day at BACKUP_AT (UTC, HH:MM), forever
#   messora-backup once    one backup now (docker compose run --rm backup once)
#   messora-backup health  healthcheck: the last backup is younger than 26 h
#                          (or the container started less than 26 h ago)
#
# Each backup: pg_dump --format=custom into the work volume, `pg_restore
# --list` to prove the archive is readable, upload to
# s3://BACKUP_S3_BUCKET/BACKUP_S3_PREFIX/messora-<UTC stamp>.dump with
# server-side encryption, then the local file is removed. Old backups are
# expired by an S3 lifecycle rule on the prefix (docs/OPERATIONS.md).
#
# Not in the dump, on purpose: the secrets volume (APP_ENCRYPTION_KEY
# decrypts the stored credentials; keep it apart from the dumps) and Redis
# (queues and caches only).
set -eu

WORK=/work
log() { printf '{"time":"%s","service":"messora-backup","msg":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "error: $*"; return 1; }

setup() {
  : "${BACKUP_S3_BUCKET:?set BACKUP_S3_BUCKET}"
  BACKUP_S3_PREFIX="${BACKUP_S3_PREFIX:-ops/backups/postgres}"
  BACKUP_S3_PREFIX="${BACKUP_S3_PREFIX%/}"
  # Empty variables would hide the instance role from the AWS CLI.
  [ -n "${AWS_ACCESS_KEY_ID:-}" ] || unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
  [ -n "${BACKUP_S3_ENDPOINT:-}" ] && ENDPOINT="--endpoint-url $BACKUP_S3_ENDPOINT" || ENDPOINT=""
  case "${BACKUP_S3_SSE:-AES256}" in
    none|"") SSE="" ;;
    *) SSE="--sse ${BACKUP_S3_SSE:-AES256}" ;;
  esac
  if [ -z "${PGPASSWORD:-}" ] && [ -n "${PGPASSFILE_SOURCE:-}" ]; then
    PGPASSWORD="$(cat "$PGPASSFILE_SOURCE")"
    export PGPASSWORD
  fi
}

backup() {
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  file="$WORK/messora-$stamp.dump"
  key="$BACKUP_S3_PREFIX/messora-$stamp.dump"
  rm -f "$WORK"/messora-*.dump
  log "dump $PGDATABASE"
  pg_dump --format=custom --no-owner --file="$file" || { rm -f "$file"; fail "pg_dump failed"; return 1; }
  pg_restore --list "$file" >/dev/null || { rm -f "$file"; fail "the dump is not a readable archive"; return 1; }
  size="$(wc -c <"$file" | tr -d ' ')"
  sum="$(sha256sum "$file" | cut -d' ' -f1)"
  log "upload s3://$BACKUP_S3_BUCKET/$key ($size bytes, sha256 $sum)"
  # shellcheck disable=SC2086 # word splitting of the optional flags is intended
  aws $ENDPOINT s3 cp --only-show-errors $SSE --metadata "sha256=$sum" "$file" "s3://$BACKUP_S3_BUCKET/$key" \
    || { rm -f "$file"; fail "upload failed"; return 1; }
  rm -f "$file"
  date -u +%s >"$WORK/last-success"
  echo "$key" >"$WORK/last-key"
  log "backup done: $key"
}

case "${1:-run}" in
  once)
    setup
    backup
    ;;
  run)
    setup
    at="${BACKUP_AT:-02:30}"
    case "$at" in [0-2][0-9]:[0-5][0-9]) ;; *) log "BACKUP_AT must be HH:MM (UTC), got $at"; exit 2 ;; esac
    date -u +%s >"$WORK/started"
    log "scheduled daily at $at UTC to s3://$BACKUP_S3_BUCKET/$BACKUP_S3_PREFIX/"
    if [ "${BACKUP_RUN_ON_START:-false}" = true ]; then backup || true; fi
    last_day=""
    while :; do
      now="$(date -u +%H:%M)"
      today="$(date -u +%Y%m%d)"
      if [ "$now" = "$at" ] && [ "$today" != "$last_day" ]; then
        last_day="$today"
        # A failed night is retried the next day; the healthcheck turns
        # the container unhealthy after 26 hours without a success.
        backup || true
      fi
      sleep 20
    done
    ;;
  health)
    now="$(date -u +%s)"
    limit=$((26 * 3600))
    for f in last-success started; do
      if [ -f "$WORK/$f" ] && [ $((now - $(cat "$WORK/$f"))) -lt "$limit" ]; then exit 0; fi
    done
    exit 1
    ;;
  *)
    echo "usage: messora-backup run|once|health" >&2
    exit 2
    ;;
esac
