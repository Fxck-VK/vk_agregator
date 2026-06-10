# Merge Checklist

Use before merging another branch into `fastlife_dev` or moving `fastlife_dev`
forward.

## Read Order

Read only the current active routing/context docs unless a conflict requires
deeper archaeology:

1. `AGENTS.md`
2. `.agents/state.json`
3. `TASKS.md`
4. `DECISIONS.md`
5. `docs/DOMAIN_DEPLOYMENT_PLAN.md`

Historical merge notes live under `docs/archive/**` and are not active context
by default.

## Checks

- Confirm the target branch with `git branch --show-current`.
- Inspect local changes with `git status --short`.
- Refresh remote refs before merge/push.
- Run the checks relevant to touched surfaces.
- Do not weaken auth, billing, provider, webhook, idempotency or secret-handling
  invariants to resolve conflicts.
