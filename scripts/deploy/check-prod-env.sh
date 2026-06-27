#!/usr/bin/env bash
set -euo pipefail

env_file=".env"
with_cloudflare="false"
backup_before_deploy="false"
include_observability="false"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/check-prod-env.sh [options]

Options:
  --env-file <path>          Staging/production env file, default: .env
  --with-cloudflare          Require CLOUDFLARED_TUNNEL_TOKEN
  --backup-before-deploy     Require backup settings used by deploy-prod
  --include-observability    Require Grafana/exporter/alert settings
  -h, --help                 Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) env_file="$2"; shift 2 ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --backup-before-deploy) backup_before_deploy="true"; shift ;;
    --include-observability) include_observability="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

if [[ ! -f "${env_file}" ]]; then
  echo "Server env file not found: ${env_file}. Copy .env.staging.example or .env.prod.example to .env and fill real values." >&2
  exit 1
fi

declare -A env_values=()
while IFS= read -r line || [[ -n "${line}" ]]; do
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%"${line##*[![:space:]]}"}"
  [[ -z "${line}" || "${line}" == \#* ]] && continue
  [[ "${line}" != *=* ]] && continue
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

problems=()

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

add_problem() {
  problems+=("$1 - $2")
}

require_value() {
  local name="$1"
  local reason="$2"
  local value
  value="$(get_value "${name}")"
  if [[ -z "${value//[[:space:]]/}" ]]; then
    add_problem "${name}" "${reason}"
    return
  fi
  if [[ "${value}" == CHANGE_ME* || "${value}" == *CHANGE_ME* ]]; then
    add_problem "${name}" "${reason}; replace CHANGE_ME placeholder"
  fi
}

require_https_url() {
  local name="$1"
  local reason="$2"
  require_value "${name}" "${reason}"
  local value
  value="$(get_value "${name}")"
  if [[ -z "${value//[[:space:]]/}" || "${value}" == CHANGE_ME* || "${value}" == *CHANGE_ME* ]]; then
    return
  fi
  if [[ "${value}" != https://* ]]; then
    add_problem "${name}" "${reason}; must start with https://"
  fi
}

normalize_data_service_mode() {
  local name="$1"
  local value="$2"
  value="$(printf '%s' "${value:-local}" | tr '[:upper:]' '[:lower:]' | xargs)"
  if [[ -z "${value}" ]]; then
    value="local"
  fi
  case "${value}" in
    local|external|managed) ;;
    *) add_problem "${name}" "must be one of local, external, managed" ;;
  esac
  printf '%s' "${value}"
}

app_env="$(get_value APP_ENV | tr '[:upper:]' '[:lower:]')"
case "${app_env}" in
  production|prod) app_env="production" ;;
  staging|stage) app_env="staging" ;;
  *) add_problem APP_ENV "must be staging or production" ;;
esac

data_services_mode="$(normalize_data_service_mode DATA_SERVICES_MODE "$(get_value DATA_SERVICES_MODE local)")"
postgres_mode="$(normalize_data_service_mode POSTGRES_MODE "$(get_value POSTGRES_MODE "${data_services_mode}")")"
redis_mode="$(normalize_data_service_mode REDIS_MODE "$(get_value REDIS_MODE "${data_services_mode}")")"
s3_mode="$(normalize_data_service_mode S3_MODE "$(get_value S3_MODE "${data_services_mode}")")"

if is_true_value "$(get_value MIGRATION_ALLOW_DESTRUCTIVE false)" && [[ "$(get_value MIGRATION_DESTRUCTIVE_CONFIRM)" != "I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS" ]]; then
  add_problem MIGRATION_DESTRUCTIVE_CONFIRM "required when MIGRATION_ALLOW_DESTRUCTIVE=true"
fi
if is_true_value "$(get_value RESTORE_ALLOW_DESTRUCTIVE false)"; then
  add_problem RESTORE_ALLOW_DESTRUCTIVE "must be false in the persistent deploy env; set it only for a manual restore command"
fi
if [[ "${app_env}" == "production" ]] && ! is_true_value "$(get_value MIGRATION_BACKUP_CONFIRMED false)"; then
  require_value BACKUP_IMAGE_TAG "required for automatic production migration backup"
  require_value BACKUP_DIR "required for automatic production migration backup"
  require_value BACKUP_RETENTION_DAYS "required for automatic production migration backup"
  if [[ "$(get_value BACKUP_POSTGRES_ENABLED true | tr '[:upper:]' '[:lower:]')" == "false" ]]; then
    add_problem BACKUP_POSTGRES_ENABLED "must not be false unless MIGRATION_BACKUP_CONFIRMED=true"
  fi
