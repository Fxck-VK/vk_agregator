#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

digest="$(printf 'a%.0s' {1..64})"
image_ref="ghcr.io/example/project/api@sha256:${digest}"
identity="https://github.com/example/project/.github/workflows/docker-images.yml@refs/heads/main"
issuer="https://token.actions.githubusercontent.com"

printf '%s\n' '{"bomFormat":"CycloneDX","components":[]}' > "${tmp}/predicate.json"

make_response() {
  local subject_digest="$1"
  local predicate_file="$2"
  local predicate
  local payload
  predicate="$(tr -d '\r\n' < "${predicate_file}")"
  payload="$(printf '{"_type":"https://in-toto.io/Statement/v1","subject":[{"name":"image","digest":{"sha256":"%s"}}],"predicateType":"https://cyclonedx.org/bom","predicate":%s}' "${subject_digest}" "${predicate}" | base64 | tr -d '\r\n')"
  printf '{"payloadType":"application/vnd.in-toto+json","payload":"%s","signatures":[{"sig":"verified"}]}\n' "${payload}" > "${tmp}/response.json"
}

cat > "${tmp}/cosign" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" == "verify-attestation" ]] || exit 90
cat "${FAKE_COSIGN_RESPONSE}"
FAKE
chmod +x "${tmp}/cosign"

make_response "${digest}" "${tmp}/predicate.json"
FAKE_COSIGN_RESPONSE="${tmp}/response.json" COSIGN_BIN="${tmp}/cosign" \
  bash "${root}/scripts/release/verify-attestation-predicate.sh" \
    "${image_ref}" cyclonedx "${tmp}/predicate.json" "${identity}" "${issuer}" >/dev/null

printf '%s\n' '{"bomFormat":"CycloneDX","components":[{"name":"tampered"}]}' > "${tmp}/tampered.json"
if FAKE_COSIGN_RESPONSE="${tmp}/response.json" COSIGN_BIN="${tmp}/cosign" \
  bash "${root}/scripts/release/verify-attestation-predicate.sh" \
    "${image_ref}" cyclonedx "${tmp}/tampered.json" "${identity}" "${issuer}" >/dev/null 2>&1; then
  echo "Tampered predicate was accepted." >&2
  exit 1
fi

wrong_digest="$(printf 'b%.0s' {1..64})"
make_response "${wrong_digest}" "${tmp}/predicate.json"
if FAKE_COSIGN_RESPONSE="${tmp}/response.json" COSIGN_BIN="${tmp}/cosign" \
  bash "${root}/scripts/release/verify-attestation-predicate.sh" \
    "${image_ref}" cyclonedx "${tmp}/predicate.json" "${identity}" "${issuer}" >/dev/null 2>&1; then
  echo "Attestation for another digest was accepted." >&2
  exit 1
fi

cat > "${tmp}/cosign-reject" <<'FAKE'
#!/usr/bin/env bash
exit 1
FAKE
chmod +x "${tmp}/cosign-reject"
if COSIGN_BIN="${tmp}/cosign-reject" \
  bash "${root}/scripts/release/verify-attestation-predicate.sh" \
    "${image_ref}" cyclonedx "${tmp}/predicate.json" "${identity}" "${issuer}" >/dev/null 2>&1; then
  echo "Rejected signature identity was accepted." >&2
  exit 1
fi

echo "attestation predicate tests OK"
