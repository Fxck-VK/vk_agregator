#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  prepare-dev-env.sh --input <raw-env> --output <rendered-env> --ghcr-username <name> --ghcr-token <token>

Builds a sanitized DEV runtime .env from assembled raw env content.
The script intentionally prints only non-secret readiness flags.
USAGE
}

input_file=""
output_file=""
ghcr_username=""
ghcr_token=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --input)
      input_file="${2:-}"
      shift 2
      ;;
    --output)
      output_file="${2:-}"
      shift 2
      ;;
    --ghcr-username)
      ghcr_username="${2:-}"
      shift 2
      ;;
    --ghcr-token)
      ghcr_token="${2:-}"
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

required_args=()
[[ -n "${input_file}" ]] || required_args+=(--input)
[[ -n "${output_file}" ]] || required_args+=(--output)
[[ -n "${ghcr_username}" ]] || required_args+=(--ghcr-username)
[[ -n "${ghcr_token}" ]] || required_args+=(--ghcr-token)

if (( ${#required_args[@]} > 0 )); then
  printf 'Missing required arguments: %s\n' "${required_args[*]}" >&2
  usage >&2
  exit 2
fi

if [[ ! -f "${input_file}" ]]; then
  echo "Input env file does not exist: ${input_file}" >&2
  exit 1
fi
if grep -Eq '^(IMAGE_TAG|BACKUP_IMAGE_TAG|API_IMAGE|WORKER_IMAGE|PROVIDER_WEBHOOK_IMAGE|PROVIDER_BALANCE_BOT_IMAGE|MINIAPP_IMAGE|MIGRATE_IMAGE|BACKUP_IMAGE|RELEASE_COMMIT_SHA|RELEASE_MANIFEST_SHA256|RELEASE_WORKFLOW_IDENTITY)=' "${input_file}"; then
  echo "Image release state must come from the separately verified release env file" >&2
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

get_raw_env_value() {
  local name="$1"
  local value
  value="$(grep -E "^${name}=" "${input_file}" | tail -n 1 | cut -d= -f2- || true)"
  unquote "${value}"
}

is_true_value() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ "${value}" == "1" || "${value}" == "true" || "${value}" == "yes" || "${value}" == "on" ]]
}

has_raw_env_value() {
  local name="$1"
  local value
  value="$(get_raw_env_value "${name}")"
  value="$(printf '%s' "${value}" | tr '[:upper:]' '[:lower:]')"
  [[ -n "${value//[[:space:]]/}" && "${value}" != *change_me* && "${value}" != *placeholder* && "${value}" != *example* ]]
}

require_dev_url() {
  local name="$1"
  local prefix="$2"
  local value
  value="$(get_raw_env_value "${name}")"
  if [[ -z "${value}" ]]; then
    printf '%s is required for DEV deploy\n' "${name}" >&2
    exit 1
  fi
  if [[ "${value}" != "${prefix}"* ]]; then
    printf '%s must start with %s for DEV deploy\n' "${name}" "${prefix}" >&2
    exit 1
  fi
}

app_env="$(get_raw_env_value APP_ENV)"
if [[ -z "${app_env}" ]]; then
  app_env="development"
fi
app_env_lc="$(printf '%s' "${app_env}" | tr '[:upper:]' '[:lower:]')"
case "${app_env_lc}" in
  dev|development) ;;
  *)
    echo "APP_ENV must be development/dev for DEV deploy" >&2
    exit 1
    ;;
esac

require_dev_url PUBLIC_VK_BASE_URL "https://dev-vk.neiirohub.ru"
require_dev_url PUBLIC_APP_BASE_URL "https://dev-app.neiirohub.ru"
require_dev_url PUBLIC_PAYMENT_WEBHOOK_URL "https://dev.neiirohub.ru"

for forbidden in \
  "PUBLIC_VK_BASE_URL=https://vk.neiirohub.ru" \
  "PUBLIC_APP_BASE_URL=https://app.neiirohub.ru" \
  "PUBLIC_PAYMENT_WEBHOOK_URL=https://neiirohub.ru"
do
  if grep -Fq "${forbidden}" "${input_file}"; then
    echo "Production public URL found in DEV env: ${forbidden%%=*}" >&2
    exit 1
  fi
