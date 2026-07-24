#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
retry_script="${script_dir}/compose-pull-retry.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

mkdir -p "${tmp_dir}/bin"
cat > "${tmp_dir}/bin/timeout" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail

while [[ $# -gt 0 ]]; do
  case "$1" in
    --foreground) shift ;;
    --kill-after=*) shift ;;
    *s) shift; break ;;
    *) echo "unexpected timeout argument: $1" >&2; exit 2 ;;
  esac
done

"$@"
STUB
chmod +x "${tmp_dir}/bin/timeout"

cat > "${tmp_dir}/bin/docker" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail

state_file="${COMPOSE_PULL_TEST_STATE:?}"
failures="${COMPOSE_PULL_TEST_FAILURES:-0}"
attempt=0
if [[ -f "${state_file}" ]]; then
  attempt="$(cat "${state_file}")"
fi
attempt=$((attempt + 1))
printf '%s' "${attempt}" > "${state_file}"

if (( attempt <= failures )); then
  echo "simulated stalled pull" >&2
  exit 124
fi

printf '%s\n' "$*" > "${COMPOSE_PULL_TEST_ARGS:?}"
STUB
chmod +x "${tmp_dir}/bin/docker"

run_pull() {
  local failures="$1"
  local output_file="$2"
  local state_file="$3"
  local args_file="$4"

  PATH="${tmp_dir}/bin:${PATH}" \
    COMPOSE_PULL_TEST_STATE="${state_file}" \
    COMPOSE_PULL_TEST_FAILURES="${failures}" \
    COMPOSE_PULL_TEST_ARGS="${args_file}" \
    bash "${retry_script}" \
      --max-attempts 3 \
      --attempt-timeout-seconds 1 \
      --initial-delay-seconds 0 \
      -- docker compose --project-name test pull api worker >"${output_file}" 2>&1
}

success_output="${tmp_dir}/success.log"
success_state="${tmp_dir}/success.state"
success_args="${tmp_dir}/success.args"
run_pull 2 "${success_output}" "${success_state}" "${success_args}"
[[ "$(cat "${success_state}")" == "3" ]]
[[ "$(cat "${success_args}")" == "compose --project-name test pull api worker" ]]
grep -Fq 'succeeded on attempt 3/3' "${success_output}"

failure_output="${tmp_dir}/failure.log"
failure_state="${tmp_dir}/failure.state"
failure_args="${tmp_dir}/failure.args"
if run_pull 3 "${failure_output}" "${failure_state}" "${failure_args}"; then
  echo "expected bounded compose pull failure" >&2
  exit 1
fi
[[ "$(cat "${failure_state}")" == "3" ]]
grep -Fq 'failed after 3 attempts' "${failure_output}"

invalid_output="${tmp_dir}/invalid.log"
if PATH="${tmp_dir}/bin:${PATH}" bash "${retry_script}" \
  --max-attempts 0 \
  -- docker compose pull api >"${invalid_output}" 2>&1; then
  echo "expected invalid max attempts to fail" >&2
  exit 1
fi
grep -Fq 'max attempts must be between 1 and 10' "${invalid_output}"

wrong_command_output="${tmp_dir}/wrong-command.log"
if PATH="${tmp_dir}/bin:${PATH}" bash "${retry_script}" \
  --max-attempts 1 \
  -- docker run alpine >"${wrong_command_output}" 2>&1; then
  echo "expected non-compose-pull command to fail" >&2
  exit 1
fi
grep -Fq 'retry command must be docker compose pull' "${wrong_command_output}"

echo "compose pull retry tests passed"
