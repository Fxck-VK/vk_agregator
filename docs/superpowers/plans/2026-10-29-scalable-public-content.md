# Scalable public content implementation plan

**Goal:** Все 24 действующих шаблона обслуживают данные каталога без копирования вёрстки.

**Architecture:** Явные именованные источники → проверенный русскоязычный каталог → существующие компоненты. Сервер разрешает публикацию и адрес, клиент получает только источники текущей записи. Managed-адаптеры сохраняют юридический и платёжный контракты.

**Tech Stack:** Existing TypeScript, React, Next App Router, Vitest; no new dependency or server.

## Global constraints

- Один текущий checkout и localhost:7159; без изменения стилей.
- Только ru. Без подмены backend billing, provider, auth или юридических документов.
- Согласование уже получено; без commit/push/deploy.

## Tasks

- [x] Реестр T01–T24 и источников, контракты записей, отрицательные тесты валидации.
- [x] Контекст именованных источников и явные привязки существующих компонентов; устранить зависимость производных данных от примера.
- [x] Общий серверный рендерер, каталог существующих адресов, динамические страницы и безопасный fallback.
- [x] Общая публикация, SEO и sitemap, проверки коллизий/ссылок/медиа.
- [x] Команды создания/проверки записи и русская инструкция с 24 примерами.
- [x] Проверить все адаптеры, две разные записи каждого типа, существующие регрессии, типы/линтер/сборку/HTTP; записать результаты.

## Verification

- Existing public regression run: 348 passed. Expanded run: 401 passed with three 5s timeouts during concurrent build; only those three files rerun sequentially: 41 passed. No failed check treated as success.
- Final template/example/category checks: 91 passed; final validation including parent-course identity: 48 passed; final renderer/legal checks: 29 passed.
- Publication/media Node contract: 8 passed. All 24 full Russian example JSON files pass schema and media checks.
- Full lint, production build/type checks, language/publication guards, asset validation and packaging passed. Final build compile 5.7s, TypeScript 9.2s.
- Native browser: new article/category correct title, canonical, noindex, loaded images, no horizontal overflow. Temporary entries removed after verification. HTTP confirms actual lesson/article/category/sitemap 200, unknown 404, English 308 to Russian.
- Independent review found ID coercion, object/null/array admission, and directory-as-media bugs. Reproduced red, fixed and independently rechecked. Own category-query stale-state regression reproduced/fixed; complete-example tests cover relative posters and optional video fields.
- No CSS, backend, billing, auth, provider or approved legal text edits in this task. Existing unrelated work preserved. No commit/push/deploy.
