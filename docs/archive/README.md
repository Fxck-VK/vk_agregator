# Archive

This directory contains historical logs, audits, merge handoffs and completed
PR context.

Agents must not read `docs/archive/**` as current project context by default.
Use it only when the user explicitly asks for historical investigation,
regression archaeology or a past-PR audit.

Current context lives in:

- `docs/MANIFEST.json`
- `.agents/current/state.json`
- `.agents/current/context.json`
- `README.md`
- `RUNBOOK.md`
- `TASKS.md`
- `DECISIONS.md`
- `docs/ARCHITECTURE.md`
