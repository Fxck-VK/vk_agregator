# Inline returning account implementation plan

**Superseded:** The owner removed the extra remembered-email card and demonstration controls. Current implementation and verification: `docs/superpowers/plans/2026-10-31-provider-login-reference.md`.

> **For agentic workers:** Use the executing-plans workflow, task by task, in the existing checkout. User approved the inline common-form design. No commit, push or deployment is authorized.

**Goal:** Keep remembered-account login inside the ordinary sign-in form, with other providers and email always available.

**Architecture:** CanonicalLogin owns presentation and the explicit email-versus-remembered selection. Existing server-owned hints, password verification, OAuth start, forget and session confirmation remain authoritative. LoginWindow and the showcase do not change.

**Tech Stack:** Existing React/Next.js client components, CSS modules, Vitest/Testing Library and isolated Chrome QA. No dependencies added.

## Global constraints

- One checkout and localhost:7159; Russian-only publication; no browser credential/identity storage.
- Only server-advertised visible providers; Google stays disabled. Hint is presentation, never authentication.
- Preserve email/password, six-cell verification, recovery, signup, persistence and safe return behavior.
- Use the neutral account artwork, shared Button and existing provider icons; no duplicate brand avatar or separate welcome-back screen.
- Browser viewport widths 360, 390, 768, 1024, 1440, 1920, 2560 x 1440 and 3840 x 2160 CSS pixels, zoom 1 and DPR 1.

### Task 1: Regression tests and unified choice

Files: `web/platform/src/features/auth/CanonicalLogin.tsx`, new `CanonicalLogin.remembered.test.tsx`, `LoginForm/LoginForm.module.css`, `web/platform/src/i18n/account-auth.ts`.

- [x] Write tests showing an available provider hint personalizes its first button while other providers, email and signup remain visible; assert no standalone welcome-back heading or brand avatar.
- [x] Verify tests fail on the current exclusive returning screen: `npx vitest run src/features/auth/CanonicalLogin.remembered.test.tsx` from web/platform.
- [x] Keep choice rendering unconditional; sort the remembered provider first, replace its generic button with a compact account-labelled button and use the same OAuth action. Add a remembered-email button which navigates to password with explicit remembered selection. Normal email continuation clears that selection.
- [x] Keep forget explicit, server-confirmed and retryable. Ignore invalid/disabled hints, bound visible labels and retain delayed-lookup race guards.
- [x] Run remembered and all auth regression tests. Verify remembered password requests contain `remembered:true`, normal requests contain the typed email, OAuth sends only locale, and server-required code is still necessary.

### Task 2: Verification and documentation

Files: `web/platform/docs/ui-catalog.md`, `docs/INDEX.md`; ignored `.cache/responsive-qa/2026-10-31-inline-returning-login/`.

- [x] Run scoped ESLint and full TypeScript check, asset/packaging checks and production build.
- [x] Use the existing localhost for the genuine default form. Use isolated, clearly labelled synthetic fixtures with the actual components for provider/email hints, long names, forget failure, password and verification. Never create a QA route or seed credentials in the application.
- [x] Check all eight widths, button alignment, no horizontal overflow, keyboard controls, provider/email/forget transitions. Save actual viewport metrics, screenshots and outcomes; inspect screenshots.
- [x] Update the current UI contract, link this plan, inspect the final scoped diff, and record actual results. Report local-only status.

## Verification outcome

- RED reproduced the exclusive returning screen; new assertions failed before implementation.
- 269 tests in 31 auth/sign-in/i18n files passed, including 12 new remembered-account cases. Scoped ESLint and full TypeScript passed.
- Production build, asset validation and packaging passed. The first sandbox build could not create a Windows junction for sharp; the same build passed with the required local filesystem access. Original dev next-env imports restored afterward.
- 56 browser cases passed: genuine local default preview plus six actual-component synthetic states at all eight required widths. DPR 1, zoom 1; original login font loaded, no horizontal overflow, internal form scroll or page errors. Keyboard continuation, recovery, signup and successful/error forget were exercised.
- Evidence: `.cache/responsive-qa/2026-10-31-inline-returning-login/results.json`, screenshots and QA script; `.cache/returning-login-{tests,regression,lint,types,build,assets,packaging,responsive}.log`.
- Independent scoped review found no material issues. No live OAuth, email sending, backend changes, commit, push or deployment.

## Owner-requested local demonstration

- [x] Add server-gated `LoginPreview` on the initial local login screen, with ordinary/provider/email selection and safe `?demo=provider|email` links.
- [x] Reuse the actual form with synthetic `previewAccount` hints, ignored unless preview is enabled. Provider actions remain disabled, email password/code uses the existing no-request preview. Switching resets input; no cookies or browser storage are written.
- [x] Verify new tests fail before implementation, then pass with the whole auth/i18n regression: 276 tests in 33 files, lint and full typecheck.
- [x] Check all three states on actual localhost at all eight widths: 24 cases, keyboard switching, reload, password clearing and local dismissal passed with zero auth requests or page errors. Evidence: `.cache/responsive-qa/2026-10-31-login-demo/results.json` and screenshots.
- [x] Production build passed; demo query cannot opt a non-preview server into demonstration. Documentation updated; no publication.