done

has_provider_balance_key() {
  has_raw_env_value APIMART_API_KEY ||
    has_raw_env_value POYO_API_KEY ||
    has_raw_env_value RUNWAYML_API_SECRET ||
    has_raw_env_value DEEPINFRA_API_KEY
}

provider_tokens=()
append_provider_token() {
  local token="$1"
  token="$(printf '%s' "${token}" | tr '[:upper:]' '[:lower:]')"
  token="$(unquote "${token}")"
  case "${token}" in
    deepinfra|apimart|poyo|runway|mock) ;;
    ""|none|openai) return 0 ;;
    *) return 0 ;;
  esac
  for existing in "${provider_tokens[@]}"; do
    if [[ "${existing}" == "${token}" ]]; then
      return 0
    fi
  done
  provider_tokens+=("${token}")
}

raw_provider="$(get_raw_env_value PROVIDER)"
raw_chain="$(get_raw_env_value PROVIDER_CHAIN)"
if [[ -z "${raw_chain}" ]]; then
  raw_chain="${raw_provider}"
fi

IFS=',' read -ra chain_parts <<< "${raw_chain}"
for token in "${chain_parts[@]}"; do
  append_provider_token "${token}"
done

image_provider="$(get_raw_env_value IMAGE_PROVIDER)"
if [[ "${image_provider,,}" != "deepinfra" ]]; then
  append_provider_token "${image_provider}"
fi

video_provider="$(get_raw_env_value VIDEO_PROVIDER)"
if [[ "${video_provider,,}" != "deepinfra" ]]; then
  append_provider_token "${video_provider}"
fi

if has_raw_env_value DEEPINFRA_API_KEY; then
  append_provider_token deepinfra
fi
if has_raw_env_value APIMART_API_KEY; then
  append_provider_token apimart
fi
if has_raw_env_value POYO_API_KEY; then
  append_provider_token poyo
fi
if has_raw_env_value RUNWAYML_API_SECRET; then
  append_provider_token runway
fi

