#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

script="scripts/deploy/check-env-parity.sh"

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

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    printf 'Expected %s to contain %s\n' "${label}" "${needle}" >&2
    exit 1
  fi
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf 'Expected failure did not happen: %s\n' "${label}" >&2
    exit 1
  fi
}

write_prod_contract() {
  local output="$1"
  cat > "${output}" <<'EOF'
APP_ENV=production
PUBLIC_VK_BASE_URL=https://vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://neiirohub.ru/billing/webhooks/yookassa
PAYMENT_PROVIDER=yookassa
YOOKASSA_SHOP_ID=CHANGE_ME
YOOKASSA_SECRET_KEY=CHANGE_ME
FEATURE_VIDEO_ROUTER_ENABLED=false
POYO_API_KEY=
BACKUP_DIR=/var/backups/vk-ai-aggregator
EOF
}

write_dev_template() {
  local output="$1"
  cat > "${output}" <<'EOF'
APP_ENV=development
PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/billing/webhooks/yookassa
PAYMENT_PROVIDER=mock
FEATURE_VIDEO_ROUTER_ENABLED=true
DEV_EXPECTED_VK_GROUP_ID=239658332
EOF
}

bash -n "${script}"

prod_contract="${tmpdir}/prod.template.env"
dev_template="${tmpdir}/dev.template.env"
write_prod_contract "${prod_contract}"
write_dev_template "${dev_template}"

valid_dev="${tmpdir}/valid.dev.env"
valid_prod="${tmpdir}/valid.prod.env"
cat > "${valid_dev}" <<'EOF'
APP_ENV=development
PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/billing/webhooks/yookassa
PAYMENT_PROVIDER=yookassa
YOOKASSA_SHOP_ID=dev-secret-shop
YOOKASSA_SECRET_KEY=dev-secret-key
FEATURE_VIDEO_ROUTER_ENABLED=true
POYO_API_KEY=dev-secret-poyo
DEV_EXPECTED_VK_GROUP_ID=239658332
EOF

cat > "${valid_prod}" <<'EOF'
APP_ENV=production
PUBLIC_VK_BASE_URL=https://vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://neiirohub.ru/billing/webhooks/yookassa
PAYMENT_PROVIDER=yookassa
YOOKASSA_SHOP_ID=prod-secret-shop
YOOKASSA_SECRET_KEY=prod-secret-key
FEATURE_VIDEO_ROUTER_ENABLED=true
POYO_API_KEY=prod-secret-poyo
BACKUP_DIR=/var/backups/vk-ai-aggregator
EOF

valid_log="$(bash "${script}" --dev "${valid_dev}" --prod "${valid_prod}" --dev-template "${dev_template}" --prod-template "${prod_contract}" 2>&1)"
assert_not_contains "${valid_log}" "dev-secret" "valid parity log"
assert_not_contains "${valid_log}" "prod-secret" "valid parity log"
assert_not_contains "${valid_log}" "${valid_dev}" "valid parity log"
assert_not_contains "${valid_log}" "${valid_prod}" "valid parity log"
assert_not_contains "${valid_log}" "${dev_template}" "valid parity log"
assert_not_contains "${valid_log}" "${prod_contract}" "valid parity log"

missing_prod="${tmpdir}/missing.prod.env"
grep -v '^POYO_API_KEY=' "${valid_prod}" > "${missing_prod}"
expect_failure \
  "DEV key missing from PROD" \
  bash "${script}" --dev "${valid_dev}" --prod "${missing_prod}" --dev-template "${dev_template}" --prod-template "${prod_contract}"

missing_contract="${tmpdir}/missing-contract.prod.env"
grep -v '^YOOKASSA_SECRET_KEY=' "${valid_prod}" > "${missing_contract}"
expect_failure \
  "PROD contract key missing" \
  bash "${script}" --dev "${valid_dev}" --prod "${missing_contract}" --dev-template "${dev_template}" --prod-template "${prod_contract}"

bad_log="$(bash "${script}" --dev "${valid_dev}" --prod "${missing_prod}" --dev-template "${dev_template}" --prod-template "${prod_contract}" 2>&1 || true)"
assert_not_contains "${bad_log}" "dev-secret" "failing parity log"
assert_not_contains "${bad_log}" "prod-secret" "failing parity log"
assert_not_contains "${bad_log}" "${valid_dev}" "failing parity log"
assert_not_contains "${bad_log}" "${missing_prod}" "failing parity log"
assert_contains "${bad_log}" "Missing in PROD:" "failing parity log"
assert_contains "${bad_log}" "- POYO_API_KEY" "failing parity log"

echo "Env parity script tests passed"
