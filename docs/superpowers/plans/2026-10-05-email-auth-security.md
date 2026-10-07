# Email auth security implementation plan

> **For agentic workers:** Use subagent-driven-development with bounded ownership and task reviews. The user approved the fixes and security notices; proceed without another approval gate.

**Goal:** Consume recovery proofs once, prevent stale password logins, commit password changes and audits together, revoke other sessions, and deliver security notices independently of SMTP availability.

**Architecture:** Redis consumes the exact validated challenge atomically. PostgreSQL serializes login/session issuance and password mutations with the existing account row lock and conditional credential checks. Security notices enter a transactional database outbox and are delivered by a bounded background dispatcher using the existing SMTP adapter.

**Tech stack:** Existing Go, PostgreSQL, Redis, SMTP and browser cookie APIs; no new dependencies.

## Constraints

- Preserve unrelated music/sidebar work. No commit, push or live-account changes are part of this request.
- Never log email addresses, passwords, proof hashes, session tokens, raw SMTP errors or notice bodies.
- Keep Origin/CSRF, cookie ownership, rate limiting and verified-email checks intact.
- An unavailable store cannot authorize a request. SMTP failures retry asynchronously and cannot change a successful password/identity response.
- A current session may be preserved only when established by the server-side authenticated principal, never by request JSON.

## Task 1: Atomic proof consumption

Files: `internal/service/accountlink/{service.go,memory_store.go,backup_email.go}`, `internal/adapter/storage/redis/account_link.go` and adjacent tests.

- [x] Add regressions for concurrent redemption, store failure, invalid proof not consuming a valid proof, and replacement between read and consume; run them and record the expected failure.
- [x] Add `ConsumeChallenge(ctx context.Context, key string, expected Challenge) error` to the store. Validate the code as today, then atomically compare and delete the exact stored challenge. Use Redis Lua and one memory mutex; propagate failures. Apply to email recovery/link, backup replacement and shared phone verification.
- [x] Run accountlink/account boundary suites and real Redis integration coverage with an isolated key prefix.

## Task 2: Password security transactions

Files: accountauth password/session implementation; PostgreSQL/memory security and session adapters; account/websession handlers and adjacent tests.

- [x] Add deterministic regressions for reset/change/unlink between password validation and session creation, audit failure rollback, concurrent changes, and preserving only the server-established current session; observe failure before implementation.
- [x] Introduce a combined password-authentication/session operation. Capture the verified credential and email binding, recheck them under the same account lock used by password mutation and unlink, and insert the session inside that transaction. Both browser and token-returning password login use it; unsupported adapters fail closed.
- [x] Commit credential CAS, verified email ownership, audit, revocation of other sessions and notice enqueue together. Initial setup remains insert-only. Browser principal supplies the preserved session; legacy calls without a current session revoke all. Recovery continues revoking all sessions.
- [x] Use the notice helper from Task 3 inside successful password transactions:

```go
EnqueueAccountSecurityNotifications(ctx context.Context, db Querier, accountID uuid.UUID, kind domain.AccountSecurityNoticeKind, extraRecipients []string, at time.Time) error
```

- [x] Run accountauth, account/websession boundary suites and isolated PostgreSQL regressions. Keep ordinary password proof failure behavior stable.

## Task 3: Security notices

Files: new domain notice contract, PostgreSQL notice adapter, migration `000055_account_security_notices`, accountdelivery SMTP extension, bounded dispatcher service and `cmd/api` wiring; verified email mutation hooks.

- [x] Add tests proving SMTP failure does not fail the security operation, retry works, concurrent dispatchers lease separate notices, and failed/rolled-back mutations enqueue nothing; observe missing behavior before implementing.
- [x] Create a durable outbox containing a fixed event kind, recipient, event time and retry/lease state. Capture only verified recipient addresses in the transaction; for email removal/replacement include the old address. Deduplicate recipient addresses. Do not include passwords, codes, tokens or private URLs.
- [x] Implement the helper signature above for password mutation integration. Enqueue notices transactionally for adding/replacing/removing a backup email. Deliver fixed Russian security templates with the existing SMTP connection handling. Disabled delivery stays fail-closed.
- [x] Lease bounded batches, send with deadlines, retry with increasing delay and remove sent/expired rows under a documented retention bound. Log only notice UUID/event enum/status. Start the dispatcher only when SMTP is configured; avoid unbounded goroutines or SMTP inside mutation transactions.
- [x] Run dispatcher, sender and isolated PostgreSQL tests using fake delivery only.

## Task 4: Integration and review

- [x] Review task diffs for requirements and code quality; resolve important findings.
- [x] Update browser account/security contract, architecture security boundary and documentation index.
- [x] Run affected Go suites/vet, real isolated PostgreSQL/Redis tests, documentation validation and frontend checks appropriate to the browser contract.
- [x] Obtain an independent final review of the authentication/transaction/outbox changes and resolve important findings.
- [x] Report verified behavior and limits. No real mail or real account mutation is needed for these checks.

## Verified outcome

Implemented locally without committing or pushing. Task reviews and the final independent whole-scope review found no blocking issues. Parent verification passed 14 affected Go packages and vet, 10 actual isolated PostgreSQL tests, the actual isolated Redis concurrency test, 37 frontend auth/registration/account-security tests and typecheck, documentation/infrastructure/migration validation. Both minor review suggestions were strengthened and their actual PostgreSQL tests rerun successfully. Local race could not run without CGO/compiler; Linux CI race coverage is configured but not executed in this task. Migration 000055 is required before updating runtime. SMTP tests use fake delivery and do not confirm inbox receipt.
