#!/usr/bin/env bash
set -euo pipefail

branch="dev-deploy"
env_file=".env"
project_name="vk-ai-aggregator-dev"
release_bundle_dir=""
release_env_file=""
docker_config_dir=""
skip_pull="false"
allow_dirty="false"
build_on_vps="false"
skip_migrate="false"
rollback_verified_release="false"
with_cloudflare="false"
pull_base_images="false"
no_health_check="false"
skip_public_smoke="false"
dry_run="false"
timeout_seconds="180"
health_status="skipped"
public_smoke_status="skipped"
deploy_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/deploy-dev.sh [options]

Options:
  --branch <name>              Git branch to deploy, default: dev-deploy
  --env-file <path>            DEV env file, default: .env
  --project-name <name>        Compose project name, default: vk-ai-aggregator-dev
  --release-bundle-dir <path>  Signed release bundle for seven digests (required)
  --skip-pull                  Do not fetch/checkout/pull git
  --allow-dirty                Allow tracked worktree changes before git pull
  --build-on-vps               Rejected: verified digest deploys cannot build on the VPS
  --skip-migrate               Do not run migrate service
  --rollback-verified-release  Allow a previous verified release commit; requires --skip-migrate
  --with-cloudflare            Start cloudflared profile too
  --pull-base-images           Pass --pull to docker compose build
  --no-health-check            Skip local HTTP health checks
  --skip-public-smoke          Skip public DEV Cloudflare smoke after cloudflared startup
  --dry-run                    Verify trust and Compose config without changing runtime state
  --timeout-seconds <seconds>  Health check timeout, default: 180
  -h, --help                   Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch) branch="$2"; shift 2 ;;
    --env-file) env_file="$2"; shift 2 ;;
    --project-name) project_name="$2"; shift 2 ;;
    --release-bundle-dir) release_bundle_dir="$2"; shift 2 ;;
    --skip-pull) skip_pull="true"; shift ;;
    --allow-dirty) allow_dirty="true"; shift ;;
    --build-on-vps) build_on_vps="true"; shift ;;
    --skip-migrate) skip_migrate="true"; shift ;;
    --rollback-verified-release) rollback_verified_release="true"; shift ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --pull-base-images) pull_base_images="true"; shift ;;
    --no-health-check) no_health_check="true"; shift ;;
    --skip-public-smoke) skip_public_smoke="true"; shift ;;
    --dry-run) dry_run="true"; shift ;;
    --timeout-seconds) timeout_seconds="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ "${build_on_vps}" == "true" ]]; then
  echo "--build-on-vps is incompatible with verified digest-only deployment." >&2
  exit 2
fi
if [[ "${rollback_verified_release}" == "true" && "${skip_migrate}" != "true" ]]; then
  echo "--rollback-verified-release requires --skip-migrate; schema rollback is forbidden." >&2
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

if ! command -v flock >/dev/null 2>&1; then
  echo "flock is required to serialize DEV deploys on the VPS" >&2
  exit 1
fi

deploy_lock_file="${DEPLOY_LOCK_FILE:-/tmp/${project_name}.deploy.lock}"
exec 9>"${deploy_lock_file}"
if ! flock -w "${timeout_seconds}" 9; then
  echo "Timed out waiting for DEV deploy lock: ${deploy_lock_file}" >&2
  exit 1
fi
echo "DEV deploy lock acquired: ${deploy_lock_file}"

run_step() {
  echo "==> $*"
  "$@"
}

cleanup_release_env() {
  [[ -z "${release_env_file}" ]] || rm -f -- "${release_env_file}"
  [[ -z "${docker_config_dir:-}" ]] || rm -rf -- "${docker_config_dir}"
}
trap cleanup_release_env EXIT

load_verified_release_env() {
  local line key value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    key="${line%%=*}"
    value="${line#*=}"
    case "${key}" in
      API_IMAGE|WORKER_IMAGE|PROVIDER_WEBHOOK_IMAGE|PROVIDER_BALANCE_BOT_IMAGE|MINIAPP_IMAGE|MIGRATE_IMAGE|BACKUP_IMAGE|RELEASE_COMMIT_SHA|RELEASE_MANIFEST_SHA256|RELEASE_WORKFLOW_IDENTITY)
        printf -v "${key}" '%s' "${value}"
        export "${key}"
        ;;
      *) echo "Unexpected verified release key ${key}." >&2; return 1 ;;
    esac
  done < "$1"
}

