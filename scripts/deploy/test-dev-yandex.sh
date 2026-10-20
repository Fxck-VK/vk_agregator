#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
input="${tmpdir}/input.env"; output="${tmpdir}/output.env"; settings="${tmpdir}/yandex.env"
printf 'APP_ENV=development\nVK_APP_SECRET=fixture\nACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp\nACCOUNT_OAUTH_YANDEX_CLIENT_ID=old-client\nACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=old-secret\n' > "${input}"
unset DEV_ACCOUNT_OAUTH_YANDEX_CLIENT_ID DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET
bash scripts/deploy/prepare-dev-yandex.sh --input "${input}" --output "${output}" --settings-output "${settings}"
cmp "${input}" "${output}"
[[ ! -s "${settings}" ]]
export DEV_ACCOUNT_OAUTH_YANDEX_CLIENT_ID=0123456789abcdef0123456789abcdef
if bash scripts/deploy/prepare-dev-yandex.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
  echo 'Incomplete Yandex configuration accepted' >&2; exit 1
fi
export DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=synthetic-yandex-secret
log="$(bash scripts/deploy/prepare-dev-yandex.sh --input "${input}" --output "${output}" --settings-output "${settings}")"
[[ "${log}" != *"${DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET}"* ]]
grep -Fx 'VK_APP_SECRET=fixture' "${output}" >/dev/null
grep -Fx 'ACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp' "${output}" >/dev/null
grep -Fx "ACCOUNT_OAUTH_YANDEX_CLIENT_ID=${DEV_ACCOUNT_OAUTH_YANDEX_CLIENT_ID}" "${output}" >/dev/null
grep -Fx "ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=${DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET}" "${output}" >/dev/null
[[ "$(grep -c '^ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=' "${output}")" == 1 ]]
[[ "$(wc -l < "${settings}")" == 2 ]]
[[ "$(stat -c '%a' "${output}")" == 600 && "$(stat -c '%a' "${settings}")" == 600 ]]
if grep -qE '^(VK_APP_SECRET|ACCOUNT_EMAIL_SMTP_PASSWORD)=' "${settings}"; then
  echo 'Unrelated credential copied to Yandex settings' >&2; exit 1
fi
if bash scripts/deploy/prepare-dev-yandex.sh --input "${input}" --output "${output}" --settings-output "${input}" >/dev/null 2>&1; then
  echo 'Input accepted as settings output' >&2; exit 1
fi
for invalid in $'synthetic\ninjected=true' 'synthetic$substitution' ''; do
  export DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET="${invalid}"
  if bash scripts/deploy/prepare-dev-yandex.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
    echo 'Invalid Yandex credential accepted' >&2; exit 1
  fi
done
export DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=synthetic-yandex-secret
export DEV_ACCOUNT_OAUTH_YANDEX_CLIENT_ID='https://invalid.example.test'
if bash scripts/deploy/prepare-dev-yandex.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
  echo 'Invalid Yandex client ID accepted' >&2; exit 1
fi
echo 'DEV Yandex configuration tests passed'
