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
PROVIDER_CHAIN=deepinfra,poyo,runway
IMAGE_PROVIDER=none
VIDEO_PROVIDER=none
DEEPINFRA_API_KEY=fixture
FEATURE_VIDEO_ROUTER_ENABLED=true
FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=false
FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=false
FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=true
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=true
FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=true
FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=true
POYO_PROVIDER_ENABLED=true
POYO_API_KEY=fixture
POYO_BASE_URL=https://poyo.invalid
RUNWAY_PROVIDER_ENABLED=true
RUNWAYML_API_SECRET=fixture
RUNWAYML_BASE_URL=https://runway.invalid
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
  if command -v powershell.exe >/dev/null 2>&1 && command -v cygpath >/dev/null 2>&1; then
    powershell.exe -NoProfile -ExecutionPolicy Bypass \
      -File "$(cygpath -w "${repo_root}/${ps_checker}")" \
      -EnvFile "$(cygpath -w "${env_file}")"
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

kling_disabled_env="${tmpdir}/kling-disabled.env"
sed 's/^FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=true$/FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=false/' "${bypass_env}" > "${kling_disabled_env}"
expect_failure "bash Kling O3 disabled" bash "${bash_checker}" --env-file "${kling_disabled_env}"
expect_failure "PowerShell Kling O3 disabled" run_powershell_checker "${kling_disabled_env}"

hailuo_enabled_env="${tmpdir}/hailuo-enabled.env"
sed 's/^FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=false$/FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=true/' "${bypass_env}" > "${hailuo_enabled_env}"
expect_failure "bash Hailuo enabled" bash "${bash_checker}" --env-file "${hailuo_enabled_env}"
expect_failure "PowerShell Hailuo enabled" run_powershell_checker "${hailuo_enabled_env}"

missing_poyo_key_env="${tmpdir}/missing-poyo-key.env"
sed '/^POYO_API_KEY=/d' "${bypass_env}" > "${missing_poyo_key_env}"
expect_failure "bash PoYo key missing" bash "${bash_checker}" --env-file "${missing_poyo_key_env}"
expect_failure "PowerShell PoYo key missing" run_powershell_checker "${missing_poyo_key_env}"

missing_runway_secret_env="${tmpdir}/missing-runway-secret.env"
sed '/^RUNWAYML_API_SECRET=/d' "${bypass_env}" > "${missing_runway_secret_env}"
expect_failure "bash Runway secret missing" bash "${bash_checker}" --env-file "${missing_runway_secret_env}"
expect_failure "PowerShell Runway secret missing" run_powershell_checker "${missing_runway_secret_env}"

openai_env="${tmpdir}/openai.env"
cp "${bypass_env}" "${openai_env}"
sed -i 's/ARTIFACT_SCANNER=none/ARTIFACT_SCANNER=openai/' "${openai_env}"
expect_failure "bash OpenAI scanner without key" bash "${bash_checker}" --env-file "${openai_env}"
expect_failure "PowerShell OpenAI scanner without key" run_powershell_checker "${openai_env}"

echo "Production scanner and video profile env regression tests passed"
