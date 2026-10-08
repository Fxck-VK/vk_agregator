#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
input="${tmpdir}/input.env"
output="${tmpdir}/output.env"
settings="${tmpdir}/google.env"
printf 'APP_ENV=staging\nVK_APP_SECRET=fixture\nACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp\nACCOUNT_OAUTH_GOOGLE_CLIENT_IDS=mobile.apps.googleusercontent.com\nACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=old-fixture\n' > "${input}"
unset DEV_ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET
bash scripts/deploy/prepare-dev-google.sh --input "${input}" --output "${output}" --settings-output "${settings}"
cmp "${input}" "${output}"
[[ ! -s "${settings}" ]]
export DEV_ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS=browser.apps.googleusercontent.com
if bash scripts/deploy/prepare-dev-google.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
  echo 'Incomplete Google configuration accepted' >&2; exit 1
fi
export DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=fixture-google-value
log="$(bash scripts/deploy/prepare-dev-google.sh --input "${input}" --output "${output}" --settings-output "${settings}")"
[[ "${log}" != *"${DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET}"* ]]
grep -Fx 'VK_APP_SECRET=fixture' "${output}" >/dev/null
grep -Fx 'ACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp' "${output}" >/dev/null
grep -Fx 'ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS=browser.apps.googleusercontent.com,mobile.apps.googleusercontent.com' "${output}" >/dev/null
grep -Fx "ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=${DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET}" "${output}" >/dev/null
[[ "$(grep -c '^ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=' "${output}")" == 1 ]]
[[ "$(wc -l < "${settings}")" == 2 ]]
[[ "$(stat -c '%a' "${settings}")" == 600 ]]
if grep -qE '^(VK_APP_SECRET|ACCOUNT_EMAIL_SMTP_PASSWORD)=' "${settings}"; then
  echo 'Unrelated credential copied to Google settings' >&2; exit 1
fi
if bash scripts/deploy/prepare-dev-google.sh --input "${input}" --output "${output}" --settings-output "${input}" >/dev/null 2>&1; then
  echo 'Input accepted as settings output' >&2; exit 1
fi
export DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=$'fixture\ninjected=true'
if bash scripts/deploy/prepare-dev-google.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
  echo 'Multiline Google credential accepted' >&2; exit 1
fi
export DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=fixture-google-value
export DEV_ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS='https://invalid.example.test'
if bash scripts/deploy/prepare-dev-google.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
  echo 'Invalid Google client ID accepted' >&2; exit 1
fi
echo 'DEV Google configuration tests passed'
