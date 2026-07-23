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

echo "Split env assembly override tests passed"
