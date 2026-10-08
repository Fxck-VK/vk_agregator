#!/usr/bin/env bash
set -euo pipefail
input=""; output=""; settings_output=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    --settings-output) settings_output="${2:-}"; shift 2 ;;
    *) echo 'Unsupported DEV Google configuration argument' >&2; exit 2 ;;
  esac
done
if [[ ! -f "${input}" || -z "${output}" || "${input}" == "${output}" ]]; then
  echo 'DEV Google configuration needs separate input and output files' >&2; exit 2
fi
if [[ -n "${settings_output}" && ( "${settings_output}" == "${input}" || "${settings_output}" == "${output}" ) ]]; then
  echo 'DEV Google settings need a separate output file' >&2; exit 2
fi
umask 077
if [[ -n "${settings_output}" ]]; then
  : > "${settings_output}"
  chmod 600 "${settings_output}"
fi
client_ids="${DEV_ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS:-}"
client_secret="${DEV_ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET:-}"
if [[ -z "${client_ids}" && -z "${client_secret}" ]]; then
  cat "${input}" > "${output}"
  exit 0
fi
if [[ -z "${client_ids}" || ! "${client_secret}" =~ ^[A-Za-z0-9_-]+$ || ! "${client_ids}" =~ ^[A-Za-z0-9_-]+\.apps\.googleusercontent\.com(,[A-Za-z0-9_-]+\.apps\.googleusercontent\.com)*$ ]]; then
  echo 'Invalid or incomplete DEV Google configuration' >&2; exit 1
fi
existing_ids="$(sed -n 's/^ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS=//p' "${input}" | tail -n 1 | tr -d '\r')"
client_ids="$(printf '%s,%s' "${client_ids}" "${existing_ids}" | awk -F, '{ for (i=1; i<=NF; i++) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", $i); if ($i != "" && !seen[$i]++) { if (count++) printf ","; printf "%s", $i } } }')"
if [[ ! "${client_ids}" =~ ^[A-Za-z0-9_-]+\.apps\.googleusercontent\.com(,[A-Za-z0-9_-]+\.apps\.googleusercontent\.com)*$ ]]; then
  echo 'Invalid existing DEV Google client IDs' >&2; exit 1
fi
sed -e '/^ACCOUNT_OAUTH_GOOGLE_CLIENT_IDS=/d' -e '/^ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=/d' "${input}" > "${output}"
{
  printf '\nACCOUNT_OAUTH_GOOGLE_CLIENT_IDS=%s\n' "${client_ids}"
  printf 'ACCOUNT_WEB_OAUTH_GOOGLE_CLIENT_SECRET=%s\n' "${client_secret}"
} >> "${output}"
chmod 600 "${output}"
if [[ -n "${settings_output}" ]]; then
  grep -E '^ACCOUNT_(OAUTH_GOOGLE_CLIENT_IDS|WEB_OAUTH_GOOGLE_CLIENT_SECRET)=' "${output}" > "${settings_output}"
fi
echo 'DEV Google login configured'
