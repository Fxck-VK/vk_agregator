#!/usr/bin/env bash
set -euo pipefail

registry=""
username=""
max_attempts="3"
initial_delay_seconds="5"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/docker-login-retry.sh --registry <host> --username <name> [options]

Reads the registry token from standard input.

Options:
  --registry <host>                  Registry hostname
  --username <name>                  Registry username
  --max-attempts <count>             Login attempts, default: 3
  --initial-delay-seconds <seconds>  Initial retry delay, default: 5
  -h, --help                         Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --registry) registry="${2:-}"; shift 2 ;;
    --username) username="${2:-}"; shift 2 ;;
    --max-attempts) max_attempts="${2:-}"; shift 2 ;;
    --initial-delay-seconds) initial_delay_seconds="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "${registry}" || -z "${username}" ]]; then
  usage >&2
  exit 2
fi
if [[ ! "${max_attempts}" =~ ^[1-9][0-9]*$ ]] || (( max_attempts > 10 )); then
  echo "max attempts must be between 1 and 10" >&2
  exit 2
fi
if [[ ! "${initial_delay_seconds}" =~ ^[0-9]+$ ]] || (( initial_delay_seconds > 60 )); then
  echo "initial delay must be between 0 and 60 seconds" >&2
  exit 2
fi

token="$(cat)"
if [[ -z "${token}" ]]; then
  echo "registry token is required on standard input" >&2
  exit 2
fi

delay_seconds="${initial_delay_seconds}"
for ((attempt = 1; attempt <= max_attempts; attempt++)); do
  if printf '%s' "${token}" | docker login "${registry}" -u "${username}" --password-stdin >/dev/null; then
    unset token
    exit 0
  fi

  if (( attempt == max_attempts )); then
    unset token
    echo "docker login ${registry} failed after ${max_attempts} attempts" >&2
    exit 1
  fi

  echo "docker login ${registry} failed (attempt ${attempt}/${max_attempts}); retrying in ${delay_seconds}s" >&2
  sleep "${delay_seconds}"
  delay_seconds=$((delay_seconds * 2))
done
