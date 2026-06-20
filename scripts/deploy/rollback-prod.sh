#!/usr/bin/env bash
set -euo pipefail

image_tag=""
env_file=".env"
project_name="vk-ai-aggregator-prod"
with_cloudflare="false"
skip_backup="false"
no_health_check="false"
timeout_seconds="180"
rollback_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
backup_status="skipped"
health_status="skipped"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/rollback-prod.sh --image-tag <tag> [options]

Options:
  --image-tag <tag>            Previous known-good Docker image tag to run
  --env-file <path>            Production env file, default: .env
  --project-name <name>        Compose project name, default: vk-ai-aggregator-prod
  --with-cloudflare            Start cloudflared profile too
  --skip-backup                Do not run backup first; use only with a fresh verified backup
  --no-health-check            Skip local HTTP health checks
  --timeout-seconds <seconds>  Health check timeout, default: 180
  -h, --help                   Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image-tag) image_tag="$2"; shift 2 ;;
    --env-file) env_file="$2"; shift 2 ;;
    --project-name) project_name="$2"; shift 2 ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --skip-backup) skip_backup="true"; shift ;;
    --no-health-check) no_health_check="true"; shift ;;
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

normalize_data_service_mode() {
  local value
  value="$(printf '%s' "${1:-local}" | tr '[:upper:]' '[:lower:]')"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  echo "${value:-local}"
}

get_data_service_mode() {
  local name="$1"
  local default_mode
  default_mode="$(normalize_data_service_mode "$(get_env_value DATA_SERVICES_MODE local)")"
  normalize_data_service_mode "$(get_env_value "${name}" "${default_mode}")"
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

if [[ -z "${image_tag}" ]]; then
  echo "--image-tag is required. Use the previous known-good Docker image tag." >&2
  exit 2
fi
if [[ ! -f docker-compose.prod.yml ]]; then
  echo "docker-compose.prod.yml not found" >&2
  exit 1
fi
if [[ ! -f "${env_file}" ]]; then
  echo "Production env file not found: ${env_file}" >&2
  exit 1
fi

echo "Rollback target IMAGE_TAG=${image_tag}"
echo "WARNING: this script does not run migrate down. Schema rollback must be a separate reviewed operation after a verified backup." >&2

run_step check_docker

check_args=(--env-file "${env_file}")
if [[ "${with_cloudflare}" == "true" ]]; then
  check_args+=(--with-cloudflare)
fi
if [[ "${skip_backup}" != "true" ]]; then
  check_args+=(--backup-before-deploy)
fi
run_step bash scripts/deploy/check-prod-env.sh "${check_args[@]}"

export APP_ENV_FILE="${env_file}"
export IMAGE_TAG="${image_tag}"

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

backup_compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml)
if [[ ${#stateful_services[@]} -gt 0 ]]; then
  backup_compose+=(-f docker-compose.data.yml)
fi
backup_compose+=(--profile backup)

run_step "${compose[@]}" config >/dev/null

ghcr_username="$(get_env_value GHCR_USERNAME "")"
ghcr_token="$(get_env_value GHCR_TOKEN "")"
if ! is_placeholder_value "${ghcr_username}" && ! is_placeholder_value "${ghcr_token}"; then
  echo "==> docker login ghcr.io"
  printf '%s' "${ghcr_token}" | docker login ghcr.io -u "${ghcr_username}" --password-stdin >/dev/null
fi

if [[ ${#stateful_services[@]} -gt 0 ]]; then
  run_step "${compose[@]}" pull "${stateful_services[@]}"
  run_step "${compose[@]}" up -d --no-build --wait --wait-timeout "${timeout_seconds}" "${stateful_services[@]}"
else
  echo "Skipping local stateful containers; DATA_SERVICES_MODE/POSTGRES_MODE/REDIS_MODE/S3_MODE point to external or managed services."
fi

if [[ "${skip_backup}" != "true" ]]; then
  run_step "${backup_compose[@]}" pull backup-postgres backup-minio
  run_step "${backup_compose[@]}" run --rm backup-postgres
  run_step "${backup_compose[@]}" run --rm backup-minio
  backup_status="completed"
else
  echo "WARNING: skipping backup before rollback. Use only if a fresh verified backup already exists." >&2
  backup_status="skipped by operator"
fi

rollback_services=(api worker maintenance-worker provider-webhook miniapp reverse-proxy)
if [[ "${with_cloudflare}" == "true" ]]; then
  rollback_services+=(cloudflared)
fi

run_step "${compose[@]}" pull "${rollback_services[@]}"
run_step "${compose[@]}" up -d --no-build --no-deps "${rollback_services[@]}"
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
fi

run_step "${compose[@]}" ps
echo
echo "Production rollback completed."
echo "Started at: ${rollback_started_at}"
echo "Finished at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Project: ${project_name}"
echo "Env file: ${env_file}"
echo "Rollback IMAGE_TAG: ${image_tag}"
echo "Backup before rollback: ${backup_status}"
echo "Migrations: not run; migrate down is intentionally forbidden"
echo "Runtime services: ${rollback_services[*]}"
echo "Health checks: ${health_status}"
echo "Verify payment/referral/job smoke manually before considering the incident closed."
