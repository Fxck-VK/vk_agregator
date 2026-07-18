#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

bash_checker="scripts/deploy/check-prod-env.sh"
ps_checker="scripts/deploy/check-prod-env.ps1"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

write_base_env() {
  local output="$1"
  cat > "${output}" <<'EOF'
APP_ENV=production
DATA_SERVICES_MODE=managed
POSTGRES_MODE=managed
REDIS_MODE=managed
S3_MODE=managed
MIGRATION_BACKUP_CONFIRMED=true
APP_IMAGE_REGISTRY=ghcr.io/fxck-vk/vk_agregator
IMAGE_TAG=sha-fixture
DATABASE_URL=postgres://db/app
REDIS_ADDR=redis:6379
S3_ENDPOINT=https://storage.invalid
S3_ACCESS_KEY=fixture
S3_SECRET_KEY=fixture
S3_BUCKET=fixture
VK_ACCESS_TOKEN=fixture
VK_SECRET=fixture
VK_CONFIRMATION_TOKEN=prod-fixture
VK_APP_SECRET=fixture
ADMIN_TOKEN=fixture
PAYMENT_PROVIDER=manual
PROVIDER=deepinfra
PROVIDER_CHAIN=deepinfra
IMAGE_PROVIDER=none
VIDEO_PROVIDER=none
DEEPINFRA_API_KEY=fixture
MODERATION_PROVIDER=keyword
ARTIFACT_SCANNER=none
ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=false
EOF
}

run_powershell_checker() {
  local env_file="$1"
  if command -v pwsh >/dev/null 2>&1; then
    pwsh -NoProfile -File "${ps_checker}" -EnvFile "${env_file}"
    return
  fi
  if command -v powershell.exe >/dev/null 2>&1 && command -v wslpath >/dev/null 2>&1; then
    powershell.exe -NoProfile -ExecutionPolicy Bypass \
      -File "$(wslpath -w "${repo_root}/${ps_checker}")" \
      -EnvFile "$(wslpath -w "${env_file}")"
    return
  fi
  echo "PowerShell runtime is required for production env regression tests" >&2
  return 1
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "Expected failure did not happen: ${label}" >&2
    exit 1
  fi
}

base_env="${tmpdir}/base.env"
write_base_env "${base_env}"

expect_failure "bash scanner none without bypass" bash "${bash_checker}" --env-file "${base_env}"
expect_failure "PowerShell scanner none without bypass" run_powershell_checker "${base_env}"

bypass_env="${tmpdir}/bypass.env"
cp "${base_env}" "${bypass_env}"
sed -i 's/ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=false/ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true/' "${bypass_env}"
bash "${bash_checker}" --env-file "${bypass_env}" >/dev/null
run_powershell_checker "${bypass_env}" >/dev/null

openai_env="${tmpdir}/openai.env"
cp "${bypass_env}" "${openai_env}"
sed -i 's/ARTIFACT_SCANNER=none/ARTIFACT_SCANNER=openai/' "${openai_env}"
expect_failure "bash OpenAI scanner without key" bash "${bash_checker}" --env-file "${openai_env}"
expect_failure "PowerShell OpenAI scanner without key" run_powershell_checker "${openai_env}"

echo "Production scanner env regression tests passed"
