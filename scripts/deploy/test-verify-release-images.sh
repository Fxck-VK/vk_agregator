#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
verifier="${repo_root}/scripts/deploy/verify-release-images.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

mock_bin="${tmp_dir}/bin"
mkdir -p "${mock_bin}"

cat >"${mock_bin}/docker" <<'MOCK_DOCKER'
#!/usr/bin/env bash
set -euo pipefail

joined=" $* "
if [[ "${joined}" == *" --raw "* ]]; then
  printf '%s' "${MOCK_MANIFEST}"
elif [[ "${joined}" == *".SBOM.SPDX"* ]]; then
  if [[ "${MOCK_EMPTY_SBOM:-0}" == "1" ]]; then
    printf '{}\n'
  else
    printf '{"spdxVersion":"SPDX-2.3","packages":[{"name":"fixture"}]}\n'
  fi
elif [[ "${joined}" == *".Provenance.SLSA"* ]]; then
  material_repository="${MOCK_MATERIAL_REPOSITORY:-${MOCK_REPOSITORY}}"
  if [[ "${MOCK_PROVENANCE_SCHEMA:-v02}" == "v1" ]]; then
    build_type="${MOCK_BUILD_TYPE:-https://github.com/moby/buildkit/blob/master/docs/attestations/slsa-definitions.md}"
    builder_id="${MOCK_BUILDER_ID:-https://github.com/${MOCK_REPOSITORY}/actions/runs/123456789/attempts/1}"
    printf '{"buildDefinition":{"buildType":"%s","resolvedDependencies":[{"uri":"https://github.com/%s.git%s","digest":{"sha1":"%s"}}]},"runDetails":{"builder":{"id":"%s"}}}\n' \
      "${build_type}" "${material_repository}" "${MOCK_MATERIAL_SUFFIX:-}" "${MOCK_REVISION}" "${builder_id}"
  else
    build_type="${MOCK_BUILD_TYPE:-https://mobyproject.org/buildkit@v1}"
    builder_id="${MOCK_BUILDER_ID:-https://github.com/docker/build-push-action}"
    printf '{"buildType":"%s","builder":{"id":"%s"},"materials":[{"uri":"https://github.com/%s.git%s","digest":{"sha1":"%s"}}]}\n' \
      "${build_type}" "${builder_id}" "${material_repository}" "${MOCK_MATERIAL_SUFFIX:-}" "${MOCK_REVISION}"
  fi
else
  echo "Unexpected docker mock invocation." >&2
  exit 1
fi
MOCK_DOCKER

cat >"${mock_bin}/cosign" <<'MOCK_COSIGN'
#!/usr/bin/env bash
set -euo pipefail

expected_identity="https://github.com/${MOCK_REPOSITORY}/.github/workflows/docker-images.yml@${MOCK_WORKFLOW_REF}"
expected=(
  "--certificate-identity=${expected_identity}"
  "--certificate-oidc-issuer=https://token.actions.githubusercontent.com"
  "--certificate-github-workflow-name=${MOCK_WORKFLOW_NAME}"
  "--certificate-github-workflow-ref=${MOCK_WORKFLOW_REF}"
  "--certificate-github-workflow-repository=${MOCK_REPOSITORY}"
  "--certificate-github-workflow-sha=${MOCK_REVISION}"
)

args=" $* "
for required in "${expected[@]}"; do
  key="${required%%=*}"
  value="${required#*=}"
  if [[ "${args}" != *" ${key} ${value} "* && "${args}" != *" ${required} "* ]]; then
    echo "Missing exact Cosign verification constraint: ${key}" >&2
    exit 1
  fi
done

immutable_ref="${*: -1}"
digest="${immutable_ref##*@}"
printf '[{"critical":{"image":{"docker-manifest-digest":"%s"}},"optional":null}]\n' "${digest}"
MOCK_COSIGN

chmod 0755 "${mock_bin}/docker" "${mock_bin}/cosign"

fixture_repo="${tmp_dir}/source"
git init -q "${fixture_repo}"
git -C "${fixture_repo}" config user.name "Verifier fixture"
git -C "${fixture_repo}" config user.email "fixture@example.invalid"
mkdir -p "${fixture_repo}/.github/workflows"
printf 'name: Docker Images\n' >"${fixture_repo}/.github/workflows/docker-images.yml"
git -C "${fixture_repo}" add .
git -C "${fixture_repo}" commit -qm legacy
legacy_revision="$(git -C "${fixture_repo}" rev-parse HEAD)"
printf 'name: Release Images\n' >"${fixture_repo}/.github/workflows/docker-images.yml"
git -C "${fixture_repo}" commit -qam renamed
current_revision="$(git -C "${fixture_repo}" rev-parse HEAD)"
printf 'name: Untrusted Images\n' >"${fixture_repo}/.github/workflows/docker-images.yml"
git -C "${fixture_repo}" commit -qam unsupported
unsupported_revision="$(git -C "${fixture_repo}" rev-parse HEAD)"
# Deliberately leave HEAD on a different workflow name. Verification must read
# the requested immutable revision, including a legacy rollback, not HEAD.
cd "${fixture_repo}"

