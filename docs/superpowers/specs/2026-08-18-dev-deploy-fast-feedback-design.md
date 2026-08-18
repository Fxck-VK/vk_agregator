# DEV Deploy Fast Feedback Design

**Status:** approved for implementation on 2026-08-18.

## Objective

Reduce the normal `dev-deploy` feedback loop without weakening `main`, the
production release gates, immutable image tags, Cosign verification, SBOMs,
SLSA provenance, Trivy gates, or the protected production environment.

The target is a materially faster repeat deployment for web-platform-only
changes. A push itself should remain a short Git operation; validation, image
publication, and rollout should be observable as separate stages.

## Scope and branch boundary

- Runtime and CI implementation belongs to `dev-deploy`.
- `main` keeps its full test, image, signing, scanning, and production deploy
  path.
- The only permitted `main` change is `.github/dependabot.yml`, because GitHub
  reads Dependabot configuration from the default branch. That change is made
  through a reviewed PR.
- Dependabot security updates continue to target the default branch. Additional
  version-update entries target `dev-deploy`.

## Existing constraints

DEV releases use one immutable `sha-<40-hex>` tag for every application image.
`verify-release-images.sh` requires every image to carry a valid signature,
SBOM, and provenance material for the exact release SHA. Therefore an
unchanged image cannot simply be copied from an older release and relabelled as
the new SHA: that would break or falsify provenance.

For that reason, the safe fast path keeps publication of all eight signed
images, but makes those builds cheap when unchanged by isolating BuildKit cache
scope per service. Selective skipping is allowed for local/focused validation
and non-release work only; production-shaped publication stays complete.

## Design

### 1. One serialized DEV preflight

Add a versioned PowerShell entry point that runs, in order:

1. source tests and static checks;
2. npm dependency audits;
3. `govulncheck`;
4. infrastructure and workflow policy validation;
5. Trivy filesystem/misconfiguration scanning.

The script acquires an exclusive lock stored under Git's private metadata so
two full preflights cannot run concurrently in the same worktree. A successful
result is cached by commit SHA and validation-policy version, so retrying the
same push does not repeat the full suite. Failures are never cached.

The script must not install arbitrary latest tools. Missing pinned tools fail
with an actionable message. Trivy runs through the repository's pinned scanner
contract.

### 2. Semantic Next.js route validation

Extract route discovery into a small reusable PowerShell module. It derives a
URL from the Next.js App Router tree, ignoring route groups such as `(public)`
and parallel-route slots. `validate-infra.ps1` asks the module for the page that
implements `/` instead of naming a physical path.

Tests cover a root page inside a route group, relocation between route groups,
nested routes, missing routes, and duplicate route owners.

### 3. Reliable GitHub API monitoring

Move exact-SHA workflow polling out of inline workflow YAML into a tested Bash
script. Network calls use bounded curl retries for transient transport, 429,
and 5xx failures. Poll intervals use bounded exponential backoff. A completed
non-success conclusion still fails immediately; retry logic must never convert
a failed workflow into success.

### 4. DEV-only concurrency cancellation

Add workflow-level concurrency groups keyed by workflow and ref. New
`dev-deploy` runs cancel obsolete runs for that same workflow and branch.
`main` runs are never cancelled by this policy, and different workflows cannot
cancel each other.

### 5. Service-scoped BuildKit caches

Give every Docker matrix service its own GitHub Actions cache scope. This
prevents parallel matrix jobs from overwriting the shared default BuildKit
cache. Both PR builds and signed publication reuse their service cache with
`mode=max` export.

The build context, full immutable SHA tag, SBOM, provenance, Trivy scan, Cosign
signature, and release verifier remain unchanged.

### 6. Dependabot for DEV without weakening main

Keep the current default-branch update entries and add equivalent weekly
version-update entries with `target-branch: dev-deploy` for GitHub Actions,
Docker, Go modules, and all three npm applications. Use distinct group names
and labels so DEV update PRs are recognizable.

The default-branch file is changed through a separate reviewed PR. No
application or production workflow file is changed in that PR.

## Verification

- Route-discovery behavior tests pass.
- GitHub workflow polling retry/backoff tests pass with fake curl and sleep.
- Preflight policy tests prove locking, ordered stages, failure propagation,
  and success-cache behavior without running the expensive tools themselves.
- Infrastructure validation asserts DEV-only concurrency, per-service cache
  scopes, the external polling script, and the Dependabot target entries.
- Existing Go, frontend, workflow, secret-scan, supply-chain, and deploy checks
  remain green.
- The final `dev-deploy` run is monitored through CI, Docker Images, and Deploy
  DEV. Any regression is fixed without bypassing a gate.

## Security impact

No auth, billing, provider, moderation, storage ownership, TLS, signature, or
production-deploy boundary changes. Faster feedback comes from eliminating
duplicate work, preserving caches correctly, and cancelling obsolete DEV runs,
not from skipping production safety controls.
