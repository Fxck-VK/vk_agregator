#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

prepare_script="scripts/deploy/prepare-dev-env.sh"
check_script="scripts/deploy/check-dev-env.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    printf 'Secret value leaked in %s\n' "${label}" >&2
    exit 1
  fi
}

assert_file_contains() {
  local file="$1"
  local needle="$2"
  if ! grep -Fxq "${needle}" "${file}"; then
    printf 'Expected %s to contain: %s\n' "${file}" "${needle}" >&2
    exit 1
  fi
}

write_common_dev_env() {
  local output="$1"
  local payment_provider="$2"
  cat > "${output}" <<EOF
APP_ENV=development
DEV_EXPECTED_VK_GROUP_ID=239658332
PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/billing/webhooks/yookassa
WEB_ORIGIN=https://dev-web.neiirohub.ru
VK_GROUP_ID=239658332
VK_ACCESS_TOKEN=VK_TEST
VK_SECRET=VK_CB_TEST
VK_CONFIRMATION_TOKEN=VK_CONFIRM_TEST
CLOUDFLARED_TUNNEL_TOKEN=CF_TEST
PAYMENT_PROVIDER=${payment_provider}
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
DEEPINFRA_API_KEY=DI_TEST
APIMART_API_KEY=AM_TEST
APIMART_BASE_URL=https://api.aimlapi.com/v1
POYO_API_KEY=POYO_TEST
POYO_BASE_URL=https://api.poyo.ai
RUNWAYML_API_SECRET=RUNWAY_TEST
RUNWAYML_BASE_URL=https://api.dev.runwayml.com/v1
DEV_ALLOW_REAL_PAYMENTS=false
YOOKASSA_SHOP_ID=dev-test-shop
YOOKASSA_SECRET_KEY=YK_TEST
YOOKASSA_RETURN_URL=https://dev-app.neiirohub.ru/
EOF
}

write_nonvideo_dev_env() {
  local output="$1"
  cat > "${output}" <<EOF
APP_ENV=development
DEV_EXPECTED_VK_GROUP_ID=239658332
PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru
PUBLIC_APP_BASE_URL=https://dev-app.neiirohub.ru
PUBLIC_PAYMENT_WEBHOOK_URL=https://dev.neiirohub.ru/billing/webhooks/yookassa
WEB_ORIGIN=https://dev-web.neiirohub.ru
VK_GROUP_ID=239658332
VK_ACCESS_TOKEN=VK_TEST
VK_SECRET=VK_CB_TEST
VK_CONFIRMATION_TOKEN=VK_CONFIRM_TEST
CLOUDFLARED_TUNNEL_TOKEN=CF_TEST
PAYMENT_PROVIDER=mock
PROVIDER=mock
PROVIDER_CHAIN=mock
IMAGE_PROVIDER=mock
VIDEO_PROVIDER=mock
MEDIA_PIPELINE_ENABLED=false
MEDIA_VIDEO_PROBE_POLICY=disabled
MEDIA_VIDEO_TRANSCODE_POLICY=never
MEDIA_DELIVER_RAW_PROVIDER_VIDEO=always_dev_only
MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,mov,webm
FFPROBE_PATH=/opt/custom/ffprobe
DEV_ALLOW_REAL_PAYMENTS=false
YOOKASSA_SHOP_ID=dev-test-shop
YOOKASSA_SECRET_KEY=YK_TEST
YOOKASSA_RETURN_URL=https://dev-app.neiirohub.ru/
EOF
}

