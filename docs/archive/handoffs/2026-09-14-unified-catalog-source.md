Status: archived
Do not use for current implementation.
See: docs/HANDOFF_CURRENT.md

# Передача для merge: единый каталог моделей NeiroHub

Status: active
Дата: 2026-09-14
Назначение: агенту, который переносит и объединяет изменения в репозитории друга.

## 1. Сначала прочитать

Это инструкция по переносу существующей реализации. Она не означает, что все
возможности моделей уже реализованы или подтверждены у провайдеров.

- Исходная ветка: `fix/remove-workspace-dividers`.
- Базовый HEAD до фиксации передаваемых изменений: `40be170d4767b0e7a9820fa4c8d2adbd14e6d286`
  (`fix: keep header model selector sizing consistent`).
- Исходное рабочее дерево:
  `C:/Users/Lenovo/Documents/агрегатор/remove-workspace-dividers`.
- **Указанный базовый HEAD не содержит передаваемые изменения.** После подготовки
  этого документа пользователь поручил закоммитить текущий набор и отправить его
  в `origin/dev-deploy`. Эта версия документа включается в тот же коммит;
  получателю нужно брать коммит с обновлённым handoff, а не базовый HEAD.
  Фактическое наличие коммита на удалённой ветке проверить через Git.
- Пользователь назначил веткой отправки `dev-deploy`. Ветку, в которую агент
  друга объединяет работу в своём checkout, определять по поручению его владельца.
- Сам Markdown не содержит исходников или применяемого patch. Получателю нужен
  также коммит в `origin/dev-deploy` с этой передачей либо актуальный исходный
  checkout, включая новые файлы, удаления и бинарные assets.
- В исходном checkout были изменения ещё до миграции каталога. Полный diff к HEAD
  включает предыдущие работы по UI, платежам и диалогу. Не называть его целиком
  чистым патчем единого каталога.

Прочитать текущие `AGENTS.md`, `.agents/state.json`, `docs/ARCHITECTURE.md`,
правила затронутого пакета, затем этот файл и профильные документы.
Git-состояние выше проверено непосредственно; старый state-файл не является
доказательством текущей ветки.

Старая передача по аккаунтам сохранена без изменения её исходного содержимого:
[архив](archive/handoffs/2026-07-05-account-identity-preserved-2026-09-14.md).
Она не задаёт текущую цель merge.

## 2. Что пользователь согласовал

Один backend-источник моделей для всего сайта. У каждой модели общий публичный
ID, название, описание, тип, категории, версия/статус проверки и типизированные
операции. Назначения: text, image, video, audio.

Категории интерфейса независимы от назначения:

- Популярные — `popular`.
- Изображения — `images`.
- Текст — `text`.
- Видео и аудио — `video-audio`.
- Бесплатные — `free`.
- Учёба и работа — `study-work`.

Модель может принадлежать нескольким категориям. Текстовая модель, принимающая
фото, не превращается из-за этого в модель генерации изображений.

Все рабочие точки используют одну выдачу: хедер, текущий диалог, новый чат,
полный каталог, главная, генератор изображений и инструменты файлов.

Поведение селектора:

- Хедер выбирает модель для нового чата.
- Селектор у инпута меняет модель текущего диалога, сохраняя URL и черновик.
- В меню до пяти моделей на категорию; выбор категории поднимает её блок наверх,
  другие блоки остаются в прокручиваемой ленте.
- Полный каталог показывает все подходящие модели.
- Подборки хранят только ID. Названия, описания, доступность и цены не копируются
  в отдельные UI-таблицы.
- Только у инпута описание отображается подсказкой при наведении/фокусе.
  Используется расширение общего компонента, а не отдельный каталог.

## 3. Что реализовано и должно пережить merge

### Backend

`productcatalog.WorkspaceCatalog` собирает безопасную публичную выдачу из
существующего реестра, доступных маршрутов и серверных снимков цен.
Авторизованный `GET /web/v1/models` возвращает `Cache-Control: no-store`.
В ответе нет ключей, provider endpoints, внутренних provider IDs или отчётов допуска.

Старые image/chat/video API остаются совместимыми представлениями. Рабочий
фронтенд обращается к единому endpoint.

