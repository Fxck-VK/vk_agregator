#!/usr/bin/env bash
set -euo pipefail

env_file=""
vk_base_url=""
app_base_url=""
payment_webhook_url=""
timeout_seconds="${TIMEOUT_SECONDS:-10}"
skip_local_health="false"

usage() {
  cat <<'USAGE'
Usage: scripts/deploy/smoke-dev.sh [options]

Options:
  --env-file PATH                    DEV server env file to read, for example .env
  --vk-base-url URL                  DEV VK/API base URL. Default: PUBLIC_VK_BASE_URL or https://dev-vk.neiirohub.ru
  --app-base-url URL                 DEV Mini App base URL. Default: PUBLIC_APP_BASE_URL or https://dev-app.neiirohub.ru
  --payment-webhook-url URL          DEV YooKassa webhook URL. Default: PUBLIC_PAYMENT_WEBHOOK_URL or https://dev.neiirohub.ru/billing/webhooks/yookassa
  --timeout-seconds SECONDS          HTTP timeout. Default: 10
  --skip-local-health                Skip local API/worker/provider-webhook/Mini App/reverse-proxy health checks
  -h, --help                         Show help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) env_file="${2:?missing value for --env-file}"; shift 2 ;;
    --vk-base-url) vk_base_url="${2:?missing value for --vk-base-url}"; shift 2 ;;
    --app-base-url) app_base_url="${2:?missing value for --app-base-url}"; shift 2 ;;
    --payment-webhook-url) payment_webhook_url="${2:?missing value for --payment-webhook-url}"; shift 2 ;;
    --timeout-seconds) timeout_seconds="${2:?missing value for --timeout-seconds}"; shift 2 ;;
    --skip-local-health) skip_local_health="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

declare -A env_values=()

load_env_file() {
  local path="$1"
  if [[ -z "${path}" ]]; then
    return
  fi
  if [[ ! -f "${path}" ]]; then
    echo "[FAIL] env file not found: ${path}" >&2
    exit 1
  fi
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    [[ "${line}" != *=* ]] && continue
    local key="${line%%=*}"
    local value="${line#*=}"
    key="${key//[[:space:]]/}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    env_values["${key}"]="${value}"
  done < "${path}"
}

get_env_value() {
  local name="$1"
  local default="${2:-}"
  if [[ -v "env_values[${name}]" ]]; then
    printf '%s' "${env_values[${name}]}"
  else
    printf '%s' "${default}"
  fi
}

assert_dev_url() {
  local name="$1"
  local url="$2"
  local expected_prefix="$3"

  if [[ "${url}" != https://* ]]; then
    echo "[FAIL] ${name} must use https:// in DEV smoke: ${url}" >&2
    exit 1
  fi
  if [[ "${url}" != "${expected_prefix}"* ]]; then
    echo "[FAIL] ${name} must point to ${expected_prefix}, got ${url}" >&2
    exit 1
  fi
}

load_env_file "${env_file}"

vk_base_url="${vk_base_url:-$(get_env_value PUBLIC_VK_BASE_URL "https://dev-vk.neiirohub.ru")}"
app_base_url="${app_base_url:-$(get_env_value PUBLIC_APP_BASE_URL "https://dev-app.neiirohub.ru")}"
payment_webhook_url="${payment_webhook_url:-$(get_env_value PUBLIC_PAYMENT_WEBHOOK_URL "https://dev.neiirohub.ru/billing/webhooks/yookassa")}"

vk_base_url="${vk_base_url%/}"
app_base_url="${app_base_url%/}"

assert_dev_url "DEV VK base URL" "${vk_base_url}" "https://dev-vk.neiirohub.ru"
assert_dev_url "DEV Mini App base URL" "${app_base_url}" "https://dev-app.neiirohub.ru"
assert_dev_url "DEV payment webhook URL" "${payment_webhook_url}" "https://dev.neiirohub.ru/billing/webhooks/yookassa"

args=(
  --vk-base-url "${vk_base_url}"
  --app-base-url "${app_base_url}"
  --payment-webhook-url "${payment_webhook_url}"
  --timeout-seconds "${timeout_seconds}"
)
if [[ -n "${env_file}" ]]; then
  args=(--env-file "${env_file}" "${args[@]}")
fi
if [[ "${skip_local_health}" == "true" ]]; then
  args+=(--skip-local-health)
fi

echo "Running safe DEV smoke checks"
bash scripts/deploy/smoke-prod.sh "${args[@]}"
