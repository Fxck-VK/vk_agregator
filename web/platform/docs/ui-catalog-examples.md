# Примеры подключения компонентов

Проверены по текущим типам `web/platform` 12 сентября 2026 года. Нужный пример выбирается через [короткий индекс](ui-index.md); правила и ограничения — в [каталоге](ui-catalog.md). Каждый TSX-блок ниже — отдельный модуль; CSS из первого раздела сохраняется рядом под именем `CatalogExample.module.css`. Он задаёт раскладку примеров, а визуальные состояния берутся из общего стиля.

Обработчики операций передаются извне. Примеры не создают задачи, не удаляют данные и не подключают платные операции самостоятельно. В приложении тексты следует брать из соответствующего раздела локализации; здесь подписи оставлены рядом с компонентами для читаемости.

<a id="example-1"></a>

## 1. Действия в панели — вариант 1

Общая панель с действием переименования и опасным действием. У обоих серый обычный текст; при наведении/фокусе переименование становится светлым, удаление — красным. `onRequestDelete` должен открывать принятый в продукте сценарий удаления; здесь показано только подключение меню.

```tsx
"use client";

import { useId, useRef, useState } from "react";
import { EditIcon } from "@/components/icons/EditIcon";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import controls from "@/components/ui/selectable-control.module.css";
import styles from "./CatalogExample.module.css";

export function ActionMenuExample({ onRename, onRequestDelete }: {
  onRename: () => void;
  onRequestDelete: () => void;
}) {
  const anchorRef = useRef<HTMLButtonElement>(null);
  const panelID = useId();
  const [open, setOpen] = useState(false);
  const choose = (action: () => void) => {
    setOpen(false);
    anchorRef.current?.focus();
    action();
  };

  return <>
    <InputControlChip
      aria-controls={open ? panelID : undefined}
      aria-expanded={open}
      aria-haspopup="dialog"
      className={styles.compactChip}
      onClick={() => setOpen(!open)}
      ref={anchorRef}
    >Действия</InputControlChip>
    <PopoverPanel
      anchorRef={anchorRef}
      id={panelID}
      isOpen={open}
      itemVariant="action"
      label="Действия с элементом"
      onClose={() => setOpen(false)}
      width={280}
    >
      <div className={styles.menu}>
        <button className={controls.control} onClick={() => choose(onRename)} type="button">
          <EditIcon className={styles.icon} />Переименовать
        </button>
        <button
          className={`${controls.control} ${styles.danger}`}
          onClick={() => choose(onRequestDelete)}
          type="button"
        >
          <svg aria-hidden="true" className={styles.strokeIcon} viewBox="0 0 24 24">
            <path d="M4 7h16m-10 4v6m4-6v6M9 4h6l1 3H8l1-3Zm-3 3 1 13h10l1-13" />
          </svg>
          Удалить
        </button>
      </div>
    </PopoverPanel>
  </>;
}
```

<a id="example-styles"></a>

Общий CSS для примеров. Правило `.danger` намеренно точнее селектора общего action-стиля, чтобы красный цвет не зависел от порядка CSS. В обычном состоянии цвет удаления не переопределяется.

```css
.compactChip[data-ui="input-control-chip"] {
  --input-control-size: 2.5rem;
  --input-control-padding-inline: var(--space-3);
  border-radius: var(--radius-sm);
}

.menu { display: grid; gap: var(--space-1); }
.menu button {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  inline-size: 100%;
  min-block-size: 2.75rem;
  padding-inline: var(--space-2);
  text-align: start;
  cursor: pointer;
}
.menu button.danger:where(:not(:disabled):hover, :focus-visible) {
  color: var(--color-danger);
}
.icon, .strokeIcon { inline-size: 1.25rem; block-size: 1.25rem; flex: 0 0 auto; }
.strokeIcon { fill: none; stroke: currentColor; stroke-width: 1.7; }
.choices { display: flex; gap: var(--space-2); }
.choice { min-inline-size: 4rem; min-block-size: 2.5rem; }
.field { padding: var(--space-3); }
.input { inline-size: 100%; border: 0; background: transparent; color: inherit; font: inherit; }
.scrollExample { block-size: 12rem; }
```

Если позиционирование и закрытие уже есть, используется `PopoverSurface itemVariant="action"`. Для смешанной панели назначать `controls.actionItems` только группе действий, как в [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx).

<a id="example-2"></a>

## 2. Значения в панели — вариант 2

