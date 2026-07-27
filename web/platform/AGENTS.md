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

There is no framework scaffold in this directory yet. Add dependencies and
runtime code only with an approved first web feature, including lint,
typecheck, tests, build, Docker packaging, and CI coverage in the same change.