export MOCK_REPOSITORY="example/project"
export MOCK_REVISION="${legacy_revision}"
export MOCK_WORKFLOW_NAME="Docker Images"
export MOCK_WORKFLOW_REF="refs/heads/main"
export MOCK_MANIFEST='{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[]}'

run_verifier() {
  PATH="${mock_bin}:${PATH}" bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >"${tmp_dir}/negative.out" 2>"${tmp_dir}/negative.err"; then
    echo "Expected failure was accepted: ${description}" >&2
    exit 1
  fi
  echo "PASS negative: ${description}"
}

success_output="$(run_verifier)"
verified_count="$(grep -c '^Verified signed release image:' <<<"${success_output}")"
[[ "${verified_count}" == "8" ]] || { echo "Expected eight verified release images; got ${verified_count}." >&2; exit 1; }
echo "PASS positive: eight exact signed image digests"

export MOCK_REVISION="${current_revision}"
export MOCK_WORKFLOW_REF="refs/heads/dev-deploy"
export MOCK_WORKFLOW_NAME="Release Images"
renamed_output="$(run_verifier)"
[[ "$(grep -c '^Verified signed release image:' <<<"${renamed_output}")" == "8" ]]
echo "PASS positive: renamed workflow verified from its exact source revision"

expect_failure "wrong signed workflow name" env MOCK_WORKFLOW_NAME="Docker Images" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "unsupported source workflow name" env MOCK_REVISION="${unsupported_revision}" MOCK_WORKFLOW_NAME="Untrusted Images" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${unsupported_revision}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${unsupported_revision}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

v1_success_output="$(MOCK_PROVENANCE_SCHEMA=v1 run_verifier)"
v1_verified_count="$(grep -c '^Verified signed release image:' <<<"${v1_success_output}")"
[[ "${v1_verified_count}" == "8" ]] || { echo "Expected eight verified SLSA v1 release images; got ${v1_verified_count}." >&2; exit 1; }
echo "PASS positive: eight exact signed image digests with SLSA v1 provenance"

fragment_success_output="$(MOCK_PROVENANCE_SCHEMA=v1 MOCK_MATERIAL_SUFFIX="#${MOCK_REVISION}" run_verifier)"
fragment_verified_count="$(grep -c '^Verified signed release image:' <<<"${fragment_success_output}")"
[[ "${fragment_verified_count}" == "8" ]] || { echo "Expected eight verified provenance materials with pinned Git fragments; got ${fragment_verified_count}." >&2; exit 1; }
echo "PASS positive: provenance Git material fragment matches the exact revision"

expect_failure "mismatched image tag" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-ffffffffffffffffffffffffffffffffffffffff" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "short SHA image tag" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION:0:7}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "unsupported workflow ref" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "refs/heads/feature"

original_revision="${MOCK_REVISION}"
export MOCK_REVISION="ffffffffffffffffffffffffffffffffffffffff"
expect_failure "wrong signed revision" env MOCK_REVISION="${MOCK_REVISION}" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${original_revision}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${original_revision}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"
export MOCK_REVISION="${original_revision}"

original_repository="${MOCK_REPOSITORY}"
export MOCK_REPOSITORY="example/other-project"
expect_failure "wrong signed repository" env MOCK_REPOSITORY="${MOCK_REPOSITORY}" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${original_repository}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"
export MOCK_REPOSITORY="${original_repository}"

expect_failure "empty SBOM attestation" env MOCK_EMPTY_SBOM=1 PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "repository material with matching suffix" env \
  MOCK_MATERIAL_REPOSITORY="attacker/${MOCK_REPOSITORY}" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "unexpected provenance builder" env \
  MOCK_BUILDER_ID="https://github.com/example/untrusted-builder" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "unexpected provenance build type" env \
  MOCK_BUILD_TYPE="https://example.invalid/build@v1" PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "unexpected SLSA v1 Actions builder repository" env \
  MOCK_PROVENANCE_SCHEMA=v1 \
  MOCK_BUILDER_ID="https://github.com/example/untrusted/actions/runs/123456789/attempts/1" \
  PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "malformed SLSA v1 Actions builder run" env \
  MOCK_PROVENANCE_SCHEMA=v1 \
  MOCK_BUILDER_ID="https://github.com/${MOCK_REPOSITORY}/actions/runs/not-a-run/attempts/0" \
  PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

expect_failure "unexpected SLSA v1 build type" env \
  MOCK_PROVENANCE_SCHEMA=v1 \
  MOCK_BUILD_TYPE="https://example.invalid/buildkit/slsa" \
  PATH="${mock_bin}:${PATH}" \
  bash "${verifier}" \
    --image-registry "ghcr.io/example/project" \
    --image-tag "sha-${MOCK_REVISION}" \
    --repository "${MOCK_REPOSITORY}" \
    --revision "${MOCK_REVISION}" \
    --workflow-ref "${MOCK_WORKFLOW_REF}"

echo "Release image verification policy tests passed."