Настройки изображений содержат допустимые сочетания качества и пропорций,
default, количество результатов, лимит промпта и точные цены `price_by_variant`.
Нельзя заменить эту матрицу независимыми списками, декартово произведение которых
разрешает неподдерживаемый запрос. Default должен быть действительно оценённым
вариантом, в том числе при разреженной матрице.

Настройки видео содержат совместимые сочетания разрешения, длительности и
пропорций, default и серверные цены. Путь сайта сейчас принимает текст:
маршруты, требующие референсов или специального определения длительности,
не объявляются доступными через этот путь.

Непроверенные FPS/наличие звука в публичных вариантах видео — `null`.
Не заменять их на 0/false: это превратит неизвестное в утверждение.

Категория `free` определяется серверной стоимостью и не может быть назначена
платной модели произвольной записью контракта.

### Frontend

`model-catalog-cache.ts` — один запрос, общий выполняющийся запрос для параллельных
потребителей и кэш успешного ответа на 60 секунд. Ошибка не кэшируется как успех
и не заменяется локальным вымышленным списком.

`model-catalog-contract.ts` строго проверяет ответ: типы, уникальные ID,
категории, операцию основного назначения, defaults и наличие цен для вариантов.
Compatibility loaders только преобразуют эти данные.

Полный каталог показывает текст, изображения и видео из той же выдачи, что
селекторы; старые заглушки категорий удалены. Ошибки 503 и неверный JSON дают
состояние ошибки, а не ложное сообщение об отсутствии моделей.

Главная использует фактическую запись выбранной модели. После загрузки каталога
ввод текста включает отправку; смена на изображение сохраняет черновик и
показывает параметры выбранной модели. Не возвращать фиктивную text-модель
или mocks в тестах, скрывающие реальный `useGenerationControls`.

Инструменты файлов фильтруют явные операции и включённый приём изображения.
При преобразовании модели сохраняются `categories`; иначе селектор пустеет.
Кнопки исполнения этих инструментов пока отключены: соответствующий серверный
путь обработки файла ещё не подключён.

### Контракты и допуск моделей — обязательная зависимость

`internal/service/modelcontract` описывает типизированные возможности и
проверки допуска. `providermodels/onboarding` связывает договорённые возможности,
источники и отчёты с конкретным provider/version и отпечатком записи реестра.

43 существующие записи сохранены в замороженной базе как `legacy-unverified`.
Это исключение совместимости, не подтверждение возможностей.
Изменение отпечатка требует нормального допуска. Нельзя обновить legacy.json
лишь для того, чтобы merge или CI стали зелёными.

Новые/изменённые возможности неизвестного происхождения остаются выключенными.
Валидация, startup и CI не выполняют платных запросов.

### Локальный preview

JSON `model-catalog.preview.json` генерируется Go builder, а не поддерживается
вручную. В текущем офлайн-сценарии 36 web-пригодных записей; это не число доступных
моделей DEV/PROD. Preview включается только локально в development через
`NEIROHUB_LOCAL_WORKSPACE_PREVIEW`. Production fallback на preview не добавлять.

## 4. Карта файлов и зависимостей

Все пути в этом разделе относительно корня репозитория; в другом checkout
не использовать абсолютный путь машины автора.

