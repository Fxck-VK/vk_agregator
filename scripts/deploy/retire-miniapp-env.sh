#!/usr/bin/env bash
set -euo pipefail

# Normalize a private env file during transport retirement. Never print values.
# Preserve the email-proof pepper previously shared with the retired transport.
input="${1:?input env file required}"
output="${2:?output env file required}"
umask 077
tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT
awk '
  function nonempty(line, value) {
    value=line; sub(/^[^=]*=[[:space:]]*/, "", value)
    gsub(/^[[:space:]\042\047]+|[[:space:]\042\047]+$/, "", value)
    return length(value)>0
  }
  {
    line=$0; sub(/\r$/, "", line)
    key=line; sub(/=.*/, "", key); gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
    if (key=="ACCOUNT_EMAIL_PROOF_SECRET") { if (nonempty(line)) modern=line; next }
    if (key=="VK_APP_SECRET") { if (nonempty(line)) legacy=line; next }
    if (key=="VK_APP_ID" || key=="PUBLIC_APP_BASE_URL" || key=="APP_BASE_URL" ||
        key=="YOOKASSA_RETURN_URL_MINIAPP" || key=="VITE_DEV_LAUNCH_PARAMS" ||
        key ~ /^(MINIAPP_|FEATURE_MINIAPP_|FRONTEND_TELEMETRY_)/) next
    if (key=="PROVIDER_REFERENCE_BASE_URL") {
      if (line ~ /=[[:space:]\042\047]*https:\/\/dev-app\.(epifera\.com|neiirohub\.ru|neirohub\.ru)([\/\042\047[:space:]]|$)/)
        sub(/https:\/\/dev-app\.[^\/\042\047[:space:]]+/, "https://dev-vk.epifera.com", line)
      else if (line ~ /=[[:space:]\042\047]*https:\/\/app\.(epifera\.com|neiirohub\.ru|neirohub\.ru)([\/\042\047[:space:]]|$)/)
        sub(/https:\/\/app\.[^\/\042\047[:space:]]+/, "https://vk.epifera.com", line)
    }
    print line
  }
  END {
    if (modern!="") print modern
    else if (legacy!="") { sub(/VK_APP_SECRET/, "ACCOUNT_EMAIL_PROOF_SECRET", legacy); print legacy }
  }
' "${input}" > "${tmp}"
install -m 600 "${tmp}" "${output}"
