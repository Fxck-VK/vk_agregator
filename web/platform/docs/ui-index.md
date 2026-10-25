# UI: короткий индекс для агента

Статус: активный. Входная точка для задач по интерфейсу `web/platform`.

Все 24 публичных шаблона `/ru` и `/en`, словари редакционных данных,
обратный перевод и проверка новых текстов:
[контракт локализации публичных шаблонов](public-template-localization.md).

Юридические ссылки, страницы документов, принятие условий и отдельная рассылка:
[`features/legal`](../src/features/legal/LegalLinks.tsx),
[контракт компонентов](ui-catalog.md#legal-documents),
[runtime и публикация](../../../docs/runbooks/WEB_LEGAL.md).

Публичные `/legal` и `/legal/[slug]` заново оформлены по референсу Magnific №23:
`LegalPageFrame`, `LegalNavigation`, общий `HelpCenterDrawer` и серверный `LegalPageContent`.
У них собственные панель иконок, меню документов и область чтения; шапка и подвал
главной здесь не используются. Утверждённый каталог и согласия не меняются.

Тексты и язык интерфейса: [локализация](localization.md). Использовать общий `LocaleProvider`, `useDictionary` / `useMessages`; язык переключает готовый `LanguageSwitcher`. Не добавлять прямые импорты `ru` и надписи вне словарей.

Источник бренда сайта и писем — [branding/brand.json](../branding/brand.json); [config/brand.ts](../src/config/brand.ts) экспортирует его настройки: `brandName`, `brandLogoPath`, `brandIcons` для вкладки и устройств. Основной логотип выводит общий [BrandLogo](../src/components/brand/BrandLogo/BrandLogo.tsx), включая боковую панель, публичную шапку и подвалы; фирменный чат использует тот же путь. Фирменное начертание — общий [BrandName / BrandText](../src/components/brand/BrandName/BrandName.tsx), SVG-надписи зарегистрированы в `branding/brand.json` по ключу из имени бренда. Комплект Epifera (знак, надпись и значки вкладки/устройств) получен из SVG, присланного владельцем. Контуры сохранены, точка над i градиентная; цвет букв наследуется от темы. Если для нового имени SVG нет, используется исходный `--font-sans` / Geist. [Границы настройки](localization.md#общая-система-словарей).

Ссылки: `@/i18n/Link`; переходы и сравнение пути: `@/i18n/navigation`. Они сохраняют язык URL. `LanguageSwitcher` открывает текущую страницу на другом языке, сохраняя состояние. Правила: [языковые адреса](locale-routing.md).

Префикс имён скачиваемых фото и видеопримеров — `brandFilePrefix` из того же `config/brand.ts`; он автоматически вычисляется из `brandName` и отдельно не настраивается.

Найди строку → прочитай указанный раздел → проверь исходник и потребителя → используй или расширь готовое решение. Порядок и обновление записей закреплены в [AGENTS.md](../AGENTS.md#ui-reuse-workflow). Повторно загружать уже прочитанный индекс в одной задаче не нужно, если связанные сведения не изменились.

Архитектура загрузки, кеши, порядок запросов и ограничения: [preloading.md](preloading.md).
Восстановление сессии, тихое обновление чатов и кеш списка между перезагрузками также описаны в этом контракте; cold-error sidebar использует отдельный `StateNotice`, а не `LoadFeedback`.

| Запрос / задача | Готовое решение | Читать точечно |
| --- | --- | --- |
| Английская страница автора по референсу 24 | `AuthorLanding` на `/en/blog/author/epifera`: редакционный бренд, 25 существующих материалов, сетка 12/12/1, query-пагинация и поиск всего архива; RU пустой, draft/noindex | [Автор](ui-catalog.md#author-archive-template) |
| Английский шаблон события / проекта по референсу 22 | `EventLanding` на `/en/events/creative-sessions`: концепт, программа, локальный трейлер, обсуждение, блоки встречи, журнал и FAQ; RU пустой, draft/noindex | [Событие / проект](ui-catalog.md#event-and-project-template) |
| Английская страница о компании и контактах по референсу 21 | `AboutLanding` на `/en/company/about-us`: направления работы, сценарии, ценности, модели, FAQ и реальные контакты; RU пустой, draft/noindex | [О компании](ui-catalog.md#about-and-contacts-template) |
| Английская история клиента по референсу 20 | `CustomerStory` на `/en/customers/football-campaign`: attributed CaseStudy, показатели, 4 раздела/YouTube-ссылки, галерея и титры; это reference case Magnific, RU пустой, draft/noindex | [История клиента](ui-catalog.md#customer-story-template) |
| Английская страница для команд по референсу 19 | `BusinessLanding` на `/en/business`: видео/мозаика, 8 сценариев, 5 разделов, trust/capacity, 4 шага и enquiry 2–100 человек; Business planned, RU пустой, draft/noindex | [Для команд](ui-catalog.md#business-template) |
| Английские тарифы по референсу 18 | `PricingLanding` на `/en/pricing`: individual/team, месяц/годовая оценка, capacity selector, сравнение, справка, enquiry brief и 10 FAQ; цены из общего UI-preview, оплаты нет, RU пустой, draft/noindex | [Тарифы](ui-catalog.md#pricing-template) |
| Английская интеграция MCP по референсу 17 | `IntegrationLanding` на `/en/mcp`: 8 клиентов, status/guide, reference video, 5 feature rows, 6 copyable briefs и 11 FAQ; реальный connector пока planned, RU пустой, draft/noindex | [Интеграция](ui-catalog.md#integration-template) |
| Английская карточка материала по референсу 16 | `MaterialDetailPage` на `/en/stock/items/dried-gypsophila/gypsophila-01`: фото/автор, source rights, PNG/WebP, save/share, related carousels и video stills; русский пустой, draft/noindex | [Карточка материала](ui-catalog.md#material-detail) |
| Английская категория материалов по референсу 15 | `MaterialCategoryPage` на `/en/photos/nature`: 50 фото, поиск, фильтры/сортировка, Masonry/Square, страницы 24/24/2, просмотр и скачивание; русский маршрут пустой, обе версии draft/noindex | [Категория материалов](ui-catalog.md#material-category) |
| Английский публичный пример результата по референсу 14 | `PublicExamplePage` на `/en/inspiration/examples/aesthetic-flowers`: изображение, информация об источнике, PNG/WebP, действия, форма вариации и карусели; русский маршрут пустой, обе версии draft/noindex | [Публичный пример](ui-catalog.md#public-example) |
| Английская подборка примеров / промптов по референсу 13 | `ExampleCollection` на `/en/inspiration/cinematic-scenes`: 48 изображений, поиск, меню Use, просмотр и подсказки; сохранение публичного ID на устройстве, поделиться; русский маршрут пустой, обе версии draft/noindex | [Подборка примеров](ui-catalog.md#example-collection) |
| Имя пользователя в меню аккаунта | `AccountControl` → `AccountMenu`: серверный `display_name`, иначе прежняя подпись почты/провайдера; без запросов к VK | [Аккаунт](ui-catalog.md#browser-account) |
| Смена светлой, тёмной и системной темы | Только `AccountMenu` в личном кабинете через `ModeSwitchPanel`; в публичных шапках и других местах переключателя нет | [Ограничения](ui-catalog.md#limits) |
| Английская публичная главная по референсу Magnific | `PublicHome` + общие `Landing*`: `/en`, первый экран, вкладки, карточки, FAQ; `/ru` пустая, обе версии draft/noindex | [Публичная главная](ui-catalog.md#marketing-home) |
| Английский обзор AI-инструментов по референсу 02 | `ToolsOverview` на `/en/ai`: восемь категорий, модели, AI-клиенты, сценарии, карточки и центрированный подвал; `/ru/ai` пустая, обе версии draft/noindex | [Обзор инструментов](ui-catalog.md#ai-tools-overview) |
| Английская страница семейства моделей по референсу 04 | `KlingFamily` на `/en/ai/kling`: таблица версий, видео, рекомендации, сравнение, исследование и FAQ; `/ru/ai/kling` пустая, обе версии draft/noindex | [Семейство моделей](ui-catalog.md#model-family) |
| Английская страница отдельной версии модели по референсу 05 | `Kling30Landing` на `/en/ai/kling/kling-3-0`: характеристики, примеры, сценарии и FAQ; русский маршрут пустой, обе версии draft/noindex | [Версия модели](ui-catalog.md#model-version) |
| Английское решение конкретной задачи по референсу 06 | `TattooSolution` на `/en/ai/tattoo-generator`: форма идеи/стиля/места, пять примеров, инструкция и связанные блоки; русский маршрут пустой, обе версии draft/noindex | [Решение задачи](ui-catalog.md#task-solution) |
| Английская страница профессии / отрасли по референсу 07 | `MarketingAudience` на `/en/ai/marketing`: видеофон, фильтры примеров, четыре панели задач, инструкции и каталог инструментов; русский маршрут пустой, обе версии draft/noindex | [Профессия / отрасль](ui-catalog.md#audience-page) |
| Английское сравнение моделей по референсу 08 | `AvatarComparison` на `/en/compare/ai-avatar-models`: светлая редакционная шапка, оглавление, восемь разборов моделей, инструкции, FAQ и обмен ссылкой; русский маршрут пустой, обе версии draft/noindex | [Сравнение](ui-catalog.md#model-comparison) |
| Английская статья по референсу 09 | `ArtPromptsArticle` на `/en/blog/ai-art-prompts`: общий `EditorialPage` со сравнением, пять разделов, три иллюстрации, списки и четыре примера; русский маршрут пустой, обе версии draft/noindex | [Статья и общая редакционная оболочка](ui-catalog.md#editorial-article) |
| Английская последовательность действий / workflow по референсу 12 | `WorkflowLanding` на `/en/workflows/product-campaign`: схема процесса, десять сценариев, пошаговый guide, inputs/outputs, карусель и FAQ. Русский пустой, обе версии draft/noindex | [Workflow](ui-catalog.md#workflow-template) |
| Английская документация / помощь по референсу 11 | `DocumentationCenter` на `/en/ai/docs` и семи статьях, включая `/en/ai/docs/account-management`: поиск по тексту, дерево, оглавление, шаги, переходы и support; общий `HelpCenterFrame`. Русский пустой, все draft/noindex | [Документация и помощь](ui-catalog.md#documentation-help) |
| Английский курс / урок по референсу 10 | `AcademyCourse` на `/en/academy/courses/epifera-essentials` и 14 маршрутах уроков: семь модулей, поиск, обзор, упражнения и переходы; общий `HelpCenterFrame` с документами. Русский пустой, все draft/noindex | [Курс и учебная оболочка](ui-catalog.md#academy-course) |
| Английская страница генератора изображений по референсу 03 | `ImageGeneratorLanding` на `/en/ai/image-generator`: галерея, четыре режима, модели, сценарии, графики, FAQ; `/ru` пустая, обе версии draft/noindex | [Страница инструмента](ui-catalog.md#image-tool-preview) |
| Вход, регистрация, восстановление, основная и одна резервная почта, замена и сессии | `LoginWindow`/`LoginShowcase` (выбор способа входа и промослайды), `LoginForm`, `EmailRegistration`, общий `completeSignIn` с проверкой cookie-сессии и новым документом; `CredentialField`, `AccountSecurity`, `InputSurface`, `Button`, `AsyncState` и серверная доступность способов | [Аккаунт](ui-catalog.md#browser-account) |
| Приглашения на сайт и статистика аккаунта | `ProfileReferralProgram`: код в памяти текущей сессии, независимое обновление `/web/v1/referrals/stats`; `InviteEntry`, общий `Button`/`StateNotice`; бонусы пока выключены | [Аккаунт](ui-catalog.md#browser-account), [контракт приглашений](../../../docs/runbooks/WEB_REFERRALS.md) |
| Фирменная страница 404 с рабочим меню | `NotFoundContent` внутри WorkspaceFrame / GuestWorkspaceFrame; общий Button outline и языковой Link | [404](ui-catalog.md#not-found), [дизайн и проверка](not-found-page-plan.md) |
| Всплывающее окно у кнопки, открыть под/над | `PopoverPanel` | [Панели](ui-catalog.md#panels), [пример 1](ui-catalog-examples.md#example-1) |
| То же оформление окна; позиционирование уже есть | `PopoverSurface` | [Панели](ui-catalog.md#panels) |
| Плавное появление и закрытие всплывающей панели | `PopoverSurface` / `PopoverPanel`: анимация по умолчанию, состояние через isOpen; отключение animated=false | [Движение](ui-catalog.md#icons-motion), [пример подключения](../src/components/ui/PopoverPanel/README.md#анимация-панели) |
| «1-й вариант», подсветить текст/иконку при наведении, ссылка-действие | `itemVariant="action"` + `control`; вне панели — `actionItems` + `control` | [Состояния](ui-catalog.md#states), [пример 1](ui-catalog-examples.md#example-1) |
| «2-й вариант», выбранная кнопка белая с рамкой | `PopoverOption`, `itemVariant="selection"` | [Состояния](ui-catalog.md#states), [пример 2](ui-catalog-examples.md#example-2) |
| «Удалить чат»: серый обычно, красный при наведении | Общий action-стиль + локальное исключение `ConversationRow` | [Состояния](ui-catalog.md#states), [пример 1](ui-catalog-examples.md#example-1) |
| Подтверждение удаления | Общий `ConfirmDialog`: destructive/primary, отмена первой в фокусе, pending/error и возврат фокуса; сценарий владеет текстами и запросом | [Подтверждение действия](ui-catalog.md#confirm-dialog) |
| Тумблер, режимы, тема, только иконки | `ModeSwitchPanel`; `fullWidth` для всей доступной ширины, `iconOnly` для иконок | [Панели](ui-catalog.md#panels), [пример 3](ui-catalog-examples.md#example-3) |
| Раскрывающиеся вопросы и ответы | Общий `FAQ`, `items`; `name` для одного открытого ответа | [FAQ](ui-catalog.md#faq) |
| Кнопка или счётчик внутри поля | `InputControlChip` button/div | [Ввод](ui-catalog.md#input), [радиус и размеры](ui-catalog.md#tokens) |
| Выбор соотношения сторон или разрешения, в том числе в редакторе фото | `ImageAspectRatioSelector`, `ImageQualitySelector`; в модальном редакторе portalLayer=170 | [Ввод](ui-catalog.md#input), [панели](ui-catalog.md#panels) |
| Кнопка с заливкой / прозрачная кнопка с обводкой | `Button`: `variant="filled"` по умолчанию; `variant="outline"` для прозрачного фона во всех состояниях | [Ввод](ui-catalog.md#input) |
| Поле сообщения / запрос генерации / загрузка медиа | `ChatComposer`; в существующем сценарии — его адаптер | [Ввод](ui-catalog.md#input), [пример 5](ui-catalog-examples.md#example-5) |
| Музыкальная workspace-страница / Suno и Lyria | `MusicWorkspaceController` + `MusicWorkspace` + `MusicStudioTools`: быстрое создание, раскрываемые настройки; `MusicTrackCard` для результатов и выбора источника. «Мои треки» открывает «Мои файлы → Музыка». Продолжение не запускает платную работу автоматически | [Модели](ui-catalog.md#models), [Ввод](ui-catalog.md#input), [загрузка](preloading.md) |
| Музыка в «Моих файлах» / компактный плеер и меню | `MusicFiles` + `MusicTrackCard`; общий кеш с музыкальным экраном | [Каталог](ui-catalog.md#выбор-готового-решения), [кеш](preloading.md) |
| Озвучка текста / распознавание записи | `SpeechWorkspace` в `/app/speech`; серверный prepare/activate; Whisper на DEV: MP3/WAV до 25 МБ и 8 минут, 25 кредитов за начатую минуту; pending-модели не запускаются | [Модели](ui-catalog.md#models), [Ввод](ui-catalog.md#input) |
| Проверка готовности генераций при обрыве связи | Общий `startGenerationPolling` в чате, фото, музыке, речи и повторных фото: пауза offline, одна проверка online, прежние интервалы и сохранённые результаты | [Общие состояния](ui-catalog.md#async-states), [загрузка](preloading.md#проверки-генераций-при-потере-сети) |
| Стрелка к концу диалога, скрытая внизу | `ChatScrollToBottom` в `ConversationComposer` | [Ввод](ui-catalog.md#input) |
| Перетаскивание файлов в правую панель | `WorkspaceFileDropZone` + `ChatComposer.attachmentController` | [Вложения](ui-catalog.md#раздельные-возможности-моделей) |
| Превью фото, удаление, прогресс и повтор загрузки | `ChatAttachmentPreview` внутри `ChatComposer`: `ModalCloseButton size="compact"`, `Button variant="outline"` и общий `Tooltip` | [Вложения](ui-catalog.md#раздельные-возможности-моделей) |
| Повторное прикрепление файла | `useChatAttachments`: сравнение содержимого; `ChatAttachmentNotice` на основе `ModalBackdrop` + `Button variant="outline"` | [Вложения](ui-catalog.md#раздельные-возможности-моделей) |
| Сетевой сбой загрузки фото, плашка сверху | `ChatUploadNetworkNotice` → `PopoverSurface` + `ModalCloseButton`; ширина по `InputSurface` | [Вложения](ui-catalog.md#раздельные-возможности-моделей) |
| Стоимость с вложениями и повтор расчёта | `useReferenceQuote` после готовности файлов; `ReferenceQuoteNotice` с `PopoverSurface` и `Button variant="outline"`; неизвестную цену не выводить | [Вложения](ui-catalog.md#раздельные-возможности-моделей) |
| Поверхность отдельного input/search | `InputSurface` + нативное поле | [Ввод](ui-catalog.md#input), [пример 4](ui-catalog-examples.md#example-4) |
| Ползунок, толщина кисти | `RangeSlider` | [Ввод](ui-catalog.md#input), [пример 4](ui-catalog-examples.md#example-4) |
| Привычная сетка фото, сохранить пропорции | `MasonryGrid`, непосредственные дети `li`; CSS-колонки | [Сетка и фото](ui-catalog.md#media), [пример 6](ui-catalog-examples.md#example-6) |
| Карточка/просмотр результата в «Моих файлах» | `FileCard` + `FilePreviewDialog`; готовая сетка — `FilesGrid`; приватные превью сохраняются в бэкенде, просмотр использует оригинал | [Сетка и фото](ui-catalog.md#media), [пример 7](ui-catalog-examples.md#example-7), [хранение превью](preloading.md) |
| Удаление созданного файла из библиотеки и диалога | `ArtifactDeleteButton` + `useArtifactDeletion` и account-scoped `WorkspaceDataCache`: подтверждение, DELETE, скрытие файла/превью и отметка в диалоге; байты очищает backend | [Сетка и фото](ui-catalog.md#media), [контракт удаления](../../../docs/DATA_RETENTION_POLICY.md#explicit-user-file-deletion) |
| Видеорезультаты в «Моих файлах» | `VideoFiles` → `MasonryGrid` + `MediaVideo`; первая страница и результаты из кеша аккаунта, проверка в фоне | [Сетка и фото](ui-catalog.md#media), [загрузка](preloading.md) |
| Выбрать готовое фото для сообщения | `ChatFilePicker` → `MasonryGrid` + `FileCard selectionAction`; вкладки — `ModeSwitchPanel` | [Сетка и фото](ui-catalog.md#media) |
| Фото в диалоге, просмотр только фото этого чата | `ConversationImageGallery` → общие FileCard/FilePreviewDialog | [Сетка и фото](ui-catalog.md#media) |
| Ожидание нескольких фото по количеству и формату запроса | `ImageGenerationPlaceholders` → `MediaState`; общий `ImageGenerationGrid` для заглушек и готовых фото | [Общие состояния](ui-catalog.md#async-states), [локальный пример](http://localhost:7159/ru/app/ui-states#image-generation) |
| Открыть прикреплённое фото из инпута или сообщения без правой панели | `AttachmentPreviewDialog` → `MediaPreviewDialogTemplate` без `infoPanel` | [Сетка и фото](ui-catalog.md#media) |
| Примеры генераций, карточка и галерея примеров | `InspirationExampleCard` + `InspirationExampleDialog`; только медиа — `InspirationExampleMedia` | [Сетка и фото](ui-catalog.md#media), [пример 6](ui-catalog-examples.md#example-6) |
| Сначала обложка видео, затем ролик в карточке | `MediaVideo poster loading="lazy"`; `InspirationExampleMedia` берёт posterPath примера | [Состояния медиа](ui-catalog.md#async-states) |
| Новый вид медиапросмотра | `MediaPreviewDialogTemplate<T>`, если готовая специализация не подходит | [Сетка и фото](ui-catalog.md#media) |
| Модальное окно / крестик закрытия | `ModalBackdrop` + `ModalCloseButton`; содержимое и фокус у потребителя | [Окна](ui-catalog.md#overlays) |
| Подсказка при наведении | `Tooltip`; при своём позиционировании — `TooltipBubble` | [Подсказки](ui-catalog.md#overlays), [пример 9](ui-catalog-examples.md#example-9) |
| Описание модели в подсказке только у инпута | `ModelSelector descriptionMode="tooltip"`; по умолчанию `inline` | [Модели](ui-catalog.md#models), [пример 8](ui-catalog-examples.md#example-8) |
| Общая прокрутка / scrollbar | `ScrollArea` | [Прокрутка](ui-catalog.md#overlays), [пример 9](ui-catalog-examples.md#example-9) |
| Выбор нейросети / карточка модели | `ModelSelector`, его адаптеры; `ModelCard` и `ModelIcon` | [Модели](ui-catalog.md#models), [пример 8](ui-catalog-examples.md#example-8) |
| Назначить иконку модели по ID | `model-icon-registry.ts` → `getModelPresentation` → существующий `ModelIcon`; назначены все модели текущего каталога, версии одной семьи используют общий знак модели или разработчика | [Модели](ui-catalog.md#models), [источники SVG](../public/assets/images/models/README.md) |
| Закрепить значок чата за первой нейросетью | `ConversationRow` → `ModelIcon` по серверному `first_model_id`; смена модели внутри чата значок не меняет | [Иконки](ui-catalog.md#icons-motion) |
| Цвет запасной иконки модели, белых кнопок и QR | Общие переменные в `globals.css`; `ModelIcon` окрашивает SVG через CSS-маску | [Токены](ui-catalog.md#tokens), [модели](ui-catalog.md#models) |
| Единые данные моделей, категорий, возможностей и цен | `GenerationCatalogProvider.initialSource` → общий `loadModelCatalog` → `/web/v1/models`; специализированные загрузчики — проекции, свежий серверный снимок используется без повторного запроса | [Модели](ui-catalog.md#models), [загрузка и кеш](preloading.md), [контракт бэкенда](../../../docs/runbooks/MODEL_CATALOG.md) |
| Лента подборок моделей и умный поиск | `ModelSelector categoryModelIds` → общий `ModeSwitchPanel`; без запроса до пяти в категории, поиск по всему переданному доступному каталогу без повторов: русский ввод, опечатки, неполное название и раскладка | [Модели](ui-catalog.md#models), [панели](ui-catalog.md#panels) |
| Модель слева от отправки в начатом диалоге | `ConversationModelSelector` → `ModelSelector variant="composer"`; общий generation-model-catalog и useGenerationControls | [Модели](ui-catalog.md#models) |
| Быстрые кнопки моделей под полем на главной, переключить без перехода | `WorkspaceHero` + `FeaturedModelShortcuts`; один WorkspacePrompt → ChatComposer, сменные ImageGenerationControls | [Модели](ui-catalog.md#models), [ввод](ui-catalog.md#input) |
| Ширина страницы / публичные блоки | `WorkspacePageFrame`; публичная часть — `PageContainer` и свои компоненты | [Оболочки](ui-catalog.md#models) |
| Купить пакет токенов / вернуться из ЮKassa | `TokenTopUpDialog`, `PaymentStatus`, `PaymentReturn`; серверный каталог и account-native платёж | [Окна](ui-catalog.md#overlays), [billing runbook](../../../docs/runbooks/BILLING.md#web-platform-test-checkout) |
| Цена или баланс со звездой | `CreditAmount` | [Ввод](ui-catalog.md#input) |
| Ожидание, ошибка, пустой список, предзагрузка фото | `LoadingIndicator`, `Skeleton`, `SkeletonGrid`, `StateNotice`, `MediaState`, `RetryAction`; байты медиа — `MediaImage` / `MediaVideo` | [Общие состояния](ui-catalog.md#async-states), [карта 20 мест](shared-async-states.md) |
| Общие цвета, обводка, скругления, анимация | Токены `globals.css`, готовые состояния и движения компонентов | [Токены](ui-catalog.md#tokens), [иконки и движение](ui-catalog.md#icons-motion) |
| Фон страницы или карточки отличается от общей палитры | Страница приложения и её футер — `--color-workspace`; нейтральные карточки и их состояния — `--color-panel`; левая панель — `--color-sidebar` с fallback на `--color-panel`. Проверить вложенные перекрытия | [Роли поверхностей](ui-catalog.md#tokens) |
| Иконка должна менять цвет вместе с текстом | Существующий SVG с `currentColor` или `AssetIcon` | [Иконки](ui-catalog.md#icons-motion) |
| Значок повтора загрузки | `RetryUploadIcon`; кнопка и подсказка задаются отдельно | [Иконки](ui-catalog.md#icons-motion) |
| Проверить загрузку фото и отправку сообщений локально без API | `LocalDevelopmentTools` в режиме `dev:preview`; общие переключатель, кнопки и popover | [Локальные сценарии](local-development.md) |
| Показать прикреплённые фото в сообщении | `ConversationInputImages`: чтение через API, ожидание и повтор | [Вложения](ui-catalog.md#chat-attachments) |
| Готового решения нет / похожие реализации расходятся | Сначала проверить ограничения и поискать в коде | [Ограничения](ui-catalog.md#limits) |

Важные границы: `itemVariant` действует на потомков с `control`; 0,5 rem не является радиусом всех компонентов; кнопки анимации/улучшения/удаления фона пока не запускают операции. Удаление файла — отдельное рабочее действие с подтверждением. Полные сигнатуры и исключения открывай только по нужной строке.

Поддержка: индекс хранит маршрут к решению, каталог — его контракт и ограничения, примеры — способ подключения. При изменении решения обновляй эти места по необходимости в той же правке; не превращай индекс в копию каталога или журнал работ. Архивный черновик и JSON не являются обычной точкой входа.
