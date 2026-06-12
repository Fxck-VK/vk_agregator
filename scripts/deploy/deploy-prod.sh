#!/usr/bin/env bash
set -euo pipefail

branch="main"
env_file=".env"
project_name="vk-ai-aggregator-prod"
image_tag=""
skip_pull="false"
allow_dirty="false"
skip_build="false"
skip_migrate="false"
with_cloudflare="false"
backup_before_deploy="false"
pull_base_images="false"
no_health_check="false"
timeout_seconds="180"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/deploy-prod.sh [options]

Options:
  --branch <name>              Git branch to deploy, default: main
  --env-file <path>            Production env file, default: .env
  --project-name <name>        Compose project name, default: vk-ai-aggregator-prod
  --image-tag <tag>            Docker image tag to build and run, default from env/.env or prod
  --skip-pull                  Do not fetch/checkout/pull git
  --allow-dirty                Allow tracked worktree changes before git pull
  --skip-build                 Do not run docker compose build
  --skip-migrate               Do not run migrate service
  --with-cloudflare            Start cloudflared profile too
  --backup-before-deploy       Run Postgres and MinIO backup services before rollout
  --pull-base-images           Pass --pull to docker compose build
  --no-health-check            Skip local HTTP health checks
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
    --skip-build) skip_build="true"; shift ;;
    --skip-migrate) skip_migrate="true"; shift ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --backup-before-deploy) backup_before_deploy="true"; shift ;;
    --pull-base-images) pull_base_images="true"; shift ;;
    --no-health-check) no_health_check="true"; shift ;;
    --timeout-seconds) timeout_seconds="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

if [[ ! -f docker-compose.prod.yml ]]; then
  echo "docker-compose.prod.yml not found" >&2
  exit 1
fi
if [[ ! -f "${env_file}" ]]; then
  echo "Production env file not found: ${env_file}. Copy .env.prod.example to .env on the server and fill real secrets there." >&2
  exit 1
fi

compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml)
if [[ "${with_cloudflare}" == "true" ]]; then
  compose+=(--profile cloudflare)
fi

run_step() {
  echo "==> $*"
  "$@"
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

if [[ "${backup_before_deploy}" == "true" ]]; then
  backup_compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml --profile backup)
  if [[ "${with_cloudflare}" == "true" ]]; then
    backup_compose+=(--profile cloudflare)
  fi
  run_step "${backup_compose[@]}" run --rm backup-postgres
  run_step "${backup_compose[@]}" run --rm backup-minio
fi

run_step "${compose[@]}" up -d postgres redis minio

if [[ "${skip_build}" != "true" ]]; then
  build_args=(build)
  if [[ "${pull_base_images}" == "true" ]]; then
    build_args+=(--pull)
  fi
  build_args+=(api worker provider-webhook miniapp migrate)
  if [[ "${backup_before_deploy}" == "true" ]]; then
    build_args+=(backup-postgres backup-minio)
  fi
  run_step "${compose[@]}" "${build_args[@]}"
fi

if [[ "${skip_migrate}" != "true" ]]; then
  "${compose[@]}" rm -f -s migrate >/dev/null 2>&1 || true
  run_step "${compose[@]}" up --no-deps --exit-code-from migrate migrate
else
  echo "WARNING: skipping migrations. Runtime services still require a successful migrate service state in this compose project." >&2
fi

runtime_services=(api worker provider-webhook miniapp reverse-proxy)
if [[ "${with_cloudflare}" == "true" ]]; then
  runtime_services+=(cloudflared)
fi
run_step "${compose[@]}" up -d "${runtime_services[@]}"

if [[ "${no_health_check}" != "true" ]]; then
  reverse_proxy_port="$(get_env_value REVERSE_PROXY_HTTP_PORT 8088)"
  wait_http reverse-proxy "http://127.0.0.1:${reverse_proxy_port}/proxy-health"
  wait_http api "http://127.0.0.1:8080/health"
  wait_http provider-webhook "http://127.0.0.1:8082/health"
  wait_http worker "http://127.0.0.1:9090/healthz"
  wait_http miniapp "http://127.0.0.1:5173/"
fi

run_step "${compose[@]}" ps
echo
echo "Production deploy completed."
