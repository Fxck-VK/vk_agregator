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
VK_ACCESS_TOKEN=secret-vk-token-for-test
VK_SECRET=secret-vk-callback-for-test
VK_CONFIRMATION_TOKEN=secret-vk-confirmation-for-test
CLOUDFLARED_TUNNEL_TOKEN=secret-cloudflare-token-for-test
PAYMENT_PROVIDER=${payment_provider}
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
DEV_ALLOW_REAL_PAYMENTS=false
YOOKASSA_SHOP_ID=dev-test-shop
YOOKASSA_SECRET_KEY=secret-yookassa-key-for-test
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
    "${prepare_script}" \
      --input "${raw}" \
      --output "${rendered}" \
      --image-tag sha-test123 \
      --ghcr-username test-ghcr-user \
      --ghcr-token secret-ghcr-token-for-test
    "${check_script}" --env-file "${rendered}"
  } 2>&1)"

  assert_not_contains "${log}" "secret-vk-token-for-test" "${name} log"
  assert_not_contains "${log}" "secret-vk-callback-for-test" "${name} log"
  assert_not_contains "${log}" "secret-vk-confirmation-for-test" "${name} log"
  assert_not_contains "${log}" "secret-cloudflare-token-for-test" "${name} log"
  assert_not_contains "${log}" "secret-yookassa-key-for-test" "${name} log"
  assert_not_contains "${log}" "secret-ghcr-token-for-test" "${name} log"
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
expect_failure "prod URL in DEV env" "${check_script}" --env-file "${prod_url_env}"

echo "DEV deploy env script tests passed"
