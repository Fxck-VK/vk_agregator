# Conversation origin model icons

Goal: each chat uses the icon of its first accepted model, independently of later model selection.

Use existing durable, account-owned Jobs as the source of the first public model ID, ordered by creation time and ID. Exclude jobs that have not been accepted (prepared/validated/awaiting_payment). Return only `first_model_id` in the safe conversation DTO; never expose job parameters. No provider, billing or authentication changes. Reuse ModelIcon and its explicit public-ID registry, including the existing fallback for unknown models. Existing job records remain the source of truth across browser reloads/devices; message pagination does not affect the icon.

- [x] Add failing tests for pinned artwork, cache round trips, accepted/rejected model switches and backend owner-scoped earliest-job reads.
- [x] Extend conversation reads/DTO/schema and index the account/conversation job lookup. Keep metadata cache and replace/reconcile paths aware of the new field.
- [x] Pin visible metadata immediately after accepted sends; mirror behavior in local fixtures/simulator. Render existing ModelIcon in collapsed sidebar.
- [x] Run focused frontend/backend tests, typecheck/lint, inspect the same local website and update the UI contract. No commit/push/deploy requested.

Validation: 286 frontend tests passed across the conversation suite/API schema/icon registry; a final 65-test focused run also covers the added unknown-model fallback. TypeScript and ESLint passed. Go tests and vet passed for websession, PostgreSQL/memory storage and dialogcontext. The earliest-model and conversation-management integration tests passed on PostgreSQL 16 in disposable WSL schemas. Browser preview: a new Claude chat retains Claude's icon after an accepted GPT-5.5 turn; the seeded Nano Banana chat retains Google's mark after a Claude turn. No paid provider calls or remote deployment.
