#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/deploy/check-env-parity.sh --dev <dev-env> --prod <prod-env> [options]

Options:
  --dev-template <path>     DEV env template, default: .env.dev.example
  --prod-template <path>    PROD env contract template, default: .env.prod.example
  --allowlist <path>        DEV/PROD env-name allowlist, default: scripts/deploy/env-parity.allowlist
  --allow-dev-only <regex>  Additional allowed DEV-only key regex
  --allow-prod-only <regex> Additional allowed PROD-only key regex
  -h, --help                Show this help

Compares only variable names. Values, env contents and source file paths are
never printed. Environment-specific values such as APP_ENV and public URLs are
validated by check-dev-env.sh and check-prod-env.sh.
USAGE
}

dev_env=""
prod_env=""
dev_template=".env.dev.example"
prod_template=".env.prod.example"
allowlist="scripts/deploy/env-parity.allowlist"
extra_dev_only_patterns=()
extra_prod_only_patterns=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dev)
      dev_env="${2:-}"
      shift 2
      ;;
    --prod)
      prod_env="${2:-}"
      shift 2
      ;;
    --dev-template)
      dev_template="${2:-}"
      shift 2
      ;;
    --prod-template)
      prod_template="${2:-}"
      shift 2
      ;;
    --allowlist)
      allowlist="${2:-}"
      shift 2
      ;;
    --allow-dev-only)
      extra_dev_only_patterns+=("${2:-}")
      shift 2
      ;;
    --allow-prod-only)
      extra_prod_only_patterns+=("${2:-}")
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${dev_env}" || -z "${prod_env}" ]]; then
  echo "Missing required arguments: --dev and --prod" >&2
  usage >&2
  exit 2
fi

require_file() {
  local label="$1"
  local file="$2"
  if [[ ! -f "${file}" ]]; then
    echo "Env source not found: ${label}" >&2
    exit 1
  fi
}

require_file "DEV env" "${dev_env}"
require_file "PROD env" "${prod_env}"
require_file "DEV template" "${dev_template}"
require_file "PROD template" "${prod_template}"
require_file "allowlist" "${allowlist}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

extract_keys() {
  local file="$1"
  awk '
    {
      sub(/\r$/, "", $0)
      line = $0
      sub(/^[[:space:]]+/, "", line)
      sub(/[[:space:]]+$/, "", line)
      if (line == "" || substr(line, 1, 1) == "#") {
        next
      }
      if (line ~ /^export[[:space:]]+/) {
        sub(/^export[[:space:]]+/, "", line)
      }
      equals = index(line, "=")
      if (equals == 0) {
        next
      }
      key = substr(line, 1, equals - 1)
      gsub(/[[:space:]]/, "", key)
      if (key ~ /^[A-Za-z_][A-Za-z0-9_]*$/) {
        print key
      }
    }
  ' "${file}" | sort -u
}

dev_only_patterns=()
prod_only_patterns=()

load_allowlist() {
  local file="$1"
  local section=""
  local line

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"

    if [[ -z "${line}" || "${line:0:1}" == "#" ]]; then
      continue
    fi

    case "${line}" in
      "[dev-only]")
        section="dev"
        ;;
      "[prod-only]")
        section="prod"
        ;;
      \[*\])
        echo "Unknown allowlist section in ${file}: ${line}" >&2
        exit 1
        ;;
      *)
        case "${section}" in
          dev)
            dev_only_patterns+=("${line}")
            ;;
          prod)
            prod_only_patterns+=("${line}")
            ;;
          *)
            echo "Allowlist pattern outside a section in ${file}: ${line}" >&2
            exit 1
            ;;
        esac
        ;;
    esac
  done < "${file}"
}

matches_any_pattern() {
  local key="$1"
  shift
  local pattern
  for pattern in "$@"; do
    [[ -z "${pattern}" ]] && continue
    if [[ "${key}" =~ ${pattern} ]]; then
      return 0
    fi
  done
  return 1
}

filter_allowed() {
  local input_file="$1"
  local allowed_file="$2"
  local blocked_file="$3"
  shift 3
  : > "${allowed_file}"
  : > "${blocked_file}"

  local key
  while IFS= read -r key || [[ -n "${key}" ]]; do
    [[ -z "${key}" ]] && continue
    if matches_any_pattern "${key}" "$@"; then
      printf '%s\n' "${key}" >> "${allowed_file}"
    else
      printf '%s\n' "${key}" >> "${blocked_file}"
    fi
  done < "${input_file}"
}

print_list() {
  local title="$1"
  local file="$2"
  echo "${title}:"
  if [[ -s "${file}" ]]; then
    sed 's/^/- /' "${file}"
  else
    echo "- none"
  fi
}

extract_keys "${dev_env}" > "${tmpdir}/dev.keys"
extract_keys "${prod_env}" > "${tmpdir}/prod.keys"
extract_keys "${dev_template}" > "${tmpdir}/dev-template.keys"
extract_keys "${prod_template}" > "${tmpdir}/prod-template.keys"
load_allowlist "${allowlist}"

comm -13 "${tmpdir}/prod.keys" "${tmpdir}/dev.keys" > "${tmpdir}/dev-only.keys"
comm -23 "${tmpdir}/prod.keys" "${tmpdir}/dev.keys" > "${tmpdir}/prod-only.keys"
comm -23 "${tmpdir}/prod-template.keys" "${tmpdir}/prod.keys" > "${tmpdir}/missing-prod-contract.keys"

dev_patterns=("${dev_only_patterns[@]}" "${extra_dev_only_patterns[@]}")
prod_patterns=("${prod_only_patterns[@]}" "${extra_prod_only_patterns[@]}")

filter_allowed \
  "${tmpdir}/dev-only.keys" \
  "${tmpdir}/dev-only.allowed.keys" \
  "${tmpdir}/dev-only.blocked.keys" \
  "${dev_patterns[@]}"

filter_allowed \
  "${tmpdir}/prod-only.keys" \
  "${tmpdir}/prod-only.allowed.keys" \
  "${tmpdir}/prod-only.blocked.keys" \
  "${prod_patterns[@]}"

print_list "Allowed DEV-only keys" "${tmpdir}/dev-only.allowed.keys"
echo
print_list "Allowed PROD-only keys" "${tmpdir}/prod-only.allowed.keys"
echo
print_list "Missing in PROD" "${tmpdir}/dev-only.blocked.keys"
echo
print_list "Missing in DEV" "${tmpdir}/prod-only.blocked.keys"
echo
print_list "Missing in PROD from production template (non-blocking)" "${tmpdir}/missing-prod-contract.keys"

if [[ -s "${tmpdir}/dev-only.blocked.keys" ||
      -s "${tmpdir}/prod-only.blocked.keys" ]]; then
  echo
  echo "Env parity check failed. Only variable names were printed; values were not read from output."
  exit 1
fi

echo
echo "Env parity check passed"
