# TASKS

This file is the short human-readable backlog. Machine-readable current context
and routing live in `.agents/state.json`. Historical completed work and old
phase plans live under `docs/archive/**` and are not active context by default.

## Current Focus

- YooKassa billing and shared VK Bot / Mini App top-up rollout.
- Keep credits granted only by provider-verified webhook/reconciliation ledger
  paths.
- Keep VK Bot and Mini App as job-creation surfaces only; provider calls stay in
  `cmd/worker` / provider adapters.

## Open Follow-Ups

- Configure the public HTTPS route
  `https://neiirohub.ru/billing/webhooks/yookassa` to reach
  `cmd/provider-webhook` / `PAYMENT_WEBHOOK_ADDR` rather than `cmd/api`.
- Repeat YooKassa live smoke with dashboard-delivered webhooks for
  `payment.succeeded`, `payment.canceled` and `refund.succeeded`.
- Retest `payment.canceled` through the protected operator `capture:false`
  local-intent smoke path: create intent, pay to `waiting_for_capture`, cancel,
  verify webhook/reconciliation moves the intent to `canceled` with no top-up.
- Implement lot/FIFO attribution before automatic, partial or already-spent
  credit refunds.
- Finish production deployment shape for `neiirohub.ru`: static Mini App,
  `cmd/api`, `cmd/worker`, dedicated payment webhook runtime, TLS/proxy headers
  and service units.
- Run live credential-bound smoke for real OpenAI, DeepInfra and VK delivery
  paths before external users.
- Complete the full video media pipeline: scan/probe, transcode and VK-ready
  variants before relying on real video delivery at scale.
- Add production retention/archival for old `conversation_messages`; keep
  compact summaries and recent hot turns only.
- Replace local/extractive dialog summary compaction with a dedicated cheap
  summarizer job/model if semantic summaries become necessary.
- Add admin DLQ inspection/replay tooling and worker per-pool isolation when
  traffic requires it.

## Recently Completed

- Product catalog admin routes and immutable payment intent snapshots.
- Mini App top-up product selection, payment intent creation and safe payment
  history UI.
- YooKassa webhook inbox, reconciliation, idempotent top-up ledger grants and
  protected manual full-refund MVP.
- Local YooKassa smoke: successful checkout through reconciliation, webhook
  replay dedup, idempotent manual refund and safe Mini App payment history.
- Merge/document routing cleanup: stale merge handoff docs archived and the
  current merge checklist shortened.
- Mini App referral/account UI over the shared referral backend.

## Archived History

- `docs/archive/2026-06/TASKS.legacy.md`
- `docs/archive/2026-06/ROADMAP.legacy.md`
- `docs/archive/2026-06/TESTING.legacy.md`
- `docs/archive/2026-06/BILLING_AGENT_HANDOFF.legacy.md`
- Other completed audits, merge handoffs and historical logs under
  `docs/archive/**`.
