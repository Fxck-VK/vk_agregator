#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
input="${tmpdir}/input.env"
output="${tmpdir}/output.env"
printf 'APP_ENV=staging\nVK_APP_SECRET=fixture\nACCOUNT_EMAIL_SMTP_FROM=old@example.test\nACCOUNT_EMAIL_SMTP_PASSWORD=old-fixture\n' > "${input}"
unset DEV_ACCOUNT_EMAIL_SMTP_PASSWORD
bash scripts/deploy/prepare-dev-email.sh --input "${input}" --output "${output}"
cmp "${input}" "${output}"
export DEV_ACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp-value
log="$(bash scripts/deploy/prepare-dev-email.sh --input "${input}" --output "${output}")"
if [[ "${log}" == *"${DEV_ACCOUNT_EMAIL_SMTP_PASSWORD}"* ]]; then
  echo 'SMTP password leaked to output' >&2; exit 1
fi
grep -Fx 'VK_APP_SECRET=fixture' "${output}" >/dev/null
grep -Fx 'ACCOUNT_EMAIL_SMTP_FROM=noreply@notify.neiirohub.ru' "${output}" >/dev/null
grep -Fx 'ACCOUNT_EMAIL_SMTP_HOST=smtp.resend.com' "${output}" >/dev/null
grep -Fx 'ACCOUNT_EMAIL_SMTP_PORT=587' "${output}" >/dev/null
grep -Fx 'ACCOUNT_EMAIL_SMTP_TLS_MODE=starttls' "${output}" >/dev/null
[[ "$(grep -c '^ACCOUNT_EMAIL_SMTP_PASSWORD=' "${output}")" == 1 ]]
grep -Fx "ACCOUNT_EMAIL_SMTP_PASSWORD=${DEV_ACCOUNT_EMAIL_SMTP_PASSWORD}" "${output}" >/dev/null
export DEV_ACCOUNT_EMAIL_SMTP_PASSWORD=$'fixture\ninjected=true'
if bash scripts/deploy/prepare-dev-email.sh --input "${input}" --output "${output}" >/dev/null 2>&1; then
  echo 'Multiline credential accepted' >&2; exit 1
fi
echo 'DEV email configuration tests passed'