verify_compose_release_images() {
  local configured_images expected_image
  configured_images="$("${compose[@]}" --profile backup config --images)"
  for expected_image in "${API_IMAGE}" "${WORKER_IMAGE}" "${PROVIDER_WEBHOOK_IMAGE}" "${PROVIDER_BALANCE_BOT_IMAGE}" "${MINIAPP_IMAGE}" "${MIGRATE_IMAGE}" "${BACKUP_IMAGE}"; do
    grep -Fxq -- "${expected_image}" <<< "${configured_images}" || {
      echo "Compose did not select verified digest ${expected_image}." >&2
      return 1
    }
  done
}

check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "Docker CLI is not installed or not in PATH" >&2
    return 1
  fi
  docker version >/dev/null
  docker compose version >/dev/null
  docker info >/dev/null
  echo "Docker OK: $(docker version --format '{{.Server.Version}}')"
  echo "Docker Compose OK: $(docker compose version --short 2>/dev/null || docker compose version)"
}

check_compose_cli() {
  command -v docker >/dev/null 2>&1 || { echo "Docker CLI is not installed or not in PATH" >&2; return 1; }
  docker compose version >/dev/null
  echo "Docker Compose CLI OK: $(docker compose version --short 2>/dev/null || docker compose version)"
}

get_env_value() {
  local name="$1"
  local default="$2"
  local value
  value="$(grep -E "^${name}=" "${env_file}" | tail -n 1 | cut -d= -f2- || true)"
  if [[ -z "${value}" ]]; then
    echo "${default}"
  else
    value="${value%$'\r'}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    echo "${value}"
  fi
}

is_true_value() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ "${value}" == "1" || "${value}" == "true" || "${value}" == "yes" || "${value}" == "on" ]]
}

is_placeholder_value() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ -z "${value//[[:space:]]/}" || "${value}" == *change_me* || "${value}" == *placeholder* || "${value}" == *example* ]]
}

normalize_data_service_mode() {
  local value
  value="$(printf '%s' "${1:-local}" | tr '[:upper:]' '[:lower:]')"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  case "${value:-local}" in
    local|external|managed) echo "${value:-local}" ;;
    *) echo "Invalid data service mode: ${value}" >&2; exit 1 ;;
  esac
}

get_data_service_mode() {
  local name="$1"
  local default_mode
  default_mode="$(normalize_data_service_mode "$(get_env_value DATA_SERVICES_MODE local)")"
  normalize_data_service_mode "$(get_env_value "${name}" "${default_mode}")"
}

require_value() {
  local name="$1"
  local reason="$2"
  local value
  value="$(get_env_value "${name}" "")"
  if is_placeholder_value "${value}"; then
    echo "Missing/invalid DEV env ${name}: ${reason}" >&2
    exit 1
  fi
}

require_dev_url() {
  local name="$1"
  local prefix="$2"
  local value
  value="$(get_env_value "${name}" "")"
  require_value "${name}" "required for DEV public routing"
  if [[ "${value}" != "${prefix}"* ]]; then
    echo "Invalid DEV env ${name}: expected ${prefix}*, got ${value}" >&2
    exit 1
  fi
}

wait_http() {
  local name="$1"
  local url="$2"
  local deadline=$((SECONDS + timeout_seconds))
  while [[ ${SECONDS} -lt ${deadline} ]]; do
    if curl -fsS --max-time 5 "${url}" >/dev/null 2>&1; then
      echo "${name} OK: ${url}"
      return 0
    fi
    sleep 2
  done
  echo "${name} health check timed out at ${url}" >&2
  return 1
}

