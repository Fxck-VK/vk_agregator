#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf -- "${test_dir}"' EXIT
export RETRY_STATE="${test_dir}/attempts"
cat >"${test_dir}/download" <<'SH'
#!/usr/bin/env bash
count=0
[[ ! -f "$RETRY_STATE" ]] || count="$(cat "$RETRY_STATE")"
echo "$((count + 1))" >"$RETRY_STATE"
case "$1" in
  transient) [[ $count -gt 0 ]] ;;
  hang) sleep 20 ;;
  transaction)
    [[ ! -f "${RETRY_STATE}.lock" ]] || exit 100
    touch "${RETRY_STATE}.lock"
    sleep 20
    ;;
  fail) exit 7 ;;
  cancelled) exit 130 ;;
  arguments) [[ "$2" == 'one argument with spaces' ]] ;;
esac
SH
chmod +x "${test_dir}/download"
run_case() {
  local mode="$1" expected_exit="$2" expected_attempts="$3" attempts="${4:-2}" status=0
  rm -f "${RETRY_STATE}"
  bash "${repo_root}/scripts/ci/retry-network.sh" --attempts "$attempts" --timeout-seconds 1 --delay-seconds 0 -- \
    "${test_dir}/download" "$mode" 'one argument with spaces' >"${test_dir}/log" 2>&1 || status=$?
  [[ "$status" == "$expected_exit" ]] || { cat "${test_dir}/log"; echo "$mode: unexpected exit $status" >&2; exit 1; }
  [[ "$(cat "$RETRY_STATE")" == "$expected_attempts" ]] || { echo "$mode: unexpected retry count" >&2; exit 1; }
}
run_case transient 0 2
run_case hang 124 2
run_case fail 7 2
run_case cancelled 130 1
run_case arguments 0 1
run_case transaction 124 1 1
if bash "${repo_root}/scripts/ci/retry-network.sh" --attempts 0 --timeout-seconds 1 -- true >/dev/null 2>&1; then
  echo 'Invalid limits must fail' >&2; exit 1
fi
echo 'Bounded network retry: downloads, cancellation, arguments and single-attempt package transaction tests passed'
