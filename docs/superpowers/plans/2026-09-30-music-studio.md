# Music studio implementation plan

**Goal:** Make music creation start with an idea and task cards, matching the approved reference in the platform's own style.

**Architecture:** Keep the existing MusicWorkspaceController, catalog, draft and prepare/activate contracts. Reshape MusicWorkspace and add a small task-card component; reveal existing forms on demand. No provider, model capability or billing changes.

**Constraints:** Reuse InputSurface, Button, CreditAmount and current theme tokens. Russian and English copy. Only catalog operations and prices. Preserve drafts, source selection, validation, safe artifact playback and confirmation. No paid calls or deployment.

- [x] Add focused landing/disclosure tests in MusicWorkspace.test.tsx; verify failure before implementation.
- [x] Implement quick creation, genre chips, compact model selection, task cards and a collapsible track library. Reuse current parameter forms in the selected scenario.
- [x] Style desktop and narrow layouts in MusicWorkspace.module.css; add translations in src/i18n/music.ts.
- [x] Adapt existing tests to explicit settings/library navigation; run music and i18n tests, lint, typecheck and build.
- [x] Inspect the local music page and mobile layout; verify opening tools and settings does not launch a job.
- [x] Update the UI catalog and index with the new entry flow and verification results.

Primary files are under web/platform/src/features/music/MusicWorkspace. Run checks from web/platform: `npx vitest run src/features/music src/i18n`, `npm run lint`, `npm run typecheck`, `npm run build`.

Verification: 104 music/i18n tests passed; lint and production build (including TypeScript) passed. Build needed network access for the existing Geist font. Visually checked 1440 px desktop and 390 px mobile: four/one card columns, no horizontal overflow. Settings, catalog popover, Escape dismissal and track-library disclosure were checked in the browser; fresh reload reported no console errors. No paid generation was run; pending models remain unavailable. Backend, account/session, billing and provider contracts are unchanged.
