# Workspace display scale implementation plan

**Goal:** Match the owner's first screenshot proportions on QHD/UHD instead of leaving a small fixed-size interface in a large canvas.

**Approved visual reference:** Owner's first screenshot in the 2026-10-30 request. Keep the existing layout, colours, shapes, hover and business behaviour; enlarge the complete workspace consistently.

**Architecture (corrected scope):** Activate the mounted AppShell's rem scale only inside `@media (min-width: 2560px)`, including its body portals. All widths below2560 retain the original root size, particularly1440 and1920. The QHD/UHD-only formula is1.25vw, capped at300% of the browser's default:32px at2560,48px at3840 with the default16px browser font. The1280px canvas is a proportion reference, never the activation breakpoint. Public pages and authentication keep their existing size. Viewport height and native document coordinates remain untransformed; no CSS zoom/transform or browser zoom.

**Files:** AppShell.module.css owns the workspace-only scale. PopoverPanel.tsx converts its design-pixel width/gap/margin to the current root-font scale while preserving viewport boundary calculations. Existing CSS rem tokens already cover sidebar, header, content, dialogs and icons.

## Constraints

- Work only in the active checkout, localhost7159; preserve unrelated dirty work.
- No backend, billing, identity, provider, content or payment changes.
- No commit/push/deploy.
- Validate all8 required widths and QHD2560×1440/UHD3840×2160 at100% browser zoom.

## Execution

- [x] Capture the existing wide-screen failure: sidebar/content proportions versus the owner's reference.
- [x] Add and run failing PopoverPanel regressions for scaled widths, viewport clamping and resize updates.
- [x] Add the scoped CSS rem scale and scaled anchored-panel geometry.
- [x] Check home/chat/files/catalog/image/music/speech/inspiration/profile and shared panels at all8 widths; verify public/auth isolation. Baseline visualViewport.scale=1 and CSS zoom=1; user zoom modes were not separately exercised. Wide full-page screenshots have fixed-layer/tile artifacts; see the evidence report rather than marking those visual cases unconditionally passed.
- [x] Run focused component tests, types and lint; inspect the diff. 20 files / 195 tests passed; typecheck, focused ESLint and diff whitespace checks passed.
- [x] Save routes, viewport/root scale, geometry and screenshots under ignored responsive-qa evidence; update the UI reuse contract.

**Review:** The rem scale applies to root only at viewport widths≥2560 while the existing body-direct AppShell is present, so body portals inherit it and public pages reset automatically. Widths below2560 retain their original styles. Anchored menus must use scaled design widths but unscaled measured DOM rectangles. Long dialogs may use their existing internal scroll region. Browser verification is the acceptance check for CSS sizing, not a source-string assertion. Original evidence applied scale too broadly and is superseded by `.cache/responsive-qa/2026-10-30-workspace-scale/qhd-only-correction/`.
