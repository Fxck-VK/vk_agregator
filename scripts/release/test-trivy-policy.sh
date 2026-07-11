#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
digest="$(printf 'a%.0s' {1..64})"
image_ref="ghcr.io/example/project/api@sha256:${digest}"

printf '{"SchemaVersion":2,"ArtifactName":"%s","Results":[{"Vulnerabilities":null}]}\n' "${image_ref}" > "${tmp}/clean.json"
bash "${root}/scripts/release/enforce-trivy-policy.sh" "${tmp}/clean.json" "${image_ref}" >/dev/null

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "Expected Trivy policy failure: ${label}" >&2
    exit 1
  fi
}

for severity in HIGH CRITICAL; do
  printf '{"SchemaVersion":2,"ArtifactName":"%s","Results":[{"Vulnerabilities":[{"VulnerabilityID":"TEST-1","Severity":"%s"}]}]}\n' "${image_ref}" "${severity}" > "${tmp}/${severity}.json"
  expect_failure "${severity}" bash "${root}/scripts/release/enforce-trivy-policy.sh" "${tmp}/${severity}.json" "${image_ref}"
done

other_ref="ghcr.io/example/project/api@sha256:$(printf 'b%.0s' {1..64})"
expect_failure "digest mismatch" bash "${root}/scripts/release/enforce-trivy-policy.sh" "${tmp}/clean.json" "${other_ref}"

echo "Trivy clean/HIGH/CRITICAL/digest policy tests OK"
