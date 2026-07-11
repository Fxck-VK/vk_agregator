#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

prepare_script="scripts/deploy/prepare-dev-env.sh"
check_script="scripts/deploy/check-dev-env.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    printf 'Secret value leaked in %s\n' "${label}" >&2
    exit 1
  fi
}

assert_file_contains() {
  local file="$1"
  local needle="$2"
  if ! grep -Fxq "${needle}" "${file}"; then
    printf 'Expected %s to contain: %s\n' "${file}" "${needle}" >&2
    exit 1
  fi
}

write_common_dev_env() {
  local output="$1"
  local payment_provider="$2"
  cat > "${output}" <<EOF
APP_ENV=development
DEV_EXPECTED_VK_GROUP_ID=239658332
PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/billing/webhooks/yookassa
VK_GROUP_ID=239658332
VK_ACCESS_TOKEN=vk-token-placeholder
VK_SECRET=vk-callback-placeholder
VK_CONFIRMATION_TOKEN=vk-confirmation-placeholder
CLOUDFLARED_TUNNEL_TOKEN=dev-test-token-value
PAYMENT_PROVIDER=${payment_provider}
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
DEEPINFRA_API_KEY=deepinfra-placeholder
APIMART_API_KEY=apimart-dev-test-key
APIMART_BASE_URL=https://api.aimlapi.com/v1
POYO_API_KEY=poyo-dev-test-key
POYO_BASE_URL=https://api.poyo.ai
RUNWAYML_API_SECRET=runway-dev-test-key
RUNWAYML_BASE_URL=https://api.dev.runwayml.com/v1
DEV_ALLOW_REAL_PAYMENTS=false
YOOKASSA_SHOP_ID=dev-test-shop
YOOKASSA_SECRET_KEY=yookassa-key-placeholder
YOOKASSA_RETURN_URL=https://dev-app.neiirohub.ru/
EOF
}

run_valid_case() {
  local name="$1"
  local payment_provider="$2"
  local raw="${tmpdir}/${name}.raw.env"
  local rendered="${tmpdir}/${name}.rendered.env"
  local log

  write_common_dev_env "${raw}" "${payment_provider}"
  if [[ "${payment_provider}" == "yookassa" ]]; then
    {
      echo "DEV_ALLOW_REAL_PAYMENTS=true"
      echo "YOOKASSA_RETURN_URL_MINIAPP=https://dev-app.neiirohub.ru/"
      echo "YOOKASSA_RETURN_URL_VK_BOT=https://dev-vk.neiirohub.ru/payments/return"
    } >> "${raw}"
  fi

  log="$({
    bash "${prepare_script}" \
      --input "${raw}" \
      --output "${rendered}" \
      --ghcr-username test-ghcr-user \
      --ghcr-token ghcr-token-placeholder
    bash "${check_script}" --env-file "${rendered}"
  } 2>&1)"

  assert_not_contains "${log}" "vk-token-placeholder" "${name} log"
  assert_not_contains "${log}" "vk-callback-placeholder" "${name} log"
  assert_not_contains "${log}" "vk-confirmation-placeholder" "${name} log"
  assert_not_contains "${log}" "dev-test-token-value" "${name} log"
  assert_not_contains "${log}" "yookassa-key-placeholder" "${name} log"
  assert_not_contains "${log}" "ghcr-token-placeholder" "${name} log"
  assert_not_contains "${log}" "deepinfra-placeholder" "${name} log"
  assert_not_contains "${log}" "apimart-dev-test-key" "${name} log"
  assert_not_contains "${log}" "poyo-dev-test-key" "${name} log"
  assert_not_contains "${log}" "runway-dev-test-key" "${name} log"

  assert_file_contains "${rendered}" "APIMART_PROVIDER_ENABLED=true"
  assert_file_contains "${rendered}" "POYO_PROVIDER_ENABLED=true"
  assert_file_contains "${rendered}" "RUNWAY_PROVIDER_ENABLED=true"
  assert_file_contains "${rendered}" "VK_MENU_VIDEO_ENABLED=true"
  assert_file_contains "${rendered}" "VK_MENU_IMAGE_ENABLED=true"
  assert_file_contains "${rendered}" "VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_MOCK_ENABLED=false"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTER_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=false"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED=false"
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf 'Expected failure did not happen: %s\n' "${label}" >&2
    exit 1
  fi
}

for script in scripts/deploy/*.sh; do
  bash -n "${script}"
done

run_valid_case "mock-dev" "mock"
run_valid_case "yookassa-dev" "yookassa"

prod_url_env="${tmpdir}/prod-url.env"
write_common_dev_env "${prod_url_env}" "mock"
sed -i 's#PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru#PUBLIC_VK_BASE_URL=https://vk.neiirohub.ru#' "${prod_url_env}"
expect_failure "prod URL in DEV env" bash "${check_script}" --env-file "${prod_url_env}"

echo "DEV deploy env script tests passed"
