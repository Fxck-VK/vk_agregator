#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage: verify-release-images.sh \
  --image-registry <registry/repository-prefix> \
  --image-tag <sha-abcdef0> \
  --repository <owner/repository> \
  --revision <40-hex-commit> \
  --workflow-ref <refs/heads/main|refs/heads/dev-deploy>
USAGE
  exit 2
}

image_registry=""
image_tag=""
repository=""
revision=""
workflow_ref=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image-registry) image_registry="${2:-}"; shift 2 ;;
    --image-tag) image_tag="${2:-}"; shift 2 ;;
    --repository) repository="${2:-}"; shift 2 ;;
    --revision) revision="${2:-}"; shift 2 ;;
    --workflow-ref) workflow_ref="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "${image_registry}" && -n "${image_tag}" && -n "${repository}" && -n "${revision}" && -n "${workflow_ref}" ]] || usage
[[ "${revision}" =~ ^[0-9a-f]{40}$ ]] || { echo "Expected a lowercase 40-hex source revision." >&2; exit 1; }
if [[ "${image_tag}" != "sha-${revision:0:7}" && "${image_tag}" != "sha-${revision}" ]]; then
  echo "Image tag does not match the expected source revision." >&2
  exit 1
fi
[[ "${repository}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo "Invalid source repository." >&2; exit 1; }
[[ "${workflow_ref}" =~ ^refs/heads/(main|dev-deploy)$ ]] || { echo "Unsupported signed workflow ref." >&2; exit 1; }

for command_name in docker cosign jq sha256sum; do
  command -v "${command_name}" >/dev/null 2>&1 || { echo "Required verifier is unavailable: ${command_name}" >&2; exit 1; }
done

services=(api worker provider-webhook provider-balance-bot miniapp migrate backup)
certificate_identity="https://github.com/${repository}/.github/workflows/docker-images.yml@${workflow_ref}"
oidc_issuer="https://token.actions.githubusercontent.com"
repository_lc="${repository,,}"
repository_uri="https://github.com/${repository_lc}"
expected_build_type="https://mobyproject.org/buildkit@v1"
expected_builder_id="https://github.com/docker/build-push-action"

for service in "${services[@]}"; do
  tagged_ref="${image_registry}/${service}:${image_tag}"
  raw_manifest="$(mktemp)"
  trap 'rm -f "${raw_manifest}"' EXIT

  docker buildx imagetools inspect "${tagged_ref}" --raw >"${raw_manifest}"
  digest="sha256:$(sha256sum "${raw_manifest}" | awk '{print $1}')"
  immutable_ref="${image_registry}/${service}@${digest}"

  signature_json="$(cosign verify \
    --certificate-identity "${certificate_identity}" \
    --certificate-oidc-issuer "${oidc_issuer}" \
    --certificate-github-workflow-name "Docker Images" \
    --certificate-github-workflow-ref "${workflow_ref}" \
    --certificate-github-workflow-repository "${repository}" \
    --certificate-github-workflow-sha "${revision}" \
    --output json \
    "${immutable_ref}")"

  jq -e \
    --arg digest "${digest}" \
    --arg repository "${repository}" \
    --arg revision "${revision}" \
    --arg workflow_ref "${workflow_ref}" \
    'type == "array" and length > 0 and any(.[];
      (.critical.image["docker-manifest-digest"] // .critical.image["Docker-manifest-digest"]) == $digest
    )' <<<"${signature_json}" >/dev/null

  sbom="$(docker buildx imagetools inspect "${immutable_ref}" --format '{{ json .SBOM.SPDX }}')"
  provenance="$(docker buildx imagetools inspect "${immutable_ref}" --format '{{ json .Provenance.SLSA }}')"

  jq -e 'type == "object" and (.spdxVersion | startswith("SPDX-")) and ((.packages // []) | length > 0)' <<<"${sbom}" >/dev/null
  jq -e \
    --arg repository_uri "${repository_uri}" \
    --arg revision "${revision}" \
    --arg build_type "${expected_build_type}" \
    --arg builder_id "${expected_builder_id}" \
    'def normalized_provenance:
       if (.buildDefinition? | type) == "object" then
         {
           build_type: .buildDefinition.buildType,
           builder_id: .runDetails.builder.id,
           materials: (.buildDefinition.resolvedDependencies // [])
         }
       else
         {
           build_type: .buildType,
           builder_id: .builder.id,
           materials: (.materials // [])
         }
       end;
     def normalized_material_uri:
       ascii_downcase
       | sub("^git\\+"; "")
       | sub("\\.git$"; "")
       | sub("/$"; "");
     type == "object" and
     (normalized_provenance | .build_type == $build_type) and
     (normalized_provenance | .builder_id == $builder_id) and
     any((normalized_provenance | .materials)[];
       ((.uri // "") | normalized_material_uri) == $repository_uri and
       ((.digest.sha1 // "") == $revision)
     )' <<<"${provenance}" >/dev/null

  rm -f "${raw_manifest}"
  trap - EXIT
  echo "Verified signed release image: service=${service} digest=${digest}"
done
