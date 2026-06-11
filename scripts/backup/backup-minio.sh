#!/usr/bin/env bash
set -eu

target="minio"
backup_root="${BACKUP_DIR:-.runtime/backups}"
textfile_dir="${BACKUP_TEXTFILE_DIR:-.runtime/observability/textfile}"
retention_days="${BACKUP_RETENTION_DAYS:-7}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
out_dir="${backup_root}/minio/${timestamp}"
metrics_file="${textfile_dir}/vkagg_backup_${target}.prom"
tmp_metrics="${metrics_file}.tmp"
started="$(date +%s)"

mkdir -p "${out_dir}" "${textfile_dir}"

write_metrics() {
  result="$1"
  reason="$2"
  size_bytes="${3:-0}"
  finished="$(date +%s)"
  duration="$((finished - started))"
  {
    echo "# HELP vkagg_backup_last_success_timestamp Unix timestamp of the last successful backup by target."
    echo "# TYPE vkagg_backup_last_success_timestamp gauge"
    if [ "${result}" = "success" ]; then
      echo "vkagg_backup_last_success_timestamp{target=\"${target}\"} ${finished}"
    fi
    echo "# HELP vkagg_backup_duration_seconds Backup duration by target and result."
    echo "# TYPE vkagg_backup_duration_seconds gauge"
    echo "vkagg_backup_duration_seconds{target=\"${target}\",result=\"${result}\"} ${duration}"
    echo "# HELP vkagg_backup_size_bytes Backup artifact size in bytes by target."
    echo "# TYPE vkagg_backup_size_bytes gauge"
    echo "vkagg_backup_size_bytes{target=\"${target}\"} ${size_bytes}"
    echo "# HELP vkagg_backup_failures_total Backup failures by target and reason."
    echo "# TYPE vkagg_backup_failures_total counter"
    if [ "${result}" != "success" ]; then
      echo "vkagg_backup_failures_total{target=\"${target}\",reason=\"${reason}\"} 1"
    fi
  } > "${tmp_metrics}"
  mv "${tmp_metrics}" "${metrics_file}"
}

if [ "${BACKUP_MINIO_ENABLED:-true}" != "true" ]; then
  write_metrics "skipped" "disabled" 0
  exit 0
fi

if ! command -v aws >/dev/null 2>&1; then
  write_metrics "error" "aws_cli_missing" 0
  exit 1
fi

if [ -z "${S3_ENDPOINT:-}" ] || [ -z "${S3_BUCKET:-}" ]; then
  write_metrics "error" "missing_s3_config" 0
  exit 1
fi

scheme="http"
if [ "${S3_USE_SSL:-false}" = "true" ]; then
  scheme="https"
fi
endpoint="${scheme}://${S3_ENDPOINT}"

export AWS_ACCESS_KEY_ID="${S3_ACCESS_KEY:-${AWS_ACCESS_KEY_ID:-}}"
export AWS_SECRET_ACCESS_KEY="${S3_SECRET_KEY:-${AWS_SECRET_ACCESS_KEY:-}}"
export AWS_EC2_METADATA_DISABLED=true

if [ -z "${AWS_ACCESS_KEY_ID}" ] || [ -z "${AWS_SECRET_ACCESS_KEY}" ]; then
  write_metrics "error" "missing_s3_credentials" 0
  exit 1
fi

if aws --endpoint-url "${endpoint}" s3 sync "s3://${S3_BUCKET}" "${out_dir}" --only-show-errors; then
  size_bytes="$(find "${out_dir}" -type f -printf '%s\n' | awk '{s+=$1} END {print s+0}')"
  find "${backup_root}/minio" -mindepth 1 -maxdepth 1 -type d -mtime +"${retention_days}" -exec rm -rf {} +
  write_metrics "success" "none" "${size_bytes}"
else
  rm -rf "${out_dir}"
  write_metrics "error" "s3_sync_failed" 0
  exit 1
fi
