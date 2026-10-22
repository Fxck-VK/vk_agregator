#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
target="${script_dir}/wait-for-github-workflow.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

fake_curl="${tmp_dir}/fake-curl"
fake_sleep="${tmp_dir}/fake-sleep"

cat >"${fake_curl}" <<'FAKE_CURL'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_CURL_STATE}/requests.log"
count_file="${FAKE_CURL_STATE}/count"
count=0
if [[ -f "${count_file}" ]]; then
  count="$(<"${count_file}")"
fi
count=$((count + 1))
printf '%s' "${count}" >"${count_file}"
fixture="${FAKE_CURL_STATE}/${count}"
if [[ -f "${fixture}.exit" ]]; then
  exit "$(<"${fixture}.exit")"
fi
if [[ ! -f "${fixture}.json" ]]; then
  fixture="${FAKE_CURL_STATE}/last.json"
else
  fixture="${fixture}.json"
fi
cat "${fixture}"
FAKE_CURL

cat >"${fake_sleep}" <<'FAKE_SLEEP'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$1" >>"${FAKE_SLEEP_LOG}"
FAKE_SLEEP

chmod +x "${fake_curl}" "${fake_sleep}"

write_response() {
  local path="$1"
  local status="$2"
  local conclusion="$3"
  cat >"${path}" <<JSON
{"workflow_runs":[{"head_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","event":"push","run_number":1,"status":"${status}","conclusion":"${conclusion}"}]}
JSON
}

run_monitor() {
  local state_dir="$1"
  shift
  GH_TOKEN="test-token" \
  CURL_BIN="${fake_curl}" \
  SLEEP_BIN="${fake_sleep}" \
  FAKE_CURL_STATE="${state_dir}" \
  FAKE_SLEEP_LOG="${state_dir}/sleep.log" \
  GITHUB_EVENT_NAME="${TEST_EVENT_NAME:-push}" \
  GITHUB_REF="${TEST_REF:-refs/heads/dev-deploy}" \
  GITHUB_REPOSITORY="${TEST_REPOSITORY:-owner/repository}" \
  GITHUB_SHA="${TEST_SHA:-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa}" \
  GITHUB_RUN_ID="${TEST_RUN_ID:-12345}" \
  GITHUB_WORKFLOW_REF="${TEST_WORKFLOW_REF:-owner/repository/.github/workflows/docker-images.yml@refs/heads/dev-deploy}" \
    bash "${target}" \
      --repository owner/repository \
      --workflow ci.yml \
      --sha aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
      --event push \
      "$@"
}

retry_state="${tmp_dir}/retry"
mkdir -p "${retry_state}"
printf '7' >"${retry_state}/1.exit"
write_response "${retry_state}/2.json" "in_progress" ""
write_response "${retry_state}/3.json" "completed" "success"
retry_output="$(run_monitor "${retry_state}" --max-attempts 4 --initial-delay-seconds 1 --max-delay-seconds 4 2>&1)"
grep -Fq 'GitHub API request failed' <<<"${retry_output}"
grep -Fq 'Exact-SHA workflow conclusion == "success".' <<<"${retry_output}"
printf '1\n2\n' >"${retry_state}/expected-sleep.log"
cmp "${retry_state}/expected-sleep.log" "${retry_state}/sleep.log"

failure_state="${tmp_dir}/failure"
mkdir -p "${failure_state}"
write_response "${failure_state}/1.json" "completed" "failure"
if run_monitor "${failure_state}" --max-attempts 3 >"${failure_state}/output" 2>&1; then
  echo "terminal workflow failure must fail immediately" >&2
  exit 1
fi
grep -Fq 'completed with conclusion=failure' "${failure_state}/output"
if [[ -f "${failure_state}/sleep.log" ]]; then
  echo "terminal workflow failure must not sleep" >&2
  exit 1
fi

timeout_state="${tmp_dir}/timeout"
mkdir -p "${timeout_state}"
write_response "${timeout_state}/last.json" "in_progress" ""
if run_monitor "${timeout_state}" --max-attempts 3 --initial-delay-seconds 1 --max-delay-seconds 4 >"${timeout_state}/output" 2>&1; then
  echo "monitor timeout must fail" >&2
  exit 1
fi
grep -Fq 'Timed out waiting for successful workflow' "${timeout_state}/output"
printf '1\n2\n' >"${timeout_state}/expected-sleep.log"
cmp "${timeout_state}/expected-sleep.log" "${timeout_state}/sleep.log"

write_cancel_fixture() {
  local state_dir="$1"
  local head_sha="$2"
  mkdir -p "${state_dir}"
  write_response "${state_dir}/1.json" "completed" "cancelled"
  printf '{"object":{"sha":"%s"}}\n' "${head_sha}" >"${state_dir}/2.json"
  printf '{}' >"${state_dir}/3.json"
}

assert_no_cancel() {
  if grep -Fq '/actions/runs/12345/cancel' "$1/requests.log"; then
    echo "unexpected workflow cancellation: $1" >&2
    exit 1
  fi
}

superseded_state="${tmp_dir}/superseded"
write_cancel_fixture "${superseded_state}" bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
if run_monitor "${superseded_state}" --cancel-superseded-dev-run >"${superseded_state}/output" 2>&1; then
  echo "a cancellation request must never authorize image publication" >&2
  exit 1
