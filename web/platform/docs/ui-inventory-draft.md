# Черновая инвентаризация интерфейса NeiroHub

Status: archived
Do not use for current implementation.
See: [проверенный каталог](ui-catalog.md) и [примеры подключения](ui-catalog-examples.md).

Сохранён как исходный материал инвентаризации. Рекомендации и актуальные уточнения находятся в проверенном каталоге.

Статус: черновик по текущему коду, 12 сентября 2026 года. Это материал для следующего этапа разбора, а не утверждённый каталог или правило выбора компонентов. Исходники остаются источником истины.

Область: `web/platform/src`, включая текущие незакоммиченные изменения. Найдены 139 экспортируемых компонентов, 32 локальных JSX-компонента и 101 файл CSS. В CSS найдено 190 объявлений custom properties с 125 уникальными именами; это глобальные токены вместе с локальными настройками и переопределениями.

Полный снимок: [ui-inventory-draft.json](ui-inventory-draft.json). Для каждого экспорта сохранены файл и строка, сигнатура параметров, связанные локальные типы, стили и статические потребители. Ниже — описание основных строительных блоков и перечень всех найденных экспортов.

## Как получен и что означает этот черновик

- Проверены компоненты в `src/components` и `src/features`; потребители искались во всех 210 файлах TypeScript/TSX без тестов. Маршрутные `page`/`layout`, вложенные функции, хуки и серверные сущности не включены в число компонентов.
- «Потребитель» означает статический импорт или реэкспорт с разрешением локальных путей и barrel-файлов. Это не доказательство доступности элемента на работающем маршруте. Использования JSX внутри файла объявления записаны отдельно в `localUsages`.
- `types` содержит связанные объявления типов из того же файла. Внешние доменные типы не развёрнуты; полную сигнатуру следует смотреть в исходнике. Наличие параметров в типе не означает, что все они обязательны.
- CSS разобран по правилам, классам, состояниям и custom properties. Значения, которые JavaScript выставляет во время работы, например размеры ползунка или URL маски иконки, в подсчёт CSS-объявлений не входят.
- Описанные состояния подтверждены исходниками. Полная проверка всех экранов в браузере, тем оформления, клавиатуры и доступности в этот этап не входила.
- Похожие реализации отмечены как кандидаты для сравнения. Черновик не объявляет их устаревшими и не предлагает автоматически заменить или удалить их.

## Общие компоненты и их реальные границы

### Всплывающие панели, выбор и наведение

