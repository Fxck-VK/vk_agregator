# Public Homepage Study24 Reference Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Построить индексируемую главную NeiroHub на `/`, повторяющую структуру и UX главной Study24, не связывая публичный рендер с приватной сессией `/app`.

**Architecture:** `app/page.tsx` остаётся Server Component и собирает статический `PublicHome`; интерактивность выделяется в небольшие Client Components. Публичный shell не импортирует `WorkspaceFrame`, session providers и историю чатов. Контент первого среза типизирован и хранится локально, а API-интеграции добавляются только после стабилизации макета.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, CSS Modules, `next/image`, Vitest, Testing Library, Playwright.

## Обязательные ограничения

- Не изменять backend, базу, биллинг, сессии и контракты `/web/v1`.
- Не менять поведение существующих `/app/**`, `/login` и Cloudflare Basic Auth DEV-контура.
- Не переносить код, логотипы, фотографии, видео и длинные тексты Study24.
- Не публиковать неподтверждённые цены, рейтинги и счётчики.
- Не включать всё дерево страницы в один Client Component.
- Не отправлять сетевые запросы для статичных блоков при каждом открытии `/`.
- Каждый компонент хранить в собственной папке с `.tsx`, `.module.css` и тестом.

### Task 1: Зафиксировать публичные контракты и контент

**Files:**
- Create: `web/platform/src/features/landing/landing-contracts.ts`
- Create: `web/platform/src/features/landing/landing-content.ts`
- Create: `web/platform/src/features/landing/landing-content.test.ts`
- Modify: `web/platform/src/i18n/ru.ts`

- [ ] Написать падающий тест, проверяющий уникальные `id`, непустые alt-тексты, существующие внутренние `href` и отсутствие неподтверждённых рейтингов/счётчиков.
- [ ] Описать типы `LandingTool`, `LandingNewsItem`, `LandingModel`, `LandingCapability`, `LandingFaqItem`, `LandingFooterGroup`.
- [ ] Создать локальный набор данных для всех секций с контентом NeiroHub.
- [ ] Вынести пользовательские строки в `ru.landing`, не смешивая их с `ru.workspace`.
- [ ] Запустить `npm test -- landing-content.test.ts` из `web/platform` и добиться зелёного результата.

### Task 2: Подготовить SEO-основу публичного маршрута

**Files:**
- Modify: `web/platform/src/app/layout.tsx`
- Modify: `web/platform/src/app/layout.test.tsx`
- Modify: `web/platform/src/app/page.tsx`
- Modify: `web/platform/src/app/page.test.tsx`
- Create: `web/platform/src/app/robots.ts`
- Create: `web/platform/src/app/robots.test.ts`
- Create: `web/platform/src/app/sitemap.ts`
- Create: `web/platform/src/app/sitemap.test.ts`
- Create: `web/platform/src/features/landing/seo/landing-json-ld.ts`
- Create: `web/platform/src/features/landing/seo/landing-json-ld.test.ts`

- [ ] Добавить тесты на `metadataBase`, canonical, title, description, Open Graph, Twitter card и index/follow главной.
- [ ] Добавить тест, подтверждающий, что `/app` и `/login` по-прежнему `noindex`.
- [ ] Реализовать `robots.ts`, разрешив публичные страницы и запретив `/app`, `/login`, `/web`.
- [ ] Реализовать `sitemap.ts` только с реально существующими публичными URL.
- [ ] Создать безопасный сериализатор JSON-LD и схемы `Organization`, `WebSite`, `FAQPage` из единого FAQ-набора.
- [ ] Убрать `connection()` из публичной страницы, если CSP-архитектура допускает статический рендер без него; подтвердить production build. Если nonce требует динамики всего layout, вынести решение в отдельный security review, а не маскировать проблему.
- [ ] Запустить focused SEO-тесты.

### Task 3: Создать публичный shell

**Files:**
- Create: `web/platform/src/features/landing/PublicShell/PublicShell.tsx`
- Create: `web/platform/src/features/landing/PublicShell/PublicShell.module.css`
- Create: `web/platform/src/features/landing/PublicShell/PublicShell.test.tsx`
- Create: `web/platform/src/features/landing/PublicSidebar/PublicSidebar.tsx`
- Create: `web/platform/src/features/landing/PublicSidebar/PublicSidebar.module.css`
- Create: `web/platform/src/features/landing/PublicSidebar/PublicSidebar.test.tsx`
- Create: `web/platform/src/features/landing/PublicHeader/PublicHeader.tsx`
- Create: `web/platform/src/features/landing/PublicHeader/PublicHeader.module.css`
- Create: `web/platform/src/features/landing/PublicHeader/PublicHeader.test.tsx`