fi
if ! grep -Fq 'https://api.github.com/repos/owner/repository/actions/runs/12345/cancel' "${superseded_state}/requests.log"; then
  echo "superseded cancelled CI must request cancellation of its own image run" >&2
  cat "${superseded_state}/output" >&2
  exit 1
fi
grep -Fq -- '--request POST' "${superseded_state}/requests.log"
grep -Fq 'Cancellation requested for superseded DEV image workflow' "${superseded_state}/output"
if grep -Fq 'conclusion == "success"' "${superseded_state}/output"; then
  echo "cancelled CI must not produce a successful gate" >&2
  exit 1
fi

current_state="${tmp_dir}/current"
write_cancel_fixture "${current_state}" aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
if run_monitor "${current_state}" --cancel-superseded-dev-run >"${current_state}/output" 2>&1; then
  echo "cancelled CI for the current commit must fail" >&2
  exit 1
fi
assert_no_cancel "${current_state}"
grep -Fq 'completed with conclusion=cancelled' "${current_state}/output"

for context in main pr repository sha run_id workflow; do
  context_state="${tmp_dir}/context-${context}"
  write_cancel_fixture "${context_state}" bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
  (
    case "${context}" in
      main) export TEST_REF=refs/heads/main ;;
      pr) export TEST_EVENT_NAME=pull_request ;;
      repository) export TEST_REPOSITORY=another/repository ;;
      sha) export TEST_SHA=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb ;;
      run_id) export TEST_RUN_ID=not-a-run-id ;;
      workflow) export TEST_WORKFLOW_REF=owner/repository/.github/workflows/ci.yml@refs/heads/dev-deploy ;;
    esac
    if run_monitor "${context_state}" --cancel-superseded-dev-run >"${context_state}/output" 2>&1; then
      echo "invalid cancellation context must not pass: ${context}" >&2
      exit 1
    fi
  )
  assert_no_cancel "${context_state}"
  [[ "$(<"${context_state}/count")" -eq 1 ]]
done

for api_error in head_http head_invalid cancel_http; do
  error_state="${tmp_dir}/${api_error}"
  write_cancel_fixture "${error_state}" bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
  case "${api_error}" in
    head_http) printf '22' >"${error_state}/2.exit" ;;
    head_invalid) printf '{"message":"not a branch ref"}' >"${error_state}/2.json" ;;
    cancel_http) printf '22' >"${error_state}/3.exit" ;;
  esac
  if run_monitor "${error_state}" --cancel-superseded-dev-run >"${error_state}/output" 2>&1; then
    echo "cancellation API failure must not authorize publication: ${api_error}" >&2
    exit 1
  fi
  if [[ "${api_error}" != cancel_http ]]; then assert_no_cancel "${error_state}"; fi
done

disabled_state="${tmp_dir}/disabled"
write_cancel_fixture "${disabled_state}" bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
if run_monitor "${disabled_state}" >"${disabled_state}/output" 2>&1; then
  echo "default callers must still reject cancelled CI" >&2
  exit 1
fi
assert_no_cancel "${disabled_state}"
[[ "$(<"${disabled_state}/count")" -eq 1 ]]

failed_opt_in_state="${tmp_dir}/failed-opt-in"
mkdir -p "${failed_opt_in_state}"
write_response "${failed_opt_in_state}/1.json" "completed" "failure"
if run_monitor "${failed_opt_in_state}" --cancel-superseded-dev-run >"${failed_opt_in_state}/output" 2>&1; then
  echo "real CI failure must not be hidden by cancellation" >&2
  exit 1
fi
assert_no_cancel "${failed_opt_in_state}"
[[ "$(<"${failed_opt_in_state}/count")" -eq 1 ]]

success_opt_in_state="${tmp_dir}/success-opt-in"
mkdir -p "${success_opt_in_state}"
write_response "${success_opt_in_state}/1.json" "completed" "success"
run_monitor "${success_opt_in_state}" --cancel-superseded-dev-run >"${success_opt_in_state}/output" 2>&1
assert_no_cancel "${success_opt_in_state}"
[[ "$(<"${success_opt_in_state}/count")" -eq 1 ]]
grep -Fq 'conclusion == "success"' "${success_opt_in_state}/output"

for mismatch in sha event; do
  mismatch_state="${tmp_dir}/mismatched-${mismatch}"
  mkdir -p "${mismatch_state}"
  write_response "${mismatch_state}/1.json" "completed" "success"
  if [[ "${mismatch}" == sha ]]; then
    sed -i 's/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/' "${mismatch_state}/1.json"
  else
    sed -i 's/"event":"push"/"event":"pull_request"/' "${mismatch_state}/1.json"
  fi
  if run_monitor "${mismatch_state}" --cancel-superseded-dev-run --max-attempts 1 >"${mismatch_state}/output" 2>&1; then
    echo "CI for a different ${mismatch} must not authorize publication" >&2
    exit 1
  fi
  assert_no_cancel "${mismatch_state}"
  grep -Fq 'Timed out waiting for successful workflow' "${mismatch_state}/output"
done

echo "GitHub workflow monitor tests passed"