fi

for required in \
  APP_IMAGE_REGISTRY IMAGE_TAG \
  DATABASE_URL REDIS_ADDR \
  S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY S3_BUCKET S3_REGION S3_ADDRESSING_STYLE \
  VK_ACCESS_TOKEN VK_SECRET VK_CONFIRMATION_TOKEN VK_APP_SECRET ADMIN_TOKEN; do
  require_value "${required}" "required for server runtime"
done

s3_addressing_style="$(printf '%s' "$(get_value S3_ADDRESSING_STYLE path)" | tr '[:upper:]' '[:lower:]')"
case "${s3_addressing_style}" in
  auto|path|virtual-hosted|virtual|dns) ;;
  *) add_problem S3_ADDRESSING_STYLE "must be auto, path, or virtual-hosted" ;;
esac

if [[ "${postgres_mode}" == "local" ]]; then
  require_value POSTGRES_PASSWORD "required when POSTGRES_MODE=local"
fi

if [[ "${s3_mode}" == "local" ]]; then
  require_value MINIO_ROOT_USER "required when S3_MODE=local"
  require_value MINIO_ROOT_PASSWORD "required when S3_MODE=local"
fi

image_registry="$(get_value APP_IMAGE_REGISTRY)"
if [[ "${image_registry}" != ghcr.io/* ]]; then
  add_problem APP_IMAGE_REGISTRY "must point at the GitHub Container Registry image namespace, for example ghcr.io/fxck-vk/vk_agregator"
fi
ghcr_username="$(get_value GHCR_USERNAME)"
ghcr_token="$(get_value GHCR_TOKEN)"
if [[ -n "${ghcr_username}${ghcr_token}" ]]; then
  require_value GHCR_USERNAME "required when GHCR_TOKEN is configured"
  require_value GHCR_TOKEN "required when GHCR_USERNAME is configured"
fi

if [[ "$(get_value VK_CONFIRMATION_TOKEN)" == "dev-confirmation" ]]; then
  add_problem VK_CONFIRMATION_TOKEN "must not be dev-confirmation in production"
fi

payment_provider="$(get_value PAYMENT_PROVIDER mock | tr '[:upper:]' '[:lower:]')"
if [[ "${payment_provider}" == "mock" ]]; then
  add_problem PAYMENT_PROVIDER "mock is not allowed in production"
fi
if [[ "${payment_provider}" == "yookassa" ]]; then
  for required in YOOKASSA_SHOP_ID YOOKASSA_SECRET_KEY YOOKASSA_RETURN_URL; do
    require_value "${required}" "required when PAYMENT_PROVIDER=yookassa"
  done
  if is_true_value "$(get_value YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED)"; then
    require_value YOOKASSA_WEBHOOK_IP_ALLOWLIST "required when YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED=true"
  fi
fi

if is_true_value "$(get_value PAYMENT_WEBHOOK_REQUIRE_HTTPS)"; then
  require_value PAYMENT_WEBHOOK_TRUSTED_PROXIES "required when PAYMENT_WEBHOOK_REQUIRE_HTTPS=true behind Cloudflare/nginx"
fi

provider_values="$(get_value PROVIDER),$(get_value PROVIDER_CHAIN),$(get_value IMAGE_PROVIDER),$(get_value VIDEO_PROVIDER)"
provider_values_lc="$(printf '%s' "${provider_values}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
if [[ ",${provider_values_lc}," == *",mock,"* ]]; then
  add_problem "PROVIDER/PROVIDER_CHAIN" "mock provider is not allowed in production"
fi
if [[ ",${provider_values_lc}," == *",deepinfra,"* ]]; then
  require_value DEEPINFRA_API_KEY "required when a DeepInfra provider is configured"
fi

if is_true_value "$(get_value PROVIDER_BALANCE_BOT_ENABLED false)"; then
  require_value ALERT_TELEGRAM_BOT_TOKEN "required when PROVIDER_BALANCE_BOT_ENABLED=true"
  require_value TELEGRAM_ADMIN_CHAT_ID "required when PROVIDER_BALANCE_BOT_ENABLED=true"
  require_value APIMART_API_KEY "required when PROVIDER_BALANCE_BOT_ENABLED=true"
  require_value APIMART_BASE_URL "required when PROVIDER_BALANCE_BOT_ENABLED=true"
  if is_true_value "$(get_value POYO_PROVIDER_ENABLED false)"; then
    require_value POYO_API_KEY "required when PROVIDER_BALANCE_BOT_ENABLED=true and POYO_PROVIDER_ENABLED=true"
    require_value POYO_BASE_URL "required when PROVIDER_BALANCE_BOT_ENABLED=true and POYO_PROVIDER_ENABLED=true"
  fi
  if is_true_value "$(get_value RUNWAY_PROVIDER_ENABLED false)"; then
    require_value RUNWAYML_API_SECRET "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
    require_value RUNWAYML_BASE_URL "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
  fi
  if is_true_value "$(get_value DEEPINFRA_BALANCE_PROVIDER_ENABLED false)"; then
    require_value DEEPINFRA_API_KEY "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
    require_value DEEPINFRA_BALANCE_BASE_URL "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
  fi
fi

moderation_provider="$(get_value MODERATION_PROVIDER keyword | tr '[:upper:]' '[:lower:]')"
artifact_scanner="$(get_value ARTIFACT_SCANNER none | tr '[:upper:]' '[:lower:]')"
if [[ ",${provider_values_lc}," == *",openai,"* || "${moderation_provider}" == "openai" || "${artifact_scanner}" == "openai" ]]; then
  require_value OPENAI_API_KEY "required when OpenAI provider/moderation/scanner is configured"
fi
if [[ "${app_env}" == "production" && ( -z "${artifact_scanner}" || "${artifact_scanner}" == "none" ) ]] && ! is_true_value "$(get_value ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION)"; then
  add_problem ARTIFACT_SCANNER "must be openai in production unless ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true"
elif [[ -z "${artifact_scanner}" || "${artifact_scanner}" == "none" ]]; then
  :
elif [[ "${artifact_scanner}" != "openai" ]]; then
  add_problem ARTIFACT_SCANNER "must be none or openai"
fi

prices="$(get_value PRICES)"
if [[ ",${prices}," == *",image_generate=0,"* ]]; then
  add_problem PRICES "image_generate=0 is not allowed in production env"
fi

if is_true_value "$(get_value VK_MENU_TOP_UP_ENABLED)"; then
  email="$(get_value VK_TOP_UP_RECEIPT_EMAIL)"
  phone="$(get_value VK_TOP_UP_RECEIPT_PHONE)"
  if [[ -z "${email//[[:space:]]/}" && -z "${phone//[[:space:]]/}" ]]; then
    add_problem "VK_TOP_UP_RECEIPT_EMAIL/VK_TOP_UP_RECEIPT_PHONE" "one receipt contact is required when VK_MENU_TOP_UP_ENABLED=true"
  fi
  if [[ "${email}" == *CHANGE_ME* || "${phone}" == *CHANGE_ME* ]]; then
    add_problem "VK_TOP_UP_RECEIPT_EMAIL/VK_TOP_UP_RECEIPT_PHONE" "replace CHANGE_ME placeholder before enabling VK top-up"
  fi
fi

if [[ "${with_cloudflare}" == "true" ]]; then
  require_value CLOUDFLARED_TUNNEL_TOKEN "required when deploying with Cloudflare tunnel; store it only in the server .env"
  require_https_url PUBLIC_VK_BASE_URL "required for Cloudflare deploy smoke, expected https://vk.neiirohub.ru"
  require_https_url PUBLIC_APP_BASE_URL "required for Cloudflare deploy smoke, expected https://app.neiirohub.ru"
  require_https_url PUBLIC_PAYMENT_WEBHOOK_URL "required for Cloudflare deploy smoke, expected https://neiirohub.ru/billing/webhooks/yookassa"
fi

if [[ "${backup_before_deploy}" == "true" ]]; then
  require_value BACKUP_IMAGE_TAG "required when backup-before-deploy is enabled"
  require_value BACKUP_DIR "required when backup-before-deploy is enabled"
  require_value BACKUP_RETENTION_DAYS "required when backup-before-deploy is enabled"
fi

if [[ "${include_observability}" == "true" ]]; then
  for required in GRAFANA_ADMIN_PASSWORD GRAFANA_SECRET_KEY POSTGRES_EXPORTER_DATA_SOURCE_NAME; do
    require_value "${required}" "required for production observability"
  done
  if is_true_value "$(get_value ALERT_TELEGRAM_ENABLED)"; then
    require_value ALERT_TELEGRAM_BOT_TOKEN "required when ALERT_TELEGRAM_ENABLED=true"
    require_value ALERT_TELEGRAM_CHAT_ID "required when ALERT_TELEGRAM_ENABLED=true"
  fi
fi

if (( ${#problems[@]} > 0 )); then
  echo "Server env check failed for ${env_file}"
  echo "Missing/invalid variables:"
  for problem in "${problems[@]}"; do
    echo " - ${problem}"
  done
  exit 1
fi

echo "Server env check OK: ${env_file} (${app_env})"
