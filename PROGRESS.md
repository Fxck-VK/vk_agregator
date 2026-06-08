# PROGRESS

Status: current progress is machine-readable.

This file is intentionally no longer an implementation log. Agents must not use
old PR/build logs as current context.

Current state:

- `.agents/current/state.json`
- `.agents/current/context.json`
- `docs/MANIFEST.json`

Historical progress log:

- `docs/archive/2026-06/PROGRESS.legacy.md`

Machine-readable append-only logs:

- `.agents/logs/actions.jsonl`
- `.agents/logs/errors.jsonl`
- `.agents/logs/context.jsonl`

Rule: read archived progress only when the user explicitly asks for historical
investigation or archaeology.
