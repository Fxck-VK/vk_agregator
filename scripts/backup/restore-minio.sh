#!/usr/bin/env bash
set -eu

backup_root="${BACKUP_DIR:-.runtime/backups}"
restore_dir="${RESTORE_MINIO_DIR:-}"

require_restore_confirmation() {
  if [ "${RESTORE_ALLOW_DESTRUCTIVE:-false}" != "true" ]; then
    echo "RESTORE_ALLOW_DESTRUCTIVE=true is required for MinIO/S3 restore." >&2
    exit 1
  fi
  if [ "${RESTORE_CONFIRM:-}" != "I_UNDERSTAND_RESTORE_OVERWRITES_DATA" ]; then
    echo "RESTORE_CONFIRM=I_UNDERSTAND_RESTORE_OVERWRITES_DATA is required for MinIO/S3 restore." >&2
    exit 1
  fi
}

require_restore_confirmation

if ! command -v objectsync >/dev/null 2>&1; then
  echo "Object sync tool is missing." >&2
  exit 1
fi

if [ -z "${S3_ENDPOINT:-}" ] || [ -z "${S3_BUCKET:-}" ]; then
  echo "S3_ENDPOINT and S3_BUCKET are required for MinIO/S3 restore." >&2
  exit 1
fi

if [ -z "${restore_dir}" ]; then
  echo "RESTORE_MINIO_DIR is required. Use an absolute path or a directory under ${backup_root}/minio." >&2
  exit 1
fi

if [ ! -d "${restore_dir}" ] && [ -d "${backup_root}/${restore_dir}" ]; then
  restore_dir="${backup_root}/${restore_dir}"
fi

if [ ! -d "${restore_dir}" ] && [ -d "${backup_root}/minio/${restore_dir}" ]; then
  restore_dir="${backup_root}/minio/${restore_dir}"
fi

if [ ! -d "${restore_dir}" ]; then
  echo "MinIO/S3 restore directory not found: ${restore_dir}" >&2
  exit 1
fi

access_key="${S3_ACCESS_KEY:-${AWS_ACCESS_KEY_ID:-}}"
secret_key="${S3_SECRET_KEY:-${AWS_SECRET_ACCESS_KEY:-}}"

if [ -z "${access_key}" ] || [ -z "${secret_key}" ]; then
  echo "S3 credentials are required for MinIO/S3 restore." >&2
  exit 1
fi

echo "Restoring the configured MinIO/S3 bucket from ${restore_dir}"
if [ "${RESTORE_MINIO_DELETE:-false}" = "true" ]; then
  objectsync restore --directory "${restore_dir}" --delete
else
  objectsync restore --directory "${restore_dir}"
fi
echo "MinIO/S3 restore completed. Verify artifact access before opening traffic."
