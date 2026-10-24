# Release contract and agent rules

Approved: execute the user's six-step plan, then commit/push all agreed changes
including VK Mini App retirement to dev-deploy and verify the actual rollout.

Goal: catch inconsistencies between CI, image publication, signatures, env and
deployment before push, preserving every existing security/quality gate.

Constraints: one active checkout/website; no main changes, forced pushes, paid
calls, user-data deletion or weakened signatures/auth/billing. Existing SQL and
stored historical Mini App source values stay immutable. No new worktrees.

- [x] Add failing regression tests for actual release-source inconsistencies:
  mismatched image inventories, hardcoded certificate name, missing exact SHA,
  missing rollback history and missing smoke. A dependency-free Node validator
  checks the repository's deliberately literal workflow/service declarations.
- [x] Run the same existing deploy/signature/env regression suite from one
  PowerShell entrypoint in CI and as the first local preflight stage. Check
  Docker/tool prerequisites before expensive stages.
- [x] Extend the existing serialized cache with validated marker metadata,
  runtime-input fingerprint, stage-set handling, 24-hour security expiry and
  measured stage receipts. Regression tests cover retries, changed inputs,
  corrupt markers, new stages and security expiry without rerunning valid tests.
- [x] Update the workspace AGENTS.md, tracked root AGENTS.md, current state,
  architecture and DEV runbook: mandatory complete inventory, exact commit,
  contract gate, safe retries and verified runtime receipts. The parent routing
  file is local workspace policy; its normative release contract also lives in
  the tracked project AGENTS.md.
- [ ] Verify focused changes; capture/review every agreed source path in a
  release inventory; commit the complete removal and release improvements.
  Run the full exact-commit preflight once, resume only failed stages if needed.
- [ ] Push to dev-deploy, follow exact-SHA CI and seven signed images, verify
  rollout/smoke and runtime inventory, retire only the scoped obsolete DEV
  Mini App container, preserve pending email proofs and provider reference reads.
  Record local/GitHub/rollout timings; synchronize fastlife_dev after success.

Commands: node --test scripts/ci/release-contract.test.mjs;
pwsh -NoProfile -File scripts/ci/test-dev-deploy-preflight.ps1;
pwsh -NoProfile -File scripts/ci/validate-release-contract.ps1;
pwsh -NoProfile -File scripts/ci/dev-deploy-preflight.ps1 -ManifestPath <reviewed>.

No repeated standalone full suites before the full preflight. Preserve mandatory
CI and independent signature verification at deployment: those cross different
trust boundaries and are intentional, not redundant retries.

Publication and final stage timing evidence is recorded outside Git in
`.cache/release-2026-10-24/status.json`; unchecked release steps below are verified
there after this immutable source commit, without a status-only second push.
