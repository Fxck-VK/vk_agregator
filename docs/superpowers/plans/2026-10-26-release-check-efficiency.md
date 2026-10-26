# Release Check Efficiency Implementation Plan

> **For agentic workers:** Use subagent-driven-development for bounded independent edits and requesting-code-review for final integration. Steps use checkbox syntax.

**Goal:** Run the complete frontend suite once per final-commit preflight, catch Linux CI differences before publication, and resume failed checks without repeating successful unrelated work.

**Architecture:** Keep the existing preparation/inventory entrypoint and exact-commit receipt engine. Preparation/source-fast select lint, types, mandatory filesystem/login guards and assets; they explicitly defer complete web suites to final stages. Split the final stages under policy v7; never reuse receipts across commits or runtime fingerprints. Run Linux release contracts in native Linux or a bounded disposable pinned Node container on Windows. Make the file-picker regression control its asynchronous preview explicitly.

**Tech Stack:** Node.js, PowerShell, Git, Docker/Linux, Vitest/React Testing Library.

## Global Constraints

- One active checkout and localhost:7159; no alternate worktrees or servers.
- No commit, push or deployment is authorized by this implementation request.
- Preserve source inventories, source-fast first, security before final source tests, security expiry, exact-SHA CI, all seven signed images, serialized deployment and smoke.
- Do not increase test timeouts/retries, suppress failures, fabricate cache receipts, or reuse another commit's receipts.
- Do not mount Git metadata, local env files or credentials in Linux test containers.

## File Map

- `scripts/ci/release-source-plan.mjs` and `release-source-check.test.mjs`: automatic fast-only web selection and deferred-suite reporting.
- `scripts/ci/dev-deploy-preflight.ps1`, `test-dev-deploy-preflight.ps1`: independently cached final backend/admin/platform check, test and build stages.
- `scripts/ci/check-release-contract.mjs`, `release-contract.test.mjs`: enforce the complete stage inventory, security order and single platform-suite invocation.
- `scripts/ci/release-linux-check.mjs` and `.test.mjs`: bounded native/container Linux checks with source-only read-only mounts and cleanup.
- `.github/workflows/ci.yml`: run the same Linux release guard in CI.
- `web/platform/src/components/chat/ChatFilePicker/ChatFilePicker.test.tsx`: controlled promises and async act, preserving selection/retry assertions.
- `AGENTS.md`, `docs/runbooks/DEV.md`, `docs/INDEX.md`, `.agents/state.json`: durable rules, commands and current local-only status.

## Tasks

- [x] **1. Fast-only source selection.** Update tests to reject full/related web-suite expansion in preparation, preserve mandatory guards and require explicit deferred suites. Observe failures, implement selection, run selector regressions. Include native Linux guard first for release-tooling changes.
- [x] **2. Granular final stages.** Add failing source-contract tests for one platform suite and a complete ordered stage list. Split backend, admin checks/tests/build, platform checks/tests/build/packaging; bump policy v7. Add a simulated late-test failure/retry regression proving earlier stages remain cached and pending build/packaging still run. Retain changed-commit/runtime, corrupt-cache, expiry and force-failure regressions.
- [x] **3. Linux parity.** Test native and Docker execution plans, pinned image validation, timeout/failure propagation, source-only mounts and cleanup. Execute release-contract/inventory tests and real selector CLI fixtures in Linux before other source checks when tooling changes. Wire the same guard into CI; verify it in an actual disposable Linux container.
- [x] **4. Deterministic picker regression.** Control preview completion with a deferred promise and async act. Verify loading before completion, exact selected artifact after completion and explicit retry behavior. Keep all timeout/retry settings unchanged; run focused tests.
- [x] **5. Integration and review.** Run affected Node/PowerShell regressions, Linux guard, workflow lint, focused picker tests, packaging/docs checks and diff checks. Update durable instructions and mark results below. Independently review spec compliance and quality; fix actionable findings. Do not run or claim a clean final-commit release preflight without an authorized commit.

## Verification Results

- Automatic early source entrypoint passed on all current changes in 74.3 seconds. Its command log confirms no complete or related web-suite execution; full web suites are explicitly deferred.
- Windows Node contracts/inventory/selector/Linux-runner regressions passed (55 distinct cases); the same portable suite passed natively inside Linux (52 cases).
- Exact-commit cache regressions passed, including a platform-suite failure followed only by failed/pending platform tests/build/packaging; changed commit/runtime, expiry, corruption and forced failure remain covered.
- Picker tests passed 4/4 with controlled deferred previews; platform types, lint, assets, packaging, documentation validation and actionlint passed.
- Shared signature/env/deployment release regressions passed. No provider, billing, authentication, storage or production UI behavior was changed.
- Independent read-only review found no blocking issues. Its container cancellation note was addressed with --init and an independent workload deadline; a real one-second deadline stopped a twenty-second fixture and removed the owned container in 1880ms.
- No commit, push, deployment or clean-final-commit full preflight was performed for this local implementation. The next explicitly authorized release still requires the reviewed inventory and complete v7 preflight for its actual final SHA.
