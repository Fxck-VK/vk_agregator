#!/usr/bin/env bash
set -euo pipefail
input=""; output=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    *) echo 'Unsupported DEV email configuration argument' >&2; exit 2 ;;
  esac
done
if [[ ! -f "${input}" || -z "${output}" || "${input}" == "${output}" ]]; then
  echo 'DEV email configuration needs separate input and output files' >&2; exit 2
fi
umask 077
password="${DEV_ACCOUNT_EMAIL_SMTP_PASSWORD:-}"
if [[ -z "${password}" ]]; then
  cat "${input}" > "${output}"
  exit 0
fi
if [[ "${password}" == *$'\n'* || "${password}" == *$'\r'* ]]; then
  echo 'Invalid SMTP credential format' >&2; exit 1
fi
sed \
  -e '/^ACCOUNT_EMAIL_DELIVERY_PROVIDER=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_HOST=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_PORT=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_USERNAME=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_PASSWORD=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_FROM=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_TLS_MODE=/d' \
  -e '/^ACCOUNT_EMAIL_SMTP_TIMEOUT=/d' "${input}" > "${output}"
{
  printf '\nACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\n'
  printf 'ACCOUNT_EMAIL_SMTP_HOST=smtp.resend.com\nACCOUNT_EMAIL_SMTP_PORT=587\n'
  printf 'ACCOUNT_EMAIL_SMTP_USERNAME=resend\nACCOUNT_EMAIL_SMTP_PASSWORD=%s\n' "${password}"
  printf 'ACCOUNT_EMAIL_SMTP_FROM=noreply@notify.neiirohub.ru\n'
  printf 'ACCOUNT_EMAIL_SMTP_TLS_MODE=starttls\nACCOUNT_EMAIL_SMTP_TIMEOUT=10s\n'
} >> "${output}"
chmod 600 "${output}"
echo 'DEV email delivery configured'
