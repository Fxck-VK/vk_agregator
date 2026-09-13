# web/platform/AGENTS.md - Web Platform Local Rules

This file applies to `web/platform/**`. Read the root `AGENTS.md` first.

## Role

The standalone web platform is a thin client over the shared backend. It is an
independent deployable frontend, not a provider client, billing authority, or
data store.

## Must Not

- Do not call AI, payment, VK, storage, or database providers directly.
- Do not store backend secrets, provider keys, refresh tokens, or raw PII in
  source code, browser storage, logs, analytics, or error reports.
- Do not trust client-supplied account IDs, roles, balances, prices, job
  statuses, ownership claims, or moderation results.
- Do not render user/provider content as trusted HTML.
- Do not expose raw provider or private storage URLs.
- Do not make `/miniapp/*` the permanent API contract for the web platform.

## Must

- Use the shared Account Layer and server-issued web sessions for identity.
- Use backend APIs for jobs, payments, ledger-backed balances, and artifacts.
- Send stable idempotency keys for paid or otherwise repeat-sensitive writes.
- Treat disabled controls as UX only; backend idempotency and authorization
  remain mandatory.
- Keep authentication material in secure, server-managed cookie/session flows;
  never put credentials in query strings or local storage.
- Escape untrusted content and normalize user-facing errors.
- Keep the app independently buildable, testable, and deployable from the VK
  Mini App and operator frontend.

## Implementation Gate

The app scaffold and shared components already exist. Reuse the current package
configuration and scripts. New dependencies or runtime surfaces must belong to
the user-authorized feature and include the relevant lint, typecheck, tests,
build, packaging and CI checks for that change.

## UI Reuse Workflow

For UI changes and text-only UI proposals within `web/platform`, use this order:

1. Find the task in [the short UI index](docs/ui-index.md). If it is already in
   the current task context, reuse it; reread affected entries only as needed.
2. Read only the linked section of [the catalog](docs/ui-catalog.md) and, when
   needed, the matching [example](docs/ui-catalog-examples.md). Do not load the
   entire catalog, all examples or the archived inventory by default.
3. Check the actual component export, props, shared styles and one current
   consumer before recommending or changing it. The catalog is a navigation
   aid; current code determines implemented behavior and the user's request
   determines intended behavior. Do not present an unfinished action as working.
4. Reuse the existing component/style, or extend it when the intended behavior
   fits its purpose. Check other affected consumers when extending a shared
   contract. If no indexed solution fits, search the relevant source directories
   before adding a new component; briefly explain why a separate one is needed.
5. In the same change, update the corresponding catalog entry and any affected
   index row/example when props, behavior, defaults, paths, limitations or reuse
   recommendations change. Keep still-accurate entries as they are; do not
   regenerate the archived inventory for routine work. Check changed links and
   TypeScript examples when their code changes.

When relevant, tell the user which existing component or style already covers
their request. Keep implementation within the requested scope: a text-only
proposal remains text-only, and UI-only documentation is not needed for unrelated
backend work. These instructions add no separate approval step.