Этот пример показывает новый небольшой селектор. Для реального разрешения или соотношения сторон в генераторе уже существуют [ImageQualitySelector](../src/features/image-generation/ImageQualitySelector/ImageQualitySelector.tsx) и [ImageAspectRatioSelector](../src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.tsx): подключать их, а не создавать копию ниже. Стрелки между radio-элементами здесь обрабатывает потребитель; `PopoverOption` не реализует эту логику сам.

Сопутствующий файл: [CatalogExample.module.css](#example-styles).

```tsx
"use client";

import { useId, useRef, useState } from "react";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import { PopoverOption } from "@/components/ui/PopoverOption/PopoverOption";
import styles from "./CatalogExample.module.css";

const qualities = ["2K", "4K"] as const;
type Quality = typeof qualities[number];

export function SelectionExample() {
  const [quality, setQuality] = useState<Quality>("2K");
  const [open, setOpen] = useState(false);
  const panelID = useId();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);

  return <>
    <InputControlChip
      aria-controls={open ? panelID : undefined}
      aria-expanded={open}
      aria-haspopup="dialog"
      aria-label={`Разрешение: ${quality}`}
      className={styles.compactChip}
      onClick={() => setOpen(!open)}
      ref={anchorRef}
    >{quality}</InputControlChip>
    <PopoverPanel anchorRef={anchorRef} id={panelID} isOpen={open}
      label="Разрешение" onClose={() => setOpen(false)} width={220}>
      <div aria-label="Разрешение" className={styles.choices} role="radiogroup">
        {qualities.map((value, index) => <PopoverOption
          className={styles.choice}
          key={value}
          onClick={() => {
            setQuality(value);
            setOpen(false);
            anchorRef.current?.focus();
          }}
          onKeyDown={(event) => {
            const keys = ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"];
            if (!keys.includes(event.key)) return;
            event.preventDefault();
            const next = event.key === "Home" ? 0 : event.key === "End" ? 1 : 1 - index;
            setQuality(qualities[next]!);
            optionRefs.current[next]?.focus();
          }}
          ref={(button) => { optionRefs.current[index] = button; }}
          selected={quality === value}
          tabIndex={quality === value ? 0 : -1}
        >{value}</PopoverOption>)}
      </div>
    </PopoverPanel>
  </>;
}
```

<a id="example-3"></a>

## 3. Переключатель с иконками

`iconOnly` отвечает только за вид подписей. Чтобы показывать подписи рядом с иконками, убрать этот параметр. Пример хранит только выбранное значение; для смены настоящей темы используется существующий адаптер из [AccountMenu](../src/features/account/AccountMenu/AccountMenu.tsx), который связывает переключатель с настройкой темы.

```tsx
"use client";

import { useState } from "react";
import { MonitorIcon } from "@/components/icons/MonitorIcon";
import { SunIcon } from "@/components/icons/SunIcon";
import { MoonIcon } from "@/components/icons/MoonIcon";
import { ModeSwitchPanel, type ModeSwitchPanelItem } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";

type Appearance = "system" | "light" | "dark";
const choices: readonly ModeSwitchPanelItem<Appearance>[] = [
  { id: "system", label: "Системная", icon: <MonitorIcon /> },
  { id: "light", label: "Светлая", icon: <SunIcon /> },
  { id: "dark", label: "Тёмная", icon: <MoonIcon /> },
];

export function ModeSwitchExample() {
  const [selected, setSelected] = useState<Appearance>("system");
  return <ModeSwitchPanel<Appearance>
    activeID={selected}
    ariaLabel="Предпочтение оформления"
    iconOnly
    items={choices}
    onChange={setSelected}
  />;
}
```

Для настоящих вкладок передать `semantics="tabs"`; каждой записи назначить `elementID` и `ariaControls`, а соответствующей секции — `role="tabpanel"`, `id` и `aria-labelledby`. Сами секции и их видимость создаёт потребитель. Для обычного переключателя режимов достаточно toolbar-семантики по умолчанию.

<a id="example-4"></a>

## 4. Поверхность поля и ползунок

`InputSurface` оформляет оболочку, поэтому внутренний input получает собственные базовые layout-правила. `RangeSlider` принимает значение и callback, не строковый DOM-event.

Сопутствующий файл: [CatalogExample.module.css](#example-styles).

```tsx
"use client";

import { useState } from "react";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { RangeSlider } from "@/components/ui/RangeSlider/RangeSlider";
import styles from "./CatalogExample.module.css";

export function InputAndRangeExample() {
  const [query, setQuery] = useState("");
  const [size, setSize] = useState(24);
  return <>
    <InputSurface className={styles.field}>
      <input aria-label="Поиск" className={styles.input}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Поиск" type="search" value={query} />
    </InputSurface>
    <RangeSlider aria-label="Толщина кисти" max={100} min={1}
      onValueChange={setSize} value={size} />
  </>;
}
```

<a id="example-5"></a>

## 5. Поле сообщения

Общая сборка уже включает управление высотой, загрузку медиа и кнопку отправки. Связь с отправкой сообщения и файлами принадлежит вызывающему коду. Для существующего диалога использовать `ConversationComposer`, а этот пример — ориентир для нового контекста.

```tsx
"use client";

import { useState } from "react";
import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";

export function ComposerExample({ pending, onSend, onFilesSelected }: {
  pending: boolean;
  onSend: (text: string) => void;
  onFilesSelected: (files: File[]) => void;
}) {
  const [value, setValue] = useState("");
  const send = () => {
    if (!pending && value.trim()) onSend(value.trim());
  };
  return <ChatComposer
    canSubmit={value.trim().length > 0}
    disabled={pending}
    label="Сообщение"
    mediaLabel="Загрузить медиа"
    onChange={(event) => setValue(event.target.value)}
    onFilesSelected={onFilesSelected}
    onSend={send}
    placeholder="Задайте вопрос"
    submitLabel="Отправить"
    value={value}
    variant="conversation"
  />;
}
```

<a id="example-6"></a>

## 6. Сетка примеров и просмотр только переданного набора

Внешний `onOpen` связывает карточки с одной галереей. В просмотрщик передаётся ровно `examples`; другие фото коллекции туда не попадут. Данные — существующие `InspirationExample`, а не придуманные записи пользовательских файлов. Пример рассчитан на стабильный набор на время просмотра.

```tsx
"use client";

import { useRef, useState } from "react";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { InspirationExampleCard } from "@/features/inspiration/InspirationExampleCard/InspirationExampleCard";
import { InspirationExampleDialog } from "@/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate";
import type { InspirationExample } from "@/features/inspiration/inspiration-examples";

export function ExampleGallery({ examples }: { examples: readonly InspirationExample[] }) {
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const close = () => {
    setSelectedIndex(null);
    requestAnimationFrame(() => {
      if (triggerRef.current?.isConnected) triggerRef.current.focus();
    });
  };

  return <>
    <MasonryGrid aria-label="Примеры генераций">
      {examples.map((example, index) => <li key={example.id}>
        <InspirationExampleCard example={example} onOpen={(trigger) => {
          triggerRef.current = trigger;
          setSelectedIndex(index);
        }} />
      </li>)}
    </MasonryGrid>
    {selectedIndex !== null && examples.length > 0 ? <InspirationExampleDialog
      examples={examples}
      onClose={close}
      onSelect={setSelectedIndex}
      selectedIndex={selectedIndex}
    /> : null}
  </>;
}
```

Количество фотографий задаёт переданный массив, а не сетка. За карточками пользовательских результатов обращаться к `FilesGrid`/`FileCard`; для истории чата уже есть `ConversationImageGallery` с поддержкой данных диалога и пагинации.

<a id="example-7"></a>

## 7. Карточка результата и просмотр файлов

У карточки нет режима «просто URL картинки»: ей нужны данные задачи, результат и обработчики загрузки. `ComponentProps` позволяет сохранить реальный контракт без копирования доменных типов. В диалоге этот флаг уже выставляет готовая галерея.

```tsx
"use client";

import { useState, type ComponentProps } from "react";
import { FileCard } from "@/features/files/FileCard/FileCard";
import { FilePreviewDialog } from "@/features/files/FilePreviewDialog/FilePreviewDialog";

export function ChatFileCardExample(props: ComponentProps<typeof FileCard>) {
  return <FileCard {...props} showDeleteControl={false} />;
}

type PreviewProps = Pick<ComponentProps<typeof FilePreviewDialog>,
  "items" | "onClose" | "returnFocusTo">;

export function FilePreviewExample({ items, onClose, returnFocusTo }: PreviewProps) {
  const [selectedIndex, setSelectedIndex] = useState(0);
  if (items.length === 0) return null;
  return <FilePreviewDialog
    items={items}
    onClose={onClose}
    onSelect={setSelectedIndex}
    returnFocusTo={returnFocusTo}
    selectedIndex={selectedIndex}
  />;
}
```

Карточка и просмотрщик в этом блоке — отдельные примеры подключения. Связь их обработчиков, догрузка результата и исходный индекс уже реализованы в [FilesWorkspace](../src/features/files/FilesWorkspace/FilesWorkspace.tsx) и [ConversationImageGallery](../src/features/conversations/ConversationImageGallery/ConversationImageGallery.tsx); использовать эти реализации при работе в соответствующих разделах.

<a id="example-8"></a>

## 8. Селектор модели

Контролируемый пример для нового контекста. В существующей шапке нужен `WorkspaceModelSelector`, а в инструментах обработки — `FileTaskModelSelector`, чтобы сохранить связь с моделью задачи.

Описание по умолчанию находится в строке (`descriptionMode="inline"`). Для компактного списка с описанием при наведении/фокусе передать `descriptionMode="tooltip"`; это уже включено в `ConversationModelSelector` у инпута. Параметр независим от `variant`, поэтому добавление нового варианта кнопки само по себе не меняет режим описаний.

Лента содержит до пяти моделей в каждом блоке. Без `categoryModelIds` выбираются первые пять моделей с соответствующей серверной категорией. Для своего контекста можно передать, например, `categoryModelIds={{ popular: ["nano_banana_2", "chatgpt"], images: ["nano_banana_2", "gpt_image_2"], free: [] }}`: порядок ID сохраняется, бесплатный блок скрывается, остальные категории используют значения по умолчанию. Данные, `categories` и доступность каждой модели берутся только из единого каталога через `models`; сам селектор не определяет категории по типу или цене. Выбор категории поднимает её блок, остальные остаются в ленте; поиск охватывает все подборки.

```tsx
"use client";

import { ModelSelector, type ModelSelectorModel } from "@/features/models/WorkspaceModelSelector/ModelSelector";

export function ModelSelectionExample({ models, selectedModelId, onSelect }: {
  models: readonly ModelSelectorModel[];
  selectedModelId: string;
  onSelect: (model: ModelSelectorModel) => void;
}) {
  return <ModelSelector
    models={models}
    onSelect={onSelect}
    renderInPortal
    selectedModelId={selectedModelId}
    variant="panel"
  />;
}
```

<a id="example-9"></a>

## 9. Общая прокрутка и короткая подсказка

Размер задаётся контейнером. Свойства прокручиваемого списка передаются через `viewportProps`, а не в props внешнего div. Кнопка с иконкой получает собственное доступное имя независимо от Tooltip.

Сопутствующий файл: [CatalogExample.module.css](#example-styles).

```tsx
"use client";

import { MoreIcon } from "@/components/icons/MoreIcon";
import { Button } from "@/components/ui/Button/Button";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { Tooltip } from "@/components/ui/Tooltip/Tooltip";
import styles from "./CatalogExample.module.css";

export function ScrollAndTooltipExample({ labels, onMore }: {
  labels: readonly { id: string; text: string }[];
  onMore: () => void;
}) {
  return <>
    <ScrollArea className={styles.scrollExample}
      viewportAs="ul" viewportProps={{ "aria-label": "Элементы" }}>
      {labels.map((label) => <li key={label.id}>{label.text}</li>)}
    </ScrollArea>
    <Tooltip label="Дополнительные действия" placement="bottom">
      <Button aria-label="Дополнительные действия" onClick={onMore} type="button">
        <MoreIcon className={styles.icon} />
      </Button>
    </Tooltip>
  </>;
}
```

Для самостоятельной модалки готового универсального компонента со всем содержимым и полным управлением фокусом пока нет. `ModalBackdrop` и `ModalCloseButton` — её части. Для фотографий сразу использовать существующий просмотрщик; пример интеграции общего медиакаркаса — [InspirationExampleDialog](../src/features/inspiration/InspirationExampleCard/InspirationExampleDialogTemplate.tsx).

## Сверка примеров

Все 9 TSX-модулей проверены компилятором с настройками TypeScript проекта: ошибок типов нет. Проверены 25 импортов из проекта, 12 обращений к локальным CSS-классам и существование файлов по локальным ссылкам. Проверка выполнялась на виртуальных модулях без добавления примеров в приложение.

CSS-состояния сверены с `selectable-control.module.css`; исключение удаления — с текущим `ConversationRow.module.css`. Локальный `CatalogExample.module.css` целиком приведён выше и не является новым глобальным стилем приложения. Примеры не проходили отдельный визуальный прогон как страницы приложения.

Базовые React-паттерны дополнительно сверены через Context7: [управляемое состояние в React 19.2.7](https://github.com/react/react/blob/v19.2.7/packages/react/README.md), [типизированная nullable DOM-ref в официальном примере React](https://github.com/react/react/blob/main/fixtures/flight-parcel/src/Dialog.tsx). Поведение компонентов NeiroHub определяют их исходники, а не внешние примеры.