| Компонент / стиль | Параметры и варианты | Что уже делает | Где используется |
| --- | --- | --- | --- |
| [PopoverPanel](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | `anchorRef`, `isOpen`, `onClose`, `label`, `width`, `children`; `align` start/end, `role` dialog/menu, `itemVariant` selection/action | Portal, позиционирование относительно кнопки, сначала снизу при достаточном месте, иначе сверху/в пределах окна. Реагирует на scroll/resize. Закрывается по внешнему нажатию и Escape; Escape возвращает фокус кнопке | Загрузка медиа, соотношение сторон, разрешение |
| [PopoverSurface](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Параметры `ScrollArea`, `itemVariant="selection"` по умолчанию | Только общая поверхность и прокрутка: тонировка, обводка, тень, радиус 0,5 rem. Выставляет светлые локальные токены текста на тёмной поверхности | Меню аккаунта, меню диалога; внутри `PopoverPanel` |
| [PopoverOption](../src/components/ui/PopoverOption/PopoverOption.tsx) | Нативные параметры кнопки, обязательный `selected`; роль и состояние выбора задаёт сам | Кнопка `role="radio"`, `aria-checked`, общий стиль выбора | Соотношение сторон и разрешение |
| [selectable-control.module.css](../src/components/ui/selectable-control.module.css) | `.control`; дополнительный контейнер `.actionItems`; необязательный `--selectable-active-border-color` | Общее поведение обводки и цвета, без заливки. Это стиль, отдельного универсального компонента ActionButton здесь нет | `PopoverOption`, `ModeSwitchPanel`, меню и ссылка «Посмотреть больше» |
| [ModeSwitchPanel](../src/components/ui/ModeSwitchPanel/ModeSwitchPanel.tsx) | `items`, `activeID`, `onChange`, `ariaLabel`; `iconOnly=false`; `semantics="toolbar"` либо `tabs`. Элемент: id/label, необязательные icon/disabled/title/elementID/ariaControls | Горизонтальная прокрутка, выбор, стрелки/Home/End с пропуском disabled, движущаяся рамка по реальной ширине кнопки. Для tabs — соответствующие ARIA-атрибуты | Тема аккаунта, инструменты просмотра файла, категории каталога моделей |

Два существующих варианта поведения:

| Состояние | `selection` — вариант 2 | `action` / `.actionItems` — вариант 1 |
| --- | --- | --- |
| Обычная невыбранная кнопка | Приглушённый текст, прозрачный фон и граница | То же |
| Наведение или фокус | Фиолетовая граница, текст остаётся приглушённым | Фиолетовая граница, текст становится светлым |
| Выбранная кнопка | Светлый текст и постоянная граница | То же, если кнопка имеет состояние выбора |

Состояние выбора определяется через `aria-checked`, `aria-pressed` или `aria-selected`. В `ModeSwitchPanel` собственная граница выбранной кнопки прозрачна: её заменяет движущийся индикатор. Анимация индикатора — 520 ms, при reduced motion отключается.

Важная граница: `itemVariant` влияет на потомков с классом `.control`, а не на любые вложенные кнопки. У иконки цвет меняется вместе с текстом, только если её реализация использует `currentColor` или соответствующее наследование. Картинка с фиксированным белым SVG сама от изменения `color` не перекрасится. `.actionItems` использует CSS-селектор потомка, поэтому вложенные группы не создают независимую область вариантов автоматически.

Существующее описание панели: [PopoverPanel/README.md](../src/components/ui/PopoverPanel/README.md). Панель не является модальным окном и не обеспечивает сама по себе полный сценарий удержания фокуса.

### Кнопки и ввод

| Компонент | Параметры и варианты | Поведение / ограничения |
| --- | --- | --- |
| [Button](../src/components/ui/Button/Button.tsx) | Нативные параметры и ref кнопки | Радиус 0,5 rem; **есть заливка** `surface-raised`, свои hover/active/disabled. Это другой вид кнопки, чем `.control` без заливки |
| [InputSurface](../src/components/ui/InputSurface/InputSurface.tsx) | Параметры div и `className` | Общая оболочка поля: радиус 1 rem, граница и тёмная тонировка. Фокус вложенного input/textarea/contenteditable оформляется через `:has`. Используется в `ChatComposer`, `FileEditorPanel`, `ModelCatalogToolbar` |
| [InputControlChip](../src/components/ui/InputControlChip/InputControlChip.tsx) | `as="button"` по умолчанию или `as="div"`; соответствующие нативные параметры/ref | Кнопка либо группа внутри поля, `data-ui="input-control-chip"`. Обводка по hover/expanded. По умолчанию радиус 999 px; **ChatComposer переопределяет его до 0,5 rem**, а высоту до 2,5 rem. Компонент сам не открывает панель |
| [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) | `variant`: conversation/hero/newChat/workspace; value/onChange/onSend/canSubmit/disabled, подписи; дополнительные/ведущие контролы, примечание, обработчики медиа | Собирает `InputSurface`, `ChatTextInput`, `ChatMediaMenu`, `ChatSubmitButton`; используется в диалоге, генерации и стартовом поле. Специфические операции задаются потребителем |
| [ChatTextInput](../src/components/chat/ChatTextInput/ChatTextInput.tsx) | appearance/size, value/onChange/onSend, rows/disabled/placeholder | Автоматическая высота и ручное разворачивание; параметры вида определяются отдельными типами исходника. Максимум автоматического роста задаётся локальным `--chat-text-input-max-auto-rows: 9` |
| [ChatSubmitButton](../src/components/chat/ChatSubmitButton/ChatSubmitButton.tsx) | disabled/label; type submit/button/reset | Кнопка отправки с `Tooltip`, своим оформлением доступности действия и наведением; используется в `ChatComposer` |
| [RangeSlider](../src/components/ui/RangeSlider/RangeSlider.tsx) | min/max/value/onValueChange, aria-label; step=1, disabled=false, className | Нативный range, визуальная заливка и ползунок, кратковременный показ значения. Используется для толщины кисти в `FileEditorPanel` |
| [CreditAmount](../src/components/ui/CreditAmount/CreditAmount.tsx) | value, необязательный prefix, параметры span | Общий вывод числовой стоимости/баланса со звездой; сам не отвечает за расчёт или списание |

### Модальные окна, подсказки, прокрутка и сетка

| Компонент | Параметры и варианты | Что включено / что принадлежит потребителю |
| --- | --- | --- |
| [ModalBackdrop](../src/components/ui/ModalBackdrop/ModalBackdrop.tsx) | onClose; children либо функция с requestClose; closeOnBackdropClick=true, closeOnEscape=true, testId | Portal и фон, блокировка прокрутки, анимированное закрытие, Escape/клик по фону. Содержимое, имя диалога и полный сценарий управления фокусом не следует считать автоматически реализованными оболочкой |
| [ModalCloseButton](../src/components/ui/ModalCloseButton/ModalCloseButton.tsx) | Нативные параметры/ref button | Общая кнопка закрытия с CSS-крестиком. Размер 3 rem, радиус 0,875 rem, своя тонировка. Используется в шаблоне медиапросмотра и выборе шаблона |
| [Tooltip](../src/components/ui/Tooltip/Tooltip.tsx) | children/label, placement top/bottom | Обёртка с показом по hover/focus-within; используется у отправки. Автоматической связи `aria-describedby` в компоненте нет |
| [TooltipBubble](../src/components/ui/Tooltip/Tooltip.tsx) | children/className/style | Визуальная часть с `role="tooltip"`; позиционирование задаётся отдельно. Sidebar использует её напрямую |
| [ScrollArea](../src/components/ui/ScrollArea/ScrollArea.tsx) | orientation vertical/horizontal; trackPlacement inset/outside; viewportAs; viewportProps/viewportClassName/viewportRef; параметры корня | Общая прокрутка и собственный scrollbar, включая textarea. Ref корня и ref прокручиваемой области — разные параметры |
| [MasonryGrid](../src/components/ui/MasonryGrid/MasonryGrid.tsx) | Нативные параметры `ol`, className, children | CSS-колонки шириной 17 rem, интервал `space-4`, запрет разрыва непосредственных li. Пропорции фото определяет карточка. Используется в файлах, вдохновении, выборе шаблона и шести примерах генерации |
| [WorkspacePageFrame](../src/components/layout/WorkspacePageFrame/WorkspacePageFrame.tsx) | children, параметры из `WorkspacePageFrameProps` | Общая ширина/отступы страницы в workspace. Используется файлами, каталогом моделей и галереей вдохновения |

## Готовые составные блоки

| Блок | Параметры / варианты и места применения | Граница повторного использования |
| --- | --- | --- |
| [MediaPreviewDialogTemplate](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx) | Обобщённый тип элемента T; items/selectedIndex/onSelect/onClose; renderPreview/renderThumbnail/getItemKey/getPreviewDimensions/getThumbnailLabel; infoPanel и необязательные действия/нижняя панель | Общий каркас просмотра: модальное окно, основное изображение, правая панель, миниатюры и навигация. Для одного элемента переключение скрывается. Содержимое и редактор/масштабирование задаются специализацией |
| [MediaPreviewPromptHeader](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx) | title, подписи копирования, isCopied/onCopy, className | Общий заголовок запроса для просмотра файла и примера |
| [FileCard](../src/features/files/FileCard/FileCard.tsx) | job/result/resultState; onOpenPreview/onRetryJob/onRequestResult, isRetrying; showDeleteControl=true | Карточка результата с доменными состояниями задачи. Используется в «Моих файлах» и диалоге; в диалоге `showDeleteControl=false`. CSS-параметры позволяют ограничить размер медиа |
| [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) | items/selectedIndex/onSelect/onClose и параметры загрузки/повтора из типа; элементы ready/loading/unavailable | Специализация общего шаблона для файлов, с `ModeSwitchPanel` инструментов. Её используют файлы и диалог |
| [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx) | children/conversationID/hasMoreBefore/messages | Объединяет общую карточку и просмотрщик с набором изображений текущего диалога. Размер в истории: максимум `min(60dvh, 650px)`, ширина до 600 px и доступной ширины. Это ограничение контекста диалога, не глобальная настройка `FileCard` |
| [InspirationExampleMedia](../src/features/inspiration/InspirationExampleMedia/InspirationExampleMedia.tsx) | example, className, priority=false, videoProps/videoRef | Отрисовка изображения/видео примера с исходными пропорциями. Применяется карточкой, просмотром примера и выбором шаблона |
| [InspirationExampleCard](../src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx) | example, onOpen, priority=false | Карточка примера: при внешнем onOpen передаёт открытие галерее, иначе открывает собственный просмотр. Имеет своё затемнение фото и кнопку подробностей |
| [InspirationExampleDialog](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx) | Параметры галереи из `InspirationExampleDialogProps` | Другая специализация того же шаблона просмотра. Не требует `job`, в отличие от файлов |
| [FilesGrid](../src/features/files/FilesGrid/FilesGrid.tsx) | Набор файлов и обработчики из `FilesGridProps` | Адаптер `MasonryGrid` + `FileCard`, а не отдельная реализация раскладки |
| [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx) | Публичных props нет; получает контекст выбранной модели | Вкладки инструкции/примеров; шесть изображений через общую сетку и карточки примеров. Ссылка «Посмотреть больше» использует вариант action, без стрелки |
| [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) | Значение/обработчик выбора и остальные параметры из типа | `ModalBackdrop` + `ModalCloseButton` + `ScrollArea` + `MasonryGrid` + общий рендер примера. Поля поиска сейчас нет |
| [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx) | Параметры каталога/выбора из `ModelSelectorProps` | Общий выбор модели для верхней панели и инструментов файла. Собственный portal, поиск, позиционирование и анимация; это не обёртка над `PopoverPanel` |
| [ModelCard](../src/features/models/ModelCard/ModelCard.tsx) | variant catalog/selector. catalog: model, interactive/revealed; selector: model, selected/onActivate | Общая карточка модели в каталоге, селекторе и избранных моделях; параметры зависят от варианта |
| [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) | Параметры аккаунта/выхода из типа | `PopoverSurface`; четыре пункта используют `.actionItems`, тема — отдельный `ModeSwitchPanel` с иконками и поведением selection |

Подробный уже существующий контракт диалоговой галереи: [ConversationImageGallery/README.md](../src/features/conversations/ConversationImageGallery/README.md).

`FileEditorPanel` и `FileEditPreview` работают с отдельным контроллером редактора; кисть, лассо и масштабирование относятся к этому блоку. `FileAnimationPanel`, `FileEnhancementPanel`, `FileBackgroundRemovalPanel` оборачивают локальный `FileModelActionPanel`: интерфейс выбора модели есть, но у основной кнопки действия в нём сейчас нет `onClick`. Наличие панели не означает готовность соответствующей серверной операции.

Компоненты `*Provider`, синхронизация заголовка, восстановление сессии и метрики навигации включены в полный перечень отдельно от визуальных примитивов. Это инфраструктура данных/маршрутов, а не готовый стиль.

## Общие стили и токены

Источник глобальных значений: [globals.css](../src/app/globals.css). Полные объявления со значениями, строками и контекстами селекторов/медиазапросов находятся в `customProperties` JSON-снимка. Все 101 CSS-файл, их классы, потребители и селекторы состояний находятся в `styles`.

| Группа | Значения / существующая логика |
| --- | --- |
| Отступы | `--space-1` … `--space-8`: 0,25 / 0,5 / 0,75 / 1 / 1,25 / 1,5 / 1,75 / 2 rem |
| Скругления | sm 0,5; md 0,75; lg 1; xl 1,25; 2xl 1,5 rem; pill 999 px |
| Основные тёмные поверхности | background `#0c0c0f`, workspace `#111217`, panel `#15161c`, surface `#1a1b22`, raised `#20212a`; граница `#2a2b35` |
| Текст и акцент | В тёмной теме текст `#f5f5f7`, приглушённый `#9b9da8`, акцент `#9a7cf5`. В светлой — `#17171b`, `#6b6c76`, `#7563e6` |
| Всплывающая поверхность | `--panel-surface-background: rgb(8 8 12 / 92%)`; собственные светлые text/muted; обводка `0.0625rem solid rgb(255 255 255 / 12%)`; тень `0 0.5rem 1.5rem rgb(0 0 0 / 30%)` |
| Ширины | Контейнеры 48 / 66 / 76 rem; оболочка страницы workspace 66 rem; боковой отступ `clamp(var(--space-4), 4.5vw, 3.5rem)` |
| Типографика | Display 2,5/2,75 rem; section 2/2,375; subsection 1,5/2; body 1/1,5; navigation 0,9375/1,375; UI 0,875/1,25. Веса 400/500/600; есть мобильные переопределения |
| Движение | fast `150ms ease`, normal `220ms ease`; отдельные элементы имеют собственные длительности. В globals есть общий reduced-motion reset |
| Фокус | Общие `--input-focus-border-color` и `--input-focus-ring`; глобальный reset убирает outline, интерактивный фокус оформляется через box-shadow. Локальный outline не следует считать независимым от этого каскада |

Локальные настройки, которые уже участвуют в повторном использовании:

- `InputControlChip`: размеры, внутренние отступы, иконка и шрифт управляются локальными `--input-control-*`; радиус переопределяется отдельным CSS-правилом `ChatComposer`, переменной для него у Chip нет.
- `FileCard`: `--file-card-media-inline-size` и `--file-card-media-max-block-size` позволяют контексту диалога ограничить фото без изменения карточки в файлах.
- `ModeSwitchPanel`: `--selectable-active-border-color` отделяет собственную границу от анимированного индикатора.
- `ChatTextInput`: число автоматических строк, высота, свободное место для кнопок, длительность ручного разворачивания — локальные параметры поля.
- `ScrollArea`: часть геометрии вычисляется во время работы; это не набор статических дизайн-токенов.
- `AssetIcon`: URL маски задаётся через `--asset-icon-source`, цвет берётся из `currentColor`.

## Дубли, отдельные реализации и вопросы для следующего разбора

| Наблюдение по коду | Что нужно сравнить перед объединением |
| --- | --- |
| `Button`, `.control`, `InputControlChip`, публичные `PrimaryButton`/`SecondaryButton` имеют разные фон, форму и семантику; публичные «кнопки» — ссылки | Какие различия намеренные, какие стоит представить вариантами. Нельзя выбирать только по слову «кнопка» |
| `ModalCloseButton` используется в двух местах; `SubscriptionPlansDialog` и `TokenTopUpDialog` сохраняют локальные `CloseIcon` и стили кнопки | Размер, положение, контраст и обработка закрытия; возможен общий компонент после сравнения |
| Тема аккаунта использует `ModeSwitchPanel`, `PublicThemeSwitcher` имеет свои три кнопки и заливку активного варианта | Данные темы общие, визуальный контракт разный. Решить, должно ли оформление совпадать |
| `FileTypeTabs`, вкладки `ImageGenerationGuide` и `ModeSwitchPanel` реализованы отдельно | В первых двух подчёркивание, в последнем рамка. Сопоставить клавиатурную логику и визуальную задачу, а не механически заменить |
| `ModelSelector` имеет собственную всплывающую оболочку; другие селекторы используют `PopoverPanel` | Поиск, размер, анимация, размещение, фокус и закрытие могут требовать отдельной специализации |
| `InputSurface` содержит буквальное значение тёмной тонировки, совпадающее с токеном панели; некоторые поля поиска оформлены собственными CSS | Возможность общей основы, с учётом темы и контекста поля |
| Локальный `CreditStar` есть в `FileAnimationPanel` и `FileEditorPanel`, при этом существует `CreditAmount` | Общая иконка или общий вывод стоимости; подписи и форматирование могут отличаться |
| Есть `AssetIcon`, SVG-компоненты, локальные функции иконок и отдельные SVG-картинки | Что должно наследовать цвет текста, а что является самостоятельным изображением |
| `FileCard` и `InspirationExampleCard` похожи, но первая связана с задачами/состояниями результата, вторая — с примерами | Сохранять смысловые обёртки; отдельно рассматривать общую визуальную часть. Общий просмотрщик уже есть |
| `FAQ` использует нативные details/summary; в `WorkspaceLanding` и профиле свои FAQ-блоки | Отличия в данных, анимации и допустимом числе открытых ответов |

Без найденных статических потребителей вне тестов: `EmptyState`, `FAQ`, `ModelPreviewCard`, `SecondaryButton`, `NewConversationButton`, `FilesToolbar`, `ImageJobHistory`. Это кандидаты для проверки использования, а не список для удаления.

Отдельно требует проверки в браузере: `TooltipBubble` использует тёмную тонировку, но цвет текста берёт из общего токена, тогда как `PopoverSurface` переопределяет текст локально. По одному исходнику нельзя подтвердить видимость проблемы во всех темах и местах использования.

## Полный перечень экспортов

В таблицах ниже «параметры» — ссылка на фактический контракт по имени типа, а не пересказ всех полей. Точные объявления локальных типов и значения по умолчанию сохранены в JSON. Потребители указаны как файлы, включая маршрутные страницы; отсутствие найденного импорта отмечено явно.

<!-- generated-component-list -->

### components/chat

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [AssistantMessageContent](../src/components/chat/AssistantMessageContent/AssistantMessageContent.tsx) | Содержимое ответа ассистента; markdown и исключение артефактов, показанных отдельной галереей. | `Readonly<AssistantMessageContentProps>` | [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx) |
| [AssistantTypingIndicator](../src/components/chat/AssistantTypingIndicator/AssistantTypingIndicator.tsx) | Индикатор активности ассистента. | `AssistantTypingIndicatorProps` | [ChatScrollToBottom](../src/components/chat/ChatScrollToBottom/ChatScrollToBottom.tsx), [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx), [PendingConversationBootstrap](../src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.tsx) |
| [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) | Общее составное поле отправки сообщения с медиа и дополнительными контролами; четыре контекста оформления. | `ChatComposerProps` | [ConversationComposer](../src/features/conversations/ConversationComposer/ConversationComposer.tsx), [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx), [WorkspacePrompt](../src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx) |
| [ChatFilePicker](../src/components/chat/ChatFilePicker/ChatFilePicker.tsx) | Модальный выбор медиа из библиотеки для прикрепления к сообщению. | `Readonly<ChatFilePickerProps>` | [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) |
| [ChatMediaMenu](../src/components/chat/ChatMediaMenu/ChatMediaMenu.tsx) | Кнопка и меню загрузки файла / выбора из загруженных / выбора из сгенерированных; action-вариант панели. | `Readonly<ChatMediaMenuProps>` | [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) |
| [ChatScrollToBottom](../src/components/chat/ChatScrollToBottom/ChatScrollToBottom.tsx) | Переход к последним сообщениям и отображение активности. | `ChatScrollToBottomProps` | [ConversationComposer](../src/features/conversations/ConversationComposer/ConversationComposer.tsx) |
| [ChatSubmitButton](../src/components/chat/ChatSubmitButton/ChatSubmitButton.tsx) | Кнопка отправки с подсказкой и состоянием disabled. | `ChatSubmitButtonProps` | [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) |
| [ChatTextInput](../src/components/chat/ChatTextInput/ChatTextInput.tsx) | Текстовое поле сообщения с автоматической высотой и ручным разворачиванием. | `ChatTextInputProps` | [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) |

### components/icons

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [AssetIcon](../src/components/icons/AssetIcon/AssetIcon.tsx) | Маска из файла иконки на span; цвет наследует currentColor, URL задаётся параметром source. | `Readonly<AssetIconProps>` | [LogoutIcon](../src/components/icons/LogoutIcon/LogoutIcon.tsx), [MegaphoneIcon](../src/components/icons/MegaphoneIcon/MegaphoneIcon.tsx), [MonitorIcon](../src/components/icons/MonitorIcon/MonitorIcon.tsx), [MoonIcon](../src/components/icons/MoonIcon/MoonIcon.tsx), [ProfileIcon](../src/components/icons/ProfileIcon/ProfileIcon.tsx), [SunIcon](../src/components/icons/SunIcon/SunIcon.tsx), [SupportIcon](../src/components/icons/SupportIcon/SupportIcon.tsx) |
| [CheckIcon](../src/components/icons/CheckIcon/CheckIcon.tsx) | Иконка: галочка. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [ConversationMessageActions](../src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx) |
| [CopyIcon](../src/components/icons/CopyIcon/CopyIcon.tsx) | Иконка: копирование. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [ConversationMessageActions](../src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx) |
| [EditIcon](../src/components/icons/EditIcon/EditIcon.tsx) | Иконка: редактирование. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) |
| [FileIcon](../src/components/icons/FileIcon/FileIcon.tsx) | Иконка: файл. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) |
| [GridIcon](../src/components/icons/GridIcon/GridIcon.tsx) | Иконка: сетка. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) |
| [ImageIcon](../src/components/icons/ImageIcon/ImageIcon.tsx) | Иконка: изображение. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) |
| [LogoutIcon](../src/components/icons/LogoutIcon/LogoutIcon.tsx) | Иконка: выход. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<LogoutIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [MegaphoneIcon](../src/components/icons/MegaphoneIcon/MegaphoneIcon.tsx) | Иконка: новости. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<MegaphoneIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [MonitorIcon](../src/components/icons/MonitorIcon/MonitorIcon.tsx) | Иконка: системная тема. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<MonitorIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [MoonIcon](../src/components/icons/MoonIcon/MoonIcon.tsx) | Иконка: тёмная тема. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<MoonIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [MoreIcon](../src/components/icons/MoreIcon/MoreIcon.tsx) | Иконка: дополнительные действия. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<IconProps>` | [ConversationRow](../src/features/conversations/ConversationRow/ConversationRow.tsx) |
| [ProfileIcon](../src/components/icons/ProfileIcon/ProfileIcon.tsx) | Иконка: профиль. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<ProfileIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [SearchIcon](../src/components/icons/SearchIcon/SearchIcon.tsx) | Иконка: поиск. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<SearchIconProps>` | [ModelCatalogToolbar](../src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx), [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx) |
| [SunIcon](../src/components/icons/SunIcon/SunIcon.tsx) | Иконка: светлая тема. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<SunIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [SupportIcon](../src/components/icons/SupportIcon/SupportIcon.tsx) | Иконка: поддержка. Точный тип нативных параметров и способ отрисовки — в исходнике. | `Readonly<SupportIconProps>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |

### components/layout

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [AppShell](../src/components/layout/AppShell/AppShell.tsx) | Оболочка приложения с областями боковой панели, шапки и контента. | `AppShellProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) | Боковая навигация, состояние сворачивания и управление подсказками. | `SidebarProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) | Сборка авторизованного workspace и его провайдеров. | `WorkspaceFrameProps` | [app/app/layout](../src/app/app/layout.tsx) |
| [GuestWorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) | Сборка оболочки workspace для гостя. | `Readonly<{ children: ReactNode }>` | [app/app/layout](../src/app/app/layout.tsx) |
| [BalanceTopUpButton](../src/components/layout/WorkspaceHeader/BalanceTopUpButton.tsx) | Кнопка баланса с открытием окна пополнения. | `Readonly<BalanceTopUpButtonProps>` | [WorkspaceHeader](../src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx) |
| [SubscriptionPlansButton](../src/components/layout/WorkspaceHeader/SubscriptionPlansButton.tsx) | Кнопка открытия тарифов. | `SubscriptionPlansButtonProps` | [WorkspaceHeader](../src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx), [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |
| [SubscriptionPlansDialog](../src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx) | Окно тарифов с карточками планов; ModalBackdrop и локальная кнопка закрытия. | `SubscriptionPlansDialogProps` | [SubscriptionPlansButton](../src/components/layout/WorkspaceHeader/SubscriptionPlansButton.tsx) |
| [TokenTopUpDialog](../src/components/layout/WorkspaceHeader/TokenTopUpDialog.tsx) | Окно пакетов пополнения; ModalBackdrop и локальная кнопка закрытия. | `Readonly<TokenTopUpDialogProps>` | [BalanceTopUpButton](../src/components/layout/WorkspaceHeader/BalanceTopUpButton.tsx) |
| [WorkspaceHeader](../src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx) | Шапка workspace: выбор модели, баланс и тарифы. | `WorkspaceHeaderProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [WorkspacePageFrame](../src/components/layout/WorkspacePageFrame/WorkspacePageFrame.tsx) | Общая оболочка ширины и отступов внутренних страниц. | `Readonly<WorkspacePageFrameProps>` | [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx), [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx), [ModelsCatalog](../src/features/models/ModelsCatalog/ModelsCatalog.tsx) |

