# Cancel superseded DEV image workflows

**Goal:** implement the user-approved cancellation of obsolete image workflows
without weakening successful exact-SHA CI as a publication requirement.

**Design:** CI and Docker Images have independent concurrency groups. A mail-only
push starts CI but skips Docker Images through its path filter, so an older image
workflow can receive a cancelled CI result with no replacement image workflow.
On this result only, the image monitor may request cancellation of its own DEV
push run after confirming that the remote dev-deploy head differs from its SHA.
Current-SHA cancellation, failed CI, missing CI, API failure and timeout still
block publication. Cancellation never returns a successful gate result.

Triggering image workflows for every mail-only push was rejected because a
successful no-build run would also trigger Deploy DEV without release images.
An additional repository-wide cancellation workflow adds unnecessary scope.

**Constraints:** keep main behavior, path filters, immutable image publication,
signed provenance and smoke/rollback gates. No application/runtime changes.
Grant Actions write only to the image quality-gate job, needed to cancel its
current run; do not accept a cancellation target from external input.

## Implementation

- [x] Extend monitor regression tests: superseded cancelled CI requests only
  self-cancellation; current SHA, main/PR context, incorrect identity, API errors
  and failed CI cannot request cancellation or authorize publication.
- [x] Run the tests and observe the new regression fail before implementation.
- [x] Add opt-in cancellation handling to wait-for-github-workflow.sh and its
  Docker Images invocation. Keep no-option callers unchanged.
- [x] Document behavior in the DEV runbook and validate workflow safety checks.
- [x] Run Bash tests/syntax, actionlint and deployment quality validation;
  review the final diff and preserve unrelated local changes.

## Verification limits

Local fixtures exercise real monitor control flow against a fake HTTP boundary;
GitHub delivers actual runner cancellation asynchronously. No active release
will be cancelled merely to test the implementation.

Local verification on 2026-10-22: all 19 monitor scenarios passed in WSL Bash;
Bash syntax, actionlint 1.7.7, validate-nightly-quality.ps1,
validate-docs.ps1 and git diff --check passed. The first new regression failed
before the cancellation option existed. The user delegated publication to DEV;
only this scoped fix will be committed, preserving unrelated local work.
