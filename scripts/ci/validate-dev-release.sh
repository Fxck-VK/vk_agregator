#!/usr/bin/env bash
set -euo pipefail
# The checkout and this validation always come from github.sha, not user input.
sha="${DEV_RELEASE_SHA:-${GITHUB_SHA:-}}"
[[ "${GITHUB_REF:-}" == refs/heads/dev-deploy ]] || { echo 'DEV deploy must use refs/heads/dev-deploy.' >&2; exit 1; }
[[ "${GITHUB_EVENT_NAME:-}" == push || "${GITHUB_EVENT_NAME:-}" == workflow_dispatch ]] || { echo 'Unsupported DEV source event.' >&2; exit 1; }
[[ "$sha" =~ ^[0-9a-f]{40}$ && "$sha" == "${GITHUB_SHA:-}" ]] || { echo 'DEV release must match the exact workflow commit.' >&2; exit 1; }
[[ "${GITHUB_REPOSITORY:-}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ && -n "${GH_TOKEN:-}" && -n "${GITHUB_OUTPUT:-}" ]] || { echo 'Missing DEV source validation configuration.' >&2; exit 1; }
response="$("${CURL_BIN:-curl}" --fail --silent --show-error --retry 2 --retry-max-time 20 \
  --connect-timeout 10 --max-time 30 \
  --header "Authorization: Bearer ${GH_TOKEN}" --header 'Accept: application/vnd.github+json' \
  "https://api.github.com/repos/${GITHUB_REPOSITORY}/git/ref/heads/dev-deploy")"
head_sha="$(jq -er '.object.sha | select(type == "string" and test("^[0-9a-f]{40}$"))' <<<"$response")"
printf 'sha=%s\n' "$sha" >>"$GITHUB_OUTPUT"
if [[ "$head_sha" != "$sha" ]]; then
  echo current=false >>"$GITHUB_OUTPUT"
  echo 'DEV branch advanced; superseded release will not touch the VPS.'
else
  echo current=true >>"$GITHUB_OUTPUT"
  echo 'DEV source matches the current exact commit.'
fi
