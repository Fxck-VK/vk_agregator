# Каталог компонентов и оформления NeiroHub

Статус: проверенный по коду каталог подключения. Дата сверки: 12 сентября 2026 года, текущая рабочая копия `web/platform`, включая незакоммиченные изменения.

Каталог описывает существующие строительные блоки, их область применения и договорённости об оформлении. Публичные страницы, workspace и специализированные инструменты имеют разные контексты; рекомендации ниже учитывают эти различия. «Рекомендуется» означает пригодность компонента для указанной задачи, а не готовность всех серверных операций или проверку всех экранов в браузере.

Начинать с [короткого индекса](ui-index.md), затем читать нужный раздел. Сопутствующие материалы: [примеры подключения](ui-catalog-examples.md), [исходная инвентаризация](ui-inventory-draft.md), [полные сигнатуры и статические связи](ui-inventory-draft.json). Здесь собраны рекомендуемые элементы; в инвентаризации остаётся полный перечень из 139 экспортов, включая провайдеры и компоненты конкретных страниц.

## Выбор готового решения

Язык интерфейса задаёт общий `LocaleProvider`. Для текстов использовать `useDictionary` / `useMessages`, для смены языка — `LanguageSwitcher` на базе `ModeSwitchPanel`. Последний учитывает RTL в клавиатуре, прокрутке и движении рамки; `ScrollArea` нормализует горизонтальную прокрутку RTL. Серверные тексты, формы количества и правила добавления языка описаны в [локализации](localization.md).

Корневой `RouteLocaleProvider` берёт язык из URL. В UI использовать `@/i18n/Link` и `@/i18n/navigation`: ссылки получают текущий префикс автоматически, `usePathname` возвращает путь без языка для существующих сравнений. Не импортировать Next Link/router напрямую. [Контракт маршрутизации](locale-routing.md).

