#!/usr/bin/env bash
set -euo pipefail
input=""; output=""; settings_output=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    --settings-output) settings_output="${2:-}"; shift 2 ;;
    *) echo 'Unsupported DEV Yandex configuration argument' >&2; exit 2 ;;
  esac
done
if [[ ! -f "${input}" || -z "${output}" || "${input}" == "${output}" ]]; then
  echo 'DEV Yandex configuration needs separate input and output files' >&2; exit 2
fi
if [[ -n "${settings_output}" && ( "${settings_output}" == "${input}" || "${settings_output}" == "${output}" ) ]]; then
  echo 'DEV Yandex settings need a separate output file' >&2; exit 2
fi
umask 077
if [[ -n "${settings_output}" ]]; then
  : > "${settings_output}"
  chmod 600 "${settings_output}"
fi
client_id="${DEV_ACCOUNT_OAUTH_YANDEX_CLIENT_ID:-}"
client_secret="${DEV_ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET:-}"
if [[ -z "${client_id}" && -z "${client_secret}" ]]; then
  cat "${input}" > "${output}"
  chmod 600 "${output}"
  exit 0
fi
if [[ ! "${client_id}" =~ ^[A-Fa-f0-9]{32}$ || ! "${client_secret}" =~ ^[A-Za-z0-9_-]{16,256}$ ]]; then
  echo 'Invalid or incomplete DEV Yandex configuration' >&2; exit 1
fi
sed -e '/^ACCOUNT_OAUTH_YANDEX_CLIENT_ID=/d' -e '/^ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=/d' "${input}" > "${output}"
{
  printf '\nACCOUNT_OAUTH_YANDEX_CLIENT_ID=%s\n' "${client_id}"
  printf 'ACCOUNT_WEB_OAUTH_YANDEX_CLIENT_SECRET=%s\n' "${client_secret}"
} >> "${output}"
chmod 600 "${output}"
if [[ -n "${settings_output}" ]]; then
  grep -E '^ACCOUNT_(OAUTH_YANDEX_CLIENT_ID|WEB_OAUTH_YANDEX_CLIENT_SECRET)=' "${output}" > "${settings_output}"
fi
echo 'DEV Yandex login configured'