| Узел | Файлы / директории |
| --- | --- |
| Публичный backend-каталог | `internal/service/productcatalog/workspace.go`, `workspace_types.go`, `workspace_preview.go`, `workspace_test.go`, `workspace_invariants_test.go` |
| API и фактическая валидация | `internal/adapter/inbound/websession/model_catalog.go`, `model_catalog_test.go`, `handler.go`, `conversation_media.go`, `conversation_media_test.go` |
| Общие правила генерации | `internal/service/productcatalog/catalog.go`, `internal/service/imagegeneration/resolver.go` |
| Допуск | `internal/service/modelcontract/`, `internal/service/providermodels/onboarding.go`, `onboarding_test.go`, `onboarding/`, `registry.go`, `scripts/models/check/` |
| Генерация preview / CI | `scripts/models/preview/main.go`, `.github/workflows/ci.yml` |
| Контракт / кэш / проекции UI | `web/platform/src/features/models/model-catalog-contract.ts`, `model-catalog-cache.ts`, `generation-model-catalog.ts`, `image-model-catalog-cache.ts`, `video-model-catalog.ts`, `chat-model-selector.ts`; `features/conversations/ConversationModelSelector/chat-model-catalog.ts` |
| Настройки / цены UI | `web/platform/src/features/models/generation-options.tsx`, `generation-options-contract.ts`, `features/image-generation/ImageGenerationPanel/useImageGeneration.ts`, `ImageGenerationComposer/ImageGenerationControls.tsx`, `src/lib/web-api/contracts.ts` |
| Каталог / карточки / селекторы | `features/models/ModelsCatalog/`, `ModelCard/`, `WorkspaceModelSelector/`, `features/conversations/ConversationModelSelector/` |
| Формы и главная | `features/workspace/WorkspaceHero/`, `WorkspacePrompt/`, `WorkspaceHome/`, `FeaturedModels/`, `FeaturedModelShortcuts/`; `features/conversations/ConversationComposer/`, `ConversationHistory/` |
| Инструменты файлов | `features/files/FilePreviewDialog/file-action-models.ts`, `FileTaskModelSelector.tsx`, `FileEditorPanel.tsx`, `FileAnimationPanel.tsx` |
| BFF / preview | `web/platform/src/app/web/v1/[...path]/route.ts`, `src/features/session/local-workspace-preview.ts`, `model-catalog.preview.json` |
| Регрессии / fixtures | `web/platform/src/features/models/model-catalog-*.test.ts`, `ModelsCatalog/ModelsCatalog.failure.test.tsx`, `src/test/model-catalog.ts`, `src/test/setup.ts`, тесты перечисленных потребителей |
| Канонические runtime IDs | `src/content/data/models.ts`, `src/content/domain/schemas.ts`, `features/inspiration/inspiration-examples.ts`, `features/workspace/WorkspaceLanding/WorkspaceLanding.tsx` |
| Удаления | `web/platform/src/features/models/ModelsCatalog/catalog-placeholder-models.ts` и `catalog-placeholder-models.test.ts` |

Сокращённые `features/...` и `src/...` в таблице относятся к `web/platform/src`.
Переносить также изменённые тесты соответствующих модулей.

Runtime ID `gpt_image_2` не заменять маркетинговым slug `gpt-image-2`.
Публичные ссылки/SEO slug и ключ подключения модели выполняют разные функции.

### Что ещё уже находилось в исходном дереве

Предыдущие изменения затрагивают:

- Go web payments, возврат после оплаты, account receipt и модалку пополнения.
- Мультимедийный диалог, сохранение/возобновление Jobs, worker/dialogcontext.
- Общие PopoverPanel, ModalCloseButton, ModeSwitchPanel; размеры селектора,
  прокрутку, меню диалога и галерею.
- Карточки тарифов/пакетов токенов и новые PNG assets.

Это не повод откатывать их. Если переносится весь набор работ, переносить
согласованно. Если нужен только каталог, сначала отделить зависимости и
сравнить целевой код. Отчёт git status «до» не содержит старые версии файлов:
по нему нельзя автоматически восстановить чистый отдельный патч каталога.

## 5. Что пока не завершено — не включать скрытно во время merge

1. По 43 старым записям ещё нужны сверка официальных источников, заполнение
   контрактов, проверки адаптеров и результатов. Из них две записи mock;
   реальных — 41. Успешный офлайн-тест не переводит их в verified.
2. Общий лимит количества всех вложений вместе: есть лимиты по типам и общий
   размер, отдельного общего счётчика пока нет.
3. Точные width/height выходных изображений и минимальные размеры входных.
4. Явные типизированные режимы (генерация, редактирование, продолжение и т. п.):
   операция пока использует свободный ID.
5. Передача всех возможностей внутреннего контракта в публичную выдачу и
   подключение соответствующих загрузок/контролов. Например, маски,
   прозрачность и output formats предусмотрены внутренней схемой, но это
   не означает готовую функцию сайта.
6. Несколько операций одной модели поддерживаются структурой. Текущая выдача
   публикует одну рабочую операцию на модель; общий UI переключения назначения
   одной модели ещё не реализован.
7. Реальное исполнение аудиозадач и файловых инструментов не подключено.