validate_dev_env() {
  local app_env payment_provider provider_values provider_values_lc

  app_env="$(get_env_value APP_ENV development | tr '[:upper:]' '[:lower:]')"
  case "${app_env}" in
    production|prod)
      echo "APP_ENV=production is not allowed in DEV deploy" >&2
      exit 1
      ;;
    development|dev|staging|stage|loadtest)
      ;;
    *)
      echo "Unexpected APP_ENV for DEV deploy: ${app_env}" >&2
      exit 1
      ;;
  esac

  for required in \
    DATABASE_URL REDIS_ADDR \
    S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY S3_BUCKET S3_REGION S3_ADDRESSING_STYLE \
    VK_ACCESS_TOKEN VK_SECRET VK_CONFIRMATION_TOKEN VK_GROUP_ID ADMIN_TOKEN; do
    require_value "${required}" "required for DEV server runtime"
  done

  require_dev_url PUBLIC_VK_BASE_URL "https://dev-vk.neiirohub.ru"
  require_dev_url PUBLIC_APP_BASE_URL "https://dev-app.neiirohub.ru"
  require_dev_url PUBLIC_PAYMENT_WEBHOOK_URL "https://dev.neiirohub.ru/billing/webhooks/yookassa"

  if [[ "${with_cloudflare}" == "true" ]]; then
    require_value CLOUDFLARED_TUNNEL_TOKEN "required when deploying DEV with Cloudflare tunnel"
  fi

  if [[ "$(get_data_service_mode POSTGRES_MODE)" == "local" ]]; then
    require_value POSTGRES_PASSWORD "required when POSTGRES_MODE=local"
  fi
  if [[ "$(get_data_service_mode S3_MODE)" == "local" ]]; then
    require_value MINIO_ROOT_USER "required when S3_MODE=local"
    require_value MINIO_ROOT_PASSWORD "required when S3_MODE=local"
  fi

  payment_provider="$(get_env_value PAYMENT_PROVIDER mock | tr '[:upper:]' '[:lower:]')"
  if [[ "${payment_provider}" == "yookassa" ]]; then
    for required in YOOKASSA_SHOP_ID YOOKASSA_SECRET_KEY YOOKASSA_RETURN_URL; do
      require_value "${required}" "required when DEV payment provider is YooKassa"
    done
  fi

  if is_true_value "$(get_env_value PROVIDER_BALANCE_BOT_ENABLED false)"; then
    for required in ALERT_TELEGRAM_BOT_TOKEN TELEGRAM_ADMIN_CHAT_ID; do
      require_value "${required}" "required when PROVIDER_BALANCE_BOT_ENABLED=true"
    done
    provider_balance_checker_configured=false
    if ! is_placeholder_value "$(get_env_value APIMART_API_KEY "")"; then
      provider_balance_checker_configured=true
    fi
    if is_true_value "$(get_env_value POYO_PROVIDER_ENABLED false)"; then
      provider_balance_checker_configured=true
      for required in POYO_API_KEY POYO_BASE_URL; do
        require_value "${required}" "required when PROVIDER_BALANCE_BOT_ENABLED=true and POYO_PROVIDER_ENABLED=true"
      done
    fi
    if is_true_value "$(get_env_value RUNWAY_PROVIDER_ENABLED false)"; then
      provider_balance_checker_configured=true
      for required in RUNWAYML_API_SECRET RUNWAYML_BASE_URL; do
        require_value "${required}" "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
      done
    fi
    if is_true_value "$(get_env_value DEEPINFRA_BALANCE_PROVIDER_ENABLED false)"; then
      provider_balance_checker_configured=true
      for required in DEEPINFRA_API_KEY DEEPINFRA_BALANCE_BASE_URL; do
        require_value "${required}" "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
      done
    fi
    if [[ "${provider_balance_checker_configured}" != "true" ]]; then
      echo "PROVIDER_BALANCE_BOT_ENABLED=true requires at least one provider balance checker" >&2
      exit 1
    fi
  fi

  echo "DEV env check OK: ${env_file} (${app_env})"
}

if [[ ! -f docker-compose.prod.yml ]]; then
  echo "docker-compose.prod.yml not found" >&2
  exit 1
fi
if [[ ! -f docker-compose.data.yml ]]; then
  echo "docker-compose.data.yml not found" >&2
  exit 1
fi
if [[ ! -f "${env_file}" ]]; then
  echo "DEV env file not found: ${env_file}" >&2
  exit 1
fi
if [[ -z "${release_bundle_dir}" || ! -d "${release_bundle_dir}" || -L "${release_bundle_dir}" ]]; then
  echo "--release-bundle-dir must reference a signed non-symlink release bundle." >&2
  exit 2
fi

echo "==> check Docker"
if [[ "${dry_run}" == "true" ]]; then
  check_compose_cli
else
  check_docker
fi
validate_dev_env

if [[ "${skip_pull}" != "true" ]]; then
  if [[ "${allow_dirty}" != "true" ]]; then
    dirty="$(git status --porcelain --untracked-files=no)"
    if [[ -n "${dirty}" ]]; then
      echo "Tracked worktree changes found. Commit/stash them or rerun with --allow-dirty." >&2
      echo "${dirty}" >&2
      exit 1
    fi
  fi
  run_step git fetch --prune origin
  run_step git checkout "${branch}"
  run_step git pull --ff-only origin "${branch}"
