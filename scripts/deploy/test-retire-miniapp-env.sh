#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
script=scripts/deploy/retire-miniapp-env.sh
cat > "${tmpdir}/old.env" <<'EOF'
VK_APP_ID=fixture-app
VK_APP_SECRET='fixture # proof key'
PUBLIC_VK_BASE_URL=https://dev-vk.epifera.com
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.epifera.com/billing/webhooks/yookassa
MINIAPP_JOB_RATE_LIMIT_RPS=1
FEATURE_MINIAPP_PAID_TEXT_MODELS_ENABLED=true
PUBLIC_APP_BASE_URL=https://dev-app.epifera.com
YOOKASSA_RETURN_URL_MINIAPP=https://vk.com/app123
PROVIDER_REFERENCE_BASE_URL="https://dev-app.epifera.com"
ACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp
VK_SECRET=fixture-bot
ACCOUNT_OAUTH_VK_ID_CLIENT_IDS=123
YOOKASSA_SECRET_KEY=fixture-payment
YOOKASSA_RETURN_URL_VK_BOT=https://vk.com/im
EOF
bash "${script}" "${tmpdir}/old.env" "${tmpdir}/new.env"
grep -Fx "ACCOUNT_EMAIL_PROOF_SECRET='fixture # proof key'" "${tmpdir}/new.env" >/dev/null
grep -Fx 'PROVIDER_REFERENCE_BASE_URL="https://dev-vk.epifera.com"' "${tmpdir}/new.env" >/dev/null
for line in ACCOUNT_EMAIL_SMTP_PASSWORD=fixture-smtp VK_SECRET=fixture-bot ACCOUNT_OAUTH_VK_ID_CLIENT_IDS=123 YOOKASSA_SECRET_KEY=fixture-payment YOOKASSA_RETURN_URL_VK_BOT=https://vk.com/im; do
  grep -Fx "${line}" "${tmpdir}/new.env" >/dev/null
done
! grep -Eq '^(VK_APP_|MINIAPP_|FEATURE_MINIAPP_|PUBLIC_APP_BASE_URL=|YOOKASSA_RETURN_URL_MINIAPP=)' "${tmpdir}/new.env"
bash scripts/deploy/prepare-dev-env.sh --input "${tmpdir}/old.env" --output "${tmpdir}/prepared.env" \
  --image-tag sha-fixture --ghcr-username fixture --ghcr-token fixture >/dev/null
grep -Fx "ACCOUNT_EMAIL_PROOF_SECRET='fixture # proof key'" "${tmpdir}/prepared.env" >/dev/null
printf '%s\n' 'ACCOUNT_EMAIL_PROOF_SECRET="fixture-dedicated"' >> "${tmpdir}/old.env"
bash "${script}" "${tmpdir}/old.env" "${tmpdir}/preferred.env"
grep -Fx 'ACCOUNT_EMAIL_PROOF_SECRET="fixture-dedicated"' "${tmpdir}/preferred.env" >/dev/null
[[ "$(grep -c '^ACCOUNT_EMAIL_PROOF_SECRET=' "${tmpdir}/preferred.env")" == 1 ]]
bash "${script}" "${tmpdir}/preferred.env" "${tmpdir}/again.env"
cmp "${tmpdir}/preferred.env" "${tmpdir}/again.env"
printf '%s\n' 'PROVIDER_REFERENCE_BASE_URL=https://app.epifera.com' > "${tmpdir}/prod.env"
bash "${script}" "${tmpdir}/prod.env" "${tmpdir}/prod.env"
grep -Fx 'PROVIDER_REFERENCE_BASE_URL=https://vk.epifera.com' "${tmpdir}/prod.env" >/dev/null
printf '%s\n' 'PROVIDER_REFERENCE_BASE_URL=https://dev-app.epifera.com.attacker.invalid' > "${tmpdir}/external.env"
bash "${script}" "${tmpdir}/external.env" "${tmpdir}/preserved.env"
cmp "${tmpdir}/external.env" "${tmpdir}/preserved.env"
echo 'Mini App env retirement: proof-key continuity, modern-key precedence, contour-specific references and unrelated settings passed.'
