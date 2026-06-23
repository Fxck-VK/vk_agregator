#!/usr/bin/env bash
set -euo pipefail

branch="dev-deploy"
env_file=".env"
project_name="vk-ai-aggregator-dev"
image_tag=""
skip_pull="false"
allow_dirty="false"
build_on_vps="false"
skip_migrate="false"
with_cloudflare="false"
pull_base_images="false"
no_health_check="false"
skip_public_smoke="false"
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
  --image-tag <tag>            Docker image tag to pull and run, default from env/.env
  --skip-pull                  Do not fetch/checkout/pull git
  --allow-dirty                Allow tracked worktree changes before git pull
  --build-on-vps               Fallback only: build application images on the VPS
  --skip-migrate               Do not run migrate service
  --with-cloudflare            Start cloudflared profile too
  --pull-base-images           Pass --pull to docker compose build
  --no-health-check            Skip local HTTP health checks
  --skip-public-smoke          Skip public DEV Cloudflare smoke after cloudflared startup
  --timeout-seconds <seconds>  Health check timeout, default: 180
  -h, --help                   Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch) branch="$2"; shift 2 ;;
    --env-file) env_file="$2"; shift 2 ;;
    --project-name) project_name="$2"; shift 2 ;;
    --image-tag) image_tag="$2"; shift 2 ;;
    --skip-pull) skip_pull="true"; shift ;;
    --allow-dirty) allow_dirty="true"; shift ;;
    --build-on-vps) build_on_vps="true"; shift ;;
    --skip-migrate) skip_migrate="true"; shift ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --pull-base-images) pull_base_images="true"; shift ;;
    --no-health-check) no_health_check="true"; shift ;;
    --skip-public-smoke) skip_public_smoke="true"; shift ;;
    --timeout-seconds) timeout_seconds="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

run_step() {
  echo "==> $*"
  "$@"
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
    APP_IMAGE_REGISTRY IMAGE_TAG \
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
    if ! is_true_value "$(get_env_value DEV_ALLOW_REAL_PAYMENTS false)"; then
      echo "PAYMENT_PROVIDER=yookassa in DEV requires DEV_ALLOW_REAL_PAYMENTS=true" >&2
      exit 1
    fi
    for required in YOOKASSA_SHOP_ID YOOKASSA_SECRET_KEY YOOKASSA_RETURN_URL; do
      require_value "${required}" "required when DEV payment provider is YooKassa"
    done
  fi

  provider_values="$(get_env_value PROVIDER mock),$(get_env_value IMAGE_PROVIDER mock),$(get_env_value VIDEO_PROVIDER mock)"
  provider_values_lc="$(printf '%s' "${provider_values}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  if [[ "${provider_values_lc}" != "mock,mock,mock" ]] && ! is_true_value "$(get_env_value DEV_ALLOW_REAL_AI_PROVIDERS false)"; then
    echo "Real AI providers in DEV require DEV_ALLOW_REAL_AI_PROVIDERS=true" >&2
    exit 1
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

echo "==> check Docker"
check_docker
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

export APP_ENV_FILE="${env_file}"
if [[ -n "${image_tag}" ]]; then
  export IMAGE_TAG="${image_tag}"
  echo "Using IMAGE_TAG=${image_tag}"
fi

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

compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml)
if [[ ${#stateful_services[@]} -gt 0 ]]; then
  compose+=(-f docker-compose.data.yml)
fi
if [[ "${with_cloudflare}" == "true" ]]; then
  compose+=(--profile cloudflare)
fi

run_step "${compose[@]}" config >/dev/null

ghcr_username="$(get_env_value GHCR_USERNAME "")"
ghcr_token="$(get_env_value GHCR_TOKEN "")"
if ! is_placeholder_value "${ghcr_username}" && ! is_placeholder_value "${ghcr_token}"; then
  echo "==> docker login ghcr.io"
  printf '%s' "${ghcr_token}" | docker login ghcr.io -u "${ghcr_username}" --password-stdin >/dev/null
fi

image_pull_services=("${stateful_services[@]}" reverse-proxy)
if [[ "${build_on_vps}" != "true" ]]; then
  image_pull_services+=(api worker maintenance-worker provider-webhook miniapp migrate)
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
  run_step "${compose[@]}" "${build_args[@]}"
else
  echo "Skipping VPS image build; using images pulled from registry."
fi

if [[ "${skip_migrate}" != "true" ]]; then
  run_step bash scripts/deploy/check-migrations-safe.sh --env-file "${env_file}" --migrations-dir "$(get_env_value MIGRATIONS_DIR migrations)"
  "${compose[@]}" rm -f -s migrate >/dev/null 2>&1 || true
  migrate_args=(up --no-deps --exit-code-from migrate)
  if [[ "${build_on_vps}" != "true" ]]; then
    migrate_args+=(--no-build)
  fi
  migrate_args+=(migrate)
  run_step "${compose[@]}" "${migrate_args[@]}"
else
  echo "WARNING: skipping migrations. Runtime services still require a successful migrate service state in this compose project." >&2
fi

runtime_services=(api worker maintenance-worker provider-webhook miniapp reverse-proxy)
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
