# Direct subscription checkout

Owner request: remove the forced intermediate subscription-conditions dialog.
Work remains local; no payment, commit, push or deployment authorized.

Implementation scope:

- Extract the existing immutable-request/price-check/navigation controller so
  tariff cards and the standalone checkout share one implementation.
- Keep all tariff cards, prices and server availability/trial enforcement.
- Start checkout directly from the chosen card. Guests go to login; local
  preview stays read-only. Display errors/status inside the tariff window.
- Keep renewal explicitly optional and unchecked in the tariff footer. Provide
  document links and voluntary details with Lite conversion and current retry
  rules; never infer consent from a card click.
- Preserve idempotency, duplicate-click protection, late-response guards and
  server verification of final payment status. No provider/backend changes.

Verification:

- Reproduce missing direct checkout with failing integration tests first.
- Test direct navigation, consent policy, unavailable/trial gating, retries,
  amount/star conflicts, double click, owner switch and terminal outcomes.
- Re-run standalone checkout, Lite offer and tariff/promo regressions.
- Check production compilation, types, scoped lint and docs/diff checks.
- Verify the affected UI at360/390/768/1024/1440/1920/2560/3840 CSS px,
  QHD2560x1440 and UHD3840x2160; distinguish live preview from synthetic fixtures.
- Independent review of shared checkout rules and final disclosures.

Evidence: ignored `.cache/subscription-direct-tests.log` and
`.cache/responsive-qa/2026-10-31-direct-subscription/`.

## Verified locally

- Four direct-purchase regressions reproduced the missing flow before edits.
- 80 affected tests passed, including direct and standalone checkout, explicit
  retry-policy opt-in, immutable retries, amount/star conflicts, duplicate clicks,
  owner-switch late response, Lite eligibility and terminal payment handoff.
- Scoped ESLint, production build/types, RU publication gates and docs/diff
  validation passed. No new dependency or backend change.
- Live read-only preview passed all8 widths: no nested conditions dialog,
  unavailable payment stays blocked, list/drag/track/keyboard footer access and
  inset scrollbar remain usable.
-16 static SSR/CSS browser cases passed (available + opted-in expanded details
  x8 widths); these confirm layout/keyboard disclosure, not real checkout or
  browser hydration. Evidence records exact methods and dimensions.
- Independent review confirmed unavailable feedback and full voluntary renewal
  disclosure; final review has no remaining findings. No real payment was made.
- No commit, push or deployment. Standalone checkout forms on other existing
  surfaces keep their layout and share the same purchase controller.
