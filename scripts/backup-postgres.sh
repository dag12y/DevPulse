#!/usr/bin/env bash
# Nightly Postgres backup for the DevPulse VPS deployment.
#
# Dumps the database from the running compose stack, compresses it, keeps
# the newest KEEP backups locally, and optionally uploads to Azure Blob
# storage when AZCOPY_DEST (a container SAS URL) is set.
#
# Usage (from the repository root on the VPS):
#   ./scripts/backup-postgres.sh
#
# Cron (daily 02:00, appends to a log):
#   0 2 * * * cd /opt/DevPulse && ./scripts/backup-postgres.sh >> /var/log/devpulse-backup.log 2>&1
#
# Restore is a deliberate manual procedure; see docs/production.md.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT/.env.prod}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT/backups}"
KEEP="${BACKUP_KEEP:-14}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "error: env file not found: $ENV_FILE (set ENV_FILE to override)" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
source "$ENV_FILE"
set +a

: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"

mkdir -p "$BACKUP_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
FILE="$BACKUP_DIR/devpulse-$STAMP.sql.gz"

echo "dumping $POSTGRES_DB to $FILE"
docker exec devpulse-postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  | gzip > "$FILE.tmp"
mv "$FILE.tmp" "$FILE"

echo "rotating local backups (keep $KEEP)"
ls -t "$BACKUP_DIR"/devpulse-*.sql.gz | tail -n +"$((KEEP + 1))" | xargs -r rm -f

if [[ -n "${AZCOPY_DEST:-}" ]]; then
  if ! command -v azcopy >/dev/null 2>&1; then
    echo "warning: AZCOPY_DEST is set but azcopy is not installed; skipping upload" >&2
  else
    echo "uploading $FILE to blob storage"
    azcopy copy "$FILE" "$AZCOPY_DEST" --overwrite=false >/dev/null
  fi
fi

echo "backup complete: $FILE ($(du -h "$FILE" | cut -f1))"
