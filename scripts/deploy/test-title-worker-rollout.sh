#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

assert_single_line() {
  local script="$1"
  local needle="$2"
  local label="$3"
  local lines

  lines="$(grep -nF "${needle}" "${script}" | cut -d: -f1 || true)"
  if [[ -z "${lines}" || "${lines}" == *$'\n'* ]]; then
    printf '%s must contain exactly one %s\n' "${script}" "${label}" >&2
    exit 1
  fi
  printf '%s' "${lines}"
}

assert_before() {
  local earlier="$1"
  local later="$2"
  local label="$3"
  if (( earlier >= later )); then
    printf 'Expected %s to run before the API runtime startup\n' "${label}" >&2
    exit 1
  fi
}

assert_bash_rollout() {
  local script="$1"
  local worker_line
  local runtime_line
	local relay_gate_line
  local migration_line

  bash -n "${script}"
  worker_line="$(assert_single_line "${script}" 'worker_up_args+=(worker)' 'worker startup')"
  runtime_line="$(assert_single_line "${script}" 'runtime_services=(api maintenance-worker provider-webhook miniapp' 'API runtime list without worker')"
  relay_gate_line="$(assert_single_line "${script}" 'Refusing API cutover until every relay-only worker is upgraded or stopped.' 'relay-only worker rollout gate')"
  migration_line="$(assert_single_line "${script}" 'migrate_args=(up --no-deps --exit-code-from migrate)' 'migration startup')"
  assert_before "${worker_line}" "${runtime_line}" "${script} worker startup"
	assert_before "${relay_gate_line}" "${worker_line}" "${script} relay-only rollout gate"
	assert_before "${relay_gate_line}" "${migration_line}" "${script} relay-only rollout gate"

  if ! grep -Fq 'worker_up_args=(up -d --no-deps --force-recreate --wait --wait-timeout' "${script}"; then
    printf '%s worker startup must wait for readiness before API cutover\n' "${script}" >&2
    exit 1
  fi
}

assert_powershell_rollout() {
  local script="scripts/deploy/deploy-prod.ps1"
  local worker_line
  local runtime_line
	local relay_gate_line
  local migration_line

  worker_line="$(assert_single_line "${script}" 'Invoke-Step "start jobs worker before API rollout" {' 'worker startup')"
  runtime_line="$(assert_single_line "${script}" '$runtimeServices = @("api", "maintenance-worker", "provider-webhook", "miniapp", "reverse-proxy")' 'API runtime list without worker')"
  relay_gate_line="$(assert_single_line "${script}" 'Refusing API cutover until every relay-only worker is upgraded or stopped.' 'relay-only worker rollout gate')"
  migration_line="$(assert_single_line "${script}" 'Invoke-Step "run migrations" {' 'migration startup')"
  assert_before "${worker_line}" "${runtime_line}" "${script} worker startup"
	assert_before "${relay_gate_line}" "${worker_line}" "${script} relay-only rollout gate"
	assert_before "${relay_gate_line}" "${migration_line}" "${script} relay-only rollout gate"

  if ! grep -Fq '"--wait", "--wait-timeout", "$TimeoutSeconds"' "${script}"; then
    printf '%s worker startup must wait for readiness before API cutover\n' "${script}" >&2
    exit 1
  fi
}

assert_dev_workflow_confirms_managed_relay_topology() {
  local workflow=".github/workflows/deploy-dev.yml"
  local count

  count="$(grep -Fc -- '--relay-only-workers-upgraded' "${workflow}" || true)"
  if [[ "${count}" != "2" ]]; then
    printf '%s must explicitly confirm the managed relay topology for deploy and rollback\n' "${workflow}" >&2
    exit 1
  fi
}

assert_production_workflow_requires_manual_relay_acknowledgement() {
  local workflow=".github/workflows/deploy-prod.yml"

  if ! grep -Fq 'relay_only_workers_upgraded:' "${workflow}" ||
    ! grep -Fq 'inputs.relay_only_workers_upgraded == true' "${workflow}" ||
    ! grep -Fq -- '--relay-only-workers-upgraded' "${workflow}"; then
    printf '%s must require an explicit manual relay-only acknowledgement before API rollout\n' "${workflow}" >&2
    exit 1
  fi
}

assert_bash_rollout "scripts/deploy/deploy-dev.sh"
assert_bash_rollout "scripts/deploy/deploy-prod.sh"
assert_powershell_rollout
assert_dev_workflow_confirms_managed_relay_topology
assert_production_workflow_requires_manual_relay_acknowledgement

echo "Title worker rollout tests passed"
