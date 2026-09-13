# Каталог компонентов и оформления NeiroHub

Статус: проверенный по коду каталог подключения. Дата сверки: 12 сентября 2026 года, текущая рабочая копия `web/platform`, включая незакоммиченные изменения.

Каталог описывает существующие строительные блоки, их область применения и договорённости об оформлении. Публичные страницы, workspace и специализированные инструменты имеют разные контексты; рекомендации ниже учитывают эти различия. «Рекомендуется» означает пригодность компонента для указанной задачи, а не готовность всех серверных операций или проверку всех экранов в браузере.

Начинать с [короткого индекса](ui-index.md), затем читать нужный раздел. Сопутствующие материалы: [примеры подключения](ui-catalog-examples.md), [исходная инвентаризация](ui-inventory-draft.md), [полные сигнатуры и статические связи](ui-inventory-draft.json). Здесь собраны рекомендуемые элементы; в инвентаризации остаётся полный перечень из 139 экспортов, включая провайдеры и компоненты конкретных страниц.

## Выбор готового решения

| Задача | Рекомендуемое решение | Условие применения |
| --- | --- | --- |
| Панель у кнопки: действия, настройки | [PopoverPanel](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Нужны позиционирование, portal, закрытие снаружи и по Escape |
| Поверхность панели с уже готовым позиционированием | [PopoverSurface](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Потребитель уже управляет расположением и закрытием, как меню аккаунта или диалога |
| Действие: подсветить текст и иконку | Общий [стиль control + actionItems](../src/components/ui/selectable-control.module.css), вариант 1 | Для меню можно задать `itemVariant="action"`; потомкам нужен класс `control` |
| Выбор одного значения в панели | [PopoverOption](../src/components/ui/PopoverOption/PopoverOption.tsx), вариант 2 | Радиокнопка; требуется управлять `selected` и группой выбора |
| Переключатель режимов или темы | [ModeSwitchPanel](../src/components/ui/ModeSwitchPanel/ModeSwitchPanel.tsx) | Нужны выбранная кнопка с рамкой, клавиатура, при необходимости только иконки |
| Кнопка или группа внутри поля ввода | [InputControlChip](../src/components/ui/InputControlChip/InputControlChip.tsx) | `as="button"` для действия, `as="div"` для группы, например счётчика |
| Обычная кнопка с заливкой | [Button](../src/components/ui/Button/Button.tsx) | Её собственный вид с `surface-raised` подходит задаче; это не вариант действия без заливки |
| Поле сообщения / запроса генерации | [ChatComposer](../src/components/chat/ChatComposer/ChatComposer.tsx) | Нужна уже готовая сборка ввода, загрузки медиа и отправки |
| Оболочка отдельного поля поиска/ввода | [InputSurface](../src/components/ui/InputSurface/InputSurface.tsx) | Нужна общая поверхность; нативный input/textarea, подпись и внутреннюю раскладку задаёт потребитель |
| Числовой диапазон, например толщина кисти | [RangeSlider](../src/components/ui/RangeSlider/RangeSlider.tsx) | Управляемые min/max/value/onValueChange и доступная подпись |
| Сетка фото с сохранением пропорций | [MasonryGrid](../src/components/ui/MasonryGrid/MasonryGrid.tsx) | Непосредственные дети — `li`; карточка сама задаёт пропорции медиа |
| Карточка сгенерированного файла | [FileCard](../src/features/files/FileCard/FileCard.tsx) | Есть `ImageJob`, результат и обработчики его загрузки/повтора |
| Карточка статического примера | [InspirationExampleCard](../src/features/inspiration/InspirationExampleCard/InspirationExampleCard.tsx) | Данные имеют тип `InspirationExample`, а не состояние пользовательской задачи |
| Просмотр файла с инструментами | [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) | Передать только нужный набор `items` и выбранный индекс |
| Просмотр примеров | [InspirationExampleDialog](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx) | Передать набор `examples`; используется общий каркас медиапросмотра |
| Новый вид медиапросмотра | [MediaPreviewDialogTemplate](../src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx) | Готовые специализации не подходят данным; содержимое задаётся render-функциями |
| Своя модалка, не медиапросмотр | [ModalBackdrop](../src/components/ui/ModalBackdrop/ModalBackdrop.tsx) + [ModalCloseButton](../src/components/ui/ModalCloseButton/ModalCloseButton.tsx) | Потребитель реализует содержимое, имя диалога и управление фокусом |
| Подсказка короткой кнопки | [Tooltip](../src/components/ui/Tooltip/Tooltip.tsx) | Размещение сверху/снизу; для отдельного позиционирования есть `TooltipBubble` |
| Прокрутка в готовом оформлении | [ScrollArea](../src/components/ui/ScrollArea/ScrollArea.tsx) | У прокручиваемой области должна быть ограничена доступная высота/ширина |
| Выбор нейросети | [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx) | Для шапки и файлов сначала использовать существующие адаптеры, описанные ниже |
| Общая ширина внутренней страницы | [WorkspacePageFrame](../src/components/layout/WorkspacePageFrame/WorkspacePageFrame.tsx) | Уже применяется в файлах, вдохновении и каталоге моделей |
| Стоимость или баланс со звездой | [CreditAmount](../src/components/ui/CreditAmount/CreditAmount.tsx) | Только отображение переданного числа, без расчёта и списания |
| Иконка, меняющая цвет вместе с текстом | Имеющийся [SVG-компонент](../src/components/icons/MoreIcon/MoreIcon.tsx) либо [AssetIcon](../src/components/icons/AssetIcon/AssetIcon.tsx) | Проверить наследование `currentColor`; пути существующих ресурсов — в [assetPaths](../src/assets/asset-paths.ts) |

## Правила нашего оформления

<a id="states"></a>

### Варианты наведения и выбора

Источник поведения — [selectable-control.module.css](../src/components/ui/selectable-control.module.css). При подключении готового варианта размеры и раскладку можно задавать локально, а его цвет, фон и состояния не нужно повторять новым набором hover-правил.

| Состояние | Вариант 1: action | Вариант 2: selection |
| --- | --- | --- |
| Обычный невыбранный элемент | Приглушённый текст и иконка, фон прозрачный | То же |
| Наведение / фокус с клавиатуры | Светлые текст и иконка, фиолетовая обводка, без заливки | Фиолетовая обводка; невыбранный текст остаётся приглушённым |
| Выбранный элемент | Если у элемента есть состояние выбора — светлый текст и постоянная обводка | Светлый текст и постоянная обводка |
| Радиус общей кнопки | `var(--radius-sm)` = 0,5 rem | То же |

`PopoverPanel` и `PopoverSurface` по умолчанию используют `selection`. Для первого варианта передаётся `itemVariant="action"`. Это назначает группе класс `actionItems`; **каждая кнопка или ссылка внутри всё равно должна использовать `control`**. Само вложение произвольной кнопки в панель не подключает её состояния.

Состояние выбора определяется по `aria-checked`, `aria-pressed` или `aria-selected`. Для нативной радиокнопки внутри label.control используются `:has(> input[type="radio"]:checked)` и `:has(> input[type="radio"]:focus-visible)`; дублировать состояние выбора на label не требуется. Постоянная рамка выбранного элемента — состояние выбора, а не искусственно удерживаемый `:hover`.

В смешанной панели класс `actionItems` ставится только на группы действий. Переключатель режимов остаётся за пределами такой группы. Так устроен [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx): четыре действия подсвечиваются, тема использует выбор. Вложенный `itemVariant="selection"` сам по себе не отменяет внешний CSS-селектор `.actionItems .control`.

Отдельное согласованное поведение удаления диалога: обычный текст и иконка серые, при наведении/фокусе — `--color-danger`. Фон прозрачен, граница остаётся фиолетовой. Это локальное исключение для опасного действия в [ConversationRow.module.css](../src/features/conversations/ConversationRow/ConversationRow.module.css), а не новый вариант общей панели. В [FloatingConversationPanel](../src/features/conversations/ConversationRow/FloatingConversationPanel.tsx) включён `itemVariant="action"`.

<a id="tokens"></a>

### Поверхности, размеры и токены

Использовать существующие токены из [globals.css](../src/app/globals.css) для цветов, отступов, типографики и движения. Удобный ориентир — таблица; точный контекст светлой/тёмной темы остаётся в исходнике.

| Назначение | Токены / значения |
| --- | --- |
| Всплывающая поверхность | `--panel-surface-background` = `rgb(8 8 12 / 92%)`, `--panel-surface-border`, `--panel-surface-shadow`, `--panel-surface-text`, `--panel-surface-text-muted` |
| Цвета элементов | `--color-text`, `--color-text-muted`, `--color-accent`, `--color-danger`; фиолетовый акцент адаптируется к теме |
| Отступы | `--space-1` … `--space-8` = 0,25 … 2 rem с шагом 0,25 rem |
| Радиусы | sm 0,5; md 0,75; lg 1; xl 1,25; 2xl 1,5 rem; pill 999 px |
| Типографика | `--font-size-*`, `--line-height-*`, `--font-weight-*`; выбирать роль текста, а не случайный размер |
| Ширина workspace | `--workspace-page-shell-width` = 66 rem; `--workspace-page-inline-gutter` управляет боковыми отступами |
| Движение | `--motion-fast` = `150ms ease`, `--motion-normal` = `220ms ease`; учитывать `prefers-reduced-motion` |
| Фокус ввода | `--input-focus-border-color`, `--input-focus-ring` |

**0,5 rem — радиус общих кнопок выбора и всплывающей поверхности, не всех компонентов приложения.** `InputSurface` имеет радиус 1 rem, рамка всей `ModeSwitchPanel` — 0,75 rem, её кнопки — 0,5 rem. У `ModalCloseButton` сейчас собственный радиус 0,875 rem.

У `InputControlChip` базовый радиус — 999 px. В [ChatComposer.module.css](../src/components/chat/ChatComposer/ChatComposer.module.css) он становится 0,5 rem через селектор `[data-ui="input-control-chip"]`. Переменной для радиуса у Chip сейчас нет. Размер, шрифт, gap и padding настраиваются через `--input-control-*`; компактная высота внутри `ChatComposer` — 2,5 rem.

`PopoverSurface` локально выставляет светлые токены текста на тёмной тонировке, включая светлую тему приложения. Для такой панели рекомендуется именно готовая поверхность, чтобы не потерять эти переопределения.

<a id="icons-motion"></a>

### Иконки, анимация и фокус

- Иконки действий должны наследовать цвет элемента. `AssetIcon` рисует маску через `currentColor`, её путь задаётся props `source` и `iconName`. Внутренний CSS-параметр — **`--asset-icon-source`**. Декоративная иконка скрыта от assistive technology; доступное имя задаётся кнопке.
- Цвет `img` с фиксированным белым SVG не меняется от `color` родителя. Для такого изображения нельзя обещать подсветку без проверки способа отрисовки.
- У `ModeSwitchPanel` уже есть движение индикатора, клавиатура и reduced motion; у `ModalBackdrop` — закрытие с анимацией. При подключении использовать этот жизненный цикл. Кнопка внутри модалки вызывает переданный `requestClose`, а не сразу размонтирует окно через внешний `onClose`.
- Анимация появления настройки кисти живёт в [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) и его CSS. Общего экспортируемого компонента раскрытия блока пока нет.
- Глобальный reset убирает `outline`, а фокус интерактивных элементов оформляется через `box-shadow`. Локальную декларацию outline нельзя оценивать без каскада globals.
- `Tooltip` отвечает за показ текста, но не добавляет автоматически `aria-describedby`. Кнопка с одной иконкой всё равно должна иметь `aria-label`. Для нового диалога отдельно проверить начальный фокус, Tab и возврат фокуса.

## Контракты рекомендуемых компонентов

<a id="panels"></a>

### Панели и переключатели

| Компонент | Основные параметры | Встроенное поведение и границы | Проверенный потребитель |
| --- | --- | --- | --- |
| `PopoverPanel` | Обязательные anchorRef/isOpen/onClose/label/width/children; align=start/end, role=dialog/menu, id, itemVariant, portalLayer | Позиция снизу при достаточном месте, иначе сверху/с ограничением окном; scroll/resize; внешнее нажатие; Escape с возвратом фокуса. При открытии фокусируется выбранный radio или первый доступный элемент; Tab за границей панели закрывает её и возвращает фокус к кнопке. Tab/стрелки внутри не передаются внешнему просмотрщику. При выборе закрытие задаёт потребитель. `role="menu"` не добавляет навигацию стрелками. Слой по умолчанию 50; portalLayer=170 поднимает панель над ModalBackdrop (160) | [ChatMediaMenu](../src/components/chat/ChatMediaMenu/ChatMediaMenu.tsx), [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) |
| `PopoverSurface` | Параметры ScrollArea и itemVariant | Поверхность и прокрутка; позиционирование, события закрытия и семантику задаёт потребитель | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx), [FloatingConversationPanel](../src/features/conversations/ConversationRow/FloatingConversationPanel.tsx) |
| `PopoverOption` | Обязательный selected + нативные параметры/ref button, кроме role/aria-checked | `role="radio"`, `aria-checked`, type=button. Группу, подпись и клавиатурный переход между радиокнопками сам не создаёт | [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx) |
| `ModeSwitchPanel<ID>` | items/activeID/onChange/ariaLabel; iconOnly=false; semantics=toolbar/tabs | Элемент items: id/label, необязательные icon/disabled/title/elementID/ariaControls. Стрелки/Home/End меняют выбор, disabled пропускаются. activeID должен указывать на доступный элемент; tabs-контент создаёт потребитель | [ModelCatalogToolbar](../src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx), [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |

У `ModeSwitchPanel` собственная обводка выбранной кнопки заменена движущимся индикатором через `--selectable-active-border-color: transparent`; не добавлять вторую рамку поверх него. `iconOnly` скрывает подпись визуально, сохраняя имя кнопки.

<a id="input"></a>

### Ввод и действия

| Компонент | Основные параметры | Когда использовать |
| --- | --- | --- |
| `InputSurface` | Нативные параметры div, className, children | Общая поверхность с реакцией на фокус вложенного input/textarea/contenteditable. Сам input и внутренние padding/font/background остаются у потребителя. В [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx) также оформляет статические примеры промпта: одна поверхность без внешней подложки и второй рамки |
| `InputControlChip` | Нативные параметры/ref соответствующего button или div; as=button/div | Кнопки медиа, формата, качества, шаблонов и группы счётчиков внутри общего ввода |
| `ImageAspectRatioSelector` | disabled/value/onChange; portalLayer | Готовый выбор из 10 соотношений сторон с иконками, InputControlChip и PopoverPanel. Применён в генераторе и [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) |
| `ImageQualitySelector` | disabled/label/options/value/onChange; portalLayer | Готовый выбор разрешения/качества; список кодов передаёт потребитель, подписи формирует imageQualityLabel. В FileEditorPanel: 1K/2K/4K для локального редактора Nano Banana Pro, начальное 2K; соотношение сторон — 9:16. Оба значения сохраняет контроллер при смене инструмента, сбрасывает при смене файла. Кнопка запуска редактирования пока не подключена к операции |
| `ChatComposer` | variant=conversation/hero/newChat/workspace; value/onChange/onSend/canSubmit/disabled; label/placeholder/submitLabel/mediaLabel; wrapLeadingControls=false | Поле с медиа и отправкой. leadingControls/additionalControls/note расширяют сборку; onFilesSelected и обработчики медиа связывают её с задачей. wrapLeadingControls включает перенос настроек по доступной ширине, используется в ImageGenerationComposer. Компонент не выполняет бизнес-отправку сам |
| `ChatTextInput` | appearance/size/value/onChange/onSend/rows/disabled/placeholder | Нижний уровень текстового ввода. Для обычного поля сообщения сначала выбирать ChatComposer, чтобы не собирать его поведение повторно |
| `ChatSubmitButton` | disabled/label; type=submit/button/reset | Кнопка отправки с подсказкой. Используется внутри ChatComposer |
| `Button` | Нативные параметры/ref button | Вариант с заливкой. Для другого назначения явно задавать type, особенно при размещении в форме |
| `RangeSlider` | aria-label/min/max/value/onValueChange; step=1, disabled=false, className | Управляемый диапазон; подпись значения появляется кратковременно при его изменении |
| `CreditAmount` | value, необязательный prefix, нативные параметры span | Число со звездой: стоимость, баланс. Серверная логика остаётся у потребителя |

Для генерации уже есть адаптер [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx), для диалога — [ConversationComposer](../src/features/conversations/ConversationComposer/ConversationComposer.tsx), для стартового экрана — [WorkspacePrompt](../src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx). При изменении существующего сценария использовать его адаптер.

Для готового экрана генерации использовать [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx). Его общая цепочка prepare → подтверждение стоимости → activate → отслеживание → результат находится в [useImageGeneration](../src/features/image-generation/ImageGenerationPanel/useImageGeneration.ts); не повторять эти запросы в новом поле. Контроллер принимает model (из общего каталога; null — без выбранной модели), promptValue/onPromptChange, access и необязательные initialValues для параметров страницы. С явно заданным model, включая null, он не загружает каталог повторно и не заменяет выбор значением из шапки. reset очищает подготовку, результат и настройки при смене модели, сохраняя черновик; во время busy выбор заблокирован. access=guest отправляет на вход до prepare. Для GPT Image 2.5 цена определяется сервером на шаге prepare с учётом пропорций; перед запросом проверяется ограничение 4096 байт UTF-8. Эти ограничения действуют и на главной странице, и на отдельном экране генерации.

Для переключения функционала внутри существующего поля использовать [ImageGenerationControls](../src/features/image-generation/ImageGenerationComposer/ImageGenerationControls.tsx) в leadingControls: это только готовые кнопки шаблона, пропорций, разрешения и количества. Контроллер возвращает composerProps с их параметрами и обработчиками, включая modelID и allowedAspectRatios. Список пропорций берётся из модели; при единственном качестве выбор скрыт. Для Midjourney Imagine скрыт счётчик результатов. Составные качества GPT Image 2.5 отображаются через imageQualityLabel, в запросе сохраняются исходные коды. [ImageGenerationFeedback](../src/features/image-generation/ImageGenerationPanel/ImageGenerationFeedback.tsx) показывает подтверждение, ошибки, ожидание и результат отдельно от поля. WorkspacePrompt поддерживает promptValue/onPromptChange, leadingControls и submitAction={canSubmit, disabled, label, onSubmit}; без submitAction запускает обычный диалог. Для смены модели не заменять WorkspacePrompt/ChatComposer другим компонентом и не ставить на поле key по ID модели: должны сохраняться DOM-узел textarea, позиция курсора, ручное раскрытие и выбранное вложение.

<a id="media"></a>

### Сетка и фотографии

| Компонент | Данные / параметры | Граница повторного использования |
| --- | --- | --- |
| `MasonryGrid` | Нативные параметры ol; непосредственные дети li | CSS-колонки `column-width: 17rem`, gap `space-4`, `break-inside: avoid`. Это не построчная сетка с одинаковой высотой ячеек; число колонок зависит от ширины контейнера |
| `FileCard` | job: ImageJob; result: ImageJobResult/null; resultState=idle/loading/error; isRetrying; onOpenPreview/onRequestResult/onRetryJob; showDeleteControl=true | Сам запрашивает результат через callback при появлении успешной задачи в видимой области. Его удаление сейчас disabled и помечено недоступным. showDeleteControl управляет только видимостью кнопки |
| `FilePreviewDialog` | items, selectedIndex, onSelect, onClose; returnFocusTo | Ready-элемент содержит job и artifact; другие состояния — loading/unavailable. Просмотр открывается для готового элемента; по закрытию может вернуть фокус переданному элементу |
| `InspirationExampleCard` | example: InspirationExample; onOpen?, priority=false | Без onOpen открывает просмотр одного примера. Для переключения нескольких фото нужен общий контроллер галереи и внешний onOpen |
| `InspirationExampleMedia` | example, className, priority, videoProps/videoRef | Только изображение/видео с исходными пропорциями; не создаёт карточку или просмотрщик |
| `InspirationExampleDialog` | examples, selectedIndex, onSelect, onClose | Готовая специализация просмотра примеров. Вызывающая галерея управляет возвратом фокуса и набором данных |
| `MediaPreviewDialogTemplate<T>` | items/selectedIndex/onSelect/onClose; getItemKey/getPreviewDimensions/getThumbnailLabel; renderPreview/renderThumbnail/infoPanel; обязательные подписи и testIdPrefix | Общий каркас, responsive-раскладка, миниатюры, клавиатура и переключение. Дополнительно getActions/renderPreviewFooter/isItemSelectable. Содержимое и инструменты обработки задаются специализацией |

Готовые сборки: [FilesGrid](../src/features/files/FilesGrid/FilesGrid.tsx) соединяет MasonryGrid с FileCard; [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx) — с карточками примеров и просмотрщиком; [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) использует ту же сетку в модальном выборе шаблона, без поиска. Его окно получает тонировку, обводку, тень и цвета текста из общих токенов `--panel-surface-*`, как PopoverSurface; размеры, скругление и прокрутку задаёт сам диалог, а затемнение страницы — ModalBackdrop.

В диалоге повторно используется **тот же FileCard и FilePreviewDialog** через [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx). Набор просмотра ограничен фото диалога, карточка скрывает удаление. Размер в истории ограничен `min(60dvh, 650px)` по высоте и `min(100%, 600px)` по ширине; пропорции сохраняются. Эти ограничения принадлежат [CSS диалоговой галереи](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.module.css), а не всем карточкам файлов.

Один и тот же артефакт доступен по общему пути приложения, поэтому не нужно создавать его копию для диалога. Сам факт повторного использования карточки **не гарантирует один сетевой запрос**: кеширование и загрузка зависят от браузера и API. Полный клиентский контракт — [README галереи](../src/features/conversations/ConversationImageGallery/README.md).

<a id="overlays"></a>

### Модальные окна, прокрутка и подсказки

- Окна тарифов [SubscriptionPlansDialog](../src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx) и пополнения баланса [TokenTopUpDialog](../src/components/layout/WorkspaceHeader/TokenTopUpDialog.tsx) используют общие `--panel-surface-*` для тонировки, обводки, тени и текста оболочки. Карточки тарифов сохраняют свои цвета темы; ModalBackdrop отвечает за затемнение страницы. Пакеты токенов используют label.control с нативным radio: без заливки и отдельного кружка, с общей обводкой выбора, наведения и клавиатурного фокуса. Локальное исключение для пакетов — количество, подпись и актуальная цена всегда светлые (`--panel-surface-text`), независимо от выбора; старая зачёркнутая цена остаётся менее заметной.
- `ModalBackdrop`: onClose и children обязательны; children может быть функцией с requestClose. closeOnBackdropClick/closeOnEscape по умолчанию true. Есть portal, блокировка прокрутки и анимация закрытия. Самостоятельной реализации полного focus trap в этой оболочке нет.
- `ModalCloseButton`: нативные параметры/ref button; общая визуальная кнопка с CSS-крестиком. Для доступного имени передать aria-label. Не следует считать все старые крестики уже переведёнными на неё.
- `ScrollArea`: orientation=vertical/horizontal, trackPlacement=inset/outside, viewportAs=div/aside/main/nav/pre/section/textarea/ul. Для класса и параметров прокручиваемого элемента есть viewportClassName/viewportProps; ref корня и viewportRef — разные ссылки. Источник — [ScrollArea.tsx](../src/components/ui/ScrollArea/ScrollArea.tsx).
- `Tooltip`: children/label, placement=top/bottom. `TooltipBubble` — только оболочка children/className/style с role=tooltip, её позицию определяет потребитель. Источник — [Tooltip.tsx](../src/components/ui/Tooltip/Tooltip.tsx).

<a id="models"></a>

### Модели, оболочки и публичная часть

- [WorkspaceHero](../src/features/workspace/WorkspaceHero/WorkspaceHero.tsx) связывает одно постоянное поле WorkspacePrompt → ChatComposer и [FeaturedModelShortcuts](../src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx). Параметры: access, allModelsLink, modelLinksClassName. Стартует с текстового режима NeiroHub Chat, затем выбирает одну из первых четырёх реальных моделей изображений из общего каталога. FeaturedModelShortcuts принимает selectedModelId/onSelect/disabled; null означает текстовый режим. Это кнопки с aria-pressed и фиолетовым индикатором, без перехода; «Все нейросети» остаётся ссылкой. При выборе модели меняются только ImageGenerationControls и обработчик отправки; оформление и ширина поля, текст, курсор и вложение сохраняются. Появление и исчезновение кнопок занимают 220 мс; при выходе ExitingModelControls сохраняет снимок настроек и позиции кнопок, отключает взаимодействие через inert и плавно уменьшает высоту ряда, включая перенос на несколько строк. Выбор новой модели отменяет незавершённый выход; при reduced-motion скрытие происходит сразу. Поле не пересоздаётся. Строка стоимости при переключении не добавляется; подтверждение стоимости появляется после отправки. Контроллер сбрасывает настройки под выбранную модель без пересоздания поля. Во время подготовки, подтверждения и выполнения генерации переключение заблокировано. Карточки «Популярных нейросетей» и каталог продолжают использовать навигационный ModelCard.

- [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx): models/selectedModelId/onSelect; variant=compact/panel/composer, status=loading/ready/failure, disabled=false, hideEmptySections=false, renderInPortal=false по умолчанию. Сам содержит поиск и собственную всплывающую логику; фокусирует поиск после размещения портальной панели. `composer` — компактная кнопка с названием и стрелкой, без иконки модели. `hideEmptySections` скрывает категории без доступных моделей и включён в диалоге. В шапке использовать [WorkspaceModelSelector](../src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx), в инструментах файла — [FileTaskModelSelector](../src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx) с task/selectedModelId/onSelect.
- В начатом диалоге использовать [ConversationModelSelector](../src/features/conversations/ConversationModelSelector/ConversationModelSelector.tsx), а не WorkspaceModelSelector с переходом в генератор. ConversationComposer передаёт его через modelSelector → ChatComposer.additionalControls непосредственно перед отправкой. Кнопка появляется после первой отправки/при загрузке существующей истории, с короткой анимацией и учётом reduced-motion. Пока идёт отправка/ответ, она отключена.
- `useConversationModelSelection` получает `/web/v1/chat-models`, проверяет публичные ID и хранит выбор отдельно для каждого диалога в sessionStorage; без хранилища выбор работает в текущем состоянии. Черновик и сообщения при выборе не меняются. Публичный model_id фиксируется в PendingTurn и повторно отправляется с тем же ключом идемпотентности. Go формирует каталог через общий textgeneration: NeiroHub Chat плюс включённые платные модели с действующим серверным тарифом. Записи содержат estimate_credits, max_prompt_bytes и max_output_tokens; цена показана в селекторе и под полем, лимит проверяется в байтах UTF-8. Отдельный TextModelSelector не используется. Доступность и окончательная стоимость проверяются на сервере. При сбое каталога селектор отключён, отправка без выбора использует серверную модель по умолчанию.
- [ModelCard](../src/features/models/ModelCard/ModelCard.tsx): variant=catalog/selector. У catalog параметры interactive/revealed; у selector обязательны selected/onActivate. Вариант selector использует общий стиль control (вариант 2): выбранная модель отмечена постоянной фирменной обводкой через aria-pressed, при наведении и фокусе другая строка получает такую же обводку. Заливки и отдельного квадратика справа нет; строка состоит из иконки и текста. [ModelIcon](../src/features/models/ModelIcon/ModelIcon.tsx) принимает src/className и имеет запасное изображение.
- [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx), [AppShell](../src/components/layout/AppShell/AppShell.tsx), [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx), [WorkspaceHeader](../src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx) собирают оболочку приложения. Для нового содержимого страницы обычно достаточно существующей оболочки и WorkspacePageFrame.
- В публичной части используются [PageContainer](../src/components/public/PageContainer/PageContainer.tsx), [SectionHeading](../src/components/public/SectionHeading/SectionHeading.tsx), [ContentCard](../src/components/public/ContentCard/ContentCard.tsx), [PrimaryButton](../src/components/public/PrimaryButton/PrimaryButton.tsx). PrimaryButton и SecondaryButton — ссылки с собственным pill-оформлением; не подменять ими кнопки действий workspace только из-за имени.
- [VideoPlayer](../src/components/media/VideoPlayer/VideoPlayer.tsx) принимает title/source/poster; применяется на WorkspaceLanding. Медиа примеров уже обслуживает InspirationExampleMedia, поэтому отдельно вставлять видеоплеер в каждую карточку не требуется.
- `*Provider`, SessionRefresh, WorkspaceNavigationMetrics и ConversationTitleSync — инфраструктура данных/сессии/навигации. Они остаются в инвентаризации, но не являются альтернативами визуальных элементов.

<a id="limits"></a>

## Что пока не считать унифицированным или завершённым

| Область | Проверенный факт | Решение при новой задаче |
| --- | --- | --- |
| Закрытие модалок | SubscriptionPlansDialog и TokenTopUpDialog имеют локальные CloseIcon; общий ModalCloseButton используется медиашаблоном и выбором шаблона | Для совместимого нового окна выбирать общий компонент; старые окна отдельно сравнить по размеру и расположению |
| Тема | AccountMenu использует ModeSwitchPanel; PublicThemeSwitcher — собственную группу с заливкой | Для workspace брать ModeSwitchPanel. Публичный вариант считать отдельным до решения об объединении |
| Вкладки | FileTypeTabs и ImageGenerationGuide используют подчёркивание; ModeSwitchPanel — рамку | Выбирать по нужному виду; эти варианты пока не сведены в один компонент |
| Поверхность ввода | InputSurface содержит literal-тонировку, совпадающую с токеном панели; часть старых полей имеет локальное оформление | Для совместимого нового поля использовать InputSurface. Замена всех старых полей в этом каталоге не выполнена |
| Обработка файла | FileAnimationPanel/FileEnhancementPanel/FileBackgroundRemovalPanel — интерфейс выбора модели; у общей кнопки в FileModelActionPanel нет onClick | Переиспользовать оболочку можно; запуск операции потребует отдельной реализации |
| Удаление файла | Кнопка FileCard disabled | Не обещать рабочее удаление при подключении карточки |
| Подсказка в светлой теме | TooltipBubble использует тёмный panel background и общий color-text, без локального переопределения как у PopoverSurface | Проверить контраст в конкретном месте и теме; общий дефект всех подсказок по этому факту не установлен |
| Раскрытие/FAQ | Есть нативный FAQ и локальные реализации в WorkspaceLanding/профиле; общего компонента анимированного раскрытия нет. Заливка карточек вопросов в WorkspaceLanding — `--color-panel` (#15161C в тёмной теме) | Сначала сравнить поведение нужного блока; не объявлять локальную реализацию общей |
| Экспорты без потребителей | EmptyState, FAQ, ModelPreviewCard, SecondaryButton, NewConversationButton, FilesToolbar, ImageJobHistory не имеют найденных статических потребителей вне тестов | Можно рассмотреть по исходнику; отсутствие импортов не является основанием для автоматического удаления |

## Границы проверки

Назначение, параметры, значения по умолчанию, варианты и CSS-поведение сверены с актуальными исходниками и потребителями. JSX-примеры проверяются компилятором TypeScript против типов проекта; CSS-примеры используют общие классы для состояний и локальные правила только для раскладки/оговорённого удаления. Это документация подключения, а не новая библиотека компонентов.

Каталог не меняет приложение и не заменяет проверку конкретного экрана после подключения: особенно размеров, тем, фокуса и загрузки данных. Исходная инвентаризация — вспомогательный снимок; рекомендации и исправленные уточнения находятся здесь. Точка входа для агента — [короткий индекс](ui-index.md), порядок работы и обновления записей — [локальный AGENTS.md](../AGENTS.md#ui-reuse-workflow).