Сохранить эти ограничения. Это следующий этап разработки, а не дефект merge,
который нужно обходить фиктивными значениями или активацией флагов.

## 6. Порядок переноса и разрешения конфликтов

- [ ] Зафиксировать фактические source HEAD, целевой HEAD и состояние обоих
      рабочих деревьев. Сохранить работу принимающей стороны.
- [ ] Убедиться, что получен полный коммит с изменениями автора либо его checkout:
      tracked diff, новые файлы (включая tests/JSON/PNG) и удаления.
      Один `git diff` не включает untracked-файлы. Передача только этого
      Markdown или указанного HEAD недостаточна.
- [ ] Сопоставить версии backend, frontend и контрактов у получателя. Не
      заменять целые каталоги старым checkout; разрешать пересечения по смыслу.
- [ ] Сначала объединить modelcontract/onboarding/registry и используемые
      runtime/pricing зависимости. Сохранить текущие provider mappings и
      frozen baseline; расхождение отпечатка разбирать, а не маскировать.
- [ ] Затем объединить builder, авторизованный endpoint, совместимые API и
      серверные проверки параметров.
- [ ] Далее перенести строгий парсер, единый кэш, проекции и все потребители.
      Не оставлять фронтенд на новом API при старом backend и наоборот.
- [ ] Перенести тесты, удаления заглушек, preview generator и CI check.
      Если в целевой ветке уже изменены модели, сгенерировать preview из
      окончательного объединённого backend и проверить получившийся diff.
- [ ] В смешанных файлах `handler.go`, `registry.go`, `worker.go`,
      `ConversationHistory.tsx`, BFF route, `contracts.ts` и CI сохранить
      обе совместимые функции; не выбирать целиком ours/theirs.
- [ ] `docs/ARCHITECTURE.md` исходного дерева имеет смешанную кодировку.
      Не перекодировать файл целиком: это создаст посторонний diff и риск
      порчи текста. Переносить только содержательные новые разделы.
- [ ] `web/platform/next-env.d.ts` генерируется framework: не переносить
      локальные `.next` или `.next/dev` артефакты как исходный код.
- [ ] Повторить проверки ниже на результате объединения.
- [ ] Представить владельцу целевой ветки итоговый diff, результаты и
      оставшиеся ограничения. Этот файл не даёт отдельного разрешения
      публиковать, deploy, запускать платные запросы или отправлять сообщения.

## 7. Проверки перед интеграцией

Команды выполнять из корня целевого checkout, с установленными зависимостями
проекта и существующим lockfile. Новых зависимостей в миграции каталога нет.

### Backend

```text
go run ./scripts/models/check
go run ./scripts/models/preview -check
go test ./internal/service/modelcontract ./internal/service/providermodels ./internal/service/productcatalog ./internal/service/imagegeneration ./internal/service/textgeneration ./internal/service/videorouter ./internal/adapter/inbound/websession ./internal/platform/config ./internal/app/api ./internal/service/dialogcontext ./internal/worker ./scripts/models/check ./scripts/models/preview
go vet ./internal/service/modelcontract ./internal/service/providermodels ./internal/service/productcatalog ./internal/service/imagegeneration ./internal/service/textgeneration ./internal/service/videorouter ./internal/adapter/inbound/websession ./internal/platform/config ./internal/app/api ./internal/service/dialogcontext ./internal/worker ./scripts/models/check ./scripts/models/preview
```

Для обновления preview после осознанного изменения backend:
`go run ./scripts/models/preview`, затем просмотр diff и повторный `-check`.
Не исправлять сгенерированный JSON вручную.

### Frontend

```text
npm --prefix web/platform run lint
npm --prefix web/platform run typecheck
npm --prefix web/platform run test
npm --prefix web/platform run validate:assets
npm --prefix web/platform run test:packaging
npm --prefix web/platform run build
git diff --check
```

На машине автора Vitest запускался с двумя workers:
из `web/platform` — `node node_modules/vitest/vitest.mjs run --maxWorkers=2`;
asset suite отдельно: `node --test scripts/validate-assets.test.mjs`.
Это тот же набор unit/integration-тестов. Проверять реальный exit code, особенно
при перенаправлении вывода в файл.

### Обязательные регрессии

