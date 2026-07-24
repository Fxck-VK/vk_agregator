#!/usr/bin/env bash
set -euo pipefail

max_attempts="3"
attempt_timeout_seconds="240"
initial_delay_seconds="5"
kill_after_seconds="15"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/compose-pull-retry.sh [options] -- <docker compose pull command>

Options:
  --max-attempts <count>              Pull attempts, default: 3
  --attempt-timeout-seconds <seconds> Per-attempt timeout, default: 240
  --initial-delay-seconds <seconds>   Initial retry delay, default: 5
  --kill-after-seconds <seconds>      Force-stop grace period, default: 15
  -h, --help                          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --max-attempts) max_attempts="${2:-}"; shift 2 ;;
    --attempt-timeout-seconds) attempt_timeout_seconds="${2:-}"; shift 2 ;;
    --initial-delay-seconds) initial_delay_seconds="${2:-}"; shift 2 ;;
    --kill-after-seconds) kill_after_seconds="${2:-}"; shift 2 ;;
    --) shift; break ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ ! "${max_attempts}" =~ ^[1-9][0-9]*$ ]] || (( max_attempts > 10 )); then
  echo "max attempts must be between 1 and 10" >&2
  exit 2
fi
if [[ ! "${attempt_timeout_seconds}" =~ ^[1-9][0-9]*$ ]] || (( attempt_timeout_seconds > 3600 )); then
  echo "attempt timeout must be between 1 and 3600 seconds" >&2
  exit 2
fi
if [[ ! "${initial_delay_seconds}" =~ ^[0-9]+$ ]] || (( initial_delay_seconds > 60 )); then
  echo "initial delay must be between 0 and 60 seconds" >&2
  exit 2
fi
if [[ ! "${kill_after_seconds}" =~ ^[1-9][0-9]*$ ]] || (( kill_after_seconds > 60 )); then
  echo "kill-after timeout must be between 1 and 60 seconds" >&2
  exit 2
fi
if [[ $# -eq 0 ]]; then
  echo "docker compose pull command is required after --" >&2
  exit 2
fi
if [[ "${1:-}" != "docker" || "${2:-}" != "compose" ]]; then
  echo "retry command must be docker compose pull" >&2
  exit 2
fi
pull_command_found="false"
for argument in "${@:3}"; do
  if [[ "${argument}" == "pull" ]]; then
    pull_command_found="true"
    break
  fi
done
if [[ "${pull_command_found}" != "true" ]]; then
  echo "retry command must be docker compose pull" >&2
  exit 2
fi
if ! command -v timeout >/dev/null 2>&1; then
  echo "GNU timeout is required for bounded image pulls" >&2
  exit 1
fi

delay_seconds="${initial_delay_seconds}"
for ((attempt = 1; attempt <= max_attempts; attempt++)); do
  echo "==> docker compose pull attempt ${attempt}/${max_attempts}"
  if timeout \
    --foreground \
    --kill-after="${kill_after_seconds}s" \
    "${attempt_timeout_seconds}s" \
    "$@"; then
    echo "docker compose pull succeeded on attempt ${attempt}/${max_attempts}"
    exit 0
  fi

  if (( attempt == max_attempts )); then
    echo "docker compose pull failed after ${max_attempts} attempts" >&2
    exit 1
  fi

  echo "docker compose pull failed or timed out (attempt ${attempt}/${max_attempts}); retrying in ${delay_seconds}s" >&2
  sleep "${delay_seconds}"
  delay_seconds=$((delay_seconds * 2))
done
