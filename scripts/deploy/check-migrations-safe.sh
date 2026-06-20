#!/usr/bin/env bash
set -euo pipefail

env_file=".env"
migrations_dir="migrations"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/check-migrations-safe.sh [options]

Options:
  --env-file <path>        Env file used by deploy, default: .env
  --migrations-dir <path>  Migration directory, default: migrations
  -h, --help               Show this help

Destructive *.up.sql migrations are blocked unless both env values are set:
  MIGRATION_ALLOW_DESTRUCTIVE=true
  MIGRATION_DESTRUCTIVE_CONFIRM=I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) env_file="$2"; shift 2 ;;
    --migrations-dir) migrations_dir="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

if [[ ! -f "${env_file}" ]]; then
  echo "Env file not found: ${env_file}" >&2
  exit 1
fi
if [[ ! -d "${migrations_dir}" ]]; then
  echo "Migrations directory not found: ${migrations_dir}" >&2
  exit 1
fi

declare -A env_values=()
while IFS= read -r line || [[ -n "${line}" ]]; do
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%"${line##*[![:space:]]}"}"
  [[ -z "${line}" || "${line}" == \#* || "${line}" != *=* ]] && continue
  key="${line%%=*}"
  value="${line#*=}"
  key="${key//[[:space:]]/}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  value="${value%\"}"
  value="${value#\"}"
  value="${value%\'}"
  value="${value#\'}"
  env_values["${key}"]="${value}"
done < "${env_file}"

get_value() {
  local name="$1"
  local default="${2:-}"
  if [[ -v "env_values[${name}]" ]]; then
    printf '%s' "${env_values[${name}]}"
  else
    printf '%s' "${default}"
  fi
}

is_true_value() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ "${value}" == "1" || "${value}" == "true" || "${value}" == "yes" || "${value}" == "on" ]]
}

destructive_regex='(^|[[:space:];])(DROP[[:space:]]+(TABLE|DATABASE|SCHEMA|TYPE)|TRUNCATE[[:space:]]+|DELETE[[:space:]]+FROM|ALTER[[:space:]]+TABLE[[:space:]][^;]*[[:space:]]DROP[[:space:]]+(COLUMN|CONSTRAINT))'
matches="$(grep -REin "${destructive_regex}" "${migrations_dir}"/*.up.sql 2>/dev/null || true)"

if [[ -n "${matches}" ]]; then
  if is_true_value "$(get_value MIGRATION_ALLOW_DESTRUCTIVE false)" && [[ "$(get_value MIGRATION_DESTRUCTIVE_CONFIRM "")" == "I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS" ]]; then
    echo "WARNING: destructive migration patterns allowed by explicit confirmation." >&2
    echo "${matches}" >&2
  else
    echo "Destructive migration patterns detected in *.up.sql files:" >&2
    echo "${matches}" >&2
    echo >&2
    echo "Set MIGRATION_ALLOW_DESTRUCTIVE=true and MIGRATION_DESTRUCTIVE_CONFIRM=I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS only after manual review and backup." >&2
    exit 1
  fi
fi

echo "Migration safety check OK: ${migrations_dir}"
