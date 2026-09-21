# Shared FAQ component

Approved scope: unify the existing FAQ implementation so its appearance and
disclosure behavior can be maintained centrally. Preserve the home-page card
from the supplied screenshot; reuse it on the referral page. Profile tab controls
are outside this change.

## Design

Promote the existing public FAQ to `src/components/ui/FAQ`. It accepts readonly
`items: { question: string; answer: string }[]` and optional `name: string` for a
native exclusive disclosure group. Each row uses `details`/`summary` and the
existing FAQ arrow asset. Copy stays in the caller's locale dictionary. Card,
hover, focus, arrow and expansion styles live together; reduced motion disables
the transitions. No new dependencies or client state are needed.

Home retains independent open answers; the referral page passes a unique group
name to preserve one open answer at a time. Its feature wrapper retains only the
localized section heading. Remove the duplicate local markup/styles and the old
public-only implementation, updating its existing test consumer.

## Execution

- [x] Move FAQ to shared UI and apply the existing home card styles.
- [x] Replace home/referral FAQ markup with the shared component.
- [x] Adapt existing tests to the shared owner and native disclosure semantics.
- [x] Check home/referral opening, closing, keyboard focus, exclusivity and
  narrow widths in a browser; run affected unit tests, types and lint.
- [x] Update UI index/catalog and record verification here.

No commit, deployment, account/API/provider/billing or locale-routing changes.

## Verification

- 88 affected tests across 14 files passed, including the relocated FAQ style
  contract, profile/native disclosure, public primitives, home and i18n checks.
- Typecheck and ESLint passed.
- Edge on local preview: five home questions and three profile questions;
  Enter/Space toggle, open/close height, independent home answers, exclusive
  profile answers, 390px viewport without overflow and zero reduced-motion
  transition duration all passed. No page errors. Dark/light screenshots checked.
- The initial browser assertion expected literal `0ms`; Chromium serialized
  equivalent `0s`. Corrected the check to compare numeric transition duration;
  the component required no change.
