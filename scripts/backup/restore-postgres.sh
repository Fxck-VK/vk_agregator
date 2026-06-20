#!/usr/bin/env bash
set -eu

backup_root="${BACKUP_DIR:-.runtime/backups}"
restore_file="${RESTORE_POSTGRES_FILE:-}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
pre_restore_file="${backup_root}/postgres/pre-restore-${timestamp}.dump"

require_restore_confirmation() {
  if [ "${RESTORE_ALLOW_DESTRUCTIVE:-false}" != "true" ]; then
    echo "RESTORE_ALLOW_DESTRUCTIVE=true is required for Postgres restore." >&2
    exit 1
  fi
  if [ "${RESTORE_CONFIRM:-}" != "I_UNDERSTAND_RESTORE_OVERWRITES_DATA" ]; then
    echo "RESTORE_CONFIRM=I_UNDERSTAND_RESTORE_OVERWRITES_DATA is required for Postgres restore." >&2
    exit 1
  fi
}

require_restore_confirmation

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required for Postgres restore." >&2
  exit 1
fi

if [ -z "${restore_file}" ]; then
  echo "RESTORE_POSTGRES_FILE is required. Use an absolute path or a file under ${backup_root}/postgres." >&2
  exit 1
fi

if [ ! -f "${restore_file}" ] && [ -f "${backup_root}/${restore_file}" ]; then
  restore_file="${backup_root}/${restore_file}"
fi

if [ ! -f "${restore_file}" ] && [ -f "${backup_root}/postgres/${restore_file}" ]; then
  restore_file="${backup_root}/postgres/${restore_file}"
fi

if [ ! -f "${restore_file}" ]; then
  echo "Postgres restore file not found: ${restore_file}" >&2
  exit 1
fi

if ! command -v pg_restore >/dev/null 2>&1; then
  echo "pg_restore is missing." >&2
  exit 1
fi

if ! command -v pg_dump >/dev/null 2>&1; then
  echo "pg_dump is missing." >&2
  exit 1
fi

mkdir -p "${backup_root}/postgres"

if [ "${RESTORE_SKIP_PRE_BACKUP:-false}" != "true" ]; then
  echo "Creating pre-restore Postgres backup: ${pre_restore_file}"
  pg_dump --dbname="${DATABASE_URL}" --format=custom --file="${pre_restore_file}" >/dev/null
fi

echo "Restoring Postgres from ${restore_file}"
pg_restore \
  --dbname="${DATABASE_URL}" \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges \
  "${restore_file}"

echo "Postgres restore completed. Run migrate status and smoke checks before opening traffic."