- Авторизация /web/v1/models, no-store, отсутствие внутренних provider данных.
- Один запрос каталога для нескольких потребителей, строгая ошибка при
  503/неверном JSON, отсутствие локального fallback.
- Текстовые модели одновременно видны в хедере, диалоге и полном каталоге.
- Категории, максимум пять в меню, полный список на странице каталога.
- Смена модели в диалоге сохраняет URL/черновик; хедер открывает новый чат.
- Реальные controls hook на главной: отправка текста и переход на изображение.
- Default и каждая опубликованная комбинация имеют действительную цену;
  sparse image matrix, зависимость video resolution/duration/aspect.
- Платная модель не попадает в free из-за категории контракта.
- File selector не теряет categories; неподключённые операции остаются недоступны.
- `model-catalog-ownership.test.ts` запрещает независимые определения/цены
  и запросы к старым спискам в функциональном UI.
- `TestEveryPreviewOptionResolvesWithoutCallingProviders` сверяет опубликованные
  варианты с реальными server resolvers офлайн.

Если объединяется также предшествующая платёжная работа, выполнить её отдельные
проверки payment/accountservice. Каталожный отчёт не заменяет проверку оплаты.

### Что уже прошло у автора 2026-09-14

- 192 frontend test files, **1280 tests passed**.
- 6 asset tests, asset validation, packaging, TypeScript, ESLint, production build.
- 12 backend packages passed; preview package compiled; соответствующий go vet.
- Admission check: 43 legacy-unverified. Preview check: 36 офлайн-записей.
- Независимые backend/frontend reviews: замечания исправлены, при повторной
  проверке открытых actionable findings не осталось.
- Browser smoke: полный каталог/text, header/new-chat, смена модели существующего
  диалога с сохранением черновика, главная text/image. Генерация не отправлялась.
- git diff --check прошёл.

В тестах истории исправлена изоляция одноразовых mocks и ожидание окончания
accepted-send scrolling. Не возвращать `clearAllMocks` вместо нужного reset
очереди одноразовых ответов; не начинать проверку polling-scroll до завершения
предыдущего этапа отправки.

Эти результаты относятся к исходному рабочему дереву, а не будущему merge.
На результате объединения команды надо выполнить заново.

## 8. Границы, которые нельзя нарушать

- Провайдеры вызываются только worker/adapters; API и UI не генерируют напрямую.
- Backend проверяет пользователя, доступ к файлам, параметры, цену и Jobs.
- Ledger, reserve/capture/release, idempotency и moderation сохраняются.
- Клиентская цена и disabled-кнопка не являются разрешением на платный запуск.
- Не переносить .env, ключи, токены, session cookies, private URLs, node_modules,
  .next, локальные caches или личные файлы.
- Не обновлять legacy-исключения для прохождения CI.
- Не объявлять `unknown` поддержкой и не включать неподтверждённые возможности.
- Реальные проверки запускаются только в согласованном DEV-контуре с явно
  разрешёнными провайдерами, набором случаев и пределом расходов:
  [Правила допуска моделей](../docs/runbooks/MODEL_ONBOARDING.md), «Реальные проверки».

## 9. Документы для продолжения

- [Единый каталог — фактические владельцы и ограничения](../docs/runbooks/MODEL_CATALOG.md).
- [Обязательный контракт добавления/изменения модели](../docs/runbooks/MODEL_ONBOARDING.md).
- [Матрица проверки 43 существующих записей](runbooks/model-onboarding/existing-models-2026-09-14.md).
- [Дизайн миграции](superpowers/specs/2026-09-14-unified-model-catalog-design.md).
- [План и результаты миграции](superpowers/plans/2026-09-14-unified-model-catalog.md).
- [UI index](../web/platform/docs/ui-index.md), затем только нужная секция
  [UI catalog](../web/platform/docs/ui-catalog.md).

## 10. Приложение: снимок путей перед коммитом

Ниже полное git status исходного дерева **до подготовки этого handoff**.
Это инвентарь для сверки полноты передачи, не разрешение переносить все изменения
без разбора. M — изменён tracked-файл, D — удаление, ?? — новый файл.

