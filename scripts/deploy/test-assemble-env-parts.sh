#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

script="scripts/deploy/assemble-env-parts.sh"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

export ENV_COMMON=$'APP_ENV=production\nVK_MENU_TOP_UP_ENABLED=true'
export ENV_PROVIDERS_COMMON='PROVIDER=fixture'
export ENV_SECRETS_PROD='ADMIN_TOKEN=fixture'
export ENV_PAYMENTS_PROD='PAYMENT_PROVIDER=yookassa'

preserved="${tmpdir}/preserved.env"
bash "${script}" --target prod --output "${preserved}"
grep -Fx 'VK_MENU_TOP_UP_ENABLED=true' "${preserved}" >/dev/null

overridden="${tmpdir}/overridden.env"
bash "${script}" \
  --target prod \
  --output "${overridden}" \
  --vk-menu-top-up-enabled false

if [[ "$(grep -Ec '^VK_MENU_TOP_UP_ENABLED=' "${overridden}")" -ne 1 ]]; then
  echo "Expected exactly one VK_MENU_TOP_UP_ENABLED entry" >&2
  exit 1
fi
grep -Fx 'VK_MENU_TOP_UP_ENABLED=false' "${overridden}" >/dev/null

if bash "${script}" \
  --target prod \
  --output "${tmpdir}/invalid.env" \
  --vk-menu-top-up-enabled maybe \
  >/dev/null 2>&1; then
  echo "Expected invalid VK menu top-up override to fail" >&2
  exit 1
fi

export ENV_COMMON=$'APP_ENV=development\nWEB_ORIGIN=https://stale-dev-origin.invalid'
export ENV_PROVIDERS_COMMON='PROVIDER=fixture'
export ENV_SECRETS_DEV='ADMIN_TOKEN=fixture'
export ENV_PAYMENTS_DEV='PAYMENT_PROVIDER=mock'

dev_overridden="${tmpdir}/dev-overridden.env"
bash "${script}" \
  --target dev \
  --output "${dev_overridden}" \
  --web-origin https://dev-web.epifera.com

if [[ "$(grep -Ec '^WEB_ORIGIN=' "${dev_overridden}")" -ne 1 ]]; then
  echo "Expected exactly one WEB_ORIGIN entry" >&2
  exit 1
fi
grep -Fx 'WEB_ORIGIN=https://dev-web.epifera.com' "${dev_overridden}" >/dev/null

if bash "${script}" \
  --target prod \
  --output "${tmpdir}/invalid-web-origin.env" \
  --web-origin https://dev-web.epifera.com \
  >/dev/null 2>&1; then
  echo "Expected production WEB_ORIGIN override to fail" >&2
  exit 1
fi

export ENV_COMMON=$'APP_ENV=development\nWEB_ORIGIN=https://dev-web.neiirohub.ru\nPUBLIC_VK_BASE_URL="https://dev-vk.neiirohub.ru"\nPUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru/miniapp\nPUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/webhooks/yookassa'
export ENV_SECRETS_DEV='ADMIN_TOKEN=fixture-neiirohub.ru-secret'
export ENV_PAYMENTS_DEV=$'PAYMENT_PROVIDER=mock\nYOOKASSA_RETURN_URL_MINIAPP=https://dev-app.neiirohub.ru/payment?payment_id=fixture\nYOOKASSA_RETURN_URL_VK_BOT=https://vk.com/app123'

cutover="${tmpdir}/cutover.env"
bash "${script}" --target dev --output "${cutover}" \
  --web-origin https://dev-web.epifera.com --dev-public-domain epifera.com
grep -Fx 'PUBLIC_VK_BASE_URL="https://dev-vk.epifera.com"' "${cutover}" >/dev/null
grep -Fx 'PUBLIC_APP_BASE_URL=https://dev-app.epifera.com/miniapp' "${cutover}" >/dev/null
grep -Fx 'PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.epifera.com/webhooks/yookassa' "${cutover}" >/dev/null
grep -Fx 'YOOKASSA_RETURN_URL_MINIAPP=https://dev-app.epifera.com/payment?payment_id=fixture' "${cutover}" >/dev/null
grep -Fx 'YOOKASSA_RETURN_URL_VK_BOT=https://vk.com/app123' "${cutover}" >/dev/null
grep -Fx 'ADMIN_TOKEN=fixture-neiirohub.ru-secret' "${cutover}" >/dev/null

export ENV_COMMON="${ENV_COMMON//neiirohub.ru/epifera.com}"
export ENV_PAYMENTS_DEV="${ENV_PAYMENTS_DEV//neiirohub.ru/epifera.com}"
bash "${script}" --target dev --output "${tmpdir}/idempotent.env" \
  --web-origin https://dev-web.epifera.com --dev-public-domain epifera.com
cmp "${cutover}" "${tmpdir}/idempotent.env"

export ENV_COMMON=$'APP_ENV=development\nPUBLIC_VK_BASE_URL=https://vk.neiirohub.ru\nPUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru\nPUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru'
if bash "${script}" --target dev --output "${tmpdir}/wrong-contour.env" \
  --dev-public-domain epifera.com >"${tmpdir}/cutover-error.log" 2>&1; then
  echo "Expected production URL in DEV cutover to fail" >&2
  exit 1
fi
if [[ -f "${tmpdir}/wrong-contour.env" ]] || grep -F 'fixture-neiirohub.ru-secret' "${tmpdir}/cutover-error.log" >/dev/null; then
  echo "Failed cutover must not publish an env file or reveal secrets" >&2
  exit 1
fi

export ENV_COMMON=$'APP_ENV=development\nPUBLIC_VK_BASE_URL=https://dev-vk.epifera.com\nPUBLIC_APP_BASE_URL=https://dev-app.epifera.com\nPUBLIC_PAYMENT_WEBHOOK_URL=https://dev.epifera.com'
for wrong_return_url in 'https://app.neiirohub.ru/payment' 'https://app.epifera.com/payment' 'http://dev-app.neiirohub.ru/payment'; do
  export ENV_PAYMENTS_DEV="PAYMENT_PROVIDER=mock
YOOKASSA_RETURN_URL_MINIAPP=${wrong_return_url}"
  if bash "${script}" --target dev --output "${tmpdir}/wrong-return.env" \
    --dev-public-domain epifera.com >/dev/null 2>&1; then
    echo "Expected production or non-HTTPS owned return URL in DEV to fail" >&2
    exit 1
  fi
done

for invalid_domain in 'https://epifera.com' 'epifera.com/path' 'epifera.com:443' 'epifera..com'; do
  if bash "${script}" --target dev --output "${tmpdir}/invalid-domain.env" \
    --dev-public-domain "${invalid_domain}" >/dev/null 2>&1; then
    echo "Expected invalid public domain to fail" >&2
    exit 1
  fi
done
if bash "${script}" --target prod --output "${tmpdir}/prod-cutover.env" \
  --dev-public-domain epifera.com >/dev/null 2>&1; then
  echo "Expected DEV public domain override on production to fail" >&2
  exit 1
fi

echo "Split env assembly override tests passed"
