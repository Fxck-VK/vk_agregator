#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/release/validate-sbom.sh <service> <runtime.cdx.json> <runtime.spdx.json> [<source.cdx.json> <source.spdx.json>]
USAGE
  exit 2
}

[[ $# -eq 3 || $# -eq 5 ]] || usage

service="$1"
runtime_cdx="$2"
runtime_spdx="$3"
source_cdx="${4:-}"
source_spdx="${5:-}"

case "${service}" in
  api|worker|provider-webhook|provider-balance-bot|miniapp|migrate|backup) ;;
  *) echo "Unknown release service: ${service}" >&2; exit 1 ;;
esac

for file in "${runtime_cdx}" "${runtime_spdx}"; do
  [[ -s "${file}" ]] || { echo "SBOM is missing or empty: ${file}" >&2; exit 1; }
  jq -e 'type == "object"' "${file}" >/dev/null
done

jq -e '.bomFormat == "CycloneDX" and (.specVersion | type == "string" and length > 0) and ((.components // []) | length > 0)' "${runtime_cdx}" >/dev/null
jq -e '(.spdxVersion | type == "string" and startswith("SPDX-")) and .SPDXID == "SPDXRef-DOCUMENT" and ((.packages // []) | length > 0)' "${runtime_spdx}" >/dev/null

require_purl() {
  local file="$1"
  local prefix="$2"
  local label="$3"
  jq -e --arg prefix "${prefix}" \
    '[.. | objects | (.purl? // .referenceLocator? // empty) | select(type == "string" and startswith($prefix))] | length > 0' \
    "${file}" >/dev/null || {
      echo "${label} component is missing from ${file}" >&2
      exit 1
    }
}

for file in "${runtime_cdx}" "${runtime_spdx}"; do
  require_purl "${file}" "pkg:apk/" "base OS"
done

case "${service}" in
  api|worker|provider-webhook|provider-balance-bot|migrate)
    for file in "${runtime_cdx}" "${runtime_spdx}"; do
      require_purl "${file}" "pkg:golang/" "Go"
    done
    ;;
  miniapp)
    [[ $# -eq 5 ]] || { echo "Mini App requires paired source SBOMs for npm evidence" >&2; exit 1; }
    for file in "${source_cdx}" "${source_spdx}"; do
      [[ -s "${file}" ]] || { echo "Source SBOM is missing or empty: ${file}" >&2; exit 1; }
      jq -e 'type == "object"' "${file}" >/dev/null
      require_purl "${file}" "pkg:npm/" "npm"
    done
    jq -e '.bomFormat == "CycloneDX" and (.specVersion | type == "string" and length > 0)' "${source_cdx}" >/dev/null
    jq -e '(.spdxVersion | type == "string" and startswith("SPDX-")) and .SPDXID == "SPDXRef-DOCUMENT"' "${source_spdx}" >/dev/null
    ;;
  backup) ;;
esac

echo "SBOM component policy passed for ${service}."
