# Release views implementation plan

> Execute with executing-plans and bounded independent agents. Approved design
> is ../specs/2026-11-02-release-views-design.md. No commits, push or deployment.

**Goal:** A usable DEV/PROD view and readiness inventory with actual server gates.
**Architecture:** One JSON registry, shared Go/TS predicates, server cookie plus
existing DEV authorization, global client context and existing developer panel.
**Tech stack:** Existing Go/Next/React, no new dependencies or environment copies.

**Status:** implemented locally; no commit/push/deployment. All old loading changes
remain in the same checkout. Registry currently has21features and24page records.

**Verification:** Final aggregate: 70 changed/new frontend test files, 736 tests
passed, including native Nginx regressions. Go release policy/websession/cmd-api
tests and vet passed. Fresh npm audit reports 0 advisories; infrastructure and
model admission checks passed. Full lint, types, production build, standalone
packaging and asset validation passed. The first full frontend run exposed old
fixture assumptions about unrestricted drafts and unfinished controls; fixtures
now explicitly opt into DEV. A timing test now locates the actual WorkspaceFrame
rather than indexing the removed duplicate developer-panel sibling. Profile
snapshot regressions cover late delivery, two-stage refresh and older cached DEV
data without losing child drafts. Logs are in ignored `.cache/release-views-*`.
Infrastructure validation used `-SkipPromtool`: Prometheus rule validation was
not rerun in this scoped task. This is not an exact-commit release preflight or
an assertion that every repository test was rerun after the final edit.

**Browser evidence:** `.cache/responsive-qa/2026-11-02-release-views/summary.json`.
Actual localhost/isolated Chromium, DPR1/zoom100%, all8required CSS widths:
32workspace mode/panel combinations and16file-tool mode/bounds cases passed.
Catalogue103DEV→36PROD, no pending/dev-smoke/enabled-audio inPROD. Draft, account/
balance and four chat names retained; preference survives refresh. Direct MCP/
image-generator404 retains launcher; DEV restores actual content at the same URL.
Profile/files/login/legal preservation tested1440. No paid requests or real accounts.
Tariff dialog follow-up: 16/16 mode/viewport cases passed across the same eight
widths. DEV has six cards; PROD hides only the team card. Individual plan copy,
prices and computed presentation remain identical. Evidence: `tariffs-results.json`
and screenshots alongside the summary. No checkout or plan activation was run.

**Visual limitations:** this scoped check is not a fresh24-template visual audit.
Existing768px heading clipping and360px general-file Copy-button clipping are
recorded separately; no related presentation CSS was changed. The new panel fits
all8widths. Initial400/700ms public-route observations were superseded by actual
rendered-content waits and preserved in the audit history.

**Independent review:** server environment authority, sanitized forwarding,
stored-job activation/retry and history access reviewed. Two-stage and older
cached DEV source regressions were fixed by explicit snapshot-view metadata,
profile-bound freshness and subscriber-preserving resource replacement.

## Tasks

- [x] Define registry and pure TS/Go policies. RED tests: unknown/prod fail closed,
  DEV can only expose existing admitted features, unknown feature IDs excluded.
- [x] Server profile/preference endpoint, trusted forwarding and route guards.
  Test forged cookies/headers/query, cross-origin POST, invalid/oversized JSON,
  protected DEV access, local preview and preserved legal/account routes.
- [x] Go catalogue and write guards before orchestrator side effects. Tests cover
  direct production overrides, stale DEV jobs activation/retry and live baseline.
- [x] Root profile context and shared UI filtering, cache profile invalidation.
  Test late DEV delivery after switch, menu/action/link filtering, unchanged DEV.
- [x] Extend existing developer panel, status inventory/24 page rows and explicit
  return-to-DEV on excluded pages. Test errors, pending switch and no data writes.
- [x] Fresh dependency/security/infra admission before long checks; relevant unit,
  types/lint, production build, packaging and eight-width actual-browser QA.
- [x] Independent review, fix findings, update docs/UI index and report exact
  local status and any remaining limitations without calling it deployed.
