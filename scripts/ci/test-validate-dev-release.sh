#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf -- "${test_dir}"' EXIT
export GITHUB_REPOSITORY=example/repository GITHUB_REF=refs/heads/dev-deploy GITHUB_EVENT_NAME=push
export GITHUB_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa DEV_RELEASE_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
export GH_TOKEN=fixture-not-a-real-token GITHUB_OUTPUT="${test_dir}/output"
export CURL_BIN="${test_dir}/curl" FIXTURE_HEAD="$GITHUB_SHA" FIXTURE_CALLS="${test_dir}/calls"
cat >"$CURL_BIN" <<'SH'
#!/usr/bin/env bash
echo call >>"$FIXTURE_CALLS"
printf '{"object":{"sha":"%s"}}\n' "$FIXTURE_HEAD"
SH
chmod +x "$CURL_BIN"
run_source() { : >"$GITHUB_OUTPUT"; bash "${repo_root}/scripts/ci/validate-dev-release.sh" >"${test_dir}/log" 2>&1; }
run_source
grep -qx current=true "$GITHUB_OUTPUT"
grep -qx "sha=$GITHUB_SHA" "$GITHUB_OUTPUT"
GITHUB_EVENT_NAME=workflow_dispatch run_source
grep -qx current=true "$GITHUB_OUTPUT"
DEV_RELEASE_SHA='' run_source
grep -qx "sha=$GITHUB_SHA" "$GITHUB_OUTPUT"
FIXTURE_HEAD=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb run_source
grep -qx current=false "$GITHUB_OUTPUT"
for field in GITHUB_REF GITHUB_EVENT_NAME DEV_RELEASE_SHA; do
  if env "$field=invalid" bash "${repo_root}/scripts/ci/validate-dev-release.sh" >/dev/null 2>&1; then
    echo "Unsafe $field accepted" >&2; exit 1
  fi
done
if DEV_RELEASE_SHA=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb run_source; then echo 'Different commit accepted' >&2; exit 1; fi
if FIXTURE_HEAD=malformed run_source; then echo 'Malformed API response accepted' >&2; exit 1; fi
if grep -q fixture-not-a-real-token "${test_dir}/log" "$GITHUB_OUTPUT"; then echo 'Token leaked' >&2; exit 1; fi
echo 'DEV source selection: exact commit, manual, wrong source and stale source tests passed'
