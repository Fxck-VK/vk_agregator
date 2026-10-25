# T22 Event / Project Implementation Plan

Status: completed locally. Local only; no commit, push or deploy.

Use executing-plans inline in the active checkout. The approved 24-template
workflow keeps EN implementation and RU empty. No worktree or alternate port.

Goal: /en/events/creative-sessions, matching saved reference22 Cannes Lions:
overlay hero/breadcrumbs/two actions; three programme cards; dark film/trailer
section; four discussion points; alternating venue/meet-up/workflow rows;
one journal card; five exclusive FAQ; closing callout and centered footer.

Architecture: EventProject data declares concept status, unknown dates/venue
and speakers, registrationUrl=null. EventLanding owns layout and dialogs;
EventSections renders programme/film/discussion/rows/journal. Matching shared
LandingHeader/Footer/Dialog/Arrow/ToolIcon, FAQ, HomeSearch and MediaVideo are
reused. No global defaults changed. Five local source images and one18.517sMP4
have provenance/actual formats/SHA. Reference film/map are labelled Magnific
references; Epifera does not claim festival partnership or real programme.

Files: new event-project-en/reference-en, features/event-project, public route
page/publication/test, asset folders, research media manifest. Modify exact
LocalizedPublicShell rule/test, aiToolsEn/homeEn incoming links and UI docs/state.

## Task1 — admission / contracts
- [x] Fresh npm audit before long tests: 0 vulnerabilities.
- [x] Write failing tests: draft/noindex/RU, own shell, actual concept data,
  anchors/sections, explicit trailer + cleanup, FAQ, source/details dialogs,
  native inert/focus, real support link, no forms/writes/fake registration.
- [x] Package five actual-format cached images and one MP4, record SHA/bytes.

## Task2 — implementation
- [x] Implement typed EventProject concept and original Epifera copy.
- [x] Match reference desktop/mobile layout, spacing, warm/maroon/pink palette.
- [x] Connect programme anchors, local trailer modal, project/source details,
  journal/internal tools/contact links; no PII forms or automatic sending.
- [x] Add exact own-shell and publication declarations, incoming menu/footer.

## Task3 — verification / review
- [x] Focused and affected public/shared tests, types, full lint, asset validation,
  packaging/publication checks and production build.
- [x] HTTP EN/RU200, noindex, sitemap exclusion, incoming links, six media
  MIME/bytes/SHA and unchanged CSP. No iframe, third-party scripts or live API.
- [x] Native390/820/1280/1600px; programme anchors, all FAQ, menu/search,
  trailer play/pause/close, native focus/inert and no horizontal overflow.
- [x] Inline scoped review; update UI catalog/index, docs index, current state,
  final git status. Retain page and screenshot; reset viewport.

Final expected status: T01–T23 =23/24; T24 Author remains.

Verification: 374 affected tests in41files passed, plus final40 focused tests including hidden-document pause and portal unmount. Types/full lint, audit0, assets/packaging/publication and build passed. HTTP EN/RU200/noindex/sitemap/three incoming pages, unchanged CSP and sixmedia MIME/bytes/SHA passed. Native390/820/1280/1600px without page overflow; anchors/menu/search, fiveFAQ, source/details/film focus, mobile dialogs and native trailer playback to18.517send verified. Viewport reset1280x720. Inline scoped review completed; no backend/account/Jobs/provider/billing or publication changes.