fi

checked_out_commit="$(git rev-parse HEAD)"
expected_release_commit="${checked_out_commit}"
if [[ "${rollback_verified_release}" == "true" ]]; then
  stored_release_env="${release_bundle_dir}/release-images.env"
  run_step bash scripts/deploy/validate-release-env.sh --release-env-file "${stored_release_env}"
  expected_release_commit="$(grep -E '^RELEASE_COMMIT_SHA=' "${stored_release_env}" | cut -d= -f2-)"
fi
release_workflow_identity="https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/${branch}"

ghcr_username="$(get_env_value GHCR_USERNAME "")"
ghcr_token="$(get_env_value GHCR_TOKEN "")"
if ! is_placeholder_value "${ghcr_username}" && ! is_placeholder_value "${ghcr_token}"; then
  echo "==> docker login ghcr.io"
  docker_config_dir="$(mktemp -d "${TMPDIR:-/tmp}/vk-ai-aggregator-docker.XXXXXX")"
  chmod 700 "${docker_config_dir}"
  export DOCKER_CONFIG="${docker_config_dir}"
  printf '%s' "${ghcr_token}" | docker login ghcr.io -u "${ghcr_username}" --password-stdin >/dev/null
fi

release_env_file="$(mktemp "${TMPDIR:-/tmp}/vk-ai-aggregator-release.XXXXXX")"
chmod 600 "${release_env_file}"
run_step bash scripts/release/verify-release-bundle.sh \
  --bundle-dir "${release_bundle_dir}" \
  --repository fxck-vk/vk_agregator \
  --commit "${expected_release_commit}" \
  --workflow-identity "${release_workflow_identity}" \
  --output-env "${release_env_file}"
run_step bash scripts/deploy/validate-release-env.sh --release-env-file "${release_env_file}" --expected-commit "${expected_release_commit}"
load_verified_release_env "${release_env_file}"
release_commit="${RELEASE_COMMIT_SHA}"
if [[ "${dry_run}" != "true" ]]; then
  install -m 600 "${release_env_file}" "${release_bundle_dir}/release-images.env"
fi
export APP_ENV_FILE="${env_file}"
echo "Using verified release commit ${release_commit}."

stateful_services=()
if [[ "$(get_data_service_mode POSTGRES_MODE)" == "local" ]]; then
  stateful_services+=(postgres)
fi
if [[ "$(get_data_service_mode REDIS_MODE)" == "local" ]]; then
  stateful_services+=(redis)
fi
if [[ "$(get_data_service_mode S3_MODE)" == "local" ]]; then
  stateful_services+=(minio)
fi

provider_balance_bot_enabled="false"
if is_true_value "$(get_env_value PROVIDER_BALANCE_BOT_ENABLED false)"; then
  provider_balance_bot_enabled="true"
fi

compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" --env-file "${release_env_file}" -f docker-compose.prod.yml)
if [[ ${#stateful_services[@]} -gt 0 ]]; then
  compose+=(-f docker-compose.data.yml)
fi
if [[ "${with_cloudflare}" == "true" ]]; then
  compose+=(--profile cloudflare)
fi
compose+=(--profile provider-balance)

run_step "${compose[@]}" config >/dev/null
run_step verify_compose_release_images
if [[ "${dry_run}" == "true" ]]; then
  echo "DEV deploy dry-run passed for signed release ${release_commit}."
  exit 0
fi

image_pull_services=("${stateful_services[@]}" reverse-proxy)
if [[ "${build_on_vps}" != "true" ]]; then
  image_pull_services+=(api worker maintenance-worker provider-webhook miniapp migrate)
  if [[ "${provider_balance_bot_enabled}" == "true" ]]; then
    image_pull_services+=(provider-balance-bot)
  fi
fi
if [[ "${with_cloudflare}" == "true" ]]; then
  image_pull_services+=(cloudflared)
fi
run_step "${compose[@]}" pull "${image_pull_services[@]}"

if [[ ${#stateful_services[@]} -gt 0 ]]; then
  run_step "${compose[@]}" up -d --no-build --wait --wait-timeout "${timeout_seconds}" "${stateful_services[@]}"
else
  echo "Skipping local stateful containers; data service modes point to external or managed services."
fi

if [[ "${build_on_vps}" == "true" ]]; then
  build_args=(build)
  if [[ "${pull_base_images}" == "true" ]]; then
    build_args+=(--pull)
  fi
  build_args+=(api worker maintenance-worker provider-webhook miniapp migrate)
  if [[ "${provider_balance_bot_enabled}" == "true" ]]; then
    build_args+=(provider-balance-bot)
  fi
  run_step "${compose[@]}" "${build_args[@]}"
else
  echo "Skipping VPS image build; using images pulled from registry."
fi

if [[ "${skip_migrate}" != "true" ]]; then
  run_step bash scripts/deploy/check-migrations-safe.sh --env-file "${env_file}" --migrations-dir "$(get_env_value MIGRATIONS_DIR migrations)"
  "${compose[@]}" rm -f -s migrate >/dev/null 2>&1 || true
  docker rm -f "${project_name}-migrate-1" >/dev/null 2>&1 || true
  migrate_args=(up --no-deps --exit-code-from migrate)
  if [[ "${build_on_vps}" != "true" ]]; then
    migrate_args+=(--no-build)
  fi
  migrate_args+=(migrate)
  run_step "${compose[@]}" "${migrate_args[@]}"
else
  echo "WARNING: skipping migrations. Runtime services still require a successful migrate service state in this compose project." >&2
fi

if [[ "${provider_balance_bot_enabled}" != "true" ]]; then
  "${compose[@]}" rm -f -s provider-balance-bot >/dev/null 2>&1 || true
fi

runtime_services=(api worker maintenance-worker provider-webhook miniapp reverse-proxy)
if [[ "${provider_balance_bot_enabled}" == "true" ]]; then
  runtime_services+=(provider-balance-bot)
fi
if [[ "${with_cloudflare}" == "true" ]]; then
  runtime_services+=(cloudflared)
fi
runtime_up_args=(up -d)
if [[ "${skip_migrate}" == "true" ]]; then
  runtime_up_args+=(--no-deps)
fi
if [[ "${build_on_vps}" != "true" ]]; then
  runtime_up_args+=(--no-build)
fi
runtime_up_args+=("${runtime_services[@]}")
run_step "${compose[@]}" "${runtime_up_args[@]}"
run_step "${compose[@]}" up -d --no-build --force-recreate --no-deps reverse-proxy

if [[ "${no_health_check}" != "true" ]]; then
  reverse_proxy_port="$(get_env_value REVERSE_PROXY_HTTP_PORT 8088)"
  wait_http reverse-proxy "http://127.0.0.1:${reverse_proxy_port}/proxy-health"
  wait_http api "http://127.0.0.1:8080/readyz"
  wait_http provider-webhook "http://127.0.0.1:8082/readyz"
  wait_http worker "http://127.0.0.1:9090/readyz"
  wait_http maintenance-worker "http://127.0.0.1:9091/readyz"
  wait_http miniapp "http://127.0.0.1:5173/"
  health_status="passed"

  if [[ "${with_cloudflare}" == "true" && "${skip_public_smoke}" != "true" ]]; then
    run_step bash scripts/deploy/smoke-dev.sh --env-file "${env_file}" --timeout-seconds "${timeout_seconds}"
    public_smoke_status="passed"
  fi
fi

run_step "${compose[@]}" ps
echo
echo "DEV deploy completed."
echo "Started at: ${deploy_started_at}"
echo "Finished at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Branch: ${branch}"
echo "Commit: $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
echo "Project: ${project_name}"
echo "Env file: ${env_file}"
echo "Verified release bundle: ${release_bundle_dir}"
echo "Release commit: ${release_commit}"
echo "Runtime services: ${runtime_services[*]}"
echo "Migrations: $([[ "${skip_migrate}" == "true" ]] && echo skipped || echo applied)"
echo "Image pull: completed"
echo "Build: $([[ "${build_on_vps}" == "true" ]] && echo "completed on VPS" || echo "skipped; pulled registry images")"
echo "Health checks: ${health_status}"
echo "Public DEV Cloudflare smoke: ${public_smoke_status}"
if [[ "${with_cloudflare}" == "true" ]]; then
  echo "Cloudflare tunnel profile: enabled"
else
  echo "Cloudflare tunnel profile: disabled"
fi
echo "Provider balance bot: ${provider_balance_bot_enabled}"
