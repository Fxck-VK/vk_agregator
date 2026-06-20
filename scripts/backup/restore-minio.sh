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

if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is missing." >&2
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

scheme="http"
if [ "${S3_USE_SSL:-false}" = "true" ]; then
  scheme="https"
fi
endpoint="${scheme}://${S3_ENDPOINT}"

export AWS_ACCESS_KEY_ID="${S3_ACCESS_KEY:-${AWS_ACCESS_KEY_ID:-}}"
export AWS_SECRET_ACCESS_KEY="${S3_SECRET_KEY:-${AWS_SECRET_ACCESS_KEY:-}}"
export AWS_DEFAULT_REGION="${S3_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"
export AWS_EC2_METADATA_DISABLED=true

if [ -z "${AWS_ACCESS_KEY_ID}" ] || [ -z "${AWS_SECRET_ACCESS_KEY}" ]; then
  echo "S3 credentials are required for MinIO/S3 restore." >&2
  exit 1
fi

sync_args=(--endpoint-url "${endpoint}" s3 sync "${restore_dir}" "s3://${S3_BUCKET}" --only-show-errors)
if [ "${RESTORE_MINIO_DELETE:-false}" = "true" ]; then
  sync_args+=(--delete)
fi

echo "Restoring MinIO/S3 bucket ${S3_BUCKET} from ${restore_dir}"
aws "${sync_args[@]}"
echo "MinIO/S3 restore completed. Verify artifact access before opening traffic."
