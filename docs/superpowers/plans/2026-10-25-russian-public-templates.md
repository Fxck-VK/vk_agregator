# Russian Public Templates and Round-trip Translation Plan

Status: completed locally. User-authorized implementation of all24 RU template types.
Local only, same active checkout/localhost7159; no commit, push or deploy.

Goal: render the same public components at /ru with complete Russian interface,
editorial copy, metadata and accessibility text, preserving the existing /en.
Approved RU legal documents are the source of truth and remain unchanged.

Architecture: explicit locale-scoped public-data translation with stable source
units and RU catalog, integrated through the existing LocaleProvider and server
getRequestLocale. Never a global mutable locale or DOM text replacement. Shared
components keep structural IDs/routes/model IDs/asset paths/prices/flags intact.
Stable source IDs and recorded back-translations make missing/stale translations
visible before future scaling. Technical brands/model names are allowlisted.

Translation workflow: source EN -> RU -> separate reverse EN pass -> compare
facts/negation/conditions/numbers/intent -> correct RU -> natural-language and
visual review. Save actual reverse texts and review results; do not describe
automated coverage checks as independent human linguistic review.

## Task1 — inventory / contract
- [x] Extract runtime public data across all templates and reference alt/tag data;
  inventory hardcoded user-facing strings and server-created labels separately.
- [x] Fresh dependency audit before long tests.
- [x] Failing tests: explicit locale, EN identity, concurrent RU/EN isolation,
  source completeness, stable IDs/links/prices, back-translation coverage,
  numeric/placeholders preservation and Russian page rendering/noindex.
- [x] Add public source extraction/update workflow and task-scoped docs.

## Task2 — translations and integration
- [x] Translate shared landing/editorial/selector/media/search/pagination UI.
- [x] T01-T07 home/tools/generator/model family/version/task/audience.
- [x] T08-T12 comparison/article/course+lessons/help+articles/workflow.
- [x] T13-T17 collection/public result/category/material detail/integration.
- [x] T18-T22 pricing/business/case/about/event; T24 author/archive.
- [x] T23 legal UI and existing approved RU legal content verified unchanged.
- [x] Add actual reverse EN texts and semantic review for translated units.
- [x] Enable RU rendering and metadata, locale-aware server factories and links.
  Update former RU-empty tests; retain draft/noindex/publication gates.

## Task3 — verification
- [x] Unit/source/i18n and affected public/shared regressions,
  typecheck/full lint, assets/packaging/publication and final build.
- [x] HTTP all24 RU templates, course/docs children and metadata; no empty RU,
  language-correct links, EN preserved, no unintended indexed publication.
- [x] Native desktop/mobile representative text-dense pages and all24 layouts;
  search/menus/dialogs/pagination/focus, longer Russian strings and no overflow.
- [x] Review every translation batch and unresolved items; update UI docs/index,
  localization contract, current state, final git status; retain RU output tab.

No backend/provider/model admission/account/billing/Jobs/private artifact or
deployment changes. No external translation-service or paid AI calls.

Verification: 460 tests/56files, then113 focused tests/16files after final
gallery/count fixes; typecheck, full lint, fresh dependency audit (0 advisories),
asset validation,4publication tests, packaging and production build passed.
HTTP50 main RU/EN routes passed; all24 mobile/desktop template layouts plus
820px text-dense layouts had no document overflow. Native Russian archive search
returned matching RU links. Build cache cleanup recovered capacity after C: filled;
no source/private data was removed. All42 RU/EN child routes passed. Child-route verification recorded under
.cache/ru-templates/children-http-results.json (local evidence, not committed).

Final follow-up: 10 safety/grammar/source-contract tests passed after the
last display fix. Final npm.cmd production build and packaging passed after
clearing the regenerable dev cache; source/private files preserved.

Final lint exited0 with .cache excluded as regenerable test output; no tracked
source is in that directory. Next lint-contract2tests also passed. The same
dev:preview server restarted on7159; post-restart HTTP200 and native RU page
confirmed. Generated cache removal did not touch source/user data.
