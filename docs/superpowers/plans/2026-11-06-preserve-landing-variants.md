# Preserve landing variants during DEV integration

> Execute inline using the executing-plans skill. Owner approved this scope in chat. Do not commit, push, deploy, create worktrees or start another site.

**Goal:** Integrate origin/dev-deploy into the single active checkout while retaining local work, all 24 SEO templates and independently viewable original/friend homepages.

**Architecture:** Keep the existing T01 renderer and original public-home components as the SEO homepage. Preserve the incoming editorial homepage in a separate feature directory with its own stylesheet and frozen Russian copy. Private local/protected-DEV comparison routes use the existing release admission; no preview route enters publication/sitemap. Shared auth, billing and release policies remain authoritative.

- [x] Capture byte-verified local-work and platform/SEO archives, current HEAD, incoming SHA, and resolved data for all 24 templates under ignored .runtime/merge-2026-11-06.
- [x] Stash existing tracked/untracked work, fast-forward to the reviewed origin/dev-deploy SHA, reapply the stash without dropping it, and reconcile overlapping files. Preserve both local and incoming security fixes.
- [x] Keep original public-home sources/data separate from the incoming landing variant. Add private noindex preview routes at /ru/landing-variants/original and /ru/landing-variants/friend plus a small comparison index. Do not choose/create a third landing.
- [x] Verify original SEO sources/data and all preexisting local changes against the snapshot. Test preview access denial, both variants, catalogue, Russian copy, affected merge contracts, lint and types.
- [x] Reuse localhost:7159 for browser checks at 360,390,768,1024,1440,1920,2560x1440,3840x2160. Save evidence in .cache/responsive-qa/2026-11-06-landing-variants; distinguish browser measurements from visual inspection.
- [x] Update local-development documentation/UI index with preview paths and retained source locations. Report local integration SHA and remaining limitations; no publication.

**Checks:** npm --prefix web/platform run content:check; npm --prefix web/platform run public-copy:check; npm --prefix web/platform run typecheck; focused Vitest for public-home, landing variants, content templates, release policy, assets and affected layouts; eslint changed platform sources; node --test scripts/ci/release-contract.test.mjs. Security/infra admission precedes any full build/test expansion.
