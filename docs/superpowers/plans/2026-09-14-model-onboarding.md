# Model onboarding implementation plan

> Agent workflow: execute the approved steps in this worktree; no commit, deployment or paid provider calls are authorized by this plan.

**Goal:** Require a typed capability contract and reviewable verification evidence when registering a new provider/model route or changing an existing one.

**Architecture:** Extend the existing `providermodels` registry. A separate `modelcontract` package validates type-specific input/output capabilities and admission evidence without network access. Registry validation and runtime startup enforce admission; CI already runs Go tests. Existing registry entries retain a frozen, explicitly unverified migration baseline. Changing their routing, limits or pricing keys requires onboarding rather than renewing the baseline.

**Constraints:** No fabricated provider facts, successful live tests or source dates. Draft contracts stay outside the active registry. New capabilities are disabled until implemented and verified. Provider calls remain in workers, prices remain in pricingcatalog, and existing payment/authentication/job boundaries remain intact.

- [x] Add failing contract tests for input formats and limits, typed outputs, compatibility, default values, missing/failed/stale evidence and explicit unknown capabilities.
- [x] Implement `internal/service/modelcontract` types, validation, required verification cases and strict JSON decoding; provide four synthetic draft examples and report template.
- [x] Add failing registry tests for a new model without a contract, a changed legacy route, provider changes and invalid ready contracts.
- [x] Bind contracts to exact registry fingerprints, including video aliases; freeze the existing entries as migration-only records, and enforce admission in registry validation, runtime config and product catalog construction.
- [x] Add an offline `scripts/models/check` command to validate a candidate contract or the active registry and report outstanding verification cases. Test exit behavior. No provider calls or credentials.
- [x] Extend AGENTS.md, DEV.md and the documentation index with an obligatory model onboarding runbook covering research, implementation, positive/negative/boundary tests, live semantic/output checks, billing/retries, UI consistency, reports and re-verification.
- [x] Run focused Go tests, vet, formatting and relevant runtime/catalog regression checks; inspect scoped diff and final status. Report that real provider capabilities have not been newly verified.

## Verification

- Passed Go tests for modelcontract, providermodels, modelcatalog, videorouter,
  productcatalog, imagegeneration, textgeneration, config, miniapp, vkbot,
  cmd/api, cmd/worker, worker and scripts/models/check.
- Passed focused `go vet`, formatting and tracked scoped `git diff --check`.
- Offline registry check passed with 43 `legacy-unverified` records and no
  approved provider contracts. All four synthetic draft examples were rejected
  as intended. No live provider calls or new capability claims were made.
- Independent review findings about parameter combinations, aliases and input
  counts were fixed with regression tests; the focused recheck found no remaining
  substantial issues in those fixes.
- Logs and final working-tree status are in `.cache/merge-checks/model-onboarding-*`.
  Pre-existing unrelated changes remain in the working tree; no commit or deploy.