run_valid_case() {
  local name="$1"
  local payment_provider="$2"
  local raw="${tmpdir}/${name}.raw.env"
  local rendered="${tmpdir}/${name}.rendered.env"
  local log

  write_common_dev_env "${raw}" "${payment_provider}"
  if [[ "${payment_provider}" == "yookassa" ]]; then
    {
      echo "DEV_ALLOW_REAL_PAYMENTS=true"
      echo "YOOKASSA_RETURN_URL_MINIAPP=https://dev-app.neiirohub.ru/"
      echo "YOOKASSA_RETURN_URL_VK_BOT=https://dev-vk.neiirohub.ru/payments/return"
    } >> "${raw}"
  fi

  log="$({
    bash "${prepare_script}" \
      --input "${raw}" \
      --output "${rendered}" \
      --image-tag sha-test123 \
      --ghcr-username test-ghcr-user \
      --ghcr-token GHCR_TEST
    bash "${check_script}" --env-file "${rendered}"
  } 2>&1)"

  assert_not_contains "${log}" "VK_TEST" "${name} log"
  assert_not_contains "${log}" "VK_CB_TEST" "${name} log"
  assert_not_contains "${log}" "VK_CONFIRM_TEST" "${name} log"
  assert_not_contains "${log}" "CF_TEST" "${name} log"
  assert_not_contains "${log}" "YK_TEST" "${name} log"
  assert_not_contains "${log}" "GHCR_TEST" "${name} log"
  assert_not_contains "${log}" "DI_TEST" "${name} log"
  assert_not_contains "${log}" "AM_TEST" "${name} log"
  assert_not_contains "${log}" "POYO_TEST" "${name} log"
  assert_not_contains "${log}" "RUNWAY_TEST" "${name} log"

  assert_file_contains "${rendered}" "APIMART_PROVIDER_ENABLED=true"
  assert_file_contains "${rendered}" "IMAGE_TAG=sha-test123"
  assert_file_contains "${rendered}" "BACKUP_IMAGE_TAG=sha-test123"
  assert_file_contains "${rendered}" "WORKER_CPU_LIMIT=1.00"
  assert_file_contains "${rendered}" "POYO_PROVIDER_ENABLED=true"
  assert_file_contains "${rendered}" "RUNWAY_PROVIDER_ENABLED=true"
  assert_file_contains "${rendered}" "VK_MENU_VIDEO_ENABLED=true"
  assert_file_contains "${rendered}" "VK_MENU_IMAGE_ENABLED=true"
  assert_file_contains "${rendered}" "VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_QWEN_IMAGE_3_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_OMNI_1_1_FLASH_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_OMNI_1_1_FLASH_EXT_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_KLING_V3_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_KLING_3_0_TURBO_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_MINIMAX_H3_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_KLING_2_6_MOTION_CONTROL_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_VEO_3_1_FAST_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_VEO_3_1_QUALITY_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_APIMART_VEO_3_1_LITE_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_IMAGE_MODEL_MOCK_ENABLED=false"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTER_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_DEV_MODEL_SMOKE_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED=true"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_MOCK_TEXT_TO_VIDEO_ENABLED=false"
  assert_file_contains "${rendered}" "FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED=false"
  assert_file_contains "${rendered}" "MEDIA_PIPELINE_ENABLED=true"
  assert_file_contains "${rendered}" "MEDIA_VIDEO_PROBE_POLICY=probe_required"
  assert_file_contains "${rendered}" "MEDIA_VIDEO_TRANSCODE_POLICY=never"
  assert_file_contains "${rendered}" "MEDIA_DELIVER_RAW_PROVIDER_VIDEO=if_probe_passed"
  assert_file_contains "${rendered}" "MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,webm"
  assert_file_contains "${rendered}" "FFPROBE_PATH=ffprobe"
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf 'Expected failure did not happen: %s\n' "${label}" >&2
    exit 1
  fi
}

