# Provider login reference correction

> **Execution:** Approved by the owner after comparing the reference and the implementation. Continue in the existing checkout only. No commit, push or deployment.

**Goal:** Personalize an existing provider button, with account switching inside its menu; remove added email and demonstration controls from the form.

**Architecture:** CanonicalLogin still owns the common form and authoritative server actions. RememberedProviderButton owns only the provider row, arrow and shared PopoverPanel. AuthEntry permits synthetic provider hints only behind the existing server-owned local-preview gate.

## Approved scope

- No separate remembered-email card, welcome-back screen or standalone change-account action.
- The provider row contains a neutral account icon, remembered label, secondary provider name and provider icon. No email is fabricated from a display name or fetched from the provider during rendering.
- The main button uses the existing OAuth action. The independent arrow opens the account menu; its single action calls the existing protected forget endpoint. No nested buttons.
- Email uses the ordinary typed email/password path and any server-required code. Existing backend remembered-email receipts remain unchanged, but this form does not add a separate path for them.
- Demo links are `/ru/login?demo=provider` (Yandex) and `/ru/login?demo=vk`; no selector is rendered. Former `?demo=email` is ordinary login. Production cannot enable the demonstration through query parameters.
- Preserve the selected login shell, showcase, brand, registration/recovery/session behavior and Russian-only policy.

## Implementation and checks

- [x] Reproduce old extra email/action/selector behavior with failing updated tests.
- [x] Implement the provider row/menu using shared Button, AssetIcon, PopoverPanel and selectable control styles.
- [x] Remove client remembered-email selection and the LoginPreview wrapper, stylesheet and obsolete tests.
- [x] Test provider OAuth, normal email/code path, failure/retry, disabled providers, Escape/outside close, safe demo seeds and non-preview query isolation.
- [x] Scoped ESLint and full TypeScript pass; 277 auth/sign-in/i18n tests across 32 files pass.
- [x] Production build passes.
- [x] Check default/Yandex/VK and open account menus at 360, 390, 768, 1024, 1440, 1920, 2560 x 1440 and 3840 x 2160 CSS px; inspect screenshots and interaction evidence.
- [x] Update current UI/local-development routing and report local-only status.

Final browser run: 40 actual-localhost cases passed at zoom 1/DPR 1, with no horizontal overflow, form scroll or page errors. Escape/Tab/outside close, focus restoration, successful local forget retaining typed email, ordinary password/code preview and retired email-demo fallback passed with zero auth requests. Production packaging, docs and diff checks passed. Original dev next-env imports restored after the build. No backend mutation or release.

Independent scoped review found no material issues. Browser evidence belongs in `.cache/responsive-qa/2026-10-31-provider-login-reference/`; execution logs use `.cache/login-provider-reference-*.log`.