### components/media

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [MediaPreviewPromptHeader](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx) | Заголовок запроса и действие копирования в просмотрщике. | `Readonly<MediaPreviewPromptHeaderProps>` | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx), [InspirationExampleDialogTemplate](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx) |
| [MediaPreviewDialogTemplate](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx) | Общий обобщённый каркас медиапросмотра с выбранным элементом, миниатюрами и боковой панелью. | `Readonly<MediaPreviewDialogTemplateProps<T>>` | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx), [InspirationExampleDialogTemplate](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx) |
| [VideoPlayer](../src/components/media/VideoPlayer/VideoPlayer.tsx) | Отдельный видеоплеер; найденный потребитель — WorkspaceLanding. | `Readonly<VideoPlayerProps>` | [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |

### components/public

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [ContentCard](../src/components/public/ContentCard/ContentCard.tsx) | Контентная карточка публичной страницы. | `ContentCardProps` | [app/(public)/page](../src/app/(public)/page.tsx) |
| [EmptyState](../src/components/public/EmptyState/EmptyState.tsx) | Публичный блок пустого состояния; внешние потребители не найдены. | `EmptyStateProps` | Не найдены |
| [FAQ](../src/components/public/FAQ/FAQ.tsx) | Список вопросов с нативными details/summary; внешние потребители не найдены. | `FAQProps` | Не найдены |
| [ModelPreviewCard](../src/components/public/ModelPreviewCard/ModelPreviewCard.tsx) | Карточка предварительного представления модели в публичной части. | `ModelPreviewCardProps` | Не найдены |
| [PageContainer](../src/components/public/PageContainer/PageContainer.tsx) | Контейнер ширины публичных страниц. | `PageContainerProps` | [app/(public)/page](../src/app/(public)/page.tsx), [PublicFooter](../src/components/public/PublicFooter/PublicFooter.tsx), [PublicHeader](../src/components/public/PublicHeader/PublicHeader.tsx) |
| [PrimaryButton](../src/components/public/PrimaryButton/PrimaryButton.tsx) | Основная публичная ссылка, оформленная кнопкой; собственная заливка и pill-форма. | `PrimaryButtonProps` | [app/(public)/page](../src/app/(public)/page.tsx), [PublicHeader](../src/components/public/PublicHeader/PublicHeader.tsx) |
| [PublicFooter](../src/components/public/PublicFooter/PublicFooter.tsx) | Подвал публичной оболочки. | `PublicFooterProps` | [PublicShell](../src/components/public/PublicShell/PublicShell.tsx) |
| [PublicHeader](../src/components/public/PublicHeader/PublicHeader.tsx) | Шапка публичной оболочки, навигация и тема. | `PublicHeaderProps` | [PublicShell](../src/components/public/PublicShell/PublicShell.tsx) |
| [PublicShell](../src/components/public/PublicShell/PublicShell.tsx) | Сборка публичных страниц с шапкой и подвалом. | `PublicShellProps` | [app/(public)/layout](../src/app/(public)/layout.tsx) |
| [PublicThemeSwitcher](../src/components/public/PublicThemeSwitcher/PublicThemeSwitcher.tsx) | Отдельная публичная реализация переключения system/light/dark с заливкой активного элемента. | `PublicThemeSwitcherProps` | [PublicHeader](../src/components/public/PublicHeader/PublicHeader.tsx) |
| [SecondaryButton](../src/components/public/SecondaryButton/SecondaryButton.tsx) | Вторичная публичная ссылка-кнопка; отдельные pill-стили. | `SecondaryButtonProps` | Не найдены |
| [SectionHeading](../src/components/public/SectionHeading/SectionHeading.tsx) | Заголовок секции публичной страницы. | `SectionHeadingProps` | [app/(public)/page](../src/app/(public)/page.tsx) |

### components/ui

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [Button](../src/components/ui/Button/Button.tsx) | Базовая нативная кнопка с ref, заливкой и собственными hover/active/disabled. | `ButtonProps` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx), [LoginForm](../src/features/auth/LoginForm/LoginForm.tsx), [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx), [NewConversationButton](../src/features/conversations/NewConversationButton/NewConversationButton.tsx), [PendingConversationBootstrap](../src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.tsx), [FileCard](../src/features/files/FileCard/FileCard.tsx), [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx), [ImageGenerationConfirmation](../src/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation.tsx), [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx), [ImageGenerationResult](../src/features/image-generation/ImageGenerationResult/ImageGenerationResult.tsx), [ImageJobHistory](../src/features/image-generation/ImageJobHistory/ImageJobHistory.tsx), [ImageJobTracker](../src/features/image-generation/ImageJobTracker/ImageJobTracker.tsx) |
| [CreditAmount](../src/components/ui/CreditAmount/CreditAmount.tsx) | Форматированное число кредитов со звездой и необязательным префиксом. | `Readonly<CreditAmountProps>` | [BalanceTopUpButton](../src/components/layout/WorkspaceHeader/BalanceTopUpButton.tsx), [ProfileBalanceCard](../src/features/account/ProfileBalanceCard/ProfileBalanceCard.tsx), [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx), [ImageGenerationConfirmation](../src/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation.tsx), [ImageJobHistory](../src/features/image-generation/ImageJobHistory/ImageJobHistory.tsx), [ModelCard](../src/features/models/ModelCard/ModelCard.tsx) |
| [InputControlChip](../src/components/ui/InputControlChip/InputControlChip.tsx) | Оболочка кнопки или группы контролов внутри поля; вид настраивается CSS-параметрами. | `Readonly<InputControlChipProps>` | [ChatMediaMenu](../src/components/chat/ChatMediaMenu/ChatMediaMenu.tsx), [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx), [ImageOutputCountSelector](../src/features/image-generation/ImageOutputCountSelector/ImageOutputCountSelector.tsx), [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx), [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) |
| [InputSurface](../src/components/ui/InputSurface/InputSurface.tsx) | Общая визуальная поверхность поля с реакцией на фокус вложенного ввода. | `InputSurfaceProps` | [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx), [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx), [ModelCatalogToolbar](../src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx) |
| [MasonryGrid](../src/components/ui/MasonryGrid/MasonryGrid.tsx) | Сетка на CSS-колонках, корень ol и непосредственные элементы li. | `Readonly<MasonryGridProps>` | [FilesGrid](../src/features/files/FilesGrid/FilesGrid.tsx), [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx), [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx), [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx) |
| [ModalBackdrop](../src/components/ui/ModalBackdrop/ModalBackdrop.tsx) | Фон и жизненный цикл модального окна: portal, scroll lock, Escape и анимированное закрытие. | `Readonly<ModalBackdropProps>` | [ChatFilePicker](../src/components/chat/ChatFilePicker/ChatFilePicker.tsx), [SubscriptionPlansDialog](../src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx), [TokenTopUpDialog](../src/components/layout/WorkspaceHeader/TokenTopUpDialog.tsx), [MediaPreviewDialogTemplate](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx), [ConversationDeleteDialog](../src/features/conversations/ConversationRow/ConversationDeleteDialog.tsx), [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) |
| [ModalCloseButton](../src/components/ui/ModalCloseButton/ModalCloseButton.tsx) | Общая кнопка закрытия с CSS-крестиком и ref. | `ModalCloseButtonProps` | [MediaPreviewDialogTemplate](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx), [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) |
| [ModeSwitchPanel](../src/components/ui/ModeSwitchPanel/ModeSwitchPanel.tsx) | Группа выбора с движущейся рамкой, клавиатурой, вариантами toolbar/tabs и iconOnly. | `Readonly<ModeSwitchPanelProps<ID>>` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx), [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx), [ModelCatalogToolbar](../src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx) |
| [PopoverOption](../src/components/ui/PopoverOption/PopoverOption.tsx) | Радиокнопка с общим стилем выбранного/невыбранного элемента. | `Readonly<PopoverOptionProps>` | [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx), [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) |
| [PopoverSurface](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Общая тонированная прокручиваемая поверхность всплывающего окна. | `PopoverSurfaceProps` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx), [FloatingConversationPanel](../src/features/conversations/ConversationRow/FloatingConversationPanel.tsx) |
| [PopoverPanel](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Позиционируемая относительно кнопки панель; варианты itemVariant selection/action. | `Readonly<PopoverPanelProps>` | [ChatMediaMenu](../src/components/chat/ChatMediaMenu/ChatMediaMenu.tsx), [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx), [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) |
| [RangeSlider](../src/components/ui/RangeSlider/RangeSlider.tsx) | Нативный range с оформлением ползунка и показом текущего значения. | `Readonly<RangeSliderProps>` | [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) |
| [ScrollArea](../src/components/ui/ScrollArea/ScrollArea.tsx) | Прокручиваемая область и scrollbar; vertical/horizontal, inset/outside и выбор элемента viewport. | `ScrollAreaProps` | [AssistantMessageContent](../src/components/chat/AssistantMessageContent/AssistantMessageContent.tsx), [ChatFilePicker](../src/components/chat/ChatFilePicker/ChatFilePicker.tsx), [ChatTextInput](../src/components/chat/ChatTextInput/ChatTextInput.tsx), [AppShell](../src/components/layout/AppShell/AppShell.tsx), [SubscriptionPlansDialog](../src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx), [MediaPreviewDialogTemplate](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx), [PublicHeader](../src/components/public/PublicHeader/PublicHeader.tsx), [ModalBackdrop](../src/components/ui/ModalBackdrop/ModalBackdrop.tsx), [ModeSwitchPanel](../src/components/ui/ModeSwitchPanel/ModeSwitchPanel.tsx), [PopoverPanel](../src/components/ui/PopoverPanel/PopoverPanel.tsx), [AccountUpdatesPanel](../src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.tsx), [ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx), [SidebarConversations](../src/features/conversations/SidebarConversations/SidebarConversations.tsx), [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx), [FileTypeTabs](../src/features/files/FileTypeTabs/FileTypeTabs.tsx), [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx), [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx), [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx) |
| [Tooltip](../src/components/ui/Tooltip/Tooltip.tsx) | Обёртка подсказки по наведению и фокусу; размещение top/bottom. | `Readonly<TooltipProps>` | [ChatSubmitButton](../src/components/chat/ChatSubmitButton/ChatSubmitButton.tsx) |
| [TooltipBubble](../src/components/ui/Tooltip/Tooltip.tsx) | Только визуальная часть подсказки; используется отдельно в Sidebar. | `Readonly<TooltipBubbleProps>` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) |