- [ ] Сначала тестами зафиксировать desktop layout, skip-link, navigation labels, login CTA и отсутствие приватных account/chat providers.
- [ ] Реализовать фиксированную desktop-панель и sticky header поверх отдельной прокручиваемой области контента.
- [ ] Реализовать mobile drawer: открытие, закрытие, Escape, backdrop, focus trap, возврат фокуса и закрытие после выбора ссылки.
- [ ] Переиспользовать только дизайн-токены и безопасные UI-примитивы; не импортировать `WorkspaceFrame` и `SidebarConversations`.
- [ ] Проверить тёмную, светлую и системную темы.
- [ ] Запустить тесты трёх компонентов.

### Task 4: Собрать серверный каркас PublicHome

**Files:**
- Create: `web/platform/src/features/landing/PublicHome/PublicHome.tsx`
- Create: `web/platform/src/features/landing/PublicHome/PublicHome.module.css`
- Create: `web/platform/src/features/landing/PublicHome/PublicHome.test.tsx`
- Modify: `web/platform/src/app/page.tsx`
- Modify: `web/platform/src/app/page.module.css`
- Modify: `web/platform/src/app/page.test.tsx`

- [ ] Написать тест на один H1, двенадцать секций в утверждённом порядке и корректные landmark-роли.
- [ ] Подключить `PublicShell` и серверный `PublicHome` на `/`.
- [ ] Удалить старую одиночную hero-карточку и старые стили, которые больше не используются.
- [ ] Вставить JSON-LD рядом с серверной разметкой.
- [ ] Подтвердить, что `renderToStaticMarkup` содержит критический контент без исполнения клиентского JavaScript.

### Task 5: Реализовать model-aware главный ввод

**Files:**
- Create: `web/platform/src/features/landing/HeroComposer/HeroComposer.tsx`
- Create: `web/platform/src/features/landing/HeroComposer/HeroComposer.module.css`
- Create: `web/platform/src/features/landing/HeroComposer/HeroComposer.test.tsx`
- Create: `web/platform/src/features/landing/HeroComposer/guest-draft.ts`
- Create: `web/platform/src/features/landing/HeroComposer/guest-draft.test.ts`

- [ ] Написать тесты на universal state, image state, явное переключение модели, Enter, Shift+Enter, фиксированный textarea и отсутствие автоматической intent-routing.
- [ ] Реализовать selector выбранной модели как локальный state клиентского острова.
- [ ] Для universal state показать textarea, attachment affordance и send.
- [ ] Для image state показать prompt, reference affordance, model, quality и подтверждённую цену, если она присутствует в контенте.
- [ ] При отправке гостем сохранить только текстовый черновик с timestamp и перейти на `/login?next=/app/chats`; не хранить файл, токен или персональные данные.
- [ ] Добавить ограничение длины, disabled/pending состояния и доступное сообщение об ошибке.
- [ ] Связать выбранную модель с подписью в `PublicHeader` без гидратации остальной страницы.
- [ ] Запустить focused tests.

### Task 6: Добавить быстрый ряд инструментов

**Files:**
- Create: `web/platform/src/features/landing/QuickTools/QuickTools.tsx`
- Create: `web/platform/src/features/landing/QuickTools/QuickTools.module.css`
- Create: `web/platform/src/features/landing/QuickTools/QuickTools.test.tsx`

- [ ] Тестами зафиксировать набор карточек, активную модель и keyboard navigation.
- [ ] Реализовать desktop-ряд и mobile horizontal scroll с scroll snap.
- [ ] Клик по поддерживаемой модели переключает `HeroComposer`, а «Все нейросети» ведёт к каталогу через вход.
- [ ] Не выполнять prefetch приватных маршрутов до явного взаимодействия на mobile.
- [ ] Запустить focused test.

### Task 7: Реализовать новостную секцию

**Files:**
- Create: `web/platform/src/features/landing/NewsCarousel/NewsCarousel.tsx`
- Create: `web/platform/src/features/landing/NewsCarousel/NewsCarousel.module.css`
- Create: `web/platform/src/features/landing/NewsCarousel/NewsCarousel.test.tsx`
- Create: `web/platform/public/landing/news/.gitkeep`

- [ ] Написать тест на H2, CTA, предыдущую/следующую новость, доступные labels и стабильную высоту.
- [ ] Реализовать карусель на локальных типизированных данных без сторонней библиотеки.
- [ ] Добавить нейтральный локальный poster с фиксированными width/height; реальный материал добавить только после проверки прав.
- [ ] Отключать transition при `prefers-reduced-motion`.
- [ ] Запустить focused test.

### Task 8: Реализовать каталог популярных моделей

