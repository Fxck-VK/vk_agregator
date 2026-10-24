# Automatic public sitemap

**Goal:** A published, indexable public page enters the sitemap from its own
publication declaration, without a second hand-maintained URL list.

**Architecture:** Each implemented public route owns a server-only
`publication.ts`, exporting `publications`. Discovery generates static imports
for those modules; the standalone server never scans source files or fetches its
own pages. Page metadata and sitemap consume the same publication records.
Dynamic templates can export records projected from the existing content
repository. Content without an implemented route is never discovered.

**Constraints:** Use the existing checkout and localhost:7159. Preserve the
current home/indexing, image-demo/noindex, legal/noindex and private/auth
boundaries. No new dependencies, content release, account data, provider calls,
commit, push or deployment. Only configured WEB_ORIGIN may form absolute URLs.

**Alternatives:** Keeping a central URL list retains the duplicate update;
crawling rendered pages adds network/rendering failures and misses unlinked
pages. Co-located declarations give one publication decision per real route.

- [x] Add publication-policy tests for publication/unpublication, noindex,
  scheduled dates, locale paths, canonical-only URLs and unsafe/mismatched routes.
- [x] Add discovery tests using temporary public route trees: new route,
  dynamic template, missing declaration and unchanged generated output.
- [x] Implement the publication contract, metadata/sitemap projection and
  deterministic generated imports. Require declarations for every public page;
  legal routes explicitly export an empty sitemap collection.
- [x] Migrate home and image-demo metadata to their own declarations. Preserve
  the demonstration's noindex policy and leave editorial sample records alone.
- [x] Generate before dev/build/typecheck/test/lint. Watch public route additions
  in the existing preview launcher; close the watcher with that child process.
- [x] Update the current route/locale contract and documentation index with the
  authoring rule and limits; check packaging and clean-checkout generation.
- [x] Verify focused Vitest and Node tests, typecheck, lint, packaging, docs and
  local HTTP sitemap/metadata. Exercise adding and removing a temporary public
  fixture through the existing preview server, then remove only that fixture.

Commands run from web/platform unless specified:

```powershell
node --test scripts/public-page-manifest.test.mjs
npm exec -- vitest run src/i18n/seo.test.ts src/content/publication/publication.test.ts src/app/surface-boundaries.test.ts 'src/app/(public)/ai/image-generator/page.test.tsx'
npm run typecheck
npm run lint
npm run test:packaging
```

Final verification must report that this is local implementation. Existing
approved public pages remain the only real sitemap URLs until additional pages
are implemented and published. Search Console submission and search-engine
indexing are external operations and are not claimed by this change.

## Local verification

- 98 Vitest tests passed across the locale/publication/repository and affected
  route boundaries; four Node discovery tests passed.
- Typecheck, lint, production build, packaging and documentation validation
  passed. The production build retained a dynamic `/sitemap.xml` route.
- Real HTTP test on localhost:7159: added a temporary implemented page and
  publication declaration without regenerating the manifest manually. Sitemap
  changed from two to four URLs; the English page had index/follow and the
  correct canonical. Changing the declaration to draft/noindex removed both
  URLs and changed page metadata to noindex/nofollow.
- Removed both temporary files. Final HTTP sitemap contains only the two
  approved home versions; robots points to it, English home remains indexable,
  and the image demonstration remains noindex. The preview server stays on 7159.
- No commit, push, deploy, external indexing request or account/provider change.

Subsequent authorization (2026-10-24): user requested pushing all current changes
to dev-deploy together. Release evidence is kept in ignored
.cache/release-2026-10-24-all/status.json; do not infer deployment from this plan.
