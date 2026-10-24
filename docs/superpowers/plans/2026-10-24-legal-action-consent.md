Status: superseded. Do not use for current implementation.
See: docs/superpowers/plans/2026-10-24-registration-legal-notice.md.

# Nonblocking legal acceptance implementation plan

**Goal:** Remove the page-replacing legal wall. Keep all workspace views available;
request explicit current terms only when a user starts new work or checkout.

**Architecture:** The workspace mounts an action boundary, not a page gate.
Before guarded POSTs the browser mutation transport waits for the boundary to
check cookie-owned acceptance and, if needed, show a compact confirmation. The
original mutation is sent once, with its original payload/key. The backend remains
the authority and retains all existing 428, auth, CSRF and ownership checks.
Advertising preferences stay separate in the profile and default to unchecked.

**Scope:** Existing single checkout/localhost:7159. No backend authorization or
legal-text changes, no automatic acceptance, no new packages, no paid calls,
no commit/push/deployment. Reference feedback is about presentation and flow;
it does not authorize fabricating a user's acceptance event.

## Tasks

- [x] Reproduce the global wall in a failing test in LegalConsentGate.test.tsx:
  published unaccepted terms must not replace workspace children.
- [x] Add a scoped browser legal-action hook beside browser.ts. Mirror the
  current server POST boundaries only: conversations/messages, generation,
  input uploads and payment intents. Reads, deletion, profile and marketing
  remain outside the hook. Tests cover one send, unchanged payload/key, cancel,
  concurrent callers, original abort signal, binding/account changes and XHR.
- [x] Replace the layout gate with LegalActionBoundary. Use shared ConfirmDialog
  with a primary (non-destructive) action; focus trap must include document links.
  Fetch status only after an action. Match exact revision before explicit save;
  retry lost acceptance responses with the same key. Never opt into marketing.
  Failed checks show retry/cancel while retaining the workspace and user input.
- [x] Keep LegalConsentPreferences in the profile, including independent
  withdrawal when publication is paused. Remove the obsolete page gate.
- [x] Update legal UI/transport contracts and active context. Run focused legal,
  browser/session, confirmation and affected generation/layout tests; typecheck,
  lint, production build, packaging/docs checks. Backend legal tests confirm
  server checks were preserved.
- [x] Verify local music/catalog/files/profile remain visible, save screenshot,
  and leave the same site running. Report local status without claiming DEV.

## Verification

From web/platform: `npm exec -- vitest run src/features/legal src/lib/web-api
src/components/ui/ConfirmDialog src/features/image-generation/ImageGenerationPanel
src/features/account/ProfileWorkspace src/app/app/layout.test.tsx` (use actual
existing layout test name); `npm run typecheck`, `npm run lint`, `npm run build`,
`npm run test:packaging`. Root: focused Go legal/websession tests,
`pwsh -File scripts/ci/validate-docs.ps1`, `git diff --check`.

## Verification result

- The original global gate regression failed by hiding Music workspace. The
  replacement preserves children immediately and performs no status read until
  a guarded action. Accepted accounts do not flash a confirmation while checking.
- Frontend: 196 tests passed in 19 files, including guarded JSON/XHR mutations,
  unchanged original payload/idempotency, cancellation, shared decisions, stale
  account/binding checks immediately before send, revision drift, lost-response
  retry, independent advertising withdrawal and keyboard navigation of links.
- Typecheck, full lint, production build and packaging passed. Go tests passed
  for web/platform/legal, internal/service/legalacceptance and
  internal/adapter/inbound/websession using the workspace Go build cache.
- Browser: music and model catalog show normal workspace contents. The compact
  confirmation was inspected on the existing local UI-states route, which makes
  no acceptance/payment/generation request. Document links are keyboard reachable;
  cancellation closes it. At 390px, the dialog stays within 12px side margins;
  the viewport was restored. Screenshots: .cache/legal-action-dialog.png and
  .cache/legal-action-dialog-mobile.png. Profile/reading/deletion remain outside
  the action hook and are covered by focused tests.
- Original consent history and server checks preserved; legal texts, providers,
  billing and real account choices unchanged. Everything remains local. No commit,
  push, DEV deploy or paid/provider calls performed.
