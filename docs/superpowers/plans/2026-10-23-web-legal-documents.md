# Website legal documents and consent implementation plan

Status: implemented locally and verified; publication/deployment not performed. The user selected the Study24 data policy and explicitly
requested the complete website integration after reviewing the four drafts.

**Goal:** make the reviewed legal texts accessible from the website and record
explicit account-owned acceptance separately from optional email marketing.

**Architecture:** one shared versioned text catalog feeds the server-rendered
website and Go consent service. Operator requisites remain runtime configuration,
outside Git. Cookie identity, Origin/CSRF and PostgreSQL transactions own writes.

**Execution:** inline in the single active checkout. No commit, push, deployment,
real payment, provider calls, infrastructure relocation or invented agreements.

## Design decisions

- Reuse the four prepared Epifera drafts; add a separate email marketing consent.
  Public document pages use escaped structured text and shared site components.
- Keep documents marked draft until explicitly configured for publication after
  the outstanding infrastructure/provider/legal checks. Never accept a draft.
- Guest login shows links. After verified login, a version-aware acceptance gate
  handles password, registration and OAuth users equally. Refusal does not block
  access to document pages, logout, support or withdrawal of marketing consent.
- No automatic claim that visiting/login itself authorizes advertising or recurring
  payments. Marketing is unchecked initially, optional and independently revocable.
- Acceptance evidence contains account ownership, exact version/content checksum,
  server time, locale and idempotency key. No raw IP, contact or request content.
- Marketing consent is tied to the verified primary email available at the time;
  changing that address must not subscribe a new recipient automatically.
- Immutable publication identifiers include operator configuration. Frontend/backend
  drift prevents acceptance rather than recording acceptance of a different text.
- Local preview remains an explicitly labeled read-only demo. It must not invent
  durable consent records. Existing migrations and unrelated changes are preserved.

## Tasks

- [x] Shared text catalog, safe public document pages and complete legal navigation.
- [x] Additive consent persistence, account-owned handlers and regression checks for
      stale versions, CSRF, idempotency, optional marketing and withdrawal.
- [x] Shared login notice, post-login acceptance screen and profile preferences;
      focused frontend tests, types/lint/build and browser inspection on port 7159.
- [x] Update UI routing/runbook, record concrete outstanding deployment checks and
      verify the final diff/status. Do not claim an external deployment.

## Validation

Use failing behavior tests before implementation. Backend checks cover consent
recording and HTTP authorization; SQL checks exercise transactional replay and
recipient changes when a disposable local database is available. Frontend checks
cover legal links, optional unchecked marketing, rejection of mismatched catalogs,
error/retry behavior and locale preservation. Public pages are read-only and do
not trigger account creation, payment or provider requests.

## Verification results (2026-10-23)

- Shared Go/Node checksum parity fixture and reviewed source validation passed.
- Focused Go catalogue/service/HTTP/config/API tests passed; `go vet -p 1` passed.
- Actual disposable PostgreSQL evidence/replay/email-change/paused-withdrawal tests passed. The lost-response withdrawal retry across a changed server revision was observed failing before the fix, then passing.
- Seven UI test files / 54 tests passed. Typecheck, full and final focused ESLint, final production build, packaging assertions and synthetic Compose validation passed.
- Browser verified all five public texts, resolved placeholders, no horizontal overflow at the current viewport, RU/EN navigation and login links on the existing port 7159. No real agreement was accepted in the browser.
- Source scan found no private operator name/INN in the scoped application files. Git diff whitespace check passed.
- The optional independent reviewer dispatch could not run: the project-required gpt-5.5 model was unavailable. Boundaries were reviewed directly and the withdrawal issue above was fixed with a regression test.
- No commit/push/deploy, remote migration, payment or provider request. Runtime publication remains false; factual processor/storage checks remain in the readiness report.
