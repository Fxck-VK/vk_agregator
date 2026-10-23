# DEV release reliability implementation plan

Status: implemented and verified locally; not committed, pushed or deployed.
The user approved fixing the diagnosed download stalls, duplicate DEV deployment,
pre-push checks and omitted release changes.

**Goal:** one reproducible DEV rollout from the tested commit, bounded network
setup and an explicit inventory checked before a push.

**Architecture:** reuse `deploy-dev.yml` directly from the image workflow at the
same commit. Rename the image workflow to `Release Images` so the older
default-branch `Docker Images` listener cannot launch an extra DEV rollout.
Manual DEV runs use the same file and require successful exact-SHA CI/images.
Serialize rollouts without interrupting migrations; skip superseded sources
before SSH. Keep all image scans, signatures, attestations and smoke mandatory.

**Constraints:** one active checkout, no main/production publication, no new
dependencies, no secret output, no unrequested commit/push/deploy. Preserve the
existing unrelated working changes.

- [x] Add failing behavioral tests for a bounded download retry and safe source
      selection: transient failure, timeout, permanent failure, cancellation,
      wrong ref/event/SHA and superseded commits.
- [x] Implement `retry-network.sh` and `validate-dev-release.sh`; separate browser
      installation from browser tests and bound only installation retries.
- [x] Call the reusable DEV workflow after the complete signed image matrix;
      retain manual exact-SHA gates and protected environment. Update the PROD
      listener's accepted image workflow name for future source compatibility,
      without changing its manual deployment gate or deploying production.
- [x] Test and implement `release-manifest.mjs`: capture planned Git blobs before
      commit, require a reason for deferred files, reject omitted, changed or
      unexpected committed paths. Require it before the preflight cache and add
      frontend builds/model validation to preflight.
- [x] Update validators/runbooks and run behavioral tests, actionlint, existing
      workflow-monitor/preflight regressions and documentation/infrastructure
      checks. Record what is local versus published.

Browser binaries are not cached: Playwright documents that restoring them often
costs as much as downloading and does not cache Linux system dependencies.
The fix is a bounded install step; failing tests/audits are never auto-retried.

Verification: six inventory tests (including an omitted theme edit, CRLF, new
files/deletions, unexpected changes, reasoned deferrals and executable modes);
bounded retry/source-selection fixtures; existing workflow-monitor, preflight,
documentation and signed-image verification regressions; actionlint on all four
touched workflows; documentation validation; full infrastructure/Compose policy
with `-SkipPromtool`; nightly quality policy. No live Actions run or VPS change
was performed. Prometheus rules were unchanged.

Manual recovery verifies exact-SHA CI and all eight signed images directly;
waiting for the parent image workflow's overall success would incorrectly block
recovery when its deployment child fails. Only the mutating job takes the DEV
lock, avoiding a wait cycle between manual validation and the parent workflow.

References: [reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows),
[browser CI](https://github.com/microsoft/playwright/blob/main/docs/src/ci.md).
