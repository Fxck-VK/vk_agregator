# Landing subscription cards

Status: complete, local only.

Owner request: replace the third/canonical landing's three editorial plan cards
with the existing workspace subscription cards, preserving their design.

1. Extract the existing personal cards and their catalogue projection into one
   reusable grid; leave modal checkout, consent, promotion and eligibility intact.
2. Mount the same five cards in T01's plans section, with a local dark palette;
   landing actions use the existing protected workspace entry.
3. Verify identical card markup and existing checkout regressions, then inspect
   the landing and modal at360/390/768/1024/1440/1920/2560/3840 CSS px.

No pricing, billing API, account, publication or original/friend landing changes.

Implemented: `SubscriptionPlanCards.tsx` owns the existing card markup and
five-plan presentation projection, using the unchanged modal CSS. The modal
continues to own checkout/eligibility/consent/promotion and the DEV team card.
`third/HomeFinale.tsx` replaces only its old three-card block. Its local palette
uses the shared dark token selector in `globals.css`; a scoped font override
prevents the landing's display face from changing workspace card typography.
Obsolete third-landing plan-card CSS was removed. UI index/catalog/variant docs
record reuse and the protected workspace navigation contract.

Verification:
-63 focused tests across8 files passed: landing/markup parity/navigation,
  existing tariff dialog/purchase, Lite, promotion, checkout and theme tokens.
- TypeScript and scoped ESLint passed; `git diff --check` passed.
- Native Chromium: landing and workspace modal at360x844,390x844,768x1024,
  1024x768,1440x900,1920x1080,2560x1440,3840x2160; zoom100%, DPR1.
  No card-content or page-wide overflow, all controls accessible. Dark landing
  cards retain their palette with a light root theme. Colours, typography and
  shared dimensions match the dark workspace cards (rem-normalized for the
  workspace's pre-existing2K/4K scale). No change to that scale was made.
- Evidence: `.cache/responsive-qa/2026-11-09-landing-subscription-cards/summary.json`
  and screenshots. Phone,1440 and4K captures visually inspected.
- Full release build/security admission/deployment were not run: no release
  requested. No payment, financial mutation or provider call was performed.
