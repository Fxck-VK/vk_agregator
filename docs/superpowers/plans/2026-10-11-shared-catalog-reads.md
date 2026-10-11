# Shared model catalog reads

**Approved design:** The user approved sharing identical catalog reads between components, with the visible behavior unchanged. This change is limited to model catalog reads; it does not apply a global fetch deduplicator to authenticated resources or writes.

**Goal:** Reuse a successful server catalog load in all browser catalog adapters, including speech and file actions. When no fresh seed exists, concurrent consumers share one validated response.

**Architecture:** Pass one complete, validated public catalog snapshot and its original update time from the existing bounded server loader. The existing GenerationCatalogProvider projects selector data for rendering and installs that snapshot in the browser catalog cache during a client layout effect, before descendant passive effects load their catalogs. This is a constant-time memory assignment, with no network or parsing in the layout effect. The shared read resource honors the original seed timestamp. Older seeds/responses must not overwrite a newer snapshot. Keep the existing 60-second TTL, failure/retry behavior, server session check and 200-ms initial wait.

**Constraints:** Only non-personal catalog metadata is shared in memory. No browser persistence, new dependencies, paid requests, automatic write retries, commits or deployment. Preserve the existing referral cache work in this checkout.

## Implementation and verification

- [x] Add regressions for reuse of a fresh server snapshot by raw and projected consumers, one concurrent request without a seed, expiry without extending the seed age, retry after failure, and stale responses/seeds.
- [x] Run the focused tests and observe the duplication failures before implementation.
- [x] Extend the existing catalog cache, bounded server seed and GenerationCatalogProvider; retain the provider's projected-seed API for existing consumers. Supply the original update time to createReadResource.
- [x] Update the active preloading/UI documentation and documentation index. Run catalog/resource/consumer regressions, lint, typecheck, build, packaging and whitespace checks. Inspect the scoped diff and final status.

## Verified result

- Before implementation, a real provider + child adapter test reproduced one redundant browser catalog request despite a fresh server seed. Seed expiry and raw-snapshot rendering regressions failed as expected. Original catalog/provider tests passed.
- Related consumers: 38 files, 277 tests passed. Full frontend suite: 249 files passed, 1 file skipped; 1735 tests passed, 4 skipped. No skips were added in this change.
- Final targeted verification: 4 files, 24 tests passed, including actual renderToString/hydrateRoot in StrictMode, original-age expiration on focus, concurrent retry after 503, and late response/seed ordering.
- Fresh complete server seed: zero browser catalog requests. Without a seed, selectors and raw/image/video adapters share one browser request and one response-body parse. The existing 200-ms server wait remains; a slow server seed can still require the browser fallback rather than blocking the page.
- Typecheck, lint, production build, packaging, asset and Next lint-contract checks passed. Documentation validation and diff whitespace checks passed. Vitest used a temporary cache under the ignored .next directory because the sandbox's default temporary directory rejected renames. Turbopack needed filesystem permission to create its existing sharp junction; the unchanged build command then passed.
- Only non-personal catalog metadata is shared; session checks, authenticated fetches, job/pricing authority and write behavior remain. No storage persistence, commit, push or deployment. Previous referral changes are preserved.