for script in scripts/deploy/*.sh; do
  bash -n "${script}"
done

run_valid_case "mock-dev" "mock"

nonvideo_raw="${tmpdir}/nonvideo.raw.env"
nonvideo_rendered="${tmpdir}/nonvideo.rendered.env"
write_nonvideo_dev_env "${nonvideo_raw}"
bash "${prepare_script}" --input "${nonvideo_raw}" --output "${nonvideo_rendered}" \
  --image-tag sha-test123 --ghcr-username test-ghcr-user --ghcr-token GHCR_TEST >/dev/null
bash "${check_script}" --env-file "${nonvideo_rendered}" >/dev/null
assert_file_contains "${nonvideo_rendered}" "FEATURE_VIDEO_ROUTER_ENABLED=false"
assert_file_contains "${nonvideo_rendered}" "FEATURE_DEV_MODEL_SMOKE_ENABLED=false"
assert_file_contains "${nonvideo_rendered}" "MEDIA_PIPELINE_ENABLED=false"
assert_file_contains "${nonvideo_rendered}" "MEDIA_VIDEO_PROBE_POLICY=disabled"
assert_file_contains "${nonvideo_rendered}" "MEDIA_VIDEO_TRANSCODE_POLICY=never"
assert_file_contains "${nonvideo_rendered}" "MEDIA_DELIVER_RAW_PROVIDER_VIDEO=always_dev_only"
assert_file_contains "${nonvideo_rendered}" "MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,mov,webm"
assert_file_contains "${nonvideo_rendered}" "FFPROBE_PATH=/opt/custom/ffprobe"

for apimart_flag in FEATURE_DEV_MODEL_SMOKE_ENABLED FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED FEATURE_APIMART_QWEN_IMAGE_3_ENABLED FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED FEATURE_APIMART_OMNI_1_1_FLASH_ENABLED FEATURE_APIMART_OMNI_1_1_FLASH_EXT_ENABLED FEATURE_APIMART_KLING_V3_ENABLED FEATURE_APIMART_KLING_3_0_TURBO_ENABLED FEATURE_APIMART_MINIMAX_H3_ENABLED FEATURE_APIMART_KLING_2_6_MOTION_CONTROL_ENABLED FEATURE_APIMART_VEO_3_1_FAST_ENABLED FEATURE_APIMART_VEO_3_1_QUALITY_ENABLED FEATURE_APIMART_VEO_3_1_LITE_ENABLED; do
for apimart_case in disabled missing-key; do
  apimart_raw="${tmpdir}/apimart-${apimart_case}.raw.env"
  apimart_rendered="${tmpdir}/apimart-${apimart_case}.rendered.env"
  write_common_dev_env "${apimart_raw}" mock
  if [[ "${apimart_case}" == disabled ]]; then
    printf '%s=false\n' "${apimart_flag}" >> "${apimart_raw}"
  else
    sed -i '/^APIMART_API_KEY=/d' "${apimart_raw}"
    printf '%s=true\n' "${apimart_flag}" >> "${apimart_raw}"
  fi
  bash "${prepare_script}" --input "${apimart_raw}" --output "${apimart_rendered}" \
    --image-tag sha-test123 --ghcr-username test-ghcr-user --ghcr-token GHCR_TEST >/dev/null
  assert_file_contains "${apimart_rendered}" "${apimart_flag}=false"
done
done
run_valid_case "yookassa-dev" "yookassa"

prod_url_env="${tmpdir}/prod-url.env"
write_common_dev_env "${prod_url_env}" "mock"
sed -i 's#PUBLIC_VK_BASE_URL=https://dev-vk.neiirohub.ru#PUBLIC_VK_BASE_URL=https://vk.neiirohub.ru#' "${prod_url_env}"
expect_failure "prod URL in DEV env" bash "${check_script}" --env-file "${prod_url_env}"

wrong_web_origin_env="${tmpdir}/wrong-web-origin.env"
write_common_dev_env "${wrong_web_origin_env}" "mock"
sed -i 's#WEB_ORIGIN=https://dev-web.neiirohub.ru#WEB_ORIGIN=https://dev-app.neiirohub.ru#' "${wrong_web_origin_env}"
expect_failure "wrong DEV web origin" bash "${check_script}" --env-file "${wrong_web_origin_env}"

disabled_media_pipeline_env="${tmpdir}/disabled-media-pipeline.env"
write_common_dev_env "${disabled_media_pipeline_env}" "mock"
{
  echo "FEATURE_VIDEO_ROUTER_ENABLED=true"
  echo "MEDIA_PIPELINE_ENABLED=false"
  echo "MEDIA_VIDEO_PROBE_POLICY=probe_required"
  echo "MEDIA_VIDEO_TRANSCODE_POLICY=never"
  echo "MEDIA_DELIVER_RAW_PROVIDER_VIDEO=if_probe_passed"
  echo "MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,webm"
  echo "FFPROBE_PATH=ffprobe"
} >> "${disabled_media_pipeline_env}"
expect_failure "video route without DEV media pipeline" bash "${check_script}" --env-file "${disabled_media_pipeline_env}"

missing_ffprobe_env="${tmpdir}/missing-ffprobe.env"
write_common_dev_env "${missing_ffprobe_env}" "mock"
{
  echo "FEATURE_VIDEO_ROUTER_ENABLED=true"
  echo "MEDIA_PIPELINE_ENABLED=true"
  echo "MEDIA_VIDEO_PROBE_POLICY=probe_required"
  echo "MEDIA_VIDEO_TRANSCODE_POLICY=never"
  echo "MEDIA_DELIVER_RAW_PROVIDER_VIDEO=if_probe_passed"
  echo "MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,webm"
  echo "FFPROBE_PATH="
} >> "${missing_ffprobe_env}"
expect_failure "video route without ffprobe path" bash "${check_script}" --env-file "${missing_ffprobe_env}"

wide_video_containers_env="${tmpdir}/wide-video-containers.env"
write_common_dev_env "${wide_video_containers_env}" "mock"
{
  echo "FEATURE_VIDEO_ROUTER_ENABLED=true"
  echo "MEDIA_PIPELINE_ENABLED=true"
  echo "MEDIA_VIDEO_PROBE_POLICY=probe_required"
  echo "MEDIA_VIDEO_TRANSCODE_POLICY=never"
  echo "MEDIA_DELIVER_RAW_PROVIDER_VIDEO=if_probe_passed"
  echo "MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,mov,webm"
  echo "FFPROBE_PATH=ffprobe"
} >> "${wide_video_containers_env}"
expect_failure "video route with broad DEV video containers" bash "${check_script}" --env-file "${wide_video_containers_env}"

echo "DEV deploy env script tests passed"
