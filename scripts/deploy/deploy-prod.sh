#!/usr/bin/env bash
set -euo pipefail

branch="main"
env_file=".env"
project_name="vk-ai-aggregator-prod"
image_tag=""
skip_pull="false"
allow_dirty="false"
build_on_vps="false"
skip_migrate="false"
with_cloudflare="false"
backup_before_deploy="false"
pull_base_images="false"
no_health_check="false"
skip_public_smoke="false"
timeout_seconds="180"
health_status="skipped"
public_smoke_status="skipped"
deploy_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/deploy-prod.sh [options]

Options:
  --branch <name>              Git branch to deploy, default: main
  --env-file <path>            Production env file, default: .env
  --project-name <name>        Compose project name, default: vk-ai-aggregator-prod
  --image-tag <tag>            Docker image tag to pull and run, default from env/.env
  --skip-pull                  Do not fetch/checkout/pull git
  --allow-dirty                Allow tracked worktree changes before git pull
  --build-on-vps               Fallback only: build application images on the VPS
  --skip-build                 Deprecated compatibility flag; production deploys skip VPS builds by default
  --skip-migrate               Do not run migrate service
  --with-cloudflare            Start cloudflared profile too
  --backup-before-deploy       Run Postgres and MinIO backup services before rollout
  --pull-base-images           Pass --pull to docker compose build
  --no-health-check            Skip local HTTP health checks
  --skip-public-smoke          Skip public Cloudflare/DNS smoke after cloudflared startup
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
    --skip-build) build_on_vps="false"; shift ;;
    --skip-migrate) skip_migrate="true"; shift ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --backup-before-deploy) backup_before_deploy="true"; shift ;;
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

if [[ ! -f docker-compose.prod.yml ]]; then
  echo "docker-compose.prod.yml not found" >&2
  exit 1
fi
if [[ ! -f "${env_file}" ]]; then
  echo "Server env file not found: ${env_file}. Copy .env.staging.example or .env.prod.example to .env on the server and fill real values there." >&2
  exit 1
fi

echo "==> check Docker"
check_docker

check_args=(--env-file "${env_file}")
if [[ "${with_cloudflare}" == "true" ]]; then
  check_args+=(--with-cloudflare)
fi
if [[ "${backup_before_deploy}" == "true" ]]; then
  check_args+=(--backup-before-deploy)
fi
run_step bash scripts/deploy/check-prod-env.sh "${check_args[@]}"

compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml)
if [[ "${with_cloudflare}" == "true" ]]; then
  compose+=(--profile cloudflare)
fi

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

is_placeholder_value() {
  local value
  value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ -z "${value//[[:space:]]/}" || "${value}" == *change_me* || "${value}" == *placeholder* || "${value}" == *example* ]]
}

get_public_url() {
  local primary_name="$1"
  local legacy_name="$2"
  local default="$3"
  local value
  value="$(get_env_value "${primary_name}" "")"
  if [[ -n "${value}" ]]; then
    echo "${value}"
    return
  fi
  value="$(get_env_value "${legacy_name}" "")"
  if [[ -n "${value}" ]]; then
    echo "${value}"
    return
  fi
  echo "${default}"
}

wait_http() {
  local name="$1"
  local url="$2"
  local deadline=$((SECONDS + timeout_seconds))
  local last_error=""
  while [[ ${SECONDS} -lt ${deadline} ]]; do
    if curl -fsS --max-time 5 "${url}" >/dev/null 2>&1; then
      echo "${name} OK: ${url}"
      return 0
    fi
    last_error="curl failed"
    sleep 2
  done
  echo "${name} health check timed out at ${url} (${last_error})" >&2
  return 1
}

run_public_smoke() {
  local public_vk_url="$1"
  local public_app_url="$2"
  local public_payment_webhook_url="$3"
  local deadline=$((SECONDS + timeout_seconds))
  local attempt=1
  while [[ ${SECONDS} -lt ${deadline} ]]; do
    echo "==> public smoke attempt ${attempt}"
    if bash scripts/deploy/smoke-prod.sh \
      --env-file "${env_file}" \
      --vk-base-url "${public_vk_url}" \
      --app-base-url "${public_app_url}" \
      --payment-webhook-url "${public_payment_webhook_url}" \
      --timeout-seconds "${timeout_seconds}"; then
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 5
  done
  echo "Public Cloudflare/DNS smoke did not pass within ${timeout_seconds}s" >&2
  return 1
}

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

