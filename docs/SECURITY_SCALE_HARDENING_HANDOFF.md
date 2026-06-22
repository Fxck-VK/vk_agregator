# Security Scale Hardening Handoff

This handoff summarizes the hardening work completed on `fastlife_dev` in
commit `80d8337d8e2a2ead2e04567280e48de9dbdd077c`.

It is written for the next human reviewer and for the next coding agent. It
does not contain secrets, raw launch params, prompt bodies, provider payloads,
private URLs or raw PII.

## Executive Summary

The hardening pass closed the tracked risks R1-R10 around leakage prevention,
public route abuse, local memory growth, upload pressure, retention, antispam
degradation, payment link safety and load-test safety.

The most important architectural invariants remain intact:

- VK handlers, Mini App BFF and `cmd/api` do not call AI providers directly.
- Billing still goes through append-only ledger/payment flows.
- Provider/payment/VK secrets and raw sensitive payloads are not logged.
- Job processing remains asynchronous and worker-owned.
- Retention cleanup keeps idempotency, billing, job and audit records.

## What Changed And Why It Matters

| Area | What changed | If this had not changed |
|---|---|---|
| Runtime logging | Runtime errors are logged with bounded normalized codes/classes instead of raw external details. | Provider/payment/VK errors could leak sensitive details or high-cardinality external strings into logs. |
| Payment redirect | Added a dedicated public payment redirect handler, app-level limits and nginx edge limits before repeated lookup pressure. | Public `/payments/vk/{id}` probes could create unnecessary DB pressure and easier route scanning. |
| VK top-up links | VK bot payment output now uses server-owned redirect links or fails closed with a safe unavailable message. | Missing or invalid public redirect config could expose direct provider confirmation links to users. |
| Rate limiting | Local limiter now has idle TTL, bounded sweeping and a hard bucket cap. | High-cardinality keys could grow process memory over time, especially under public traffic. |
| VK inbound payload retention | New VK inbound rows store metadata-only payloads; legacy raw payloads get batched expiry/redaction. | Raw callback payloads could live indefinitely and retain user text, URLs or other sensitive data. |
| Mini App uploads | Uploads use streaming multipart parsing, pre-read size checks, concurrency limits and aligned nginx body limits. | Concurrent large uploads could buffer too much data in memory and make API instances easier to exhaust. |
| VK local UI state | Best-effort local active menu/dialog caches have TTL and peer caps; critical dialog mode remains in Redis with TTL. | Process-local maps could grow without a clear bound and stale callbacks could behave inconsistently. |
| Antispam degradation | Dependency errors now block expensive generation candidates while allowing cheap control commands; denied/degraded events complete without retry amplification. | Antispam outages could fail open for expensive operations or amplify retries during abuse waves. |
| Load-test safety | Load-test config and k6 scripts refuse known production hosts by default and stay mock-backed unless explicitly overridden. | Generic load tests could accidentally hit production, real VK delivery, YooKassa or paid providers. |
| Command raw text retention | `commands.raw_text` is classified as `user_content`; retention columns, cleanup and tests redact old text after linked jobs are safe. | Normalized user command text could live forever without explicit TTL or redaction policy. |

## Key Files For Review

- `internal/platform/logging/logging.go`
- `internal/adapter/inbound/paymentredirect/handler.go`
- `internal/adapter/inbound/vk/handler.go`
- `internal/adapter/inbound/vk/menu.go`
- `internal/platform/ratelimit/ratelimit.go`
- `internal/adapter/inbound/miniapp/upload.go`
- `internal/service/antispam/service.go`
- `internal/service/maintenance/service.go`
- `internal/adapter/storage/postgres/maintenance.go`
- `migrations/000024_command_raw_text_retention.up.sql`
- `docs/DATA_RETENTION_POLICY.md`
- `docs/LOAD_TESTING.md`
- `docs/DEV_CONTOUR.md`

## New Or Changed Configuration

Example env files were updated only with non-secret knobs:

- `RETENTION_COMMAND_RAW_TEXT_DAYS=30`
- `COMMAND_RETENTION_BATCH_SIZE=500`
- Mini App upload size/concurrency controls.
- Rate limiter capacity/TTL controls.
- Payment redirect public-route limits.
- Load-test refusal controls such as `K6_ALLOW_PRODUCTION_LIVE_SMOKE=false`.

Production operators must still provide real secrets only through approved
secret stores and runtime env files, not through committed docs or examples.

## Verification Already Run

Final verification before push:

- `git diff --check`
- `docker compose config`
- `go test ./...`
- names-only diff secret scan
- temporary/backup/build artifact scan

The full Go suite passed after an unsandboxed rerun because the local sandbox
blocked Go build cache access and `httptest` localhost listeners. That was an
environment limitation, not a test failure.

## Rollback Notes

The pushed commit is one logical hardening commit:

```text
80d8337 security: harden public and retention surfaces
```

Rollback options:

- Before applying DB migrations, a code-only revert of the commit is enough.
- After applying `000024_command_raw_text_retention`, schema rollback requires
  running the matching down migration manually in a controlled environment.
- After command or VK payload redaction has run, redacted raw text/payloads are
  irreversible unless restored from a database backup.
- If investigating retention behavior, pause the worker maintenance loop before
  rollback or manual data inspection.
- Do not automatically roll back production schema during application rollback.

## Residual Risks And Follow-Ups

These are not reopened R1-R10 items, but they remain important before broad
public production traffic:

1. Production webhook routing still needs confirmation: the YooKassa route must
   reach `cmd/provider-webhook` / `PAYMENT_WEBHOOK_ADDR`, not `cmd/api`.
2. Live YooKassa smoke remains pending for `payment.succeeded`,
   `payment.canceled` and `refund.succeeded` using dashboard-delivered
   webhooks.
3. Production deployment shape still needs final verification for static Mini
   App hosting, `cmd/api`, `cmd/worker`, dedicated payment webhook runtime,
   TLS/proxy headers and service units.
4. Credential-bound smoke remains pending for real OpenAI, DeepInfra and VK
   delivery before external users.
5. Edge/proxy body-size limits must be verified in the real public path before
   public reference uploads.
6. Automatic, partial or already-spent credit refunds still need lot/FIFO
   attribution before they are safe to broaden.
7. The local rate limiter is now memory-bounded, but cross-instance fairness
   still depends on edge/shared rate limiting strategy.
8. Payment redirect is bounded by route/edge limits; opaque unguessable redirect
   tokens remain a possible future hardening improvement.
9. Generic k6 scripts are guarded against production by default, but local k6
   execution was not performed because k6 was not installed during the hardening
   pass.

## Agent Starting Point

Next agent should read, in order:

```text
AGENTS.md
.agents/state.json
docs/SECURITY_SCALE_HARDENING_HANDOFF.md
docs/DATA_RETENTION_POLICY.md
docs/LOAD_TESTING.md
docs/DEV_CONTOUR.md
```

Do not read `.agents/logs/**` by default. Read those logs only for repeated
known-error debugging or when explicitly asked.
