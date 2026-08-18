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

echo "GitHub workflow monitor tests passed"
