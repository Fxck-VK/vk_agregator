# DEV Deploy Fast Feedback Implementation Plan

> **Scope:** `dev-deploy`, except for the separately reviewed default-branch
> Dependabot configuration required by GitHub.

**Goal:** Remove avoidable repeated work from the DEV feedback loop while
preserving complete signed release images and every production safety gate.

**Design:** `docs/superpowers/specs/2026-08-18-dev-deploy-fast-feedback-design.md`

## Task 1: Resolve Next.js routes semantically

**Files:**

- Create: `scripts/ci/NextRouteDiscovery.psm1`
- Create: `scripts/ci/test-next-route-discovery.ps1`
- Modify: `scripts/ci/validate-infra.ps1`
- Modify: `.github/workflows/ci.yml`

1. Write behavior tests for route groups, relocation, nested routes, missing
   routes, and duplicate owners.
2. Run the test and confirm it fails because the module is absent.
3. Implement route discovery and replace the physical homepage path lookup.
4. Run the focused test and `validate-infra.ps1`.

## Task 2: Extract reliable exact-SHA workflow monitoring

**Files:**

- Create: `scripts/ci/wait-for-github-workflow.sh`
- Create: `scripts/ci/test-wait-for-github-workflow.sh`
- Modify: `.github/workflows/docker-images.yml`
- Modify: `.github/workflows/ci.yml`

1. Write tests with fake `curl` and `sleep` commands for transient API errors,
   in-progress runs, success, terminal failure, and timeout.
2. Confirm the tests fail before the script exists.
3. Implement bounded curl retry plus exponential polling backoff.
4. Replace inline Docker Images polling with the script.
5. Run the focused shell tests.

## Task 3: Add serialized, commit-cached DEV preflight

**Files:**

- Create: `scripts/ci/DevDeployPreflight.psm1`
- Create: `scripts/ci/dev-deploy-preflight.ps1`
- Create: `scripts/ci/test-dev-deploy-preflight.ps1`
- Modify: `docs/runbooks/DEV.md`
- Modify: `.github/workflows/ci.yml`

1. Write module tests for stage order, exclusive locking, failure propagation,
   success caching, and cache invalidation by policy version.
2. Confirm the tests fail before the module exists.
3. Implement the single-flight runner and the ordered production stage map.
4. Document the command, prerequisites, cache behavior, and failure recovery.
5. Run the focused tests without invoking the expensive stage commands.

## Task 4: Cancel obsolete DEV work and preserve per-service caches

**Files:**

- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/docker-images.yml`
- Modify: `.github/workflows/deploy-dev.yml`
- Modify: `scripts/ci/validate-nightly-quality.ps1`

1. Extend workflow-policy assertions first and confirm validation fails.
2. Add concurrency groups whose cancellation expression is true only for
   `refs/heads/dev-deploy`.
3. Add `scope=<service>` to Docker matrix GHA cache import/export.
4. Confirm immutable tags, SBOM, provenance, Trivy, Cosign, and release
   verification assertions are unchanged.
5. Run infrastructure and workflow policy validation.

## Task 5: Configure DEV version updates from the default branch

**Files on a separate branch from `origin/main`:**

- Modify: `.github/dependabot.yml`

1. Preserve all existing default-branch entries.
2. Add equivalent `target-branch: dev-deploy` version-update entries for
   GitHub Actions, Docker, Go, Mini App npm, Admin npm, and Platform npm.
3. Give DEV groups distinct names and labels.
4. Validate the YAML and open a focused PR containing only this file.

## Task 6: Full verification and monitored DEV rollout

1. Run focused PowerShell and Bash regression tests.
2. Run `validate-infra.ps1` and `validate-nightly-quality.ps1`.
3. Run `git diff --check`, Go tests/vet/govulncheck, frontend tests/audits, and
   Trivy according to the new preflight contract.
4. Commit the DEV implementation in rollback-friendly logical commits.
5. Push to `dev-deploy` and monitor CI, Docker Images, and Deploy DEV.
6. Report exact failures and reusable prevention if any gate fails.
