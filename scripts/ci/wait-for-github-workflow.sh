#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage: wait-for-github-workflow.sh \
  --repository <owner/repository> \
  --workflow <workflow-file> \
  --sha <40-hex> \
  --event <event> \
  [--cancel-superseded-dev-run] \
  [--max-attempts <count>] \
  [--initial-delay-seconds <seconds>] \
  [--max-delay-seconds <seconds>]

The GitHub token is read from GH_TOKEN and is never accepted on the command line.
USAGE
  exit 2
}

repository=""
workflow=""
sha=""
event="push"
max_attempts=60
initial_delay_seconds=5
max_delay_seconds=30
cancel_superseded_dev_run=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repository) repository="${2:-}"; shift 2 ;;
    --workflow) workflow="${2:-}"; shift 2 ;;
    --sha) sha="${2:-}"; shift 2 ;;
    --event) event="${2:-}"; shift 2 ;;
    --cancel-superseded-dev-run) cancel_superseded_dev_run=true; shift ;;
    --max-attempts) max_attempts="${2:-}"; shift 2 ;;
    --initial-delay-seconds) initial_delay_seconds="${2:-}"; shift 2 ;;
    --max-delay-seconds) max_delay_seconds="${2:-}"; shift 2 ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
done

[[ "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || usage
[[ "${workflow}" =~ ^[A-Za-z0-9_.-]+\.ya?ml$ ]] || usage
[[ "${sha}" =~ ^[0-9a-f]{40}$ ]] || usage
[[ "${event}" =~ ^[A-Za-z0-9_-]+$ ]] || usage
[[ "${max_attempts}" =~ ^[1-9][0-9]*$ ]] || usage
[[ "${initial_delay_seconds}" =~ ^[1-9][0-9]*$ ]] || usage
[[ "${max_delay_seconds}" =~ ^[1-9][0-9]*$ ]] || usage
[[ ${initial_delay_seconds} -le ${max_delay_seconds} ]] || usage
[[ -n "${GH_TOKEN:-}" ]] || { echo "GH_TOKEN is required." >&2; exit 2; }

curl_bin="${CURL_BIN:-curl}"
sleep_bin="${SLEEP_BIN:-sleep}"
command -v "${curl_bin}" >/dev/null 2>&1 || { echo "curl command is unavailable: ${curl_bin}" >&2; exit 2; }
command -v jq >/dev/null 2>&1 || { echo "jq is required." >&2; exit 2; }
command -v "${sleep_bin}" >/dev/null 2>&1 || { echo "sleep command is unavailable: ${sleep_bin}" >&2; exit 2; }

# This is deliberately restricted to cancelling this Docker Images DEV push
# run. A cancelled CI for the current head (or a real CI failure) must stay red.
cancel_superseded_run() {
  [[ "${cancel_superseded_dev_run}" == true && "${event}" == push &&
     "${GITHUB_EVENT_NAME:-}" == push &&
     "${GITHUB_REF:-}" == refs/heads/dev-deploy &&
     "${GITHUB_REPOSITORY:-}" == "${repository}" &&
     "${GITHUB_SHA:-}" == "${sha}" &&
     "${GITHUB_RUN_ID:-}" =~ ^[1-9][0-9]*$ &&
     "${GITHUB_WORKFLOW_REF:-}" == "${repository}/.github/workflows/docker-images.yml@refs/heads/dev-deploy" ]] || return 1

  local branch_response head_sha
  if ! branch_response="$("${curl_bin}" \
    --fail-with-body --silent --show-error \
    --retry 4 --retry-all-errors --retry-max-time 30 \
    --connect-timeout 10 --max-time 45 \
    --header "Authorization: Bearer ${GH_TOKEN}" \
    --header "Accept: application/vnd.github+json" \
    --header "X-GitHub-Api-Version: 2022-11-28" \
    "https://api.github.com/repos/${repository}/git/ref/heads/dev-deploy")"; then
    echo "Cannot verify DEV head; cancelled CI remains a blocking failure." >&2
    return 1
  fi
  if ! head_sha="$(jq -er '.object.sha | select(type == "string" and test("^[0-9a-f]{40}$"))' <<<"${branch_response}")"; then
    echo "Invalid DEV head response; cancelled CI remains a blocking failure." >&2
    return 1
  fi
  [[ "${head_sha}" != "${sha}" ]] || return 1

  if ! "${curl_bin}" \
    --fail-with-body --silent --show-error \
    --connect-timeout 10 --max-time 45 \
    --request POST \
    --header "Authorization: Bearer ${GH_TOKEN}" \
    --header "Accept: application/vnd.github+json" \
    --header "X-GitHub-Api-Version: 2022-11-28" \
    --output /dev/null \
    "https://api.github.com/repos/${repository}/actions/runs/${GITHUB_RUN_ID}/cancel"; then
    echo "Cannot cancel superseded image workflow; publication remains blocked." >&2
    return 1
  fi
  echo "Cancellation requested for superseded DEV image workflow; waiting for GitHub to stop this run."
  "${sleep_bin}" 30
  # Never return success: even if cancellation delivery fails, publish must not
  # run without successful CI for this exact commit.
  echo "GitHub did not stop this runner within 30 seconds; publication remains blocked." >&2
  return 1
}

endpoint="https://api.github.com/repos/${repository}/actions/workflows/${workflow}/runs?head_sha=${sha}&event=${event}&per_page=20"
delay_seconds="${initial_delay_seconds}"

for attempt in $(seq 1 "${max_attempts}"); do
  response=""
  if ! response="$("${curl_bin}" \
    --fail-with-body \
    --silent \
    --show-error \
    --retry 4 \
    --retry-all-errors \
    --retry-max-time 30 \
    --connect-timeout 10 \
    --max-time 45 \
    --header "Authorization: Bearer ${GH_TOKEN}" \
    --header "Accept: application/vnd.github+json" \
    --header "X-GitHub-Api-Version: 2022-11-28" \
    "${endpoint}")"; then
    echo "GitHub API request failed (attempt ${attempt}/${max_attempts}); polling will continue." >&2
    status="missing"
    conclusion=""
  else
    status="$(jq -r --arg sha "${sha}" --arg event "${event}" '
      [.workflow_runs[] | select(.head_sha == $sha and .event == $event)]
      | sort_by(.run_number)
      | last
      | .status // "missing"
    ' <<<"${response}")"
    conclusion="$(jq -r --arg sha "${sha}" --arg event "${event}" '
      [.workflow_runs[] | select(.head_sha == $sha and .event == $event)]
      | sort_by(.run_number)
      | last
      | .conclusion // ""
    ' <<<"${response}")"
  fi

  if [[ "${status}" == "completed" && "${conclusion}" == "success" ]]; then
    echo 'Exact-SHA workflow conclusion == "success".'
    exit 0
  fi
  if [[ "${status}" == "completed" ]]; then
    if [[ "${conclusion}" == cancelled ]]; then
      cancel_superseded_run || true
    fi
    echo "Exact-SHA workflow completed with conclusion=${conclusion:-unknown}." >&2
    exit 1
  fi
  if [[ ${attempt} -eq ${max_attempts} ]]; then
    break
  fi

  echo "Waiting for exact-SHA workflow (attempt ${attempt}/${max_attempts}, status=${status}, next_delay=${delay_seconds}s)."
  "${sleep_bin}" "${delay_seconds}"
  delay_seconds=$((delay_seconds * 2))
  if [[ ${delay_seconds} -gt ${max_delay_seconds} ]]; then
    delay_seconds="${max_delay_seconds}"
  fi
done

echo "Timed out waiting for successful workflow for exact commit SHA." >&2
exit 1
