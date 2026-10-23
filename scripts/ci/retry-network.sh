#!/usr/bin/env bash
# Only for idempotent dependency downloads, never tests, audits or deployments.
set -euo pipefail
attempts=2
timeout_seconds=90
delay_seconds=5
usage() { echo 'Usage: retry-network.sh [--attempts N] [--timeout-seconds N] [--delay-seconds N] -- command [args...]' >&2; exit 2; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --attempts) attempts="${2:-}"; shift 2 ;;
    --timeout-seconds) timeout_seconds="${2:-}"; shift 2 ;;
    --delay-seconds) delay_seconds="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) usage ;;
  esac
done
[[ "$attempts" =~ ^[1-9][0-9]*$ && "$timeout_seconds" =~ ^[1-9][0-9]*$ && "$delay_seconds" =~ ^(0|[1-9][0-9]*)$ && $# -gt 0 ]] || usage
[[ $attempts -le 3 && $timeout_seconds -le 300 && $delay_seconds -le 30 ]] || usage
trap 'exit 130' INT
trap 'exit 143' TERM
for ((attempt=1; attempt<=attempts; attempt++)); do
  status=0
  timeout --signal=TERM --kill-after=10s "${timeout_seconds}s" "$@" || status=$?
  [[ $status -ne 0 ]] || exit 0
  # Cancellation must stop the pipeline, rather than restart a download.
  [[ $status -ne 130 && $status -ne 143 && $status -ne 137 ]] || exit "$status"
  if [[ $attempt -eq $attempts ]]; then
    echo "Dependency setup failed after ${attempts} bounded attempts (exit ${status})." >&2
    exit "$status"
  fi
  echo "Dependency setup attempt ${attempt}/${attempts} failed (exit ${status}); retrying in ${delay_seconds}s." >&2
  sleep "$delay_seconds"
done