if (( ${#provider_tokens[@]} == 0 )); then
  if [[ "${raw_provider,,}" == "openai" || "${raw_chain,,}" == *openai* ]]; then
    provider_tokens+=(deepinfra)
  else
    provider_tokens+=(mock)
  fi
fi
append_provider_token mock
provider_chain="$(IFS=,; printf '%s' "${provider_tokens[*]}")"

telegram_bot_configured=false
if has_raw_env_value ALERT_TELEGRAM_BOT_TOKEN; then
  telegram_bot_configured=true
fi

telegram_admin_configured=false
if has_raw_env_value TELEGRAM_ADMIN_CHAT_ID; then
  telegram_admin_configured=true
fi

provider_balance_key_configured=false
if has_provider_balance_key; then
  provider_balance_key_configured=true
fi

provider_balance_bot_enabled=false
if is_true_value "$(get_raw_env_value PROVIDER_BALANCE_BOT_ENABLED)" ||
   [[ "${telegram_bot_configured}" == "true" &&
      "${telegram_admin_configured}" == "true" &&
      "${provider_balance_key_configured}" == "true" ]]; then
  provider_balance_bot_enabled=true
fi

apimart_base_url="$(get_raw_env_value APIMART_BASE_URL)"
if [[ -z "${apimart_base_url}" ]] && has_raw_env_value APIMART_API_KEY; then
  apimart_base_url="https://api.apimart.ai/v1"
fi
apimart_provider_enabled=false
if is_true_value "$(get_raw_env_value APIMART_PROVIDER_ENABLED)" || has_raw_env_value APIMART_API_KEY; then
  apimart_provider_enabled=true
fi

poyo_provider_enabled=false
if is_true_value "$(get_raw_env_value POYO_PROVIDER_ENABLED)" || has_raw_env_value POYO_API_KEY; then
  poyo_provider_enabled=true
fi
poyo_base_url="$(get_raw_env_value POYO_BASE_URL)"
if [[ -z "${poyo_base_url}" && "${poyo_provider_enabled}" == "true" ]]; then
  poyo_base_url="https://api.poyo.ai"
fi

runway_provider_enabled=false
if is_true_value "$(get_raw_env_value RUNWAY_PROVIDER_ENABLED)" || has_raw_env_value RUNWAYML_API_SECRET; then
  runway_provider_enabled=true
fi
runway_base_url="$(get_raw_env_value RUNWAYML_BASE_URL)"
if [[ -z "${runway_base_url}" && "${runway_provider_enabled}" == "true" ]]; then
  runway_base_url="https://api.dev.runwayml.com/v1"
fi

deepinfra_balance_provider_enabled=false
if is_true_value "$(get_raw_env_value DEEPINFRA_BALANCE_PROVIDER_ENABLED)" || has_raw_env_value DEEPINFRA_API_KEY; then
  deepinfra_balance_provider_enabled=true
fi
deepinfra_balance_base_url="$(get_raw_env_value DEEPINFRA_BALANCE_BASE_URL)"
if [[ -z "${deepinfra_balance_base_url}" && "${deepinfra_balance_provider_enabled}" == "true" ]]; then
  deepinfra_balance_base_url="https://api.deepinfra.com"
fi

image_nano_banana_2_enabled="${poyo_provider_enabled}"
image_nano_banana_pro_enabled="${apimart_provider_enabled}"
image_gpt_image_2_enabled="${apimart_provider_enabled}"

video_hailuo_fast_enabled="${apimart_provider_enabled}"
video_hailuo_standard_enabled="${apimart_provider_enabled}"
video_kling_o3_enabled="${poyo_provider_enabled}"
video_seedance_fast_enabled="${poyo_provider_enabled}"
video_runway_gen45_enabled="${poyo_provider_enabled}"
video_runway_turbo_enabled="${runway_provider_enabled}"
video_router_enabled=false
if [[ "${video_hailuo_fast_enabled}" == "true" ||
      "${video_hailuo_standard_enabled}" == "true" ||
      "${video_kling_o3_enabled}" == "true" ||
      "${video_seedance_fast_enabled}" == "true" ||
      "${video_runway_gen45_enabled}" == "true" ||
      "${video_runway_turbo_enabled}" == "true" ]]; then
  video_router_enabled=true
fi

image_menu_enabled=false
if [[ "${image_nano_banana_2_enabled}" == "true" ||
      "${image_nano_banana_pro_enabled}" == "true" ||
      "${image_gpt_image_2_enabled}" == "true" ]]; then
  image_menu_enabled=true
fi

tmp_output="$(mktemp)"
trap 'rm -f "${tmp_output}"' EXIT

sed \
  -e '/^APP_ENV=/d' \
  -e '/^GHCR_USERNAME=/d' \
  -e '/^GHCR_TOKEN=/d' \
  -e '/^IMAGE_TAG=/d' \
  -e '/^APP_ENV_FILE=/d' \
  -e '/^PROVIDER_BALANCE_BOT_ENABLED=/d' \
  -e '/^APIMART_PROVIDER_ENABLED=/d' \
  -e '/^APIMART_BASE_URL=/d' \
  -e '/^POYO_PROVIDER_ENABLED=/d' \
  -e '/^POYO_BASE_URL=/d' \
  -e '/^RUNWAY_PROVIDER_ENABLED=/d' \
  -e '/^RUNWAYML_BASE_URL=/d' \
  -e '/^DEEPINFRA_BALANCE_PROVIDER_ENABLED=/d' \
  -e '/^DEEPINFRA_BALANCE_BASE_URL=/d' \
  -e '/^PROVIDER=/d' \
  -e '/^PROVIDER_CHAIN=/d' \
  -e '/^IMAGE_PROVIDER=/d' \
  -e '/^VIDEO_PROVIDER=/d' \
  -e '/^OPENAI_TEXT_MODEL=/d' \
  -e '/^OPENAI_IMAGE_MODEL=/d' \
  -e '/^OPENAI_VIDEO_MODEL=/d' \
  -e '/^VK_VIDEO_DELIVERY_MODE=/d' \
  -e '/^RUNTIME_PRICING_DB_ENABLED=/d' \
  -e '/^RUNTIME_PRICING_STATIC_FALLBACK_ENABLED=/d' \
  -e '/^RUNTIME_PRICING_REFRESH_INTERVAL=/d' \
  -e '/^VK_MENU_VIDEO_ENABLED=/d' \
  -e '/^VK_MENU_IMAGE_ENABLED=/d' \
  -e '/^VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED=/d' \
  -e '/^FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=/d' \
  -e '/^FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=/d' \
  -e '/^FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=/d' \
  -e '/^FEATURE_IMAGE_MODEL_MOCK_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTER_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=/d' \
  -e '/^FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED=/d' \
  "${input_file}" > "${tmp_output}"

{
  printf '\n'
  printf 'APP_ENV=development\n'
  printf 'APP_ENV_FILE=.env\n'
  printf 'PROVIDER=%s\n' "${provider_tokens[0]}"
  printf 'PROVIDER_CHAIN=%s\n' "${provider_chain}"
  printf 'IMAGE_PROVIDER=\n'
  printf 'VIDEO_PROVIDER=\n'
  printf 'VK_VIDEO_DELIVERY_MODE=doc\n'
  printf 'RUNTIME_PRICING_DB_ENABLED=false\n'
  printf 'RUNTIME_PRICING_STATIC_FALLBACK_ENABLED=true\n'
  printf 'RUNTIME_PRICING_REFRESH_INTERVAL=0\n'
  printf 'PROVIDER_BALANCE_BOT_ENABLED=%s\n' "${provider_balance_bot_enabled}"
  printf 'APIMART_PROVIDER_ENABLED=%s\n' "${apimart_provider_enabled}"
  if [[ -n "${apimart_base_url}" ]]; then
    printf 'APIMART_BASE_URL=%s\n' "${apimart_base_url}"
  fi
  printf 'POYO_PROVIDER_ENABLED=%s\n' "${poyo_provider_enabled}"
  if [[ -n "${poyo_base_url}" ]]; then
    printf 'POYO_BASE_URL=%s\n' "${poyo_base_url}"
  fi
  printf 'RUNWAY_PROVIDER_ENABLED=%s\n' "${runway_provider_enabled}"
  if [[ -n "${runway_base_url}" ]]; then
    printf 'RUNWAYML_BASE_URL=%s\n' "${runway_base_url}"
  fi
  printf 'DEEPINFRA_BALANCE_PROVIDER_ENABLED=%s\n' "${deepinfra_balance_provider_enabled}"
  if [[ -n "${deepinfra_balance_base_url}" ]]; then
    printf 'DEEPINFRA_BALANCE_BASE_URL=%s\n' "${deepinfra_balance_base_url}"
  fi
  printf 'VK_MENU_VIDEO_ENABLED=%s\n' "${video_router_enabled}"
  printf 'VK_MENU_IMAGE_ENABLED=%s\n' "${image_menu_enabled}"
  printf 'VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED=true\n'
  printf 'FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=%s\n' "${image_nano_banana_pro_enabled}"
  printf 'FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=%s\n' "${image_gpt_image_2_enabled}"
  printf 'FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=%s\n' "${image_nano_banana_2_enabled}"
  printf 'FEATURE_IMAGE_MODEL_MOCK_ENABLED=false\n'
  printf 'FEATURE_VIDEO_ROUTER_ENABLED=%s\n' "${video_router_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=%s\n' "${video_hailuo_fast_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=%s\n' "${video_hailuo_standard_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=%s\n' "${video_kling_o3_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=%s\n' "${video_runway_turbo_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=%s\n' "${video_seedance_fast_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=%s\n' "${video_runway_gen45_enabled}"
  printf 'FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=false\n'
  printf 'FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED=false\n'
  printf 'GHCR_USERNAME=%s\n' "${ghcr_username}"
  printf 'GHCR_TOKEN=%s\n' "${ghcr_token}"
} >> "${tmp_output}"

install -m 600 "${tmp_output}" "${output_file}"

echo "DEV env prepared: providers=${provider_chain} provider_balance_bot=${provider_balance_bot_enabled} telegram_token=${telegram_bot_configured} admin_chat=${telegram_admin_configured} provider_key=${provider_balance_key_configured} apimart=${apimart_provider_enabled} poyo=${poyo_provider_enabled} runway=${runway_provider_enabled} deepinfra=${deepinfra_balance_provider_enabled} image_menu=${image_menu_enabled} video_menu=${video_router_enabled}"