**Files:**
- Create: `web/platform/src/features/landing/PopularModels/PopularModels.tsx`
- Create: `web/platform/src/features/landing/PopularModels/PopularModels.module.css`
- Create: `web/platform/src/features/landing/PopularModels/PopularModels.test.tsx`
- Create: `web/platform/src/features/landing/PublicModelCard/PublicModelCard.tsx`
- Create: `web/platform/src/features/landing/PublicModelCard/PublicModelCard.module.css`
- Create: `web/platform/src/features/landing/PublicModelCard/PublicModelCard.test.tsx`

- [ ] Написать тест: первоначально видны четыре карточки, после «Показать ещё» — десять, сетевого вызова нет.
- [ ] Написать тест, что неподтверждённые price/rating/usage не появляются.
- [ ] Реализовать responsive grid и server-rendered первые четыре карточки.
- [ ] Сделать карточку ссылкой на безопасный login/target flow.
- [ ] Сверить точное маркетинговое число с реальным каталогом; до подтверждения не утверждать «90+» в заголовке.
- [ ] Запустить focused tests.

### Task 9: Реализовать объяснение и возможности

**Files:**
- Create: `web/platform/src/features/landing/HowItWorks/HowItWorks.tsx`
- Create: `web/platform/src/features/landing/HowItWorks/HowItWorks.module.css`
- Create: `web/platform/src/features/landing/HowItWorks/HowItWorks.test.tsx`
- Create: `web/platform/src/features/landing/CapabilitiesCarousel/CapabilitiesCarousel.tsx`
- Create: `web/platform/src/features/landing/CapabilitiesCarousel/CapabilitiesCarousel.module.css`
- Create: `web/platform/src/features/landing/CapabilitiesCarousel/CapabilitiesCarousel.test.tsx`
- Create: `web/platform/public/landing/how-it-works/.gitkeep`
- Create: `web/platform/public/landing/capabilities/.gitkeep`

- [ ] Тестами зафиксировать три шага, poster fallback, capability cards и ссылки.
- [ ] Реализовать video/poster блок без autoplay и без загрузки тяжёлого видео до взаимодействия.
- [ ] Реализовать capability carousel без внешней зависимости, с кнопками и scroll snap.
- [ ] Добавить компактные ссылки дополнительных направлений только для существующих target-маршрутов.
- [ ] Запустить focused tests.

### Task 10: Добавить библиотеку промптов и социальный CTA

**Files:**
- Create: `web/platform/src/features/landing/PromptLibraryCta/PromptLibraryCta.tsx`
- Create: `web/platform/src/features/landing/PromptLibraryCta/PromptLibraryCta.module.css`
- Create: `web/platform/src/features/landing/PromptLibraryCta/PromptLibraryCta.test.tsx`
- Create: `web/platform/src/features/landing/SocialCta/SocialCta.tsx`
- Create: `web/platform/src/features/landing/SocialCta/SocialCta.module.css`
- Create: `web/platform/src/features/landing/SocialCta/SocialCta.test.tsx`

- [ ] Зафиксировать тестами корректный внутренний CTA и отсутствие фиктивных внешних ссылок.
- [ ] Реализовать prompt banner с переходом во «Вдохновение» через вход.
- [ ] Реализовать social block так, чтобы он рендерил только подтверждённые URL из конфигурации.
- [ ] Для внешних ссылок добавить безопасные `rel` и понятные accessible names.
- [ ] Запустить focused tests.

### Task 11: Реализовать FAQ и синхронизировать structured data

**Files:**
- Create: `web/platform/src/features/landing/FaqSection/FaqSection.tsx`
- Create: `web/platform/src/features/landing/FaqSection/FaqSection.module.css`
- Create: `web/platform/src/features/landing/FaqSection/FaqSection.test.tsx`
- Modify: `web/platform/src/features/landing/seo/landing-json-ld.test.ts`

- [ ] Написать тесты на пять вопросов, `aria-expanded`, keyboard activation и связь вопроса с ответом.
- [ ] Реализовать аккордеон с одной открытой панелью и progressive fallback читаемого контента.
- [ ] Добавить контрактный тест, что видимый FAQ и `FAQPage.mainEntity` построены из одного массива.
- [ ] Запустить FAQ и JSON-LD тесты.

### Task 12: Реализовать SEO-подвал

**Files:**
- Create: `web/platform/src/features/landing/PublicFooter/PublicFooter.tsx`
- Create: `web/platform/src/features/landing/PublicFooter/PublicFooter.module.css`
- Create: `web/platform/src/features/landing/PublicFooter/PublicFooter.test.tsx`

- [ ] Написать тесты на группы ссылок, legal labels, отсутствие пустых/фиктивных `href` и последовательную heading hierarchy.
- [ ] Реализовать многоколоночный desktop footer и компактный mobile layout.
- [ ] Активировать только существующие маршруты; будущие страницы не превращать в ссылки.
- [ ] Добавить организационные данные только после их подтверждения владельцем проекта.
- [ ] Запустить focused test.

### Task 13: Подготовить собственные медиаматериалы

