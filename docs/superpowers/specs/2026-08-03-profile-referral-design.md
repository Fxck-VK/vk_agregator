# NeiroHub Profile Referral Tab

## Goal

Add a selectable `Реферальная программа` tab to the profile page using the visual hierarchy approved by the user: a programme card, a three-step explainer, a statistics area, and a compact FAQ.

## Scope

- Keep the existing profile identity card and the `Общее` tab intact.
- Make the referral tab switch locally without a navigation or a server request.
- Render a stable desktop and mobile layout that follows the existing dark NeiroHub token system.
- Show an explicit launch state for the personal link, promo code, reward totals, and analytics.
- Keep the FAQ as a local disclosure control for the generic launch-state answers.

## Truthful data boundary

- The neutral web BFF currently exposes account identity and balance only. It does not expose a referral code, referral URL, rewards, programme terms, or aggregate counters.
- Existing referral data belongs to the VK Mini App flow and must not be read or proxied by the independent web frontend.
- The UI must not invent a URL, a promo code, reward percentage, reward amount, zero balance, or zero statistics.
- Copy/share buttons stay absent until a web-safe account-scoped referral contract exists.

## Components

- `ProfileWorkspace` owns the active local profile tab and keeps both panels within the existing session snapshot.
- `ProfileReferralProgram` renders the referral launch card, three-step explainer, unavailable analytics cards, and FAQ container.
- `ProfileReferralFaq` owns its local expanded question state and exposes accessible disclosure buttons.

## Interaction and accessibility

- `Общее` and `Реферальная программа` are an accessible tab pair with associated panels.
- A tab click changes only local UI state; it never triggers a fetch.
- FAQ controls use native buttons with `aria-expanded` and `aria-controls`.
- All unavailable data is labelled in text rather than hidden behind a disabled action.
- The explainer stacks to one column on narrow screens.

## Verification

- Tests cover switching from `Общее` to the referral view, absence of fake referral values, and FAQ disclosure behaviour.
- Styling tests cover the responsive explainer and statistics layout.
- Run focused tests, the complete frontend suite, typecheck, lint, and production build before the DEV deployment.