### features/account

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [AccountControl](../src/features/account/AccountControl/AccountControl.tsx) | Связь данных аккаунта с меню и выходом. | `AccountControlProps` | [app/app/layout](../src/app/app/layout.tsx) |
| [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) | Меню аккаунта на PopoverSurface; action-пункты и отдельный переключатель темы. | `AccountMenuProps` | [AccountControl](../src/features/account/AccountControl/AccountControl.tsx) |
| [AccountUpdatesPanel](../src/features/account/AccountUpdatesPanel/AccountUpdatesPanel.tsx) | Панель обновлений продукта с собственным содержимым и ScrollArea. | `AccountUpdatesPanelProps` | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) |
| [ProfileBalanceCard](../src/features/account/ProfileBalanceCard/ProfileBalanceCard.tsx) | Карточка баланса в профиле. | `ProfileBalanceCardProps` | [ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx) |
| [ProfileIdentityCard](../src/features/account/ProfileIdentityCard/ProfileIdentityCard.tsx) | Карточка данных пользователя в профиле. | `ProfileIdentityCardProps` | [ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx) |
| [ProfileLoginMethods](../src/features/account/ProfileLoginMethods/ProfileLoginMethods.tsx) | Блок способов входа в профиль. | `ProfileLoginMethodsProps` | [ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx) |
| [ProfileReferralFaq](../src/features/account/ProfileReferralFaq/ProfileReferralFaq.tsx) | Вопросы и ответы реферального раздела профиля. | Без параметров | [ProfileReferralProgram](../src/features/account/ProfileReferralProgram/ProfileReferralProgram.tsx) |
| [ProfileReferralProgram](../src/features/account/ProfileReferralProgram/ProfileReferralProgram.tsx) | Реферальный блок профиля, включая собственный FAQ. | Без параметров | [ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx) |
| [ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx) | Сборка страницы профиля из специализированных блоков. | Без параметров | [app/app/profile/page](../src/app/app/profile/page.tsx) |
| [WorkspaceAccountProvider](../src/features/account/WorkspaceAccount/WorkspaceAccount.tsx) | Провайдер состояния аккаунта, не визуальный примитив. | `WorkspaceAccountProviderProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |

### features/auth

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [LoginForm](../src/features/auth/LoginForm/LoginForm.tsx) | Форма входа с необязательным returnTo. | `Readonly<{ returnTo?: string }>` | [app/login/page](../src/app/login/page.tsx) |
| [WorkspaceLoginAction](../src/features/auth/WorkspaceLoginAction/WorkspaceLoginAction.tsx) | Действие входа в гостевой оболочке workspace. | `WorkspaceLoginActionProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |

