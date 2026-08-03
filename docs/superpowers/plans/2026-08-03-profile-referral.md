# Profile Referral Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development` or `executing-plans` to implement this plan task-by-task.

**Goal:** Add a responsive, selectable referral programme view to `/app/profile` without inventing referral data that the neutral web API does not provide.

**Architecture:** `ProfileWorkspace` will own an in-memory tab selection and render either the existing general profile content or a new `ProfileReferralProgram`. `ProfileReferralProgram` is presentation-only; `ProfileReferralFaq` holds only local disclosure state. No backend call or new API contract is introduced.

**Tech Stack:** TypeScript, React 19, Next.js 16, CSS Modules, Vitest, Testing Library.

## Global Constraints

- Do not change the backend or call Mini App/VK/admin referral endpoints.
- Do not invent a referral link, promo code, reward amount or percentage, earnings, or statistics.
- Reuse the profile session snapshot; tab switching must make no API request.
- Keep the existing `Общее` profile content and current token styling intact.

## Task 1: Implement truthful referral tab behaviour

**Files:** `web/platform/src/i18n/ru.ts`, `web/platform/src/features/account/ProfileWorkspace/ProfileWorkspace.tsx`, `web/platform/src/features/account/ProfileWorkspace/ProfileWorkspace.test.tsx`, `web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram.tsx`, `web/platform/src/features/account/ProfileReferralFaq/ProfileReferralFaq.tsx`, and `web/platform/src/features/account/ProfileReferralFaq/ProfileReferralFaq.test.tsx`.

- [ ] Add a failing profile test that selects `Реферальная программа` and expects its launch state instead of the general billing panel.
- [ ] Add a failing FAQ test that opens one answer and updates `aria-expanded`.
- [ ] Run only these tests and confirm they fail because the referral tab and FAQ do not exist.
- [ ] Add the approved Russian copy for a launch-state programme card, three generic steps, unavailable statistics, and FAQ answers that do not promise unspecified referral terms.
- [ ] Implement the local profile tab state and complete ARIA tab-panel linkage.
- [ ] Implement the referral screen with no link, code, numeric reward, numeric statistics, copy, or share action.
- [ ] Run focused profile and FAQ tests and confirm they pass.

## Task 2: Apply responsive referral styling

**Files:** `web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram.module.css`, `web/platform/src/features/account/ProfileReferralFaq/ProfileReferralFaq.module.css`, and `web/platform/src/features/account/ProfileReferralProgram/ProfileReferralProgram.styles.test.ts`.

- [ ] Use the existing dark surface, border, spacing, and accent tokens for the programme card, explainer cards, empty statistics cards, and FAQ rows.
- [ ] Keep the explainer and statistics grid multi-column on desktop and single-column on small screens.
- [ ] Add stylesheet assertions for the responsive layouts, then run the focused tests.

## Task 3: Verify and deploy

**Files:** only the reviewed files from Tasks 1-2 plus these design and plan documents.

- [ ] Run frontend tests, typecheck, lint, production build, and `git diff --check`.
- [ ] Commit only the referral UI and its specification, push to `dev-deploy`, and wait for CI, Docker Images, and Deploy DEV workflows for the exact SHA to succeed.
