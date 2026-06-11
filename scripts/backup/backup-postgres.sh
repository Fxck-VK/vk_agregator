#!/usr/bin/env bash
set -eu

target="postgres"
backup_root="${BACKUP_DIR:-.runtime/backups}"
textfile_dir="${BACKUP_TEXTFILE_DIR:-.runtime/observability/textfile}"
retention_days="${BACKUP_RETENTION_DAYS:-7}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
out_dir="${backup_root}/postgres"
out_file="${out_dir}/postgres-${timestamp}.dump"
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

if [ "${BACKUP_POSTGRES_ENABLED:-true}" != "true" ]; then
  write_metrics "skipped" "disabled" 0
  exit 0
fi

if [ -z "${DATABASE_URL:-}" ]; then
  write_metrics "error" "missing_database_url" 0
  exit 1
fi

if ! command -v pg_dump >/dev/null 2>&1; then
  write_metrics "error" "pg_dump_missing" 0
  exit 1
fi

if PGDATABASE="${DATABASE_URL}" pg_dump --format=custom --file="${out_file}" >/dev/null; then
  size_bytes="$(wc -c < "${out_file}" | tr -d ' ')"
  find "${out_dir}" -type f -name 'postgres-*.dump' -mtime +"${retention_days}" -delete
  write_metrics "success" "none" "${size_bytes}"
else
  rm -f "${out_file}"
  write_metrics "error" "pg_dump_failed" 0
  exit 1
fi
