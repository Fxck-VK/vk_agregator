#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
login_script="${script_dir}/docker-login-retry.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

mkdir -p "${tmp_dir}/bin"
cat > "${tmp_dir}/bin/docker" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail

state_file="${DOCKER_LOGIN_TEST_STATE:?}"
failures="${DOCKER_LOGIN_TEST_FAILURES:-0}"
attempt=0
if [[ -f "${state_file}" ]]; then
  attempt="$(cat "${state_file}")"
fi
attempt=$((attempt + 1))
printf '%s' "${attempt}" > "${state_file}"

token="$(cat)"
if [[ -z "${token}" ]]; then
  echo "missing token" >&2
  exit 2
fi

if (( attempt <= failures )); then
  echo "simulated registry timeout" >&2
  exit 1
fi
STUB
chmod +x "${tmp_dir}/bin/docker"

run_login() {
  local failures="$1"
  local output_file="$2"
  local state_file="$3"

  PATH="${tmp_dir}/bin:${PATH}" \
    DOCKER_LOGIN_TEST_STATE="${state_file}" \
    DOCKER_LOGIN_TEST_FAILURES="${failures}" \
    printf '%s' 'test-canary-token' |
    PATH="${tmp_dir}/bin:${PATH}" \
      DOCKER_LOGIN_TEST_STATE="${state_file}" \
      DOCKER_LOGIN_TEST_FAILURES="${failures}" \
      bash "${login_script}" \
        --registry ghcr.io \
        --username test-user \
        --max-attempts 3 \
        --initial-delay-seconds 0 >"${output_file}" 2>&1
}

success_output="${tmp_dir}/success.log"
success_state="${tmp_dir}/success.state"
run_login 2 "${success_output}" "${success_state}"
[[ "$(cat "${success_state}")" == "3" ]]
! grep -Fq 'test-canary-token' "${success_output}"

failure_output="${tmp_dir}/failure.log"
failure_state="${tmp_dir}/failure.state"
if run_login 3 "${failure_output}" "${failure_state}"; then
  echo "expected bounded registry login failure" >&2
  exit 1
fi
[[ "$(cat "${failure_state}")" == "3" ]]
grep -Fq 'failed after 3 attempts' "${failure_output}"
! grep -Fq 'test-canary-token' "${failure_output}"

echo "docker login retry tests passed"
