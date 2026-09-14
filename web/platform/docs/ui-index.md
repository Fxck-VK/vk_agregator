# UI: короткий индекс для агента

Статус: активный. Входная точка для задач по интерфейсу `web/platform`.

Найди строку → прочитай указанный раздел → проверь исходник и потребителя → используй или расширь готовое решение. Порядок и обновление записей закреплены в [AGENTS.md](../AGENTS.md#ui-reuse-workflow). Повторно загружать уже прочитанный индекс в одной задаче не нужно, если связанные сведения не изменились.

| Запрос / задача | Готовое решение | Читать точечно |
| --- | --- | --- |
| Всплывающее окно у кнопки, открыть под/над | `PopoverPanel` | [Панели](ui-catalog.md#panels), [пример 1](ui-catalog-examples.md#example-1) |
| То же оформление окна; позиционирование уже есть | `PopoverSurface` | [Панели](ui-catalog.md#panels) |
| Плавное появление и закрытие всплывающей панели | `PopoverSurface` / `PopoverPanel`: анимация по умолчанию, состояние через isOpen; отключение animated=false | [Движение](ui-catalog.md#icons-motion), [пример подключения](../src/components/ui/PopoverPanel/README.md#анимация-панели) |
| «1-й вариант», подсветить текст/иконку при наведении, ссылка-действие | `itemVariant="action"` + `control`; вне панели — `actionItems` + `control` | [Состояния](ui-catalog.md#states), [пример 1](ui-catalog-examples.md#example-1) |
| «2-й вариант», выбранная кнопка белая с рамкой | `PopoverOption`, `itemVariant="selection"` | [Состояния](ui-catalog.md#states), [пример 2](ui-catalog-examples.md#example-2) |
| «Удалить чат»: серый обычно, красный при наведении | Общий action-стиль + локальное исключение `ConversationRow` | [Состояния](ui-catalog.md#states), [пример 1](ui-catalog-examples.md#example-1) |
| Тумблер, режимы, тема, только иконки | `ModeSwitchPanel`, при необходимости `iconOnly` | [Панели](ui-catalog.md#panels), [пример 3](ui-catalog-examples.md#example-3) |
| Кнопка или счётчик внутри поля | `InputControlChip` button/div | [Ввод](ui-catalog.md#input), [радиус и размеры](ui-catalog.md#tokens) |
| Выбор соотношения сторон или разрешения, в том числе в редакторе фото | `ImageAspectRatioSelector`, `ImageQualitySelector`; в модальном редакторе portalLayer=170 | [Ввод](ui-catalog.md#input), [панели](ui-catalog.md#panels) |
| Обычная кнопка с заливкой | `Button` | [Ввод](ui-catalog.md#input) |
| Поле сообщения / запрос генерации / загрузка медиа | `ChatComposer`; в существующем сценарии — его адаптер | [Ввод](ui-catalog.md#input), [пример 5](ui-catalog-examples.md#example-5) |
| Стрелка к концу диалога, скрытая внизу | `ChatScrollToBottom` в `ConversationComposer` | [Ввод](ui-catalog.md#input) |
| Поверхность отдельного input/search | `InputSurface` + нативное поле | [Ввод](ui-catalog.md#input), [пример 4](ui-catalog-examples.md#example-4) |
| Ползунок, толщина кисти | `RangeSlider` | [Ввод](ui-catalog.md#input), [пример 4](ui-catalog-examples.md#example-4) |
| Привычная сетка фото, сохранить пропорции | `MasonryGrid`, непосредственные дети `li`; CSS-колонки | [Сетка и фото](ui-catalog.md#media), [пример 6](ui-catalog-examples.md#example-6) |
| Карточка/просмотр результата в «Моих файлах» | `FileCard` + `FilePreviewDialog`; готовая сетка — `FilesGrid` | [Сетка и фото](ui-catalog.md#media), [пример 7](ui-catalog-examples.md#example-7) |
| Фото в диалоге, просмотр только фото этого чата | `ConversationImageGallery` → общие FileCard/FilePreviewDialog | [Сетка и фото](ui-catalog.md#media) |
| Примеры генераций, карточка и галерея примеров | `InspirationExampleCard` + `InspirationExampleDialog`; только медиа — `InspirationExampleMedia` | [Сетка и фото](ui-catalog.md#media), [пример 6](ui-catalog-examples.md#example-6) |
| Новый вид медиапросмотра | `MediaPreviewDialogTemplate<T>`, если готовая специализация не подходит | [Сетка и фото](ui-catalog.md#media) |
| Модальное окно / крестик закрытия | `ModalBackdrop` + `ModalCloseButton`; содержимое и фокус у потребителя | [Окна](ui-catalog.md#overlays) |
| Подсказка при наведении | `Tooltip`; при своём позиционировании — `TooltipBubble` | [Подсказки](ui-catalog.md#overlays), [пример 9](ui-catalog-examples.md#example-9) |
| Описание модели в подсказке только у инпута | `ModelSelector descriptionMode="tooltip"`; по умолчанию `inline` | [Модели](ui-catalog.md#models), [пример 8](ui-catalog-examples.md#example-8) |
| Общая прокрутка / scrollbar | `ScrollArea` | [Прокрутка](ui-catalog.md#overlays), [пример 9](ui-catalog-examples.md#example-9) |
| Возможности API и приложения отдельно | `ModelCapabilitiesDetails` внутри `ModelSelector`, данные общего каталога | [Модели](ui-catalog.md#models) |
| Выбор нейросети / карточка модели | `ModelSelector`, его адаптеры; `ModelCard` и `ModelIcon` | [Модели](ui-catalog.md#models), [пример 8](ui-catalog-examples.md#example-8) |
| Единые данные моделей, категорий, возможностей и цен | `loadModelCatalog` → `/web/v1/models`; специализированные загрузчики — проекции | [Модели](ui-catalog.md#models), [контракт бэкенда](../../../docs/runbooks/MODEL_CATALOG.md) |
| Лента подборок моделей, до пяти в категории | `ModelSelector categoryModelIds` → общий `ModeSwitchPanel`; выбор категории поднимает её блок, поиск охватывает все подборки | [Модели](ui-catalog.md#models), [панели](ui-catalog.md#panels) |
| Модель слева от отправки в начатом диалоге | `ConversationModelSelector` → `ModelSelector variant="composer"`; общий generation-model-catalog и useGenerationControls | [Модели](ui-catalog.md#models) |
| Быстрые кнопки моделей под полем на главной, переключить без перехода | `WorkspaceHero` + `FeaturedModelShortcuts`; один WorkspacePrompt → ChatComposer, сменные ImageGenerationControls | [Модели](ui-catalog.md#models), [ввод](ui-catalog.md#input) |
| Ширина страницы / публичные блоки | `WorkspacePageFrame`; публичная часть — `PageContainer` и свои компоненты | [Оболочки](ui-catalog.md#models) |
| Купить пакет токенов / вернуться из ЮKassa | `TokenTopUpDialog`, `PaymentStatus`, `PaymentReturn`; серверный каталог и account-native платёж | [Окна](ui-catalog.md#overlays), [billing runbook](../../../docs/runbooks/BILLING.md#web-platform-test-checkout) |
| Цена или баланс со звездой | `CreditAmount` | [Ввод](ui-catalog.md#input) |
| Общие цвета, обводка, скругления, анимация | Токены `globals.css`, готовые состояния и движения компонентов | [Токены](ui-catalog.md#tokens), [иконки и движение](ui-catalog.md#icons-motion) |
| Иконка должна менять цвет вместе с текстом | Существующий SVG с `currentColor` или `AssetIcon` | [Иконки](ui-catalog.md#icons-motion) |
| Готового решения нет / похожие реализации расходятся | Сначала проверить ограничения и поискать в коде | [Ограничения](ui-catalog.md#limits) |

Важные границы: `itemVariant` действует на потомков с `control`; 0,5 rem не является радиусом всех компонентов; удаление в `FileCard` пока disabled, а кнопки анимации/улучшения/удаления фона пока не запускают операции. Полные сигнатуры и исключения открывай только по нужной строке.

Поддержка: индекс хранит маршрут к решению, каталог — его контракт и ограничения, примеры — способ подключения. При изменении решения обновляй эти места по необходимости в той же правке; не превращай индекс в копию каталога или журнал работ. Архивный черновик и JSON не являются обычной точкой входа.