~~~text
 M .github/workflows/ci.yml
 M AGENTS.md
 M cmd/api/main.go
 M docs/ARCHITECTURE.md
 M docs/INDEX.md
 M docs/runbooks/BILLING.md
 M docs/runbooks/DEV.md
 M internal/adapter/inbound/websession/handler.go
 M internal/app/api/core.go
 M internal/platform/config/config.go
 M internal/service/accountservice/service.go
 M internal/service/dialogcontext/service.go
 M internal/service/imagegeneration/resolver.go
 M internal/service/paymentservice/service.go
 M internal/service/productcatalog/builder.go
 M internal/service/productcatalog/catalog.go
 M internal/service/providermodels/registry.go
 M internal/worker/worker.go
 M web/platform/docs/ui-catalog-examples.md
 M web/platform/docs/ui-catalog.md
 M web/platform/docs/ui-index.md
 M web/platform/next-env.d.ts
 M web/platform/src/app/app/chats/page.tsx
 M web/platform/src/app/app/models/page.test.tsx
 M web/platform/src/app/app/models/page.tsx
 M web/platform/src/app/web/v1/[...path]/route.test.ts
 M web/platform/src/app/web/v1/[...path]/route.ts
 M web/platform/src/assets/asset-paths.ts
 M web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.test.tsx
 M web/platform/src/components/chat/ChatScrollToBottom/ChatScrollToBottom.tsx
 M web/platform/src/components/layout/Sidebar/Sidebar.module.css
 M web/platform/src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.module.css
 M web/platform/src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx
 M web/platform/src/components/layout/WorkspaceHeader/TokenTopUpDialog.module.css
 M web/platform/src/components/layout/WorkspaceHeader/TokenTopUpDialog.styles.test.ts
 M web/platform/src/components/layout/WorkspaceHeader/TokenTopUpDialog.tsx
 M web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css
 M web/platform/src/components/layout/WorkspaceHeader/WorkspaceHeader.test.tsx
 M web/platform/src/components/ui/ModalCloseButton/ModalCloseButton.module.css
 M web/platform/src/components/ui/ModeSwitchPanel/ModeSwitchPanel.test.tsx
 M web/platform/src/components/ui/ModeSwitchPanel/ModeSwitchPanel.tsx
 M web/platform/src/components/ui/PopoverPanel/PopoverPanel.module.css
 M web/platform/src/components/ui/PopoverPanel/PopoverPanel.test.tsx
 M web/platform/src/components/ui/PopoverPanel/PopoverPanel.tsx
 M web/platform/src/components/ui/PopoverPanel/README.md
 M web/platform/src/content/data/models.ts
 M web/platform/src/content/domain/schemas.ts
 M web/platform/src/content/repository/content-repository.test.ts
 M web/platform/src/features/account/AccountMenu/AccountMenu.tsx
 M web/platform/src/features/conversations/ConversationComposer/ConversationComposer.test.tsx
 M web/platform/src/features/conversations/ConversationComposer/ConversationComposer.tsx
 M web/platform/src/features/conversations/ConversationHistory/ConversationHistory.test.tsx
 M web/platform/src/features/conversations/ConversationHistory/ConversationHistory.tsx
 M web/platform/src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.test.tsx
 M web/platform/src/features/conversations/ConversationImageGallery/ConversationImageGallery.module.css
 M web/platform/src/features/conversations/ConversationImageGallery/ConversationImageGallery.test.tsx
 M web/platform/src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx
 M web/platform/src/features/conversations/ConversationModelSelector/ConversationModelSelector.tsx
 M web/platform/src/features/conversations/ConversationModelSelector/chat-model-catalog.test.ts
 M web/platform/src/features/conversations/ConversationModelSelector/chat-model-catalog.ts
 M web/platform/src/features/conversations/ConversationRow/ConversationRow.test.tsx
 M web/platform/src/features/conversations/ConversationRow/ConversationRow.tsx
 M web/platform/src/features/conversations/ConversationRow/FloatingConversationPanel.tsx
 M web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.test.tsx
 M web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.tsx
 M web/platform/src/features/conversations/TextModelSelector.test.tsx
 M web/platform/src/features/conversations/pending-conversation-bootstrap.ts
 M web/platform/src/features/files/FilePreviewDialog/FileAnimationPanel.tsx
 M web/platform/src/features/files/FilePreviewDialog/FileEditorPanel.tsx
 M web/platform/src/features/files/FilePreviewDialog/FilePreviewDialog.test.tsx
 M web/platform/src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx
 M web/platform/src/features/files/FilePreviewDialog/file-action-models.ts
 M web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.test.tsx
 M web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx
 M web/platform/src/features/image-generation/ImageGenerationComposer/ImageGenerationControls.tsx
 M web/platform/src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.test.tsx
 M web/platform/src/features/image-generation/ImageGenerationPanel/useImageGeneration.ts
 M web/platform/src/features/inspiration/InspirationGallery/InspirationGallery.test.tsx
 M web/platform/src/features/inspiration/inspiration-examples.test.ts
 M web/platform/src/features/inspiration/inspiration-examples.ts
 M web/platform/src/features/models/ModelCard/ModelCard.integration.test.ts
 M web/platform/src/features/models/ModelCard/ModelCard.test.tsx
 M web/platform/src/features/models/ModelCard/ModelCard.tsx
 M web/platform/src/features/models/ModelCard/model-card-content.ts
 M web/platform/src/features/models/ModelsCatalog/ModelsCatalog.test.tsx
 M web/platform/src/features/models/ModelsCatalog/ModelsCatalog.tsx
 D web/platform/src/features/models/ModelsCatalog/catalog-placeholder-models.test.ts
 D web/platform/src/features/models/ModelsCatalog/catalog-placeholder-models.ts
 M web/platform/src/features/models/ModelsCatalog/model-filters.test.ts
 M web/platform/src/features/models/ModelsCatalog/model-filters.ts
 M web/platform/src/features/models/WorkspaceModelSelector/ModelSelector.test.tsx
 M web/platform/src/features/models/WorkspaceModelSelector/ModelSelector.tsx
 M web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css
 M web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.styles.test.ts
 M web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.test.tsx
 M web/platform/src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx
 M web/platform/src/features/models/image-model-catalog-cache.test.ts
 M web/platform/src/features/models/image-model-catalog-cache.ts
 M web/platform/src/features/session/local-workspace-preview.ts
 M web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.test.tsx
 M web/platform/src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx
 M web/platform/src/features/workspace/FeaturedModels/FeaturedModels.test.tsx
 M web/platform/src/features/workspace/FeaturedModels/FeaturedModels.tsx
 M web/platform/src/features/workspace/WorkspaceHero/WorkspaceHero.test.tsx
 M web/platform/src/features/workspace/WorkspaceHero/WorkspaceHero.tsx
 M web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.test.tsx
 M web/platform/src/features/workspace/WorkspaceHome/WorkspaceHome.tsx
 M web/platform/src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx
 M web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.test.tsx
 M web/platform/src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx
 M web/platform/src/i18n/ru.ts
 M web/platform/src/lib/web-api/contracts.ts
 M web/platform/src/lib/web-api/proxy.test.ts
 M web/platform/src/lib/web-api/proxy.ts
 M web/platform/src/test/setup.ts
