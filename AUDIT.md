# AUDIT

Status: current audit source is the active docs set plus machine-readable logs.

This file is intentionally a pointer, not a rolling audit log. Agents must not
use old audit/review reports as current context by default.

Current sources:

- `docs/MANIFEST.json`
- `.agents/current/state.json`
- `.agents/current/context.json`
- `README.md`
- `RUNBOOK.md`
- `TASKS.md`
- `DECISIONS.md`
- `docs/ARCHITECTURE.md`

Historical audit artifacts:

- `docs/archive/2026-06/AUDIT.legacy.md`
- `docs/archive/2026-06/miniapp-frontend-audit.legacy.md`
- `docs/archive/2026-06/miniapp-review.legacy.md`
- `docs/archive/2026-06/integration-report.legacy.md`

Rule: use historical audit artifacts only when the user explicitly asks for old
findings, regression archaeology or merge-history analysis.
