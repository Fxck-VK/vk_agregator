#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  check-dev-env.sh --env-file <rendered-dev-env> [--expected-vk-group-id <id>]

Validates that a rendered DEV runtime .env cannot accidentally target production.
The script intentionally prints only non-secret readiness information.
USAGE
}

env_file=""
expected_vk_group_id="${DEV_EXPECTED_VK_GROUP_ID:-239658332}"
prod_vk_group_id="${PROD_VK_GROUP_ID:-239332376}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      env_file="${2:-}"
      shift 2
      ;;
    --expected-vk-group-id)
      expected_vk_group_id="${2:-}"
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

if [[ -z "${env_file}" ]]; then
  echo "Missing required argument: --env-file" >&2
  usage >&2
  exit 2
fi

if [[ ! -f "${env_file}" ]]; then
  echo "DEV env file does not exist: ${env_file}" >&2
  exit 1
fi

trim() {
  local value="${1:-}"
  value="${value%$'\r'}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

unquote() {
  local value
  value="$(trim "${1:-}")"
  value="${value%\"}"
  value="${value#\"}"
  value="${value%\'}"
  value="${value#\'}"
  printf '%s' "${value}"
}

get_env_value() {
  local name="$1"
  local value
  value="$(grep -E "^${name}=" "${env_file}" | tail -n 1 | cut -d= -f2- || true)"
  unquote "${value}"
}

is_placeholder() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ -z "${value//[[:space:]]/}" ||
     "${value}" == *change_me* ||
     "${value}" == *placeholder* ||
     "${value}" == *example* ]]
}

require_present() {
  local name="$1"
  local value
  value="$(get_env_value "${name}")"
  if is_placeholder "${value}"; then
    printf '%s is required for DEV deploy\n' "${name}" >&2
    exit 1
  fi
}

require_exact() {
  local name="$1"
  local expected="$2"
  local value
  value="$(get_env_value "${name}")"
  if [[ "${value}" != "${expected}" ]]; then
    printf '%s must be %s for DEV deploy\n' "${name}" "${expected}" >&2
    exit 1
  fi
}

require_dev_url() {
  local name="$1"
  local prefix="$2"
  local value
  value="$(get_env_value "${name}")"
  if is_placeholder "${value}"; then
    printf '%s is required for DEV deploy\n' "${name}" >&2
    exit 1
  fi
  if [[ "${value}" != "${prefix}"* ]]; then
    printf '%s must start with %s for DEV deploy\n' "${name}" "${prefix}" >&2
    exit 1
  fi
}

reject_exact_value() {
  local name="$1"
  local forbidden="$2"
  local value
  value="$(get_env_value "${name}")"
  if [[ "${value}" == "${forbidden}" ]]; then
    printf 'Production value found in DEV env: %s\n' "${name}" >&2
    exit 1
  fi
}

if [[ -z "${DEV_EXPECTED_VK_GROUP_ID:-}" ]]; then
  file_expected_vk_group_id="$(get_env_value DEV_EXPECTED_VK_GROUP_ID)"
  if ! is_placeholder "${file_expected_vk_group_id}"; then
    expected_vk_group_id="${file_expected_vk_group_id}"
  fi
fi

app_env="$(get_env_value APP_ENV)"
app_env_lc="$(printf '%s' "${app_env}" | tr '[:upper:]' '[:lower:]')"
if [[ "${app_env_lc}" != "development" ]]; then
  echo "APP_ENV must be development for DEV deploy" >&2
  exit 1
fi

require_dev_url PUBLIC_VK_BASE_URL "https://dev-vk.neiirohub.ru"
require_dev_url PUBLIC_APP_BASE_URL "https://dev-app.neiirohub.ru"
require_dev_url PUBLIC_PAYMENT_WEBHOOK_URL "https://dev.neiirohub.ru"

reject_exact_value PUBLIC_VK_BASE_URL "https://vk.neiirohub.ru"
reject_exact_value PUBLIC_APP_BASE_URL "https://app.neiirohub.ru"
reject_exact_value PUBLIC_PAYMENT_WEBHOOK_URL "https://neiirohub.ru"

vk_group_id="$(get_env_value VK_GROUP_ID)"
if is_placeholder "${vk_group_id}"; then
  echo "VK_GROUP_ID is required for DEV deploy" >&2
  exit 1
fi
if [[ -n "${prod_vk_group_id}" && "${vk_group_id}" == "${prod_vk_group_id}" ]]; then
  echo "Production VK_GROUP_ID found in DEV env" >&2
  exit 1
fi
if [[ -n "${expected_vk_group_id}" && "${vk_group_id}" != "${expected_vk_group_id}" ]]; then
  printf 'VK_GROUP_ID must be the DEV community id %s\n' "${expected_vk_group_id}" >&2
  exit 1
fi

payment_provider="$(get_env_value PAYMENT_PROVIDER)"
payment_provider_lc="$(printf '%s' "${payment_provider}" | tr '[:upper:]' '[:lower:]')"
case "${payment_provider_lc}" in
  mock|yookassa|yookassa_test) ;;
  *)
    printf 'PAYMENT_PROVIDER has unsupported DEV value: %s\n' "${payment_provider:-<empty>}" >&2
    exit 1
    ;;
esac

require_present CLOUDFLARED_TUNNEL_TOKEN

if grep -Eiq '^(PROD_|PRODUCTION_)' "${env_file}"; then
  echo "Production-prefixed variables are not allowed in DEV env" >&2
  exit 1
fi

for forbidden_domain in \
  "https://vk.neiirohub.ru" \
  "https://app.neiirohub.ru" \
  "https://neiirohub.ru/billing/webhooks/yookassa"
do
  if grep -Fq "${forbidden_domain}" "${env_file}"; then
    printf 'Production domain found in DEV env: %s\n' "${forbidden_domain}" >&2
    exit 1
  fi
done

echo "DEV env check passed: app_env=development payment_provider=${payment_provider_lc} vk_group_id=dev public_urls=dev cloudflare_token=present"