**Files:**
- Create: `web/platform/public/landing/README.md`
- Modify: `web/platform/src/features/landing/landing-content.ts`

- [ ] Описать для каждого asset источник, лицензию, назначение, размеры и обязательный alt.
- [ ] Подготовить responsive AVIF/WebP варианты hero/news/capability изображений.
- [ ] Не добавлять материалы Study24 в репозиторий.
- [ ] Проверить, что hero/LCP asset один, имеет `preload`, а все последующие изображения lazy-load.
- [ ] Проверить отсутствие layout shift по заданным размерам и `aspect-ratio`.

### Task 14: Довести адаптивность и темы

**Files:**
- Modify: `web/platform/src/app/globals.css`
- Modify: `web/platform/src/features/landing/**/*.module.css`
- Create: `web/platform/src/features/landing/landing-responsive.styles.test.ts`

- [ ] Зафиксировать CSS-контракт для 320, 375, 768, 1024, 1440 и 1920 px.
- [ ] Проверить fixed sidebar/sticky header, отсутствие горизонтального overflow и доступность всех CTA.
- [ ] Проверить светлую, тёмную и системную тему без вспышки неверной темы.
- [ ] Добавить `prefers-reduced-motion` правила для всех новых transitions.
- [ ] Запустить style-contract tests.

### Task 15: Добавить browser acceptance tests

**Files:**
- Create: `web/platform/e2e/public-home.spec.ts`
- Modify: `web/platform/playwright.config.ts` только если текущая конфигурация не запускает новый spec.

- [ ] Проверить публичное открытие `/` без сессии.
- [ ] Проверить mobile drawer, hero model switch, guest draft redirect, show-more, carousel и FAQ.
- [ ] Проверить один H1, canonical, robots meta и наличие JSON-LD.
- [ ] Проверить, что переходы в `/app` не ломают текущую авторизацию.
- [ ] Проверить отсутствие console errors, broken images и горизонтального overflow.
- [ ] Снять desktop/mobile screenshots как DEV-baseline без копирования чужих изображений.

### Task 16: Проверить производительность и SEO-бюджеты

**Files:**
- Create: `web/platform/scripts/assert-public-home.mjs`
- Modify: `web/platform/package.json`
- Modify: `web/platform/package-lock.json`

- [ ] Добавить команду `test:public-home`, проверяющую build output, обязательные metadata и отсутствие импорта приватных session-модулей в landing tree.
- [ ] Проверить route output: `/` должен быть статическим или кэшируемым без per-user данных.
- [ ] Измерить bundle публичной страницы и зафиксировать baseline в PR/коммите.
- [ ] Проверить LCP/CLS/INP на DEV с холодным и тёплым кэшем.
- [ ] Если страница не проходит целевые бюджеты, уменьшить client boundaries и media payload до перехода к следующей задаче.

### Task 17: Полная регрессия и DEV-развёртывание

**Files:**
- Modify: `docs/superpowers/specs/2026-08-04-public-homepage-study24-reference-design.md` только если реализация выявила согласованное изменение.
- Modify: `docs/superpowers/plans/2026-08-04-public-homepage-study24-reference.md` отмечая выполненные пункты.

- [ ] В `web/platform` выполнить `npm test`.
- [ ] Выполнить `npm run typecheck`.
- [ ] Выполнить `npm run lint`.
- [ ] Выполнить `npm run build`.
- [ ] Выполнить `npm run test:packaging` и `npm run test:public-home`.
- [ ] Выполнить Playwright acceptance tests.
- [ ] В корне репозитория выполнить `git diff --check` и проверить итоговый diff на случайные backend-изменения и чужие assets.
- [ ] Сделать атомарный commit только после зелёной проверки.
- [ ] Перенести проверенный commit в `dev-deploy`, push в `origin/dev-deploy` и дождаться зелёных `CI`, `Docker Images`, `Deploy DEV`.
- [ ] Проверить `https://dev-web.neiirohub.ru/` на desktop/mobile, обе темы, metadata, основные переходы и отсутствие регрессий `/app`.

## Порядок релизных срезов

1. **Срез A — каркас:** Tasks 1–4. Индексируемая серверная страница со всеми статическими секциями.
2. **Срез B — первый экран:** Tasks 5–6. Model-aware composer и быстрые инструменты.
3. **Срез C — контент:** Tasks 7–12. Новости, модели, возможности, CTA, FAQ и footer.
4. **Срез D — качество:** Tasks 13–16. Assets, responsive, темы, browser tests, SEO и performance.
5. **Срез E — DEV:** Task 17. Полная регрессия, push и визуальная приёмка.

После каждого среза изменения должны быть тестируемыми в локальной сборке. В `dev-deploy` отправляется только полностью проверенный срез, чтобы пользователь всегда видел завершённое состояние, а не половину макета.