### features/conversations

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [ConversationComposer](../src/features/conversations/ConversationComposer/ConversationComposer.tsx) | Адаптер общего ChatComposer к состоянию диалога. | `ConversationComposerProps` | [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) |
| [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) | Сборка истории сообщений, действий, изображений и поля ввода. | `ConversationHistoryProps` | [ConversationHistoryLoader](../src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.tsx) |
| [ConversationHistoryLoader](../src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.tsx) | Загрузка истории по conversationId и состояние первоначального обновления. | `{   conversationId: string;   initialRefresh?: boolean; }` | [app/app/chat/[conversationId]/page](../src/app/app/chat/[conversationId]/page.tsx) |
| [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx) | Контекст фотографий текущего диалога, общая FileCard и FilePreviewDialog. | `Readonly<{   children: ReactNode;   conversationID: string;   hasMoreBefore: boolean;   messages: readonly ConversationMessage[]; }>` | [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) |
| [ConversationAssistantMessage](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx) | Отрисовка ответа ассистента с учётом отдельной галереи изображений. | `Readonly<{ message: ConversationMessage }>` | [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) |
| [ConversationMessageActions](../src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx) | Действия под сообщением: копирование, оценка и повторное создание по условиям контекста. | `Readonly<ConversationMessageActionsProps>` | [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) |
| [ConversationDeleteDialog](../src/features/conversations/ConversationRow/ConversationDeleteDialog.tsx) | Подтверждение удаления диалога на ModalBackdrop. | `ConversationDeleteDialogProps` | [ConversationRow](../src/features/conversations/ConversationRow/ConversationRow.tsx) |
| [ConversationRow](../src/features/conversations/ConversationRow/ConversationRow.tsx) | Строка диалога в боковой панели и действия над ней. | `ConversationRowProps` | [SidebarConversations](../src/features/conversations/SidebarConversations/SidebarConversations.tsx) |
| [FloatingConversationPanel](../src/features/conversations/ConversationRow/FloatingConversationPanel.tsx) | Всплывающая панель действий диалога на PopoverSurface. | `FloatingConversationPanelProps` | [ConversationRow](../src/features/conversations/ConversationRow/ConversationRow.tsx) |
| [ConversationTitleSync](../src/features/conversations/ConversationTitleSync/ConversationTitleSync.tsx) | Синхронизация заголовка диалога; инфраструктурный компонент. | `ConversationTitleSyncProps` | [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) |
| [NewConversationButton](../src/features/conversations/NewConversationButton/NewConversationButton.tsx) | Действие создания нового диалога; внешние потребители не найдены. | Без параметров | Не найдены |
| [PendingConversationBootstrap](../src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.tsx) | Подготовка нового диалога и отображение переходного состояния. | `PendingConversationBootstrapProps` | [app/app/chat/[conversationId]/page](../src/app/app/chat/[conversationId]/page.tsx) |
| [SidebarConversations](../src/features/conversations/SidebarConversations/SidebarConversations.tsx) | Список диалогов боковой панели. | `SidebarConversationsProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [SidebarConversationsActivityProvider](../src/features/conversations/SidebarConversations/SidebarConversationsActivity.tsx) | Контекст активности списка диалогов, не визуальный примитив. | `SidebarConversationsActivityProviderProps` | [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) |
| [WorkspaceConversationListProvider](../src/features/conversations/WorkspaceConversationList/WorkspaceConversationList.tsx) | Контекст списка диалогов workspace. | `WorkspaceConversationListProviderProps` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |

### features/files

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [FileCard](../src/features/files/FileCard/FileCard.tsx) | Карточка результата задачи: медиа, загрузка, повтор, открытие; управление видимостью удаления. | `Readonly<FileCardProps>` | [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx), [FilesGrid](../src/features/files/FilesGrid/FilesGrid.tsx) |
| [FileAnimationPanel](../src/features/files/FilePreviewDialog/FileAnimationPanel.tsx) | Настройки анимации через локальный FileModelActionPanel; основная кнопка без onClick. | Без параметров | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |
| [FileEnhancementPanel](../src/features/files/FilePreviewDialog/FileAnimationPanel.tsx) | Настройки улучшения через локальный FileModelActionPanel; основная кнопка без onClick. | Без параметров | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |
| [FileBackgroundRemovalPanel](../src/features/files/FilePreviewDialog/FileAnimationPanel.tsx) | Настройки удаления фона через локальный FileModelActionPanel; основная кнопка без onClick. | Без параметров | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |
| [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) | Панель редактора, выбора инструмента и толщины кисти с внешним controller. | `Readonly<{ controller: FileEditorController }>` | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |
| [FileEditPreview](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) | Область предварительного просмотра и взаимодействия с редактором изображения. | `Readonly<FileEditPreviewProps>` | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |
| [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) | Специализация общего просмотрщика для файлов и инструментов их обработки. | `Readonly<FilePreviewDialogProps>` | [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx), [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx) |
| [FileTaskModelSelector](../src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx) | Адаптер общего ModelSelector к задаче обработки файла. | `Readonly<FileTaskModelSelectorProps>` | [FileAnimationPanel](../src/features/files/FilePreviewDialog/FileAnimationPanel.tsx), [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) |
| [FilesEmptyState](../src/features/files/FilesEmptyState/FilesEmptyState.tsx) | Пустое состояние раздела файлов. | `Readonly<FilesEmptyStateProps>` | [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx) |
| [FilesGrid](../src/features/files/FilesGrid/FilesGrid.tsx) | Адаптер MasonryGrid и FileCard для набора результатов. | `Readonly<FilesGridProps>` | [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx) |
| [FilesToolbar](../src/features/files/FilesToolbar/FilesToolbar.tsx) | Панель поиска файлов; внешние потребители не найдены. | `Readonly<FilesToolbarProps>` | Не найдены |
| [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx) | Сборка страницы файлов с состояниями загрузки, фильтрацией и просмотром. | `Readonly<FilesWorkspaceProps>` | [app/app/files/page](../src/app/app/files/page.tsx) |
| [FileTypeTabs](../src/features/files/FileTypeTabs/FileTypeTabs.tsx) | Собственная группа вкладок типов файлов с подчёркиванием и клавиатурной навигацией. | `Readonly<FileTypeTabsProps>` | [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx) |

### features/image-generation

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx) | Соотношение сторон: InputControlChip, PopoverPanel и PopoverOption. | `Readonly<ImageAspectRatioSelectorProps>` | [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx) |
| [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx) | Адаптер ChatComposer к параметрам генерации изображений. | `Readonly<ImageGenerationComposerProps>` | [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx) |
| [ImageGenerationConfirmation](../src/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation.tsx) | Подтверждение параметров и стоимости генерации. | `Readonly<ImageGenerationConfirmationProps>` | [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx) |
| [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx) | Инструкция / шесть примеров; общая MasonryGrid, карточки примеров и action-ссылка. | Без параметров | [ImageWorkspace](../src/features/image-generation/ImageWorkspace/ImageWorkspace.tsx) |
| [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx) | Связь формы, подтверждения, состояния задачи и результата генерации. | `Readonly<ImageGenerationPanelProps>` | [ImageWorkspace](../src/features/image-generation/ImageWorkspace/ImageWorkspace.tsx) |
| [ImageGenerationResult](../src/features/image-generation/ImageGenerationResult/ImageGenerationResult.tsx) | Показ результата задачи генерации и действие создания ещё одного результата. | `Readonly<ImageGenerationResultProps>` | [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx) |
| [ImageJobHistory](../src/features/image-generation/ImageJobHistory/ImageJobHistory.tsx) | История задач генерации; внешние потребители не найдены. | `Readonly<ImageJobHistoryProps>` | Не найдены |
| [ImageJobTracker](../src/features/image-generation/ImageJobTracker/ImageJobTracker.tsx) | Показ и отслеживание состояния задачи генерации. | `Readonly<ImageJobTrackerProps>` | [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx) |
| [ImageOutputCountSelector](../src/features/image-generation/ImageOutputCountSelector/ImageOutputCountSelector.tsx) | Количество результатов: InputControlChip-группа с минусом и плюсом. | `Readonly<ImageOutputCountSelectorProps>` | [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx) |
| [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) | Разрешение: компактная панель выбора через общие Chip/Panel/Option. | `Readonly<ImageQualitySelectorProps>` | [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx) |
| [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) | Выбор шаблона: модальное окно, общая сетка и медиа примеров, без поля поиска. | `Readonly<ImageTemplatePickerProps>` | [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx) |
| [ImageWorkspace](../src/features/image-generation/ImageWorkspace/ImageWorkspace.tsx) | Сборка страницы генерации из рабочей панели и инструкции/примеров. | Без параметров | [app/app/image/page](../src/app/app/image/page.tsx) |

### features/inspiration

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [InspirationExampleCard](../src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx) | Карточка примера изображения/видео, собственный или внешний обработчик открытия. | `InspirationExampleCardProps` | [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx), [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx), [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |
| [InspirationExampleDialog](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx) | Специализация MediaPreviewDialogTemplate для примеров. | `Readonly<InspirationExampleDialogProps>` | [InspirationExampleCard](../src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx), [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx) |
| [InspirationExampleMedia](../src/features/inspiration/InspirationExampleMedia/InspirationExampleMedia.tsx) | Общая отрисовка медиа примера с исходными пропорциями. | `Readonly<InspirationExampleMediaProps>` | [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx), [InspirationExampleCard](../src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx), [InspirationExampleDialogTemplate](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx) |
| [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx) | Галерея примеров с MasonryGrid и просмотрщиком выбранного набора. | Без параметров | [WorkspaceHome](../src/features/workspace/WorkspaceHome/WorkspaceHome.tsx) |

### features/models

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [ModelCard](../src/features/models/ModelCard/ModelCard.tsx) | Карточка модели: варианты catalog и selector с разными контрактами параметров. | `Readonly<ModelCardProps>` | [ModelsCatalog](../src/features/models/ModelsCatalog/ModelsCatalog.tsx), [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx), [FeaturedModels](../src/features/workspace/FeaturedModels/FeaturedModels.tsx) |
| [ModelCatalogToolbar](../src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx) | Поиск и выбор категории моделей через InputSurface и ModeSwitchPanel. | `ModelCatalogToolbarProps` | [ModelsCatalog](../src/features/models/ModelsCatalog/ModelsCatalog.tsx) |
| [ModelIcon](../src/features/models/ModelIcon/ModelIcon.tsx) | Иконка модели с выбором доступного изображения/провайдера и запасным вариантом. | `Readonly<ModelIconProps>` | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx), [InspirationExampleCard](../src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx), [InspirationExampleDialogTemplate](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx), [ModelCard](../src/features/models/ModelCard/ModelCard.tsx), [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx), [FeaturedModelShortcuts](../src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx) |
| [ModelsCatalog](../src/features/models/ModelsCatalog/ModelsCatalog.tsx) | Сборка каталога моделей с фильтрами и карточками. | `ModelsCatalogProps` | [app/app/models/page](../src/app/app/models/page.tsx) |
| [WorkspaceModelSelectionProvider](../src/features/models/WorkspaceModelSelection/WorkspaceModelSelection.tsx) | Контекст выбранной модели workspace. | `Readonly<{ children: ReactNode }>` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx) | Общий селектор моделей с поиском, собственным portal и позиционированием. | `Readonly<ModelSelectorProps>` | [FileTaskModelSelector](../src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx), [WorkspaceModelSelector](../src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx) |
| [WorkspaceModelSelector](../src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx) | Адаптер ModelSelector к текущей модели workspace. | Без параметров | [WorkspaceHeader](../src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx) |

### features/session

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [SessionProgressBar](../src/features/session/SessionProgressBar/SessionProgressBar.tsx) | Индикатор хода восстановления/загрузки сессии. | `SessionProgressBarProps` | [ConversationHistoryLoader](../src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.tsx), [SessionRestorationShell](../src/features/session/SessionRestorationShell/SessionRestorationShell.tsx) |
| [SessionRefresh](../src/features/session/SessionRefresh/SessionRefresh.tsx) | Запуск восстановления сессии; инфраструктурный компонент. | Без параметров | [app/app/layout](../src/app/app/layout.tsx) |
| [SessionRestorationShell](../src/features/session/SessionRestorationShell/SessionRestorationShell.tsx) | Оболочка отображения восстановления сессии. | `SessionRestorationShellProps` | [SessionRefresh](../src/features/session/SessionRefresh/SessionRefresh.tsx) |
| [WorkspaceLogoutBoundary](../src/features/session/WorkspaceLogout/WorkspaceLogoutBoundary.tsx) | Граница обработки выхода и соответствующего состояния workspace. | `WorkspaceLogoutBoundaryProps` | [app/app/layout](../src/app/app/layout.tsx) |

### features/workspace

| Экспорт / исходник | Назначение | Параметры | Статические потребители |
| --- | --- | --- | --- |
| [FeaturedModels](../src/features/workspace/FeaturedModels/FeaturedModels.tsx) | Блок избранных/представленных моделей на стартовой странице. | Без параметров | [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |
| [FeaturedModelShortcuts](../src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx) | Компактные действия выбора моделей на стартовой странице. | Без параметров | [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |
| [WorkspaceDataCacheProvider](../src/features/workspace/WorkspaceDataCache/WorkspaceDataCache.tsx) | Контекст кеша данных workspace, не визуальный примитив. | `{ children: ReactNode }` | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [WorkspaceHome](../src/features/workspace/WorkspaceHome/WorkspaceHome.tsx) | Сборка домашнего экрана и соответствующего содержимого workspace. | `WorkspaceHomeProps` | [app/app/chats/page](../src/app/app/chats/page.tsx), [app/app/inspiration/page](../src/app/app/inspiration/page.tsx), [app/app/layout](../src/app/app/layout.tsx), [app/app/page](../src/app/app/page.tsx) |
| [CapabilityLinks](../src/features/workspace/WorkspaceLanding/CapabilityLinks.tsx) | Ссылки на возможности продукта на стартовой странице. | Без параметров | [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |
| [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) | Стартовая композиция: возможности, модели, примеры и собственный FAQ. | `WorkspaceLandingProps` | [WorkspaceHome](../src/features/workspace/WorkspaceHome/WorkspaceHome.tsx) |
| [WorkspaceNavigationMetrics](../src/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics.tsx) | Инструментирование навигации workspace, не визуальный примитив. | Без параметров | [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) |
| [WorkspacePrompt](../src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx) | Адаптер ChatComposer к стартовому полю workspace. | `WorkspacePromptProps` | [WorkspaceHome](../src/features/workspace/WorkspaceHome/WorkspaceHome.tsx), [WorkspaceLanding](../src/features/workspace/WorkspaceLanding/WorkspaceLanding.tsx) |

## Локальные JSX-компоненты

Это функции верхнего уровня, не экспортируемые из файла. Вложенные функции в подсчёт не входят. Полные сигнатуры записаны в JSON.

| Файл | Локальные компоненты |
| --- | --- |
| [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx) | `BrandChip` |
| [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx) | `WorkspaceChrome` |
| [SubscriptionPlansDialog](../src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx) | `CloseIcon`, `PlanCard` |
| [TokenTopUpDialog](../src/components/layout/WorkspaceHeader/TokenTopUpDialog.tsx) | `CloseIcon`, `TokenPackageMark` |
| [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx) | `AccountIcon` |
| [ConversationHistory](../src/features/conversations/ConversationHistory/ConversationHistory.tsx) | `ConversationHistoryReady`, `PendingTurnItems`, `ConversationHistoryState` |
| [ConversationHistoryLoader](../src/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader.tsx) | `ConversationHistoryLoaderContent` |
| [ConversationMessageActions](../src/features/conversations/ConversationMessageActions/ConversationMessageActions.tsx) | `RecreateIcon`, `LikeIcon`, `DislikeIcon` |
| [ConversationRow](../src/features/conversations/ConversationRow/ConversationRow.tsx) | `ConversationRailIcon` |
| [FileCard](../src/features/files/FileCard/FileCard.tsx) | `RetrySpinner` |
| [FileAnimationPanel](../src/features/files/FilePreviewDialog/FileAnimationPanel.tsx) | `CreditStar`, `FileModelActionPanel` |
| [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) | `HistoryIcon`, `EraserIcon`, `CreditStar` |
| [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) | `ReadyFilePreviewDialog` |
| [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx) | `RatioIcon` |
| [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx) | `MediaIcon`, `AspectIcon`, `PromptPreview`, `ResultPreview` |
| [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) | `TuneIcon`, `ChevronIcon` |
| [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) | `TemplateIcon` |
| [ModelIcon](../src/features/models/ModelIcon/ModelIcon.tsx) | `DefaultModelArtwork` |
| [FeaturedModels](../src/features/workspace/FeaturedModels/FeaturedModels.tsx) | `CatalogActionContent` |