| Задача | Рекомендуемое решение | Условие применения |
| --- | --- | --- |
| Панель у кнопки: действия, настройки | [PopoverPanel](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Нужны позиционирование, portal, закрытие снаружи и по Escape |
| Поверхность панели с уже готовым позиционированием | [PopoverSurface](../src/components/ui/PopoverPanel/PopoverPanel.tsx) | Потребитель уже управляет расположением и закрытием, как меню аккаунта или диалога |
| Действие: подсветить текст и иконку | Общий [стиль control + actionItems](../src/components/ui/selectable-control.module.css), вариант 1 | Для меню можно задать `itemVariant="action"`; потомкам нужен класс `control` |
| Выбор одного значения в панели | [PopoverOption](../src/components/ui/PopoverOption/PopoverOption.tsx), вариант 2 | Радиокнопка; требуется управлять `selected` и группой выбора |
| Переключатель режимов или темы | [ModeSwitchPanel](../src/components/ui/ModeSwitchPanel/ModeSwitchPanel.tsx) | Нужны выбранная кнопка с рамкой, клавиатура, при необходимости только иконки |
| Кнопка или группа внутри поля ввода | [InputControlChip](../src/components/ui/InputControlChip/InputControlChip.tsx) | `as="button"` для действия, `as="div"` для группы, например счётчика |
| Кнопка с заливкой / прозрачная контурная кнопка | [Button](../src/components/ui/Button/Button.tsx) | `variant="filled"` по умолчанию; для согласованного прозрачного оформления явно указать `variant="outline"` |
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
| Фоны рабочего пространства | `--color-workspace` — правая панель, её область прокрутки, фон главной и её футера, экран недоступности приложения (тёмная тема: `#0d1117`); `--color-background` — внешний фон оболочки и публичных страниц. Вложенная страница приложения не должна перекрывать рабочий фон внешним. Цвета задаются в `globals.css` |
| Нейтральные карточки | `--color-panel`: тёмная тема `#151b23`, светлая `#fffdfa`. Общая роль для моделей, файлов, профиля, FAQ, тарифов, социальных сетей, генерации и блоков состояния. Постоянная обводка нейтральных карточек и их загрузочных состояний задаётся `--color-card-border` (`transparent`); ширина границы сохранена для стабильной геометрии. Левая панель использует `--color-sidebar` с fallback на `--color-panel`: в светлой теме она отдельно окрашена в `#f1eee8` |
| Внутренние поверхности и состояния | `--color-surface` / `--color-surface-raised` — вложенные элементы управления, отметки, ховеры и выделения; не запасная заливка обычной карточки. Медиа, поля ввода и всплывающие окна имеют собственные роли. При смене палитры проверять итоговые фоны и перекрытия в браузере, а не только вхождения старых HEX |
| Поиск в каталоге моделей | Заливка `ModelCatalogToolbar` использует `--control-group-background` с fallback на `--panel-surface-background`, как и расположенный рядом переключатель категорий. Цвет и прозрачность пары управляются общим токеном; фон остальных полей задаётся их собственной ролью |
| Всплывающая поверхность | `--panel-surface-background`, `--panel-surface-border`, `--panel-surface-shadow`, `--panel-surface-text`, `--panel-surface-text-muted`, `--panel-surface-color-scheme`. Тёмная тема сохраняет прежнюю тонировку `rgb(8 8 12 / 92%)`; светлая использует молочную тонировку `rgb(255 253 250 / 92%)` и графитовый текст |
| Цвета элементов | `--color-text` / `--color-text-muted`: тёмная тема `#f0f6fc` / `#9198a1`, светлая `#29262e` / `#635e69`. Placeholder в светлой теме — `--color-placeholder` (`#6b6570`). `--color-accent` и статусы сохраняют смысловые роли. Текст и фон всплывающих поверхностей переключаются вместе |
| Белые кнопки и QR | `--color-surface-light` / `--color-text-on-light` сохраняют контраст в обеих темах; QR использует `--color-qr-background` / `--color-qr-foreground`, по умолчанию связанные с этой парой. Локальные HEX для этих ролей не добавлять |
| Запасная иконка модели | `--color-model-icon`: прежний светлый силуэт в тёмной теме; `--color-model-icon-on-light` — спокойный серый `#9198a1` в светлой и system-light, включая наведение и выбор. SVG задаёт форму alpha-маски; цвет задаётся только в `globals.css` |
| Отступы | `--space-1` … `--space-8` = 0,25 … 2 rem с шагом 0,25 rem |
| Радиусы | sm 0,5; md 0,75; lg 1; xl 1,25; 2xl 1,5 rem; pill 999 px |
| Типографика | `--font-size-*`, `--line-height-*`, `--font-weight-*`; выбирать роль текста, а не случайный размер |
| Ширина workspace | `--workspace-page-shell-width` = 66 rem; `--workspace-page-inline-gutter` управляет боковыми отступами |
| Движение | `--motion-fast` = `150ms ease`, `--motion-normal` = `220ms ease`; учитывать `prefers-reduced-motion` |
| Фокус ввода | `--input-focus-border-color`, `--input-focus-ring` |

**0,5 rem — радиус общих кнопок выбора и всплывающей поверхности, не всех компонентов приложения.** `InputSurface` имеет радиус 1 rem, рамка всей `ModeSwitchPanel` — 0,75 rem, её кнопки — 0,5 rem. У `ModalCloseButton` радиус 0,875 rem по умолчанию и 0,5 rem при `size="compact"`.

Шесть ссылок «Дополнительные возможности» в `CapabilityLinks` на главной используют `--radius-sm` (0,5 rem).

У `InputControlChip` базовый радиус — 999 px. В [ChatComposer.module.css](../src/components/chat/ChatComposer/ChatComposer.module.css) он становится 0,5 rem через селектор `[data-ui="input-control-chip"]`. Переменной для радиуса у Chip сейчас нет. Размер, шрифт, gap и padding настраиваются через `--input-control-*`; компактная высота внутри `ChatComposer` — 2,5 rem.

В светлой теме рабочий фон — `#faf8f4`, внешняя область — `#edeae4`. Поля используют `--color-input-border`, а solid-кнопки — `--color-accent-hover`. Фирменный градиент сохранён; надпись на нём задаёт `--color-text-on-gradient`, градиентный текст — `--gradient-text`. Новые роли светлой темы имеют в потребителях fallback к прежним значениям тёмной темы.

`PopoverSurface` задаёт фон, текст и color-scheme из общих токенов текущей темы. Светлая тема использует молочные поверхности и графитовый текст; тёмная сохраняет прежнее оформление. Прозрачность задаётся фону, а не всему элементу: текст и иконки остаются непрозрачными. Плотность заливки и существующее размытие совпадают между темами по роли: поле ввода, панели и группы управления — 92%, выбор файлов и новости (`--floating-dialog-background`) — 96%, кнопка закрытия — 70%, подложка модального окна — 80%, мобильной боковой панели — 45%. Непрозрачный выбор модели использует отдельный `--floating-panel-background`. Контурные `action` / `selection` остаются прозрачными, включая hover и выбранное состояние. Лавандовая заливка `--color-selected-background` применяется только к ранее залитым элементам, например активному пункту боковой панели.

<a id="icons-motion"></a>

### Иконки, анимация и фокус

- Иконки действий должны наследовать цвет элемента. `AssetIcon` рисует маску через `currentColor`, её путь задаётся props `source` и `iconName`. Внутренний CSS-параметр — **`--asset-icon-source`**. Декоративная иконка скрыта от assistive technology; доступное имя задаётся кнопке.
- [RetryUploadIcon](../src/components/icons/RetryUploadIcon/RetryUploadIcon.tsx) — общий значок повтора загрузки на базе `AssetIcon`, использует утверждённый `restart-white.svg`. Импорт: `@/components/icons/RetryUploadIcon`. Размер по умолчанию — `1em`, цвет наследуется; `className` и `style` позволяют настроить размер и цвет. Кнопка, подсказка и обработчик нажатия задаются отдельно потребителем.
- Только в свёрнутой десктопной панели `Sidebar` четыре навигационные иконки при наведении и клавиатурном фокусе плавно окрашивают рисунок в `--color-accent`; у выбранного раздела этот цвет постоянный. Фон кнопок прозрачен, без рамки. У значков чатов прозрачная внешняя область получает фиолетовую обводку при наведении и фокусе; у выбранного чата она постоянная. Сам фиолетовый квадрат со звездой сохраняется. В раскрытой и мобильной панели наведение и фокус у пунктов меню и строк чатов дают тонкую фиолетовую обводку без заливки; текст и иконки используют `--color-text`. Обводка строки чата общая для названия и многоточия. Выбранный чат во всех режимах и выбранный пункт меню в раскрытой панели имеют постоянную фиолетовую обводку без заливки. Правила используют цвета текущей темы.
- Иконки тем (`MonitorIcon`, `SunIcon`, `MoonIcon`) используют SVG с центрированным рисунком; поля `viewBox` подобраны по видимому масштабу `ProfileIcon`. В области `1.25rem` рисунок занимает примерно 14,6px по большей стороне, как и иконка «Профиль» в меню аккаунта. Размер кнопок и наследование цвета остаются общими для `ModeSwitchPanel`.
- Название выбранного чата в `ConversationRow` сохраняет обычную толщину `--font-weight-regular`, как при наведении; выбор обозначается постоянной обводкой и основным цветом текста.
- Мобильный бургер `Sidebar` использует контурный `MenuIcon` размером 1,5 rem с `currentColor`. Кнопка 2,75 rem имеет прозрачный фон, скругление `--radius-sm` (0,5 rem) и не отбрасывает тень. При наведении и клавиатурном фокусе показывается тонкая обводка `--color-accent`. При открытии панели бургер скрывается; справа от логотипа появляется кнопка со стрелкой влево, как в раскрытой десктопной панели. Закрытие стрелкой или Escape возвращает бургер и фокус на него.
- Мобильная панель плавно выезжает и уезжает за 400 мс: при закрытии `visibility` переключается после завершения движения. Бургер находится ниже панели по слою и не перекрывает логотип во время её ухода. При `prefers-reduced-motion` панель скрывается сразу.
- Цвет `img` с фиксированным белым SVG не меняется от `color` родителя. Для такого изображения нельзя обещать подсветку без проверки способа отрисовки.
- У `ModeSwitchPanel` уже есть движение индикатора, клавиатура и reduced motion; у `ModalBackdrop` — закрытие с анимацией. При подключении использовать этот жизненный цикл. Кнопка внутри модалки вызывает переданный `requestClose`, а не сразу размонтирует окно через внешний `onClose`.
- Анимация появления настройки кисти живёт в [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) и его CSS. Общего экспортируемого компонента раскрытия блока пока нет.
- `PopoverSurface` по умолчанию анимирует появление и закрытие (`animated=true`, `isOpen=true`); для управляемого меню передавать isOpen и оставлять сам компонент смонтированным. `animated={false}` отключает переходы и ожидание, в том числе во время закрытия. Внешние motionState/usePopoverPresence не нужны: поверхность сама удерживает DOM, сразу выставляет inert/aria-hidden при закрытии, затем удаляет содержимое по собственному opacity transitionend (резервный таймер 300 мс). Переход opacity/clip-path — `--motion-normal` (220 мс), motionOrigin=top/bottom задаёт край раскрытия. Быстрое повторное открытие сохраняет DOM и отменяет таймер; reduced-motion отключает движение и задержку. Необязательный onAfterClose вызывается после завершённого закрытия; нативный onTransitionEnd не подменяет внутренний обработчик. AccountMenu, FloatingConversationPanel и PopoverPanel используют этот интерфейс; через PopoverPanel его получают ChatMediaMenu, ImageAspectRatioSelector и ImageQualitySelector. Оболочки отвечают за позицию, Escape, нажатие снаружи и фокус; PopoverPanel/FloatingConversationPanel измеряют скрытую поверхность до запуска появления и сбрасывают позицию через onAfterClose. [Пример](../src/components/ui/PopoverPanel/README.md#анимация-панели).
- Глобальный reset убирает `outline`, а фокус интерактивных элементов оформляется через `box-shadow`. Локальную декларацию outline нельзя оценивать без каскада globals.
- `Tooltip` отвечает за показ текста, но не добавляет автоматически `aria-describedby`. Кнопка с одной иконкой всё равно должна иметь `aria-label`. Для нового диалога отдельно проверить начальный фокус, Tab и возврат фокуса.

## Контракты рекомендуемых компонентов

<a id="panels"></a>

### Панели и переключатели

| Компонент | Основные параметры | Встроенное поведение и границы | Проверенный потребитель |
| --- | --- | --- | --- |
| `PopoverPanel` | Обязательные anchorRef/isOpen/onClose/label/width/children; align=start/end, role=dialog/menu, id, itemVariant, portalLayer, animated=true | Позиция снизу при достаточном месте, иначе сверху/с ограничением окном; scroll/resize; внешнее нажатие; Escape с возвратом фокуса. Появление и закрытие выполняет PopoverSurface; animated=false отключает оба перехода; управлять через isOpen, не размонтировать PopoverPanel сразу при закрытии. При открытии фокусируется выбранный radio или первый доступный элемент; Tab за границей панели закрывает её и возвращает фокус к кнопке. Tab/стрелки внутри не передаются внешнему просмотрщику. При выборе закрытие задаёт потребитель. `role="menu"` не добавляет навигацию стрелками. Слой по умолчанию 50; portalLayer=170 поднимает панель над ModalBackdrop (160) | [ChatMediaMenu](../src/components/chat/ChatMediaMenu/ChatMediaMenu.tsx), [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) |
| `PopoverSurface` | Параметры ScrollArea; isOpen=true, animated=true, itemVariant=selection, motionOrigin=top/bottom (top по умолчанию), onAfterClose? | Поверхность и прокрутка с анимацией по умолчанию; самостоятельно удерживает закрывающийся DOM и делает его inert/aria-hidden. Отключение — animated=false. Потребитель задаёт isOpen и сохраняет компонент смонтированным; позиционирование, события закрытия и семантика остаются у оболочки | [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx), [FloatingConversationPanel](../src/features/conversations/ConversationRow/FloatingConversationPanel.tsx) |
| `PopoverOption` | Обязательный selected + нативные параметры/ref button, кроме role/aria-checked | `role="radio"`, `aria-checked`, type=button. Группу, подпись и клавиатурный переход между радиокнопками сам не создаёт | [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx) |
| `ModeSwitchPanel<ID>` | items/activeID/onChange/ariaLabel; fullWidth=false; iconOnly=false; semantics=toolbar/tabs | Элемент items: id/label, необязательные icon/disabled/title/elementID/ariaControls. Стрелки/Home/End меняют выбор, disabled пропускаются. activeID должен указывать на доступный элемент; tabs-контент создаёт потребитель. Горизонтальная прокрутка через ScrollArea; активная вкладка видна после открытия, выбора и изменения ширины, ручная прокрутка сохраняется | [ModelCatalogToolbar](../src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx), [FilePreviewDialog](../src/features/files/FilePreviewDialog/FilePreviewDialog.tsx) |

У `ModeSwitchPanel` собственная обводка выбранной кнопки заменена движущимся индикатором через `--selectable-active-border-color: transparent`; не добавлять вторую рамку поверх него. `iconOnly` скрывает подпись визуально, сохраняя имя кнопки.

`fullWidth` растягивает панель на всю доступную ширину; без него она имеет ширину по содержимому. Режим включён в `ModelCatalogToolbar` и `FileTypeTabs`. Ширину задаёт сам компонент через `.root[data-full-width="true"]`, поэтому базовое `fit-content` не перебивает её при другом порядке загрузки CSS. Для полной ширины передавать `fullWidth`, не задавать конкурирующий `inline-size: 100%` в классе страницы. Размеры пунктов и мобильная прокрутка сохраняются.

[ProfileWorkspace](../src/features/account/ProfileWorkspace/ProfileWorkspace.tsx) использует тот же `ModeSwitchPanel` с `semantics="tabs"` для «Общее» / «Реферальная программа». Сохраняются ID вкладок, связь с единственной панелью и локализованные подписи. Оформление, движущаяся рамка, прокрутка и клавиши влево/вправо/Home/End принадлежат общему компоненту; локальное подчёркивание и дублирующая обработка клавиатуры удалены.

<a id="input"></a>

### Ввод и действия

| Компонент | Основные параметры | Когда использовать |
| --- | --- | --- |
| `InputSurface` | Нативные параметры/ref div, className, children | Общая поверхность с реакцией на фокус вложенного input/textarea/contenteditable. Сам input и внутренние padding/font/background остаются у потребителя. В [ImageGenerationGuide](../src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx) также оформляет статические примеры промпта: одна поверхность без внешней подложки и второй рамки |
| `InputControlChip` | Нативные параметры/ref соответствующего button или div; as=button/div | Кнопки медиа, формата, качества, шаблонов и группы счётчиков внутри общего ввода |
| `ImageAspectRatioSelector` | disabled/value/onChange; portalLayer | Выбор переданных options с иконками, InputControlChip и PopoverPanel. Для моделей options всегда берутся из каталога; общий набор иконок не задаёт возможности модели. Применён в генераторе и [FileEditorPanel](../src/features/files/FilePreviewDialog/FileEditorPanel.tsx) |
| `ImageQualitySelector` | disabled/label/options/value/onChange; portalLayer | Готовый выбор разрешения/качества; список кодов передаёт потребитель, подписи формирует imageQualityLabel. В FileEditorPanel модель, качества, пропорции и значения по умолчанию берутся из выбранной операции общего каталога. При отсутствии подходящей операции настройки и запуск недоступны. Оба значения сохраняет контроллер при смене инструмента, сбрасывает при смене файла. Кнопка запуска редактирования пока не подключена к операции |
| `ChatComposer` | variant=conversation/hero/newChat/workspace; value/onChange/onSend/canSubmit/disabled; label/placeholder/submitLabel/mediaLabel; attachmentController?; wrapLeadingControls=false | Поле с медиа и отправкой. leadingControls/additionalControls/note расширяют сборку; onFilesSelected и обработчики медиа связывают её с задачей. wrapLeadingControls включает перенос настроек по доступной ширине, используется в ImageGenerationComposer. Компонент не выполняет бизнес-отправку сам |
| `ChatTextInput` | appearance/size/value/onChange/onSend/rows/disabled/placeholder | Нижний уровень текстового ввода. Высота автоматически подстраивается под текст до девяти строк; при выходе из ручного раскрытия размер пересчитывается без влияния анимации, затем поле плавно сворачивается к нему. Текст и курсор сохраняются. Для обычного поля сообщения сначала выбирать ChatComposer, чтобы не собирать его поведение повторно |
| `ChatSubmitButton` | disabled/label; type=submit/button/reset | Кнопка отправки с подсказкой. Используется внутри ChatComposer |
| `ChatScrollToBottom` | contentVersion/forceScrollRequest/scrollContainer; isAwaitingResponse=false | Кнопка над ConversationComposer всегда скрыта у нижней границы с допуском 4 px, в том числе во время ожидания ответа. При прокрутке вверх показывает троеточие, пока нейросеть работает, и стрелку после ответа. Нажатие прокручивает к последнему сообщению; после достижения низа кнопка исчезает. Индикатор в самой ленте остаётся до ответа. Положение проверяется при scroll, resize, загрузке медиа и изменении размеров контейнера/содержимого через ResizeObserver. Эти проверки видимости сохраняют намерение автопрокрутки; новое сообщение следует вниз, если пользователь ранее был внизу |
| `Button` | Нативные параметры/ref button; `variant="filled" \| "outline"` | По умолчанию с заливкой. `outline`: прозрачный фон в покое, при наведении, фокусе и нажатии; тонкая нейтральная обводка, фиолетовая при hover/focus-visible; радиус 0.5rem, без смещения. Для другого назначения явно задавать type, особенно при размещении в форме |
| `RangeSlider` | aria-label/min/max/value/onValueChange; step=1, disabled=false, className | Управляемый диапазон; подпись значения появляется кратковременно при его изменении |
| `CreditAmount` | value, необязательный prefix, нативные параметры span | Число с фирменной звездой: стоимость, баланс, количество токенов в карточках тарифов. В тарифах звезда расположена перед числом через локальный `row-reverse`. Серверная логика остаётся у потребителя |

Все числовые суммы внутреннего баланса выводятся через `CreditAmount`: включая
цену текстового ответа (и нулевую цену на главной/в новом чате), пакеты пополнения,
подтверждение зачисления, Lite и кнопки обработки файлов. Для суммы внутри фразы
используется `RichMessage` со слотом `CreditAmount`; подпись для экранного диктора
получается через `getCreditAmountLabel`. Суммы в рублях и технический лимит
`max_output_tokens` сохраняют свои единицы.

Число в `CreditAmount` сохраняет базовую линию окружающего текста. Звезда размером
`1em` выравнивается по центру высоты прописных символов текущего шрифта (`1cap`),
а не высоты строки. Это правило действует и при расположении звезды перед числом.

Резервное описание платной текстовой модели передаётся в селектор как числовой
`responsePrice: {credits, maxOutputTokens?}`. `ModelSelectorOption` использует
один rich-текст для строки и подсказки, передавая его в необязательный
`ModelCard` selector-prop `descriptionContent`. Авторские описания из каталога
и их переводы имеют приоритет. Цена не сохраняется в локализованной строке каталога.

Стоимость генерации под полем `ConversationComposer` и `WorkspacePrompt` использует
`CreditAmount` с локализованным префиксом «Стоимость:»: число и фирменную звезду
вместо текстовой валюты. Неизвестная цена по-прежнему скрыта, источник расчёта не меняется.

Для генерации уже есть адаптер [ImageGenerationComposer](../src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx), для диалога — [ConversationComposer](../src/features/conversations/ConversationComposer/ConversationComposer.tsx), для стартового экрана — [WorkspacePrompt](../src/features/workspace/WorkspacePrompt/WorkspacePrompt.tsx). При изменении существующего сценария использовать его адаптер.

В начатом диалоге `ConversationComposer` использует ширину `--workspace-page-shell-width` и симметричные боковые отступы, как `ConversationHistory`. На средней ширине (48–82 rem) оба отступа равны `--space-4` (16px); на мобильном экране такое же значение задаёт `--workspace-page-inline-gutter`. На широком экране отступы берутся из адаптивного `--workspace-page-inline-gutter`. `ChatComposer` в варианте `conversation` заполняет этот контейнер без отдельного ограничения 58 rem: внешние края поля совпадают с границами сообщений.

Для готового экрана генерации использовать [ImageGenerationPanel](../src/features/image-generation/ImageGenerationPanel/ImageGenerationPanel.tsx). Его общая цепочка prepare → подтверждение стоимости → activate → отслеживание → результат находится в [useImageGeneration](../src/features/image-generation/ImageGenerationPanel/useImageGeneration.ts); не повторять эти запросы в новом поле. Контроллер принимает model (из общего каталога; null — без выбранной модели), promptValue/onPromptChange, access и необязательные initialValues для параметров страницы. С явно заданным model, включая null, он не загружает каталог повторно и не заменяет выбор значением из шапки. reset очищает подготовку, результат и настройки при смене модели, сохраняя черновик; во время busy выбор заблокирован. access=guest отправляет на вход до prepare. Для GPT Image 2.5 цена определяется сервером на шаге prepare с учётом пропорций; перед запросом проверяется ограничение 4096 байт UTF-8. Эти ограничения действуют и на главной странице, и на отдельном экране генерации.

Для переключения функционала внутри существующего поля использовать [ImageGenerationControls](../src/features/image-generation/ImageGenerationComposer/ImageGenerationControls.tsx) в leadingControls: это только готовые кнопки шаблона, пропорций, разрешения и количества. Контроллер возвращает composerProps с их параметрами и обработчиками, включая modelID и allowedAspectRatios. Список пропорций берётся из модели; при единственном качестве выбор скрыт. Для Midjourney Imagine скрыт счётчик результатов. Составные качества GPT Image 2.5 отображаются через imageQualityLabel, в запросе сохраняются исходные коды. [ImageGenerationFeedback](../src/features/image-generation/ImageGenerationPanel/ImageGenerationFeedback.tsx) показывает подтверждение, ошибки, ожидание и результат отдельно от поля. WorkspacePrompt поддерживает promptValue/onPromptChange, leadingControls и submitAction={canSubmit, disabled, label, onSubmit}; без submitAction запускает обычный диалог. Для смены модели не заменять WorkspacePrompt/ChatComposer другим компонентом и не ставить на поле key по ID модели: должны сохраняться DOM-узел textarea, позиция курсора, ручное раскрытие и выбранное вложение.

<a id="media"></a>

### Сетка и фотографии

| Компонент | Данные / параметры | Граница повторного использования |
| --- | --- | --- |
| `MasonryGrid` | Нативные параметры ol; непосредственные дети li | CSS-колонки `column-width: 17rem`, gap `space-4`, `break-inside: avoid`. Это не построчная сетка с одинаковой высотой ячеек; число колонок зависит от ширины контейнера |
| `FileCard` | job: ImageJob; result: ImageJobResult/null; resultState=idle/loading/error; isRetrying; onRequestResult/onRetryJob; onOpenPreview **или** selectionAction: `{label, onSelect(job, artifact)}`; showDeleteControl=true | Сам запрашивает результат через callback при появлении успешной задачи в видимой области. В режиме selectionAction нажатие на фото выбирает конкретный артефакт, подпись действия появляется поверх фото при hover/focus и постоянно на touch; скачать/удалить скрыты. В обычном режиме удаление пока disabled, showDeleteControl управляет его видимостью |
| `FilePreviewDialog` | items, selectedIndex, onSelect, onClose; returnFocusTo | Ready-элемент содержит job и artifact; другие состояния — loading/unavailable. Просмотр открывается для готового элемента; по закрытию может вернуть фокус переданному элементу |
| `InspirationExampleCard` | example: InspirationExample; onOpen?, priority=false | Без onOpen открывает просмотр одного примера. Для переключения нескольких фото нужен общий контроллер галереи и внешний onOpen |
| `InspirationExampleMedia` | example, className, priority, videoProps/videoRef | Только изображение/видео с исходными пропорциями; не создаёт карточку или просмотрщик |
| `InspirationExampleDialog` | examples, selectedIndex, onSelect, onClose | Готовая специализация просмотра примеров. Вызывающая галерея управляет возвратом фокуса и набором данных |
| `MediaPreviewDialogTemplate<T>` | items/selectedIndex/onSelect/onClose; getItemKey/getPreviewDimensions/getThumbnailLabel; renderPreview/renderThumbnail; необязательный infoPanel; обязательные подписи и testIdPrefix | Общий каркас, responsive-раскладка, миниатюры, клавиатура и переключение. Без infoPanel и действий изображение занимает освободившееся место; на узком экране крестик сверху, миниатюры снизу. Дополнительно getActions/renderPreviewFooter/isItemSelectable. Содержимое и инструменты обработки задаются специализацией |
| [AttachmentPreviewDialog](../src/components/media/AttachmentPreviewDialog/AttachmentPreviewDialog.tsx) | items: `{id, src, alt}[]`, selectedIndex/onSelect/onClose; returnFocusTo | Тонкая специализация того же MediaPreviewDialogTemplate для вложений. Без информации и инструментов генерации; изображение целиком, стрелки/миниатюры только при нескольких фото, общие Esc/крестик/фон. Владельцем URL остаётся вызывающий компонент |

Готовые сборки: [FilesGrid](../src/features/files/FilesGrid/FilesGrid.tsx) соединяет MasonryGrid с FileCard; [InspirationGallery](../src/features/inspiration/InspirationGallery/InspirationGallery.tsx) — с карточками примеров и просмотрщиком; [ImageTemplatePicker](../src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx) использует ту же сетку в модальном выборе шаблона, без поиска. Его окно получает тонировку, обводку, тень и цвета текста из общих токенов `--panel-surface-*`, как PopoverSurface; размеры, скругление и прокрутку задаёт сам диалог, а затемнение страницы — ModalBackdrop.

[ChatFilePicker](../src/components/chat/ChatFilePicker/ChatFilePicker.tsx) использует `MasonryGrid` и тот же `FileCard` в режиме `selectionAction` («Прикрепить»). Фото сохраняет пропорции, подпись и информация о модели остаются доступными для скринридера. Общие состояния загрузки/ошибки и повтор запроса превью принадлежат FileCard. Вкладки «Все / Сгенерированные / Загруженные» — `ModeSwitchPanel` с tabs-семантикой; крестик — `ModalCloseButton`, загрузка с компьютера — `Button variant="outline"`. Окно использует `--panel-surface-*`, на узком экране занимает экран, прокручивается список, кнопка загрузки остаётся снизу. Источник «Загруженные» пока показывает прежнее пустое состояние с загрузкой с компьютера; история загрузок и пагинация здесь не добавлены. Выбор возвращает существующий `ChatMediaAttachment`, API и правила допуска вложений не меняются.

В диалоге повторно используется **тот же FileCard и FilePreviewDialog** через [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx). Набор просмотра ограничен фото диалога, карточка скрывает удаление. Размер в истории ограничен `min(60dvh, 650px)` по высоте и `min(100%, 600px)` по ширине; пропорции сохраняются. Эти ограничения принадлежат [CSS диалоговой галереи](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.module.css), а не всем карточкам файлов.

Один и тот же артефакт доступен по общему пути приложения, поэтому не нужно создавать его копию для диалога. Сам факт повторного использования карточки **не гарантирует один сетевой запрос**: кеширование и загрузка зависят от браузера и API. Полный клиентский контракт — [README галереи](../src/features/conversations/ConversationImageGallery/README.md).

<a id="overlays"></a>

### Модальные окна, прокрутка и подсказки

- Кнопка «Подробнее о тарифах» расположена по центру под текстом условий. Использует общий с кнопкой промокода контурный стиль: прозрачный фон, нейтральная обводка, фиолетовая при наведении/фокусе, без смещения. У неё и у кнопки «Связаться с менеджером» радиус `--radius-sm` (0,5rem); остальное оформление кнопки менеджера сохранено.
- Контейнер `AccountMenu` использует слой 50: меню аккаунта и вложенное окно «Что нового?» находятся выше закреплённого инпута (`ConversationComposer`, слой 10) и хедера (20). Локальные слои меню (2) и окна обновлений (4) сохраняют порядок внутри контейнера. На мобильном экране контейнер остаётся внутри слоя боковой панели (30), модальные окна (160) остаются выше.
- Кнопка профиля `AccountMenu` использует скругление `--radius-sm` (0,5 rem) и сохраняет прозрачный фон при наведении, клавиатурном фокусе и открытом меню; выделение — тонкая обводка `--color-accent`. Правило общее для раскрытой и свёрнутой панели и обеих тем; фиолетовый аватар не меняется.
- Внутренние блоки всех шести тарифов используют общий `planSummary` с минимальной высотой 15rem; кнопки прижаты к низу через автоматический верхний отступ. Плашка «Популярное» расположена по центру верхней границы Ultima вне потока и не сдвигает внутренний блок относительно соседних карточек.
- В `SubscriptionPlansDialog` заголовок, кнопка промокода, карточки тарифов и нижний блок находятся внутри одного `ScrollArea`: всё содержимое прокручивается вместе, без закреплённой шапки. Отступы сетки и нижнего блока задаёт `plansContent`.
- Кнопка «Активировать промокод» в `SubscriptionPlansDialog` имеет прозрачный фон, тонкую нейтральную обводку и радиус `--radius-sm` (0,5rem). При наведении и клавиатурном фокусе обводка становится фиолетовой через `--input-focus-border-color`; заливка и положение кнопки не меняются. Она открывает [PromoCodeDialog](../src/components/layout/WorkspaceHeader/PromoCodeDialog.tsx) поверх тарифов. Подзаголовок под заголовком тарифов убран. Новое окно использует `ModalBackdrop`, `ModalCloseButton`, `InputSurface`, `Button` и общую тонировку `--panel-surface-background` (92% в обеих темах). Крестик расположен вне карточки, в правом верхнем углу экрана с отступом `--space-4`. Фокус сразу переходит в поле и удерживается внутри окна; крестик, Escape и фон закрывают только промокод, возвращая фокус на кнопку тарифов. Пока оно открыто, тарифы недоступны для взаимодействия. Пустое поле и пробелы отключают кнопку «Активировать». Серверная активация не подключена: отправка формы явно сообщает об этом и не меняет подписку или баланс.
- В `TokenTopUpDialog` правые края карточек и кнопки покупки совпадают. Окно центрировано по обеим осям и имеет высоту по содержимому, максимум 80dvh. Список пакетов не растягивается в свободное пространство; перед блоком покупки остаётся отступ 16–24 px. На низких экранах список сжимается и прокручивается, кнопка покупки остаётся видимой. Звёзды и текст центрируются единым блоком в каждой карточке; трек прокрутки находится в правом отступе окна. Общий `ModalCloseButton` закреплён в правом верхнем углу страницы поверх затемнения, вне панели с пакетами, но внутри того же доступного диалога.
- Окна тарифов [SubscriptionPlansDialog](../src/components/layout/WorkspaceHeader/SubscriptionPlansDialog.tsx) и пополнения баланса [TokenTopUpDialog](../src/components/layout/WorkspaceHeader/TokenTopUpDialog.tsx) используют общие `--panel-surface-*` для тонировки, обводки, тени и текста оболочки. Кнопки «Активировать подписку» используют скругление `--radius-sm` (0,5rem). Основная заливка всех карточек тарифов совпадает с левой панелью (`--color-sidebar` с fallback на `--color-panel`), верхний блок с ценой — с правой (`--color-workspace`). У популярного и командного тарифов нет дополнительных тонирующих заливок поверх этих цветов; фирменные метки, кнопки и обводки сохранены. ModalBackdrop отвечает за затемнение страницы. Пакеты токенов используют label.control с нативным radio: без заливки и отдельного кружка, с общей обводкой выбора, наведения и клавиатурного фокуса. Локальное исключение для пакетов — количество, подпись и актуальная цена всегда используют основной цвет поверхности (`--panel-surface-text`), независимо от выбора; зачёркнутые цены и скидки не выдумываются на клиенте.
- `TokenTopUpDialog` получает пакеты через `features/payments/payments.ts` из `/web/v1/payment-products`; сохраняет наши звёзды по размеру пакета и выбор карточек. Окно центрировано, ширина ограничена 28rem, высота — 80dvh на компьютере и телефоне; плотность карточек и отступы зависят от высоты экрана. На телефоне остаются боковые отступы по 12 px. Заголовок и кнопка покупки находятся вне прокручиваемого списка. Общий ScrollArea показывает скролл при нехватке места, сохраняя зазор между карточками и треком. Поля email нет: API получает только код пакета, контакт для чека сервер берёт из подтверждённого email аккаунта. Кнопка создаёт account-native платёж со стабильным ключом повтора и открывает проверенную ссылку ЮKassa. Без настроенного тестового магазина она недоступна. Локальный workspace preview показывает только образцы пакетов и блокирует оплату.
- Преимущества всех тарифов используют SVG из `assetPaths.icons.plans` через `AssetIcon`. У обычных тарифов каждому пункту явно назначена иконка изображений, видео, нейросетей или презентаций; у командного — лимитов, скорости, команды или поддержки. Общий `featureIcon` задаёт размер 1,5rem (24px), центрирование относительно полного текстового блока и `--color-text` текущей темы.
- `PaymentStatus` используется после возврата `/app/payment-return` и при повторном открытии незавершённой покупки; ожидает подтверждение Go API, ограничивает опрос двумя минутами и обновляет серверный баланс только после `succeeded`. Возврат по ссылке сам по себе токены не начисляет. Схемы пакетов, безопасные URL, запросы и хранение ID вынесены в `features/payments/payments.ts`.

- Иллюстрации пакетов в `TokenTopUpDialog` — прозрачные PNG из `assetPaths.images.credits.packages`, привязанные к данным пакета: 800 — одна звезда, 1500 — две, 3000 — три, 10000 и 20000 — соответствующие группы. Изображения выводятся через `next/image` с сохранением пропорций, размер — 40 px. Для отдельного баланса или цены по-прежнему используется `CreditAmount`.
- `ModalBackdrop`: onClose и children обязательны; children может быть функцией с requestClose. closeOnBackdropClick/closeOnEscape по умолчанию true. Есть portal, блокировка прокрутки и анимация закрытия. Самостоятельной реализации полного focus trap в этой оболочке нет.
- `ModalCloseButton`: нативные параметры/ref button; общая визуальная кнопка с CSS-крестиком. `size="default"` (по умолчанию) сохраняет кнопку 3rem и радиус 0.875rem; `size="compact"` — кнопка 1.5rem (2rem на сенсорных экранах), радиус `--radius-sm`, иконка 1rem с линиями 0.875rem. Крестик серый в покое (`--panel-surface-text-muted`), при наведении и клавиатурном фокусе плавно становится белым; обводка использует `--input-focus-border-color`. При reduced motion переход отключён. Для доступного имени передать aria-label. В TokenTopUpDialog и SubscriptionPlansDialog используется с классом только для позиционирования, сохраняет фокус и закрытие через ModalBackdrop. Компактный размер используется для удаления прикреплённого фото. Не следует считать все старые крестики уже переведёнными на неё.
- `ScrollArea`: orientation=vertical/horizontal, trackPlacement=inset/outside, viewportAs=div/aside/main/nav/pre/section/textarea/ul. Для класса и параметров прокручиваемого элемента есть viewportClassName/viewportProps; ref корня и viewportRef — разные ссылки. Источник — [ScrollArea.tsx](../src/components/ui/ScrollArea/ScrollArea.tsx).
- В раскрытой панели `SidebarConversations` при `data-scrollable="true"` список резервирует справа 0,625 rem под вертикальный трек и его отступ, плюс `--space-2` между треком и строками. Когда прокрутки нет, ширина строк сохраняется; свёрнутая панель использует прежние размеры значков.
- `Tooltip`: children/label, placement=top/bottom. `TooltipBubble` — только оболочка children/className/style с role=tooltip, её позицию определяет потребитель. Источник — [Tooltip.tsx](../src/components/ui/Tooltip/Tooltip.tsx).

<a id="models"></a>

### Модели, оболочки и публичная часть
- Источник данных для всех рабочих списков — backend `productcatalog.WorkspaceCatalog`, `/web/v1/models`. Типизированная схема, доступность, категории, описание, ограничения и цены проверяются в `model-catalog-contract.ts`. Ручные подборки содержат только ID. Полный ModelsCatalog и FeaturedModels показывают доступные текстовые, графические и видеомодели; ModelCard ведёт в новый чат с публичным ID, без локальной таблицы описаний или моделей-заглушек. Превью использует сгенерированный бэкендом JSON. [Контракт и проверка](../../../docs/runbooks/MODEL_CATALOG.md).
- `ImageGenerationControls` принимает `qualityLabel` и `showOutputCount` из выбранной модели. Соотношения сторон фильтруются по качеству и `price_by_variant`; у видео `variants` ограничивают длительность, разрешение и пропорции совместно. Цена в интерфейсе относится к выбранной комбинации, финальный расчёт остаётся серверным.
- `FileTaskModelSelector` загружает тот же каталог и выбирает операции: generate/video для анимации, edit/enhance/remove-background для изображения, только с включённым images input. Фиктивных моделей и цен нет. Пустой список и сбой имеют отдельное состояние. Реальное исполнение этих файловых операций ещё не подключено; локальные кисть/ластик/undo сохранены.

- `ModelSelector.descriptionMode` принимает `inline` (по умолчанию) или `tooltip`. Только `ConversationModelSelector` у инпута включает `tooltip`: в строке остаются иконка и название. `ModelSelectorOption` показывает одно описание через общий `TooltipBubble` в портале на слое 180 при наведении мыши или фокусе; выбирает свободную сторону панели, на узком экране — место над/под строкой. Подсказка скрывается при уходе указателя/фокуса, прокрутке, изменении размеров, фильтрации и закрытии списка. В `ModelCard variant="selector"` режим и уникальный `descriptionId` связывают скрытый текст с кнопкой через `aria-describedby`. Хедер, редактор файла и отдельный каталог сохраняют описания в строках/карточках.
- [WorkspaceHero](../src/features/workspace/WorkspaceHero/WorkspaceHero.tsx) связывает одно постоянное поле WorkspacePrompt → ChatComposer и [FeaturedModelShortcuts](../src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx). Параметры: access, allModelsLink, modelLinksClassName. Стартует с текстового режима NeiroHub Chat, затем выбирает одну из первых четырёх реальных моделей изображений из общего каталога. FeaturedModelShortcuts принимает selectedModelId/onSelect/disabled; null означает текстовый режим. Это кнопки с aria-pressed и фиолетовым индикатором, без перехода; «Все нейросети» остаётся ссылкой. Её подпись выровнена по первой строке названий моделей: одинаковая высота области иконки, отступ и межстрочный интервал; содержимое ссылки прижато к началу строки без растягивания внутренних рядов. При выборе модели меняются только ImageGenerationControls и обработчик отправки; оформление и ширина поля, текст, курсор и вложение сохраняются. Появление и исчезновение кнопок занимают 220 мс; при выходе ExitingModelControls сохраняет снимок настроек и позиции кнопок, отключает взаимодействие через inert и плавно уменьшает высоту ряда, включая перенос на несколько строк. Выбор новой модели отменяет незавершённый выход; при reduced-motion скрытие происходит сразу. Поле не пересоздаётся. Строка стоимости при переключении не добавляется; подтверждение стоимости появляется после отправки. Контроллер сбрасывает настройки под выбранную модель без пересоздания поля. Во время подготовки, подтверждения и выполнения генерации переключение заблокировано. Карточки «Популярных нейросетей» и каталог продолжают использовать навигационный ModelCard.

- [ModelSelector](../src/features/models/WorkspaceModelSelector/ModelSelector.tsx): models/selectedModelId/onSelect; variant=compact/panel/composer, status=loading/ready/failure, disabled=false, renderInPortal=false. Необязательный categoryErrors задаёт сообщение об ошибке внутри соответствующего блока, сохраняя доступ к рабочим подборкам. Сам содержит поиск и собственную всплывающую логику; фокусирует поиск после размещения портальной панели. `composer` — компактная кнопка с названием и стрелкой, без иконки модели. В шапке использовать [WorkspaceModelSelector](../src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx), в инструментах файла — [FileTaskModelSelector](../src/features/files/FilePreviewDialog/FileTaskModelSelector.tsx) с task/selectedModelId/onSelect.
- ModelSelector показывает общую ленту блоков с заголовками из ru.modelsCatalog.categories. В каждом блоке максимум пять моделей. `categoryModelIds` задаёт упорядоченные ID для отдельных категорий; `[]` скрывает блок и его кнопку, пропущенное поле берёт первые пять подходящих моделей из переданного каталога. Общая конфигурация по умолчанию — `modelSelectorCategoryModelIds` в [model-selector-sections.ts](../src/features/models/WorkspaceModelSelector/model-selector-sections.ts), единая для хедера и инпута. Неизвестные ID, повторы внутри блока и модели другой категории пропускаются. Принадлежность определяется серверным массивом categories; отсутствие categories не даёт членства ни в одной подборке. Серверная категория `video-audio` отображается как отдельные «Видео» и «Аудио» по основному типу модели (`category`, полученному из серверного `kind`); общий `matchesModelCatalogCategory` применяется и в полном каталоге, и в подборках. Полный каталог показывает обе вкладки, даже если моделей пока нет. В селекторе пустые блоки скрыты; прежний hideEmptySections удалён. Одна модель может встречаться в нескольких подборках.
- Под поиском используется общий ModeSwitchPanel с toolbar-семантикой: выбор поднимает соответствующий блок в начало ленты и сбрасывает её прокрутку, остальные блоки сохраняют исходный порядок. Поиск по имени/ID действует сразу во всех подборках, после ограничения до пяти; переключение категории сохраняет запрос. Категории без совпадений исчезают из результатов, но их кнопки остаются. Поиск, переключатель и ссылка полного каталога закреплены, прокручивается только лента. ARIA-ID ленты и заголовков уникальны для каждого селектора; подсказка привязана к конкретной строке, даже если модель повторяется в другом блоке.
- Хедер, диалог и [NewChatPrompt](../src/features/workspace/WorkspacePrompt/NewChatPrompt.tsx) используют [loadGenerationModelCatalog](../src/features/models/generation-model-catalog.ts): единые записи, порядок и ошибки из `/web/v1/models`. Единственный model-catalog-cache делит параллельные запросы и хранит успешный результат в памяти 60 секунд; старые загрузчики — чистые проекции. При ошибке нет локального запасного списка. Видеокаталог содержит только включённые, оценённые сервером модели генерации по тексту; маршруты, требующие референсов, до подключения загрузки не предлагаются. WorkspaceModelSelector всегда открывает `/app/chats?model=<id>` — пустую форму нового чата; история создаётся при первой отправке. NewChatPrompt проверяет ID по общему каталогу. WorkspacePrompt принимает selectedGenerationModel, сохраняя совместимость selectedChatModel; модель и generationOptions фиксируются в PendingConversationBootstrap с исходным ключом запроса.
- В начатом диалоге использовать [ConversationModelSelector](../src/features/conversations/ConversationModelSelector/ConversationModelSelector.tsx), а не WorkspaceModelSelector с переходом в генератор. ConversationComposer передаёт его через modelSelector → ChatComposer.additionalControls непосредственно перед отправкой. Кнопка появляется после первой отправки/при загрузке существующей истории, с короткой анимацией и учётом reduced-motion. Пока идёт отправка/ответ, она отключена. Портальная панель `composer` имеет фиксированную высоту 586 CSS px — размер блока «Популярные» с пятью строками. Категории, поиск и пустые результаты не меняют высоту: дополнительные заголовки и модели прокручиваются внутри. Только недостаток свободного места над/под кнопкой уменьшает панель. Поиск, категории и нижняя ссылка закреплены.
- Ссылка «Все нейросети и возможности» внизу ModelSelector показана без стрелки и использует control внутри отдельной группы actionItems: приглушённый текст плавно светлеет при наведении/фокусе, появляется общая фирменная обводка, фон остаётся прозрачным. Группа не охватывает карточки моделей и вкладки; их состояния выбора сохраняются.
- `ModelSelector variant="compact"` в хедере имеет постоянную высоту 2,75 rem и иконку 1,5 rem. Контекстный селектор `.trigger .modelIcon` сохраняет размер независимо от порядка CSS в dev/production. Видимое название занимает одну строку и максимум 40 Unicode-символов вместе с многоточием; доступное имя и название в списке остаются полными. WorkspaceHeader сохраняет ширину правых кнопок, а слева разрешает сжатие: при нехватке места текст получает CSS-многоточие без увеличения высоты. Варианты panel/composer сохраняют свои размеры и правила текста.
- `useConversationModelSelection` использует тот же общий каталог и хранит публичный ID отдельно для каждого диалога в sessionStorage. Выбор в ConversationModelSelector меняет только модель текущего чата. ConversationComposer принимает generationModel; общий [useGenerationControls](../src/features/models/generation-options.tsx) подключает существующие ImageGenerationControls, ImageAspectRatioSelector и ImageQualitySelector, сохраняя поле и черновик. Настройки и серверная стоимость зависят от модели, без неподдерживаемых комбинаций с нулевой ценой. model_id и параметры фиксируются в PendingTurn; повтор отправляет исходное тело и ключ идемпотентности. Текстовые лимиты проверяются в UTF-8. Фото отображаются через ConversationImageGallery/FileCard, видео — через same-origin `/web/v1/video-artifacts/<id>`. Ожидание медиа восстанавливается по публичному ID задания и seq; polling ограничен 15 минутами, терминальная ошибка останавливает ожидание. Итоговая стоимость, принадлежность и доступность остаются ответственностью Go API.
- Кнопка `ModelSelector variant="compact"` в хедере при наведении получает обводку `--color-accent` без изменения заливки или размеров. Disabled-кнопка не подсвечивается; правило действует в обеих темах.
- В открытом диалоге `useConversationModelSelection` передаёт текущий ID модели в `WorkspaceModelSelection` вместе с ID диалога. Хедер показывает её название и иконку, включая восстановление выбора при переключении чатов; модель диалога имеет приоритет над `model` в URL только на совпадающем маршруте `/app/chat/<id>`. При уходе из диалога эта привязка очищается. Связь односторонняя: выбор в инпуте обновляет хедер без перехода, а выбор через хедер по-прежнему открывает `/app/chats?model=<id>` и не меняет модель существующего диалога.
- [ModelCard](../src/features/models/ModelCard/ModelCard.tsx): variant=catalog/selector. У catalog параметры interactive/revealed; базовая граница прозрачная, интерактивная карточка получает фирменную обводку при наведении и клавиатурном фокусе; у selector обязательны selected/onActivate. Вариант selector использует общий стиль control (вариант 2): выбранная модель отмечена постоянной фирменной обводкой через aria-pressed, при наведении и фокусе другая строка получает такую же обводку. Заливки и отдельного квадратика справа нет; строка состоит из иконки и текста. [ModelIcon](../src/features/models/ModelIcon/ModelIcon.tsx) принимает src/className. Запасной силуэт рисуется CSS alpha-маской существующего SVG и окрашивается через `--color-model-icon`; форма для каждой темы и прозрачные вырезы сохранены. Переданное через src изображение выводится в исходных цветах.
- [WorkspaceFrame](../src/components/layout/WorkspaceFrame/WorkspaceFrame.tsx), [AppShell](../src/components/layout/AppShell/AppShell.tsx), [Sidebar](../src/components/layout/Sidebar/Sidebar.tsx), [WorkspaceHeader](../src/components/layout/WorkspaceHeader/WorkspaceHeader.tsx) собирают оболочку приложения. Для нового содержимого страницы обычно достаточно существующей оболочки и WorkspacePageFrame.
- WorkspaceHeader и вложенный список моделей находятся на слое хедера `z-index: 20`, выше закреплённого ConversationComposer (`10`). Повышать только z-index вложенного меню недостаточно: оно ограничено слоем родительского хедера. Мобильная боковая панель (`29–31`) и модальные окна (`160`) сохраняют свой приоритет.
- В публичной части используются [PageContainer](../src/components/public/PageContainer/PageContainer.tsx), [SectionHeading](../src/components/public/SectionHeading/SectionHeading.tsx), [ContentCard](../src/components/public/ContentCard/ContentCard.tsx), [PrimaryButton](../src/components/public/PrimaryButton/PrimaryButton.tsx). PrimaryButton и SecondaryButton — ссылки с собственным pill-оформлением; не подменять ими кнопки действий workspace только из-за имени.
- Левый блок тарифа Lite на `WorkspaceLanding` использует утверждённое изображение `assetPaths.images.workspace.litePlanBackground` как фон (`cover`, справа по центру). Название и цена сохраняют `--color-text-on-dark` в обеих темах; минимальная высота блока — 22rem, в том числе при мобильном размещении над преимуществами тарифа. Три декоративные иконки преимуществ имеют постоянную обводку, прозрачный фон и не меняются при наведении.
- Кнопка тарифа Lite использует тот же класс `WorkspaceHeader.tariffButton`, что и кнопка в хедере: общий градиент, цвет текста, скругления, тень и состояния наведения/нажатия. Градиент заполняет `border-box` без повторения, чтобы прозрачная граница не показывала цветные полосы по краям. Локальный `.planOffer .planButton` задаёт только размещение, ширину 100%, минимальную высоту 3,25rem и сохраняет видимость на мобильном экране.
- [VideoPlayer](../src/components/media/VideoPlayer/VideoPlayer.tsx) принимает title/source/poster; применяется на WorkspaceLanding. Медиа примеров уже обслуживает InspirationExampleMedia, поэтому отдельно вставлять видеоплеер в каждую карточку не требуется.
- `*Provider`, SessionRefresh, WorkspaceNavigationMetrics и ConversationTitleSync — инфраструктура данных/сессии/навигации. Они остаются в инвентаризации, но не являются альтернативами визуальных элементов.

<a id="faq"></a>

### Раскрывающиеся вопросы и ответы

[FAQ](../src/components/ui/FAQ/FAQ.tsx) — общий компонент главной и реферальной программы. Принимает `items: readonly { question: string; answer: string }[]` и необязательный `name`. Тексты передаёт потребитель из текущего словаря. Заголовок раздела остаётся у страницы.

Без `name` ответы открываются независимо. С одинаковым `name` нативные `details` образуют группу с одним открытым ответом; имя должно быть уникальным для группы в документе. Профиль использует `name="profile-referral-faq"`. Раскрытие работает с клавиатуры и без клиентского состояния.

Карточка повторяет оформление главной: `--color-panel`, `--color-card-border`, `--radius-2xl`, общая стрелка `faqArrow`. Обводка при наведении, видимый клавиатурный фокус, поворот стрелки и анимация раскрытия/закрытия описаны в одном CSS-модуле. При `prefers-reduced-motion` анимация отключена; без поддержки новых CSS-переходов остаётся нативное раскрытие. Использование: `<FAQ items={questions} />` или `<FAQ items={questions} name="unique-faq-group" />`. Прежний путь `components/public/FAQ` удалён.

<a id="async-states"></a>

### Общие состояния загрузки, ошибки и пустых данных

`LoadFeedback` дополняет стабильное содержимое сообщением после 3 секунд, объясняет offline и предлагает повтор ошибки. Источник данных не должен заменять готовое содержимое заглушкой при обновлении. Общий каталог — `GenerationCatalogProvider`; приватные кеши принадлежат аккаунту. [Контракт загрузки](preloading.md).

Карточки FileCard и миниатюры используют внутренний `image-artifacts/:id?preview=1`; оригинал остаётся в просмотрщике/скачивании/редакторе. Безопасные поля job.aspect_ratio/output_count резервируют геометрию новых заданий.

[AsyncState](../src/components/ui/AsyncState/AsyncState.tsx) экспортирует:

- `LoadingIndicator({ label, progress?, processingLabel?, className? })`: общий круг. Без progress — неопределённое ожидание; с числом — 0–100 и aria-valuenow; при 100 доступно описание обработки. Размер — `--loading-indicator-size`, цвет — `--loading-indicator-color`. Reduced motion отключает анимацию.
- `Skeleton`: декоративный span; потребитель задаёт размеры, общие стили — фон/обводку/скругление/движение. `SkeletonGrid({label, count=6})` использует MasonryGrid. В каталоге моделей Skeleton вставляется в существующую сетку моделей.
- `StateNotice({kind, children, action?, inline?, role?})`: loading/error/empty/success/info; сохраняет смысловой текст. По умолчанию error — alert, остальные — status; существующее спокойное оповещение можно сохранить через role. Static sidebar использует role=note. Размер/позиция окна остаются у владельца.
- `MediaState({state, label, retry?, compact?})`: центрированный индикатор либо повтор; сообщение ошибки доступно скринридеру. Не рендерит техническое объяснение lazy loading.

[RetryAction](../src/components/ui/AsyncState/RetryAction.tsx) использует общий RetryUploadIcon + Button variant=outline. `label` обязателен, `iconOnly` добавляет Tooltip. Допускает стандартные button props. Обработчик и disabled задаёт потребитель; компонент не выполняет запросы самостоятельно.

[ImageGenerationGrid / ImageGenerationPlaceholders](../src/features/image-generation/ImageGenerationGrid/ImageGenerationGrid.tsx)
задают раскладку ожидаемых и готовых изображений чата. `count` берётся из отправленного
запроса, `aspectRatio` — строка вида `9:16`. Одно фото занимает одну колонку, несколько —
две, включая узкий экран; четыре фото образуют 2×2. Вся сетка центрируется внутри
области сообщения, по общей оси с инпутом; это относится и к одному фото, и к заглушкам.
В каждой заглушке общий `MediaState`
с неопределённым прогрессом и доступной подписью номера изображения. Цвета, скругления
и reduced motion принадлежат общим состояниям. `PendingGenerationIndicator` выбирает
эту сетку для фото, а для текста/видео сохраняет `AssistantTypingIndicator`.
Количество и формат сохраняются вместе с идентификатором ожидающей задачи в sessionStorage;
перезапуск polling не теряет сетку. Готовые фото используют тот же grid и прежние FileCard/FilePreviewDialog.
Локальное превью `/app/ui-states#image-generation` позволяет выбрать 1–15 фото, формат,
загрузку/ошибку/успех; готовые фото являются повторениями локального примера.
В dev-чате тот же цикл запускается обычной отправкой: количество и формат берутся
из инпута, после задержки симулятор возвращает тестовые фото в общий просмотрщик.
Сценарий «Отправка сообщения → Медленно» оставляет восемь секунд на просмотр ожидания.
Это локальные образцы; нейросеть не вызывается. [Порядок проверки](local-development.md).
Проценты и частичная готовность не моделируются: сервер сообщает статус всей задачи.

[MediaImage](../src/components/media/MediaImage/MediaImage.tsx) отслеживает load/error браузера отдельно от результата API, повторно монтирует тот же src, сбрасывает состояние при смене src. `fit=natural|contain|cover`, `optimized` сохраняет next/image для примеров. `action` позволяет сделать область открытия соседней с повтором; `passive` используется внутри уже существующих кнопок миниатюр. `showLoading` позволяет избежать двух индикаторов при отдельном upload progress. Blob URL остаются у владельца.

[MediaVideo](../src/components/media/MediaVideo/MediaVideo.tsx) сохраняет video ref, controls и текущие события. `poster` — отдельная небольшая картинка: остаётся поверх ролика до loadeddata/canplay/playing и снова видна при ошибке/повторе; одних metadata недостаточно. Готовая обложка не перекрывается индикатором ожидания, повтор ошибки имеет прозрачный фон. Если обложка недоступна, используется общий MediaState. `loading="lazy"` откладывает подключение src до видимости карточки через IntersectionObserver; без observer работает обычная загрузка. Пока обложка загружается, preload=none отдаёт ей приоритет. Размеры width/height резервируют пропорции. Waiting/stalled после первого кадра сохраняют видео под индикатором. В passive карточке повтор выполняется из большого просмотра.

InspirationExampleMedia передаёт `example.posterPath`; карточки используют lazy video, большой просмотр — eager. `videoProps.preload="none"` с posterPath показывает только MediaImage: это режим миниатюр просмотрщика без загрузки MP4. Два текущих видеопримера имеют локальные WebP-обложки из первого кадра; новый видеопример также должен поставляться с отдельной обложкой. Пользовательские видео без poster остаются на прежнем пути. VideoPlayer сохраняет собственный сценарий play/pause/Space и использует тот же MediaState.

Оформление — `AsyncState.module.css` и `--state-*` в globals.css. Карта подключённых экранов, оговорённые исключения и локальный просмотр: [shared-async-states.md](shared-async-states.md). Локальная страница `/ru/app/ui-states` доступна только в режиме dev:preview; есть ссылка в панели с колбой.

<a id="limits"></a>

<a id="not-found"></a>

### Страница 404

[NotFoundContent](../src/features/workspace/NotFoundContent/NotFoundContent.tsx) —
общая композиция 404 в правой рабочей области, внутри `WorkspaceFrame` либо
`GuestWorkspaceFrame`. Для неизвестных адресов внутри и вне `/app` сохраняются
хедер и рабочее меню, включая мобильное раскрытие и сворачивание на десктопе.
Используются общая тема, шрифт и словарь `notFound` RU/EN через `useDictionary`.
Персонаж из `assetPaths.illustrations.notFoundCharacter` отображается через Image;
цифры 404 и картинка декоративные, код ошибки также доступен скринридеру.
Композиция центрирована, без карточки. Ровный фон — `--color-workspace`, без свечения
за персонажем и цифрами. Покачивание отключается при reduced motion.
«На главную» — языковая ссылка на `/app` с общими классами Button `button outline`:
прозрачная заливка во всех состояниях, тонкая обводка и фирменный фокус.
Catch-all `[...missing]` вызывает `notFound()` из клиентского компонента страницы,
который также рендерится на сервере при первом запросе. Это сохраняет HTTP 404,
noindex и отсутствие общего кэша, обходя ошибку RSC-замера времени в dev
([React #37561](https://github.com/facebook/react/issues/37561)).
Его серверный layout читает существующую проверенную сессию:
в аккаунте сохраняются история и управление профилем, при отсутствующей/недоступной
сессии показывается гостевая оболочка с 404. Переход в реальный раздел продолжает
обычный поток проверки/обновления сессии. Logout использует прежний общий boundary.
Корневой [not-found.tsx](../src/app/not-found.tsx) не читает аккаунт: этот fallback
может сериализоваться рядом с публичной страницей. Отсутствующие технические
ресурсы не загружают аккаунт; API и проверки языков/путей в proxy не меняются.
`ThemeBootstrapScript` исполняется в серверном HTML с CSP nonce; на клиенте тег
инертен, а layout effect восстанавливает сохранённую тему до отрисовки, если Next
смонтировал корень из документа ошибки. Повторного исполнения inline-скрипта нет.

## Что пока не считать унифицированным или завершённым

| Область | Проверенный факт | Решение при новой задаче |
| --- | --- | --- |
| Закрытие модалок | SubscriptionPlansDialog, TokenTopUpDialog, медиашаблон и выбор шаблона используют общий ModalCloseButton | Для совместимого нового окна выбирать общий компонент; старые окна отдельно сравнить по размеру и расположению |
| Тема | AccountMenu использует ModeSwitchPanel; PublicThemeSwitcher — собственную группу с заливкой | Для workspace брать ModeSwitchPanel. Публичный вариант считать отдельным до решения об объединении |
| Вкладки | [FileTypeTabs](../src/features/files/FileTypeTabs/FileTypeTabs.tsx) в «Моих файлах» использует общий ModeSwitchPanel с `fullWidth`. Пункты равномерно делят свободное место; на узком экране сохраняется горизонтальная прокрутка. Категории, ID вкладок и фильтрация сохранены | Локальные стили задают только растяжение пунктов; ширина, оформление и поведение берутся из ModeSwitchPanel. Прежняя реализация с подчёркиванием и временный FileTypeSwitch удалены. ImageGenerationGuide продолжает использовать подчёркивание |
| Поверхность ввода | InputSurface содержит literal-тонировку, совпадающую с токеном панели; часть старых полей имеет локальное оформление | Для совместимого нового поля использовать InputSurface. Замена всех старых полей в этом каталоге не выполнена |
| Обработка файла | FileAnimationPanel/FileEnhancementPanel/FileBackgroundRemovalPanel — интерфейс выбора модели; у общей кнопки в FileModelActionPanel нет onClick | Переиспользовать оболочку можно; запуск операции потребует отдельной реализации |
| Удаление файла | Кнопка FileCard disabled | Не обещать рабочее удаление при подключении карточки |
| Подсказка в светлой теме | TooltipBubble получает молочный `--panel-surface-background` и графитовый `--color-text` из светлой палитры | При добавлении подсказки проверять контраст и обе темы |
| Экспорты без потребителей | EmptyState, ModelPreviewCard, SecondaryButton, NewConversationButton, FilesToolbar, ImageJobHistory не имеют найденных статических потребителей вне тестов | Можно рассмотреть по исходнику; отсутствие импортов не является основанием для автоматического удаления |

## Границы проверки

Назначение, параметры, значения по умолчанию, варианты и CSS-поведение сверены с актуальными исходниками и потребителями. JSX-примеры проверяются компилятором TypeScript против типов проекта; CSS-примеры используют общие классы для состояний и локальные правила только для раскладки/оговорённого удаления. Это документация подключения, а не новая библиотека компонентов.

Каталог не меняет приложение и не заменяет проверку конкретного экрана после подключения: особенно размеров, тем, фокуса и загрузки данных. Исходная инвентаризация — вспомогательный снимок; рекомендации и исправленные уточнения находятся здесь. Точка входа для агента — [короткий индекс](ui-index.md), порядок работы и обновления записей — [локальный AGENTS.md](../AGENTS.md#ui-reuse-workflow).

<a id="chat-attachments"></a>
### Раздельные возможности моделей

`ModelSelectorModel.capabilities` — необязательные проверенные парсером данные
общего каталога. Техническая справка о возможностях приложения и API провайдера
удалена из интерфейса всех вариантов `ModelSelector`. Данные каталога остаются
доступны для рабочих параметров генерации.
`ConversationComposer` и `WorkspacePrompt` передают в `ChatComposer` общий
`attachmentController` из `useChatAttachments`. Он загружает PNG/JPEG для моделей
с включёнными референсами, показывает превью, удаление и повтор загрузки.
Перед загрузкой `useChatAttachments` читает содержимое и сравнивает SHA-256
с вложениями текущего сообщения. Имя и дата файла не влияют на сравнение.
Выбор через файл, перетаскивание, вставка и медиатека проходят через один контроллер;
проверки выполняются последовательно, включая повторы в одной пачке.
Лимит модели проверяется после исключения дубликатов. Во время проверки отправка
заблокирована; очистка/размонтирование отменяет незавершённые проверки.
Отпечатки хранятся только в памяти вместе с вложениями: удалённый файл можно
добавить снова. Серверная проверка артефактов остаётся дополнительной защитой.
Повтор показывает [ChatAttachmentNotice](../src/components/chat/ChatAttachmentNotice/ChatAttachmentNotice.tsx)
на основе `ModalBackdrop` и `Button variant="outline"`: «Этот файл уже прикреплён»,
прозрачная контурная кнопка «Понятно» с радиусом 0.5rem,
закрытие через Escape/фон, удержание и возврат фокуса. RU/EN-тексты из словаря,
черновик и существующие вложения сохраняются. Обычная вставка текста не меняется.
Общий `ChatAttachmentPreview` рисует фото над полем сообщения: одно фото —
квадрат 11.25rem (180px), два и более — по 4.5rem (72px), с плавной сменой размера.
При удалении остальных единственное фото снова увеличивается. Размер определяется
числом плиток в списке через `:only-child`; файлы не пересоздаются и не загружаются заново.
Необязательный `onOpen(trigger)` открывает фото в общем просмотрщике сразу при
наличии локального `previewUrl`, включая загрузку и её ошибку. `ChatComposer`
передаёт туда все фото текущего черновика с превью; смена статуса загрузки
не закрывает просмотр и не меняет выбранное фото. Кружок прогресса и затемнение
остаются на миниатюре и пропускают нажатия к кнопке просмотра. Удаление и повтор
остаются отдельными кнопками и не открывают просмотр; сам просмотр не запускает
дополнительную загрузку и не снимает блокировку отправки незагруженных файлов.

Сетевые сбои передачи (`WebNetworkError`: XHR error/timeout) дополнительно открывают
одну [ChatUploadNetworkNotice](../src/components/chat/ChatUploadNetworkNotice/ChatUploadNetworkNotice.tsx)
на весь черновик. Это `PopoverSurface` с иконкой предупреждения и нашим компактным
`ModalCloseButton`; плашка не модальная, не перехватывает фокус и прокрутку.
Иконка — присланный `file-network-error-red.svg`: красный круг с восклицательным
знаком (`#EF4444`), встроенный SVG размером 20px. Он доступен и без сети,
не требует загрузки внешнего ресурса и скрыт от экранного диктора как декоративный.
Иконка, текст и крестик выровнены по общей вертикальной середине, включая
многострочный текст на узком экране; отдельного верхнего отступа у иконки нет.
При появлении тонкий фиолетовый блик делает два прохода по контуру за 4.8s,
после чего остаётся обычная обводка. Эффект декоративный, не принимает нажатия,
отключён при `prefers-reduced-motion` и не повторяется при изменении ширины.
Она закреплена вверху правой панели с отступом 16px (с учётом safe area), вне её
прокручиваемого содержимого. Границы измеряются по `InputSurface` через нативный ref;
`ResizeObserver` сохраняет совпадение с инпутом при изменении окна и сайдбара.
Закрытие возвращает фокус в поле и сохраняет фото. Повторная сетевая ошибка вновь
показывает плашку; успешный повтор/удаление всех затронутых файлов убирает её.
HTTP-ошибки, неверный формат, отсутствие CSRF и отмена пользователем не считаются
отсутствием сети. В локальной панели сценарий загрузки «Без интернета» имитирует
тот же сбой через общий транспорт; «Ошибка» по-прежнему означает ответ HTTP 503.
Локальная проверка открывается круглой кнопкой 40px с иконкой колбы внизу справа,
левее индикатора Next.js с зазором. Название и активный режим находятся в общем
`Tooltip` и доступной подписи кнопки; открытая панель скрывает подсказку.
Положение кнопки не зависит от уведомлений; публичная версия её не показывает.

Фото заполняет квадрат (`object-fit: cover`); рамки, подложки, пустых полей и плашки
с именем нет. Радиус — `--radius-sm` (0.5rem), обрезается только превью.
Однотонные непрозрачные чёрные полосы по противоположным краям исходной картинки
распознаются при загрузке превью (каждая не больше 20% стороны). `preview-crop`
анализирует уменьшенную до 512px копию и меняет только положение/масштаб `img`.
Исходный URL, файл и байты загрузки сохраняются. Отдельный тёмный край, текстура,
прозрачность и недоступные для чтения пиксели не считаются чёрными полями.
Общий `ModalCloseButton` с `size="compact"` и `Tooltip` «Удалить файл» появляется при hover/focus; на сенсорных
экранах виден постоянно. Кнопка находится внутри фото с отступом 0.5rem сверху
и от конечного края строки, не выступая за границы превью. Удаление отменяет
загрузку и освобождает blob-превью.
Круговой индикатор поверх фото получает реальный процент передачи из
`XMLHttpRequest.upload` через `webBrowserMutation.onUploadProgress`; без известного
размера показывает ожидание. Переданные 100% ещё не означают готовый артефакт:
индикатор остаётся до проверки ответа сервера. При ошибке фото слегка затемняется,
а поверх появляется прозрачная кнопка `Button variant="outline"` с иконкой повтора
`RetryUploadIcon` (`restart-white.svg`);
под фото нет текста ошибки или ссылки. Общий `Tooltip` показывает «Повторить»
при hover/focus. Полная причина доступна экранным дикторам
через визуально скрытый alert и aria-describedby. Повтор возвращает круговой прогресс.
На единственном фото кнопка по центру (40px); на маленьких плитках — 24px, ниже
центра, чтобы её область нажатия не пересекалась с крестиком удаления на телефоне.
Слой ошибки находится вне обрезающего фото контейнера, поэтому подсказка не обрезается.
Для ручной проверки без backend используется [панель локальных сценариев](local-development.md):
успех, ошибка, медленная загрузка, ошибки расчёта стоимости и отправка сообщений
с тестовым ответом, только в `dev:preview`.
Анимации учитывают reduced-motion, подписи берутся из RU/EN-словарей.
В диалоге кнопка загрузки видна, когда контроллер возвращает `enabled: true`
по возможностям модели в каталоге. При смене модели прежняя ошибка выбора
сбрасывается; черновик и вложения сохраняются, их совместимость проверяется заново.
Пока загрузка или запрос цены не завершены, отправка заблокирована. Идентификаторы
вложений сохраняются с исходным ключом запроса при повторной отправке.

`features/conversations/ConversationInputImages/ConversationInputImages` показывает
фото в истории, ещё не принятом сообщении и при создании нового диалога. Принимает
`ids?: readonly string[]`, читает `/web/v1/input-artifacts/{id}` через
`webBrowserFetch` и создаёт отдельный blob-URL для миниатюры и просмотра оригинала.
Нажатие открывает `AttachmentPreviewDialog` с фото только этого сообщения,
без правой панели. Просмотр использует уже загруженные URL и не повторяет запросы;
закрытие возвращает фокус исходному фото и не освобождает используемый миниатюрой URL.
Поэтому локальная имитация использует те же компоненты, что серверные вложения.
Миниатюры помещаются в 160px с `object-fit: contain`, не перекрывая текст.
При ошибке чтения/декодирования вместо сломанного фото — общий `RetryUploadIcon`
в прозрачной кнопке с Tooltip «Повторить». Загрузка имеет доступную подпись,
повтор и размонтирование отменяют запрос и освобождают URL.

`useReferenceQuote` принимает модель, параметры генерации и `{ count, ready, artifactIds }`.
Расчёт начинается только после успешной загрузки всех файлов; при входе без вложений,
во время проверки/загрузки и при ошибке загрузки запрос цены не выполняется.
Цена привязана к текущим параметрам, набору артефактов и попытке запроса. Смена набора,
очистка и повтор сбрасывают прежнюю цену/ошибку; поздние ответы отменённых запросов игнорируются.
`ConversationComposer` и `WorkspacePrompt` выводят строку стоимости только при известной
цене — заглушка «Стоимость: —» не показывается. При сбое после добавления файлов
общий [ReferenceQuoteNotice](../src/components/chat/ReferenceQuoteNotice/ReferenceQuoteNotice.tsx)
показывает понятное уведомление через `PopoverSurface` и прозрачную кнопку
`Button variant="outline"` «Повторить расчёт». Повтор сохраняет текст и вложения,
убирает прежнее уведомление и возвращает фокус в поле. До успешного расчёта отправка
с вложениями остаётся заблокированной; сервер сохраняет контроль окончательной цены.

`WorkspaceFileDropZone` оборачивает правую часть `AppShell`: при перетаскивании
файла показывает затемнение и `assetPaths.images.workspace.fileDrop`, без рамки.
Боковая панель не активирует оверлей. Уход из области, Escape, отмена и drop
закрывают его. Перетаскивание текста игнорируется, файл не отправляется автоматически.
Лимит загрузки — 20 МБ и 4096 × 4096 пикселей; количество определяет каталог.
Текстовые/видеомодели и неподдерживаемые форматы отклоняются с сообщением.
Локальное демо не подменяет реальную серверную загрузку успешной фиктивной.
