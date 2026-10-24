# Early release source checks implementation plan

> **For agentic workers:** Execute inline in the existing active checkout. The user approved the four-point early-check proposal; no new commit, push or rollout is authorized by this task.

**Goal:** Find source errors before preparing a release commit and before expensive full validation, without weakening any publication gate.

**Architecture:** Extend the existing DEV preflight with a preparation mode and an obligatory first `source-fast` stage. A dependency-free Node planner selects checks from every changed tracked/untracked path relative to the DEV baseline; a PowerShell runner executes argument arrays and stops on the first failure. Existing exact-commit/policy/runtime receipts remain responsible for successful-stage reuse.

**Tech Stack:** Node built-ins, PowerShell 7, existing npm scripts and Vitest related tests, Go tooling.

## Global constraints

- Use the single active checkout and existing local server; preserve the pending MusicFiles fix.
- No commits, pushes, deployments, paid calls, automatic fixes or source mutation.
- Preparation accepts dirty source; complete preflight still requires a clean exact commit and a reviewed source inventory.
- No security, build, signature, authentication, billing or smoke checks are removed.
- Do not reuse full-check receipts across commits or toolchain changes.

### Task 1: Deterministic source check selection and execution

**Files:** `scripts/ci/release-source-plan.mjs`, `scripts/ci/release-source-check.ps1`, `scripts/ci/release-source-check.test.mjs`.

**Interface:** `createSourcePlan(changedFiles, existingFiles)` returns `{changedFiles, commands}`; commands contain a repository-relative `cwd`, executable name and individual `args`. CLI resolves `--base` to a commit, includes tracked edits/deletions and untracked nonignored files, and emits JSON. The runner accepts `-BaseRef` and executes the resulting plan without shell interpolation.

- [x] Add failing Node tests for frontend guards (including typography and login), affected package isolation, explicit changed tests, safe path handling, deleted/config-file broad fallback, Go package selection and early failure propagation.
- [x] Add the planner and runner. For affected frontend packages execute lint, typecheck, compulsory platform contracts/login regressions and statically related tests. Configuration/dependency/deletion changes fall back to the package suite. For Go execute formatting validation and affected package tests/vet; module changes select all packages. For release tooling/workflows run source contract fixtures and preflight regressions; for docs run documentation validation.
- [x] Verify `node --test scripts/ci/release-source-check.test.mjs` and execute the actual gate on the current dirty checkout.

### Task 2: Make preparation and full validation enforce the gate

**Files:** `scripts/ci/dev-deploy-preflight.ps1`, `scripts/ci/check-release-contract.mjs`, `scripts/ci/release-contract.test.mjs`, `.github/workflows/ci.yml`, `scripts/ci/test-dev-deploy-preflight.ps1`.

**Interface:** `dev-deploy-preflight.ps1 -Prepare -BaseRef origin/dev-deploy -ManifestPath .cache/<new>.json` runs the source gate before inventory capture. Default invocation checks the reviewed manifest and clean commit, then executes `source-fast` before other full stages. Policy becomes v6. No skip switch is introduced.

- [x] Add failing mutation tests: preparation must call the gate before capture; full preflight must run the same gate as its first stage; CI must exercise the planner tests.
- [x] Integrate the gate and preparation mode without changing default publication semantics. Preserve the existing npm installation helper and full checks.
- [x] Extend cache tests for source-fast failure/resume, new commit invalidation and existing runtime/policy invalidation. Run Node mutation tests and PowerShell orchestration tests with synthetic fixtures.

### Task 3: Document and verify the final workflow

**Files:** `AGENTS.md`, `docs/runbooks/DEV.md`, `docs/INDEX.md`, `.agents/state.json`.

- [x] Replace the agent preparation entrypoint with the enforced preparation mode and document automatic selection, broad fallbacks and related-test limitations.
- [x] Run release source tests, manifest tests, source-contract mutation tests, preflight cache tests and documentation validation. Confirm no full-check success is claimed for uncommitted source.
- [x] Run the real early gate against current source, inspect its duration and selected checks, then review `git diff --check` and `git status --short`.

## Verification evidence

- Real `dev-deploy-preflight.ps1 -Prepare -ManifestPath .cache/release-source-fast-verification/manifest.json -BaseRef origin/dev-deploy`: 16 changed paths, 10 automatically selected checks, **72.5 seconds**, successful capture after all checks.
- That run passed platform lint/typecheck, 9 compulsory guard files (**88 tests**), asset validation, 2 statically related files (**26 tests**), release selection/inventory/source-contract fixtures (**38 tests**), PowerShell cache/orchestration regressions and documentation validation.
- Fault fixtures confirmed that preparation stops before capture, the actual runner stops on a native exit failure, existing inventories are not overwritten, a failed early stage is not cached, an unchanged commit resumes successful stages, and a new commit invalidates them.
- `actionlint .github/workflows/ci.yml`: passed. No workflow publication was performed.
- The ignored inventory is verification evidence only. This plan's completion notes and state timestamp were updated afterwards; a future authorized release must prepare a new reviewed inventory for the final source.
- No complete preflight, commit, push or deployment was performed by this task. Runtime/API/provider/billing behavior is unchanged.