?? docs/runbooks/MODEL_CATALOG.md
?? docs/runbooks/MODEL_ONBOARDING.md
?? docs/runbooks/model-onboarding/audio.example.json
?? docs/runbooks/model-onboarding/existing-models-2026-09-14.md
?? docs/runbooks/model-onboarding/image.example.json
?? docs/runbooks/model-onboarding/text.example.json
?? docs/runbooks/model-onboarding/verification-template.md
?? docs/runbooks/model-onboarding/video.example.json
?? docs/superpowers/plans/2026-09-14-model-onboarding.md
?? docs/superpowers/plans/2026-09-14-unified-model-catalog.md
?? docs/superpowers/specs/2026-09-14-unified-model-catalog-design.md
?? internal/adapter/inbound/websession/conversation_media.go
?? internal/adapter/inbound/websession/conversation_media_test.go
?? internal/adapter/inbound/websession/model_catalog.go
?? internal/adapter/inbound/websession/model_catalog_test.go
?? internal/adapter/inbound/websession/payments.go
?? internal/adapter/inbound/websession/payments_test.go
?? internal/app/api/web_payments.go
?? internal/app/api/web_payments_test.go
?? internal/service/accountservice/receipt.go
?? internal/service/accountservice/receipt_test.go
?? internal/service/dialogcontext/media_test.go
?? internal/service/modelcontract/contract.go
?? internal/service/modelcontract/contract_test.go
?? internal/service/modelcontract/evidence.go
?? internal/service/modelcontract/media_test.go
?? internal/service/modelcontract/report_test.go
?? internal/service/modelcontract/validation.go
?? internal/service/paymentservice/account_return_test.go
?? internal/service/productcatalog/workspace.go
?? internal/service/productcatalog/workspace_invariants_test.go
?? internal/service/productcatalog/workspace_preview.go
?? internal/service/productcatalog/workspace_test.go
?? internal/service/productcatalog/workspace_types.go
?? internal/service/providermodels/onboarding.go
?? internal/service/providermodels/onboarding/approved/README.md
?? internal/service/providermodels/onboarding/legacy.json
?? internal/service/providermodels/onboarding_test.go
?? internal/worker/conversation_media_test.go
?? scripts/models/check/main.go
?? scripts/models/check/main_test.go
?? scripts/models/preview/main.go
?? web/platform/public/assets/images/credits/token-package-10000.png
?? web/platform/public/assets/images/credits/token-package-1500.png
?? web/platform/public/assets/images/credits/token-package-20000.png
?? web/platform/public/assets/images/credits/token-package-3000.png
?? web/platform/public/assets/images/credits/token-package-800.png
?? web/platform/src/app/app/payment-return/page.tsx
?? web/platform/src/components/layout/WorkspaceHeader/TokenTopUpDialog.test.tsx
?? web/platform/src/components/ui/PopoverPanel/PopoverSurface.test.tsx
?? web/platform/src/components/ui/PopoverPanel/usePopoverPresence.ts
?? web/platform/src/features/account/AccountMenu/AccountMenu.test.tsx
?? web/platform/src/features/conversations/pending-media-job.ts
?? web/platform/src/features/files/FilePreviewDialog/FileActionPanels.test.tsx
?? web/platform/src/features/files/FilePreviewDialog/file-action-models.test.ts
?? web/platform/src/features/models/ModelsCatalog/ModelsCatalog.failure.test.tsx
?? web/platform/src/features/models/WorkspaceModelSelector/ModelSelectorFeed.test.tsx
?? web/platform/src/features/models/WorkspaceModelSelector/ModelSelectorOption.test.ts
?? web/platform/src/features/models/WorkspaceModelSelector/ModelSelectorOption.tsx
?? web/platform/src/features/models/WorkspaceModelSelector/model-selector-sections.ts
?? web/platform/src/features/models/chat-model-selector.ts
?? web/platform/src/features/models/generation-model-catalog.test.ts
?? web/platform/src/features/models/generation-model-catalog.ts
?? web/platform/src/features/models/generation-options-contract.ts
?? web/platform/src/features/models/generation-options.test.ts
?? web/platform/src/features/models/generation-options.tsx
?? web/platform/src/features/models/model-catalog-cache.ts
?? web/platform/src/features/models/model-catalog-contract.test.ts
?? web/platform/src/features/models/model-catalog-contract.ts
?? web/platform/src/features/models/model-catalog-ownership.test.ts
?? web/platform/src/features/models/model-catalog-preview.test.ts
?? web/platform/src/features/models/model-catalog-test-fixtures.ts
?? web/platform/src/features/models/video-model-catalog.test.ts
?? web/platform/src/features/models/video-model-catalog.ts
?? web/platform/src/features/payments/PaymentReturn.test.tsx
?? web/platform/src/features/payments/PaymentReturn.tsx
?? web/platform/src/features/payments/PaymentStatus.test.tsx
?? web/platform/src/features/payments/PaymentStatus.tsx
?? web/platform/src/features/payments/checkout-navigation.ts
?? web/platform/src/features/payments/payments.module.css
?? web/platform/src/features/payments/payments.test.ts
?? web/platform/src/features/payments/payments.ts
?? web/platform/src/features/session/model-catalog.preview.json
?? web/platform/src/features/workspace/WorkspacePrompt/NewChatPrompt.test.tsx
?? web/platform/src/features/workspace/WorkspacePrompt/NewChatPrompt.tsx
?? web/platform/src/test/model-catalog.ts
~~~