run_step "${compose[@]}" config >/dev/null

ghcr_username="$(get_env_value GHCR_USERNAME "")"
ghcr_token="$(get_env_value GHCR_TOKEN "")"
if ! is_placeholder_value "${ghcr_username}" && ! is_placeholder_value "${ghcr_token}"; then
  echo "==> docker login ghcr.io"
  printf '%s' "${ghcr_token}" | docker login ghcr.io -u "${ghcr_username}" --password-stdin >/dev/null
fi

image_pull_services=(postgres redis minio reverse-proxy)
if [[ "${build_on_vps}" != "true" ]]; then
  image_pull_services+=(api worker provider-webhook miniapp migrate)
  if [[ "${backup_before_deploy}" == "true" ]]; then
    image_pull_services+=(backup-postgres backup-minio)
  fi
fi
if [[ "${with_cloudflare}" == "true" ]]; then
  image_pull_services+=(cloudflared)
fi
run_step "${compose[@]}" pull "${image_pull_services[@]}"

if [[ "${backup_before_deploy}" == "true" ]]; then
  backup_compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml --profile backup)
  if [[ "${with_cloudflare}" == "true" ]]; then
    backup_compose+=(--profile cloudflare)
  fi
  run_step "${backup_compose[@]}" run --rm backup-postgres
  run_step "${backup_compose[@]}" run --rm backup-minio
fi

run_step "${compose[@]}" up -d --no-build --wait --wait-timeout "${timeout_seconds}" postgres redis minio

if [[ "${build_on_vps}" == "true" ]]; then
  build_args=(build)
  if [[ "${pull_base_images}" == "true" ]]; then
    build_args+=(--pull)
  fi
  build_args+=(api worker provider-webhook miniapp migrate)
  if [[ "${backup_before_deploy}" == "true" ]]; then
    build_args+=(backup-postgres backup-minio)
  fi
  run_step "${compose[@]}" "${build_args[@]}"
else
  echo "Skipping VPS image build; using images pulled from registry."
fi

if [[ "${skip_migrate}" != "true" ]]; then
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

runtime_services=(api worker provider-webhook miniapp reverse-proxy)
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

if [[ "${no_health_check}" != "true" ]]; then
  reverse_proxy_port="$(get_env_value REVERSE_PROXY_HTTP_PORT 8088)"
  wait_http reverse-proxy "http://127.0.0.1:${reverse_proxy_port}/proxy-health"
  wait_http api "http://127.0.0.1:8080/health"
  wait_http provider-webhook "http://127.0.0.1:8082/health"
  wait_http worker "http://127.0.0.1:9090/healthz"
  wait_http miniapp "http://127.0.0.1:5173/"
  health_status="passed"

  if [[ "${with_cloudflare}" == "true" && "${skip_public_smoke}" != "true" ]]; then
    public_vk_url="$(get_public_url PUBLIC_VK_BASE_URL VK_BASE_URL https://vk.neiirohub.ru)"
    public_app_url="$(get_public_url PUBLIC_APP_BASE_URL APP_BASE_URL https://app.neiirohub.ru)"
    public_payment_webhook_url="$(get_public_url PUBLIC_PAYMENT_WEBHOOK_URL PAYMENT_WEBHOOK_URL https://neiirohub.ru/billing/webhooks/yookassa)"
    run_public_smoke "${public_vk_url}" "${public_app_url}" "${public_payment_webhook_url}"
    public_smoke_status="passed"
  fi
fi

run_step "${compose[@]}" ps
echo
echo "Production deploy completed."
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
echo "Public Cloudflare/DNS smoke: ${public_smoke_status}"
if [[ "${with_cloudflare}" == "true" ]]; then
  echo "Cloudflare tunnel profile: enabled"
else
  echo "Cloudflare tunnel profile: disabled"
fi
