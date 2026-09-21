import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import { FilePreviewDialog, type FilePreviewItem } from "./FilePreviewDialog";
import {
  resetFileActionModelCatalogLoaderForTests,
  setFileActionModelCatalogLoaderForTests,
  type WorkspaceModelCatalog,
} from "./file-action-models";

const job: ImageJob = {
  cost_estimate: 50,
  created_at: "2026-08-01T12:00:00Z",
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  image_quality: "2K",
  model_id: "nano-banana-2",
  model_name: "Nano Banana 2",
  prompt: "Портрет в саду",
  status: "succeeded",
  updated_at: "2026-08-01T12:00:00Z",
};

const artifact: ImageJobResult["artifacts"][number] = {
  height: 1536,
  id: "6ca96a58-a902-4f23-a92a-6726e1a0cd20",
  mime_type: "image/png",
  size_bytes: 42,
  width: 1024,
};

const items: FilePreviewItem[] = [
  { artifact, job },
  {
    artifact: { ...artifact, id: "d066a4a0-647e-407b-b9c4-2c5b03a4b0da" },
    job: { ...job, id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729", prompt: "Озеро в горах" },
  },
  {
    artifact: { ...artifact, id: "ab34ea1e-bb33-43fd-ab2f-076719d45bd8" },
    job: { ...job, id: "0b2c3017-3927-427e-8b75-4204bc5a0af3", prompt: "Туманный лес" },
  },
];

const emptyFileActionCatalog: WorkspaceModelCatalog = {
  default_model_id: "",
  items: [],
  schema_version: 1,
};

const fileEditCatalog: WorkspaceModelCatalog = {
  default_model_id: "catalog-edit",
  items: [{
    categories: ["popular", "images"],
    description: "Catalog backed file editor.",
    id: "catalog-edit",
    kind: "image",
    name: "Catalog Edit",
    operations: [{
      enabled: true,
      id: "edit",
      image: {
        allowed_aspect_ratios: ["1:1", "16:9"],
        default_aspect_ratio: "1:1",
        default_quality: "HD",
        max_output_count: 1,
        max_reference_images: 1,
        price_by_quality: { HD: 70, "4K": 90 },
        price_by_variant: { "HD:1:1": 70, "4K:16:9": 90 },
        quality_label: "Разрешение",
        quality_options: ["HD", "4K"],
        show_output_count: true,
        supports_reference_image: true,
      },
      inputs: {
        audio: { enabled: false, support: "unsupported" },
        documents: { enabled: false, support: "unsupported" },
        images: { enabled: true, support: "supported" },
        max_total_bytes: 10_000_000,
        video: { enabled: false, support: "unsupported" },
      },
      kind: "image",
    }],
    verification: "verified-contract",
  }],
  schema_version: 1,
};

function renderDialog({
  previewItems = items,
  onClose = vi.fn(),
  onSelect = vi.fn(),
  selectedIndex = 0,
}: {
  previewItems?: readonly FilePreviewItem[];
  onClose?: () => void;
  onSelect?: (index: number) => void;
  selectedIndex?: number;
} = {}) {
  return render(
    <FilePreviewDialog
      items={previewItems}
      onClose={onClose}
      onSelect={onSelect}
      selectedIndex={selectedIndex}
    />,
  );
}

describe("FilePreviewDialog", () => {
  beforeEach(() => {
    setFileActionModelCatalogLoaderForTests(async () => emptyFileActionCatalog);
  });

  afterEach(() => {
    cleanup();
    resetFileActionModelCatalogLoaderForTests();
    Reflect.deleteProperty(navigator, "share");
    Reflect.deleteProperty(navigator, "clipboard");
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("keeps the close control as a direct child of the dialog surface", () => {
    renderDialog();

    const dialog = screen.getByRole("dialog");
    const closeButton = screen.getByRole("button", { name: "Закрыть предпросмотр" });
    expect(closeButton.parentElement).toBe(dialog);
  });

  it("lets the loaded image keep its natural aspect ratio", () => {
    renderDialog();

    const image = screen.getByRole("img", { name: job.prompt });
    const activeTool = screen.getByRole("button", { name: "Общая" });
    expect(image).not.toHaveAttribute("width");
    expect(image).not.toHaveAttribute("height");
    expect(activeTool).toHaveAttribute("aria-pressed", "true");
  });

  it("switches the selected preview tool without starting an unavailable action", () => {
    const onSelect = vi.fn();
    renderDialog({ onSelect });

    const general = screen.getByRole("button", { name: "Общая" });
    const animate = screen.getByRole("button", { name: "Оживить" });

    expect(animate).toHaveAttribute("aria-pressed", "false");
    expect(animate).not.toBeDisabled();

    fireEvent.click(animate);

    expect(general).toHaveAttribute("aria-pressed", "false");
    expect(animate).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("mode-switch-panel-indicator")).toHaveAttribute("aria-hidden", "true");

    animate.focus();
    fireEvent.keyDown(animate, { key: "ArrowRight" });
    expect(screen.getByRole("button", { name: "Улучшить" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(onSelect).not.toHaveBeenCalled();
  });

  it.each([
    ["Оживить"],
    ["Улучшить"],
    ["Удалить фон"],
    ["Редактировать"],
  ] as const)("disables the %s model selector when the catalog has no file-input operation", async (taskLabel) => {
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: taskLabel }));
    const infoPanel = screen.getByRole("complementary");
    const modelSelector = await within(infoPanel).findByRole("button", {
      name: "Нейросети временно недоступны",
    });
    expect(modelSelector.parentElement).toHaveAttribute("data-variant", "panel");
    expect(modelSelector).toBeDisabled();
    expect(within(infoPanel).queryByText("Генератор видео")).toBeNull();
    expect(within(infoPanel).queryByText("Nano Banana Pro")).toBeNull();
  });

  it("replaces the right panel with disabled animation model controls", async () => {
    renderDialog();

    const toolbar = screen.getByRole("toolbar");
    const infoPanel = screen.getByRole("complementary");
    const animateTool = within(toolbar).getByRole("button", { name: "Оживить" });

    fireEvent.click(animateTool);

    expect(animateTool).not.toHaveAttribute("title");
    expect(within(infoPanel).getByRole("heading", { name: "Оживить" })).toBeInTheDocument();
    expect(within(infoPanel).queryByText(job.prompt)).toBeNull();
    expect(screen.queryByRole("link", { name: "Пересоздать" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Поделиться файлом" })).toBeNull();

    const modelSelector = await within(infoPanel).findByRole("button", {
      name: "Нейросети временно недоступны",
    });
    expect(modelSelector).toHaveAttribute("aria-expanded", "false");
    expect(modelSelector).toBeDisabled();
    expect(within(infoPanel).getByRole("button", { name: "Оживить недоступно" })).toBeDisabled();
    expect(within(infoPanel).queryByText("Google Veo 3.1")).toBeNull();
  });

  it("replaces the right panel with disabled image enhancement controls", async () => {
    renderDialog();

    const toolbar = screen.getByRole("toolbar");
    const infoPanel = screen.getByRole("complementary");
    const enhanceTool = within(toolbar).getByRole("button", { name: "Улучшить" });

    fireEvent.click(enhanceTool);

    expect(enhanceTool).not.toHaveAttribute("title");
    expect(within(infoPanel).getByRole("heading", { name: "Улучшить" })).toBeInTheDocument();
    expect(within(infoPanel).queryByText(job.prompt)).toBeNull();
    expect(screen.queryByRole("link", { name: "Пересоздать" })).toBeNull();

    const modelSelector = await within(infoPanel).findByRole("button", {
      name: "Нейросети временно недоступны",
    });
    expect(modelSelector).toHaveAttribute("aria-expanded", "false");
    expect(modelSelector).toBeDisabled();
    expect(within(infoPanel).getByRole("button", { name: "Улучшить недоступно" })).toBeDisabled();
    expect(within(infoPanel).queryByText("GPT Image 2")).toBeNull();
  });

  it("replaces the right panel with disabled background removal controls", async () => {
    renderDialog();

    const toolbar = screen.getByRole("toolbar");
    const infoPanel = screen.getByRole("complementary");
    const removeBackgroundTool = within(toolbar).getByRole("button", { name: "Удалить фон" });

    fireEvent.click(removeBackgroundTool);

    expect(removeBackgroundTool).not.toHaveAttribute("title");
    expect(within(infoPanel).getByRole("heading", { name: "Удалить фон" })).toBeInTheDocument();
    expect(within(infoPanel).queryByText(job.prompt)).toBeNull();
    expect(screen.queryByRole("link", { name: "Пересоздать" })).toBeNull();

    const modelSelector = await within(infoPanel).findByRole("button", {
      name: "Нейросети временно недоступны",
    });
    expect(modelSelector).toHaveAttribute("aria-expanded", "false");
    expect(modelSelector).toBeDisabled();
    expect(within(infoPanel).getByRole("button", { name: "Удалить фон недоступно" })).toBeDisabled();
    expect(within(infoPanel).queryByText("Recraft AI")).toBeNull();
  });

  it("replaces the right panel with local image editing controls", async () => {
    renderDialog();

    const toolbar = screen.getByRole("toolbar");
    const infoPanel = screen.getByRole("complementary");
    const editTool = within(toolbar).getByRole("button", { name: "Редактировать" });

    fireEvent.click(editTool);

    expect(editTool).not.toHaveAttribute("title");
    expect(within(infoPanel).getByRole("heading", { name: "Редактировать" })).toBeInTheDocument();
    expect(within(infoPanel).getByRole("button", { name: "Кисть" })).toHaveAttribute("aria-pressed", "true");
    expect(within(infoPanel).queryByRole("button", { name: "Квадрат" })).toBeNull();
    expect(within(infoPanel).getByRole("button", { name: "Лассо" })).toHaveAttribute("aria-pressed", "false");
    expect(within(infoPanel).getByRole("button", { name: "Очистить выделение" })).toBeDisabled();
    expect(within(infoPanel).getByRole("button", { name: "Отменить выделение" })).toBeDisabled();
    expect(within(infoPanel).getByRole("button", { name: "Повторить выделение" })).toBeDisabled();
    const promptField = within(infoPanel).getByRole("textbox", { name: "Промпт редактирования" });
    expect(promptField).toHaveAttribute("placeholder", "Опиши, что нужно изменить");
    expect(promptField).toHaveAttribute("data-scroll-area-viewport", "true");
    expect(promptField.closest('[data-ui="input-surface"]')).not.toBeNull();
    expect(within(infoPanel).getByRole("slider", { name: "Размер кисти" })).toHaveValue("32");
    expect(await within(infoPanel).findByRole("button", {
      name: "Нейросети временно недоступны",
    })).toBeDisabled();
    expect(within(infoPanel).queryByRole("button", { name: /Соотношение сторон:/ })).toBeNull();
    expect(within(infoPanel).queryByRole("button", { name: /Разрешение:/ })).toBeNull();
    expect(within(infoPanel).getByRole("heading", { name: "Как работает" })).toBeInTheDocument();
    expect(within(infoPanel).getByText("Изменяет часть изображения")).toBeInTheDocument();
    expect(within(infoPanel).getByText(
      "Выделяет нужную область и позволяет описать, что в ней изменить.",
    )).toBeInTheDocument();
    expect(within(infoPanel).getByRole("button", { name: "Редактировать недоступно" })).toBeDisabled();
    expect(screen.queryByRole("link", { name: "Пересоздать" })).toBeNull();
  });

  it("selects catalog editor settings, keeps them across tools and resets them for another file", async () => {
    setFileActionModelCatalogLoaderForTests(async () => fileEditCatalog);
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    await screen.findByRole("button", { name: "Выбрана модель Catalog Edit. Открыть список" });

    fireEvent.click(await screen.findByRole("button", { name: "Соотношение сторон: 1:1" }));
    expect(screen.getByRole("dialog", { name: "Соотношение сторон" })).toHaveStyle({ zIndex: "170" });
    expect(screen.queryByRole("radio", { name: "9:16" })).toBeNull();
    expect(screen.getByRole("radio", { name: "1:1" })).toHaveFocus();
    fireEvent.click(screen.getByRole("radio", { name: "16:9" }));
    expect(screen.getByRole("button", { name: "Соотношение сторон: 16:9" })).toHaveFocus();

    fireEvent.click(screen.getByRole("button", { name: "Разрешение: HD" }));
    expect(screen.getByRole("dialog", { name: "Разрешение" })).toHaveStyle({ zIndex: "170" });
    expect(screen.getAllByRole("radio").map((radio) => radio.textContent)).toEqual(["HD", "4K"]);
    fireEvent.click(screen.getByRole("radio", { name: "4K" }));
    expect(screen.getByRole("button", { name: "Разрешение: 4K" })).toHaveFocus();

    fireEvent.click(screen.getByRole("button", { name: "Общая" }));
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));
    expect(await screen.findByRole("button", { name: "Соотношение сторон: 16:9" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Разрешение: 4K" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Следующий файл" }));
    expect(await screen.findByRole("button", { name: "Соотношение сторон: 1:1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Разрешение: HD" })).toBeInTheDocument();
  });

  it("keeps editor popover keyboard navigation separate from the file viewer", async () => {
    setFileActionModelCatalogLoaderForTests(async () => fileEditCatalog);
    const onSelect = vi.fn();
    const onClose = vi.fn();
    renderDialog({ onSelect, onClose });
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));
    const trigger = await screen.findByRole("button", { name: "Разрешение: HD" });
    fireEvent.click(trigger);
    const selected = screen.getByRole("radio", { name: "HD" });

    fireEvent.keyDown(selected, { key: "ArrowRight" });
    expect(onSelect).not.toHaveBeenCalled();
    fireEvent.keyDown(selected, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Разрешение" })).not.toBeInTheDocument();
    expect(screen.getByTestId("file-preview-backdrop")).toHaveAttribute("data-state", "open");
    expect(onClose).not.toHaveBeenCalled();
    expect(trigger).toHaveFocus();

    fireEvent.click(trigger);
    const last = screen.getByRole("radio", { name: "4K" });
    last.focus();
    vi.stubGlobal("innerHeight", 600);
    fireEvent.resize(window);
    expect(last).toHaveFocus();
    fireEvent.keyDown(last, { key: "Tab" });
    expect(screen.queryByRole("dialog", { name: "Разрешение" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("turns each edit selection tool off when its active button is pressed again", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const brush = within(infoPanel).getByRole("button", { name: "Кисть" });
    const lasso = within(infoPanel).getByRole("button", { name: "Лассо" });
    const eraser = within(infoPanel).getByRole("button", { name: "Ластик" });
    const editSurface = screen.getByLabelText("Область редактирования изображения");

    fireEvent.click(brush);

    expect(brush).toHaveAttribute("aria-pressed", "false");
    expect(brush).toHaveAttribute("data-active", "false");
    expect(lasso).toHaveAttribute("aria-pressed", "false");
    expect(eraser).toHaveAttribute("aria-pressed", "false");
    expect(editSurface).toHaveAttribute("data-tool", "none");

    fireEvent.click(eraser);
    expect(eraser).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(eraser);
    expect(eraser).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(lasso);
    expect(lasso).toHaveAttribute("aria-pressed", "true");
    expect(brush).toHaveAttribute("aria-pressed", "false");
    expect(eraser).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(lasso);
    expect(lasso).toHaveAttribute("aria-pressed", "false");
    expect(editSurface).toHaveAttribute("data-tool", "none");
  });

  it("draws a closed freehand lasso selection and keeps it in shared history", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const lasso = within(infoPanel).getByRole("button", { name: "Лассо" });
    const editSurface = screen.getByLabelText("Область редактирования изображения");
    const editMask = screen.getByTestId("file-edit-mask");
    const toolCursor = screen.getByTestId("file-edit-tool-cursor");
    const undo = within(infoPanel).getByRole("button", { name: "Отменить выделение" });
    const redo = within(infoPanel).getByRole("button", { name: "Повторить выделение" });

    const brushSize = within(infoPanel).getByRole("slider", { name: "Размер кисти" });
    expect(brushSize).toBeInTheDocument();

    fireEvent.click(lasso);

    expect(lasso).toHaveAttribute("aria-pressed", "true");
    expect(editSurface).toHaveAttribute("data-tool", "lasso");
    expect(within(infoPanel).queryByRole("slider", { name: "Размер кисти" })).toBeNull();
    expect(brushSize).toBeInTheDocument();
    expect(brushSize).toBeDisabled();
    expect(brushSize.closest('[inert][aria-hidden="true"]')).not.toBeNull();

    fireEvent.pointerDown(editSurface, { clientX: 10, clientY: 10, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 10, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });
    fireEvent.pointerUp(editSurface, { clientX: 10, clientY: 60, pointerId: 1 });

    const lassoPolygon = editMask.querySelector('polygon[data-edit-tool="lasso"]');
    expect(lassoPolygon).toBeInTheDocument();
    expect(lassoPolygon).toHaveAttribute("fill", "#fff");
    expect(lassoPolygon).toHaveAttribute("points", "10,10 60,10 60,60 10,60");
    expect(toolCursor).toHaveAttribute("data-visible", "false");

    fireEvent.click(undo);
    expect(editMask.querySelector('polygon[data-edit-tool="lasso"]')).not.toBeInTheDocument();

    fireEvent.click(redo);
    expect(editMask.querySelector('polygon[data-edit-tool="lasso"]')).toHaveAttribute(
      "points",
      "10,10 60,10 60,60 10,60",
    );
  });

  it("shows the lasso trace while the pointer is held", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const editSurface = screen.getByLabelText("Область редактирования изображения");
    const editMask = screen.getByTestId("file-edit-mask");
    fireEvent.click(within(infoPanel).getByRole("button", { name: "Лассо" }));

    fireEvent.pointerDown(editSurface, { clientX: 10, clientY: 10, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 40, clientY: 20, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });

    const lassoDraft = editMask.querySelector(
      'polyline[data-edit-draft="true"][data-edit-tool="lasso"]',
    );
    expect(lassoDraft).toHaveAttribute("points", "10,10 40,20 60,60");
    expect(lassoDraft).toHaveAttribute("fill", "none");
    expect(lassoDraft).toHaveAttribute("stroke", "#fff");

    fireEvent.pointerUp(editSurface, { clientX: 10, clientY: 60, pointerId: 1 });

    expect(editMask.querySelector('[data-edit-draft="true"]')).not.toBeInTheDocument();
    expect(editMask.querySelector('polygon[data-edit-tool="lasso"]')).toHaveAttribute(
      "points",
      "10,10 40,20 60,60 10,60",
    );
  });

  it("does not draw or show a tool cursor while every edit tool is off", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const editSurface = screen.getByLabelText("Область редактирования изображения");
    const editMask = screen.getByTestId("file-edit-mask");
    const toolCursor = screen.getByTestId("file-edit-tool-cursor");

    fireEvent.click(within(infoPanel).getByRole("button", { name: "Кисть" }));
    fireEvent.pointerMove(editSurface, { clientX: 24, clientY: 30, pointerId: 1 });
    fireEvent.pointerDown(editSurface, { clientX: 20, clientY: 20, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });
    fireEvent.pointerUp(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });

    expect(toolCursor).toHaveAttribute("data-visible", "false");
    expect(editMask.querySelectorAll("[data-edit-stroke]")).toHaveLength(0);
  });

  it("scrolls editor settings separately from the fixed submit button", async () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const settingsViewport = within(infoPanel).getByLabelText("Параметры редактирования");
    const submitButton = await within(infoPanel).findByRole("button", {
      name: "Редактировать недоступно",
    });

    expect(settingsViewport).toHaveAttribute("data-scroll-area-viewport", "true");
    expect(settingsViewport).toContainElement(
      within(infoPanel).getByRole("heading", { name: "Как работает" }),
    );
    expect(settingsViewport).not.toContainElement(submitButton);
  });

  it("opens the editor model list above when the scroll viewport lacks room below", async () => {
    setFileActionModelCatalogLoaderForTests(async () => fileEditCatalog);
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const modelSelector = await within(infoPanel).findByRole("button", {
      name: "Выбрана модель Catalog Edit. Открыть список",
    });
    const modelListId = modelSelector.getAttribute("aria-controls");
    let modelSelectorTop = 620;

    expect(modelListId).not.toBeNull();

    const rectSpy = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function getBoundingClientRect(this: HTMLElement) {
        if (this === modelSelector) return new DOMRect(100, modelSelectorTop, 180, 56);
        return new DOMRect(0, 0, 0, 0);
      });

    try {
      fireEvent.click(modelSelector);

      const modelList = document.getElementById(modelListId!);
      expect(modelList).toHaveAttribute("data-placement", "top");

      modelSelectorTop = 100;
      fireEvent.scroll(window);

      expect(modelList).toHaveAttribute("data-placement", "bottom");
    } finally {
      rectSpy.mockRestore();
    }
  });

  it("keeps an upward editor model list inside the browser viewport", async () => {
    setFileActionModelCatalogLoaderForTests(async () => fileEditCatalog);
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const modelSelector = await within(infoPanel).findByRole("button", {
      name: "Выбрана модель Catalog Edit. Открыть список",
    });
    const modelListId = modelSelector.getAttribute("aria-controls");

    const rectSpy = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function getBoundingClientRect(this: HTMLElement) {
        if (this === modelSelector) return new DOMRect(100, 620, 180, 56);
        return new DOMRect(0, 0, 0, 0);
      });

    try {
      fireEvent.click(modelSelector);

      const modelList = document.getElementById(modelListId!);
      expect(modelList).toHaveAttribute("data-placement", "top");
      expect(modelList).toHaveStyle({
        insetBlockEnd: "160px",
        maxBlockSize: "592px",
        position: "fixed",
      });
    } finally {
      rectSpy.mockRestore();
    }
  });

  it("steps through the approved zoom levels in both directions and stops at the limits", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));
    const zoomIn = screen.getByRole("button", { name: "Увеличить масштаб" });
    const zoomOut = screen.getByRole("button", { name: "Уменьшить масштаб" });
    const readout = screen.getByRole("group", { name: "Масштаб изображения" }).querySelector("output");
    const canvas = screen.getByTestId("file-edit-mask").parentElement;

    expect(readout).toHaveTextContent(/^1×$/);
    expect(zoomOut).toBeDisabled();
    for (const level of [1.2, 1.5, 2, 3, 4, 5]) {
      expect(zoomIn).toBeEnabled();
      fireEvent.click(zoomIn);
      expect(readout?.textContent).toBe(`${level}×`);
      expect(canvas).toHaveStyle({ transform: `translate(0%, 0%) scale(${level})` });
    }
    expect(zoomIn).toBeDisabled();
    fireEvent.click(zoomIn);
    expect(readout).toHaveTextContent(/^5×$/);

    for (const level of [4, 3, 2, 1.5, 1.2, 1]) {
      expect(zoomOut).toBeEnabled();
      fireEvent.click(zoomOut);
      expect(readout?.textContent).toBe(`${level}×`);
    }
    expect(zoomOut).toBeDisabled();
    fireEvent.click(zoomOut);
    expect(readout).toHaveTextContent(/^1×$/);
  });

  it("reveals panning above 1x and resets the image position at 1x", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));
    const surface = screen.getByLabelText("Область редактирования изображения");
    const mask = screen.getByTestId("file-edit-mask");
    const canvas = mask.parentElement!;
    const zoomOut = screen.getByRole("button", { name: "Уменьшить масштаб" });
    const zoomIn = screen.getByRole("button", { name: "Увеличить масштаб" });
    expect(screen.queryByRole("button", { name: "Перемещать изображение" })).not.toBeInTheDocument();
    expect(zoomOut).toBeDisabled();

    fireEvent.click(zoomIn);
    const pan = screen.getByRole("button", { name: "Перемещать изображение" });
    expect(pan).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(pan);
    fireEvent.pointerDown(surface, { clientX: 50, clientY: 50, pointerId: 1 });
    fireEvent.pointerMove(surface, { clientX: 95, clientY: 5, pointerId: 1 });
    fireEvent.pointerUp(surface, { clientX: 95, clientY: 5, pointerId: 1 });

    expect(canvas).toHaveStyle({ transform: "translate(10%, -10%) scale(1.2)" });
    expect(mask.querySelectorAll("[data-edit-stroke]")).toHaveLength(0);
    expect(surface).toHaveAttribute("data-tool", "pan");

    fireEvent.click(zoomOut);
    expect(canvas).toHaveStyle({ transform: "translate(0%, 0%) scale(1)" });
    expect(screen.queryByRole("button", { name: "Перемещать изображение" })).not.toBeInTheDocument();
    expect(zoomOut).toBeDisabled();
    expect(surface).toHaveAttribute("data-tool", "brush");
    fireEvent.click(zoomIn);
    expect(screen.getByRole("button", { name: "Перемещать изображение" })).toHaveAttribute("aria-pressed", "false");
  });

  it("returns to drawing from pan mode and maps the brush to the shifted image", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));
    const surface = screen.getByLabelText("Область редактирования изображения");
    const mask = screen.getByTestId("file-edit-mask");
    const zoomIn = screen.getByRole("button", { name: "Увеличить масштаб" });
    fireEvent.click(zoomIn);
    fireEvent.click(zoomIn);
    fireEvent.click(screen.getByRole("button", { name: "Перемещать изображение" }));
    expect(screen.getByRole("button", { name: "Кисть" })).toHaveAttribute("aria-pressed", "false");
    fireEvent.pointerDown(surface, { clientX: 50, clientY: 50, pointerId: 1 });
    fireEvent.pointerMove(surface, { clientX: 55, clientY: 45, pointerId: 1 });
    fireEvent.pointerUp(surface, { clientX: 55, clientY: 45, pointerId: 1 });

    fireEvent.click(screen.getByRole("button", { name: "Кисть" }));
    expect(surface).toHaveAttribute("data-tool", "brush");
    expect(screen.getByRole("button", { name: "Перемещать изображение" })).toHaveAttribute("aria-pressed", "false");
    fireEvent.pointerDown(surface, { clientX: 50, clientY: 50, pointerId: 2 });
    fireEvent.pointerUp(surface, { clientX: 50, clientY: 50, pointerId: 2 });

    const point = mask.querySelector("[data-edit-stroke]")!.getAttribute("points")!.split(" ")[0].split(",").map(Number);
    expect(point[0]).toBeCloseTo(46.6667, 3);
    expect(point[1]).toBeCloseTo(53.3333, 3);
  });

  it("stops panning on pointer cancellation and keeps zoomed-out edges covered", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));
    const surface = screen.getByLabelText("Область редактирования изображения");
    const mask = screen.getByTestId("file-edit-mask");
    const zoomIn = screen.getByRole("button", { name: "Увеличить масштаб" });
    for (let step = 0; step < 6; step += 1) fireEvent.click(zoomIn);
    fireEvent.click(screen.getByRole("button", { name: "Перемещать изображение" }));
    fireEvent.pointerDown(surface, { clientX: 50, clientY: 50, pointerId: 1 });
    fireEvent.pointerMove(surface, { clientX: 400, clientY: 400, pointerId: 1 });
    fireEvent.pointerCancel(surface, { pointerId: 1 });
    fireEvent.pointerMove(surface, { clientX: 0, clientY: 0, pointerId: 1 });
    expect(mask.parentElement).toHaveStyle({ transform: "translate(200%, 200%) scale(5)" });
    expect(surface).toHaveAttribute("data-panning", "false");

    fireEvent.click(screen.getByRole("button", { name: "Уменьшить масштаб" }));
    expect(mask.parentElement).toHaveStyle({ transform: "translate(150%, 150%) scale(4)" });
    expect(mask.querySelectorAll("[data-edit-stroke]")).toHaveLength(0);
  });

  it("draws, undoes, redoes and clears an edit selection", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const editSurface = screen.getByLabelText("Область редактирования изображения");
    const editMask = screen.getByTestId("file-edit-mask");
    const undo = within(infoPanel).getByRole("button", { name: "Отменить выделение" });
    const redo = within(infoPanel).getByRole("button", { name: "Повторить выделение" });
    const clear = within(infoPanel).getByRole("button", { name: "Очистить выделение" });

    expect(editMask.querySelectorAll("[data-edit-stroke]")).toHaveLength(0);

    fireEvent.pointerDown(editSurface, { clientX: 20, clientY: 20, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });
    fireEvent.pointerUp(editSurface, { clientX: 60, clientY: 60, pointerId: 1 });

    expect(editMask.querySelectorAll("[data-edit-stroke]")).toHaveLength(1);
    expect(editMask.querySelector("[data-edit-stroke]")).toHaveAttribute("stroke-width", "32");
    expect(editMask.querySelector("[data-edit-stroke]")).toHaveAttribute(
      "vector-effect",
      "non-scaling-stroke",
    );
    expect(undo).toBeEnabled();
    expect(clear).toBeEnabled();

    fireEvent.click(undo);
    expect(editMask.querySelectorAll("[data-edit-stroke]")).toHaveLength(0);
    expect(redo).toBeEnabled();

    fireEvent.click(redo);
    expect(editMask.querySelectorAll("[data-edit-stroke]")).toHaveLength(1);

    fireEvent.click(clear);
    expect(editMask.querySelectorAll("[data-edit-stroke]")).toHaveLength(0);
  });

  it("erases continuously with the same diameter without deleting the brush stroke", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const editSurface = screen.getByLabelText("Область редактирования изображения");
    const editMask = screen.getByTestId("file-edit-mask");

    fireEvent.pointerDown(editSurface, { clientX: 20, clientY: 20, pointerId: 1 });
    fireEvent.pointerMove(editSurface, { clientX: 80, clientY: 80, pointerId: 1 });
    fireEvent.pointerUp(editSurface, { clientX: 80, clientY: 80, pointerId: 1 });

    fireEvent.click(within(infoPanel).getByRole("button", { name: "Ластик" }));
    fireEvent.pointerDown(editSurface, { clientX: 60, clientY: 60, pointerId: 2 });
    fireEvent.pointerMove(editSurface, { clientX: 70, clientY: 70, pointerId: 2 });
    fireEvent.pointerUp(editSurface, { clientX: 70, clientY: 70, pointerId: 2 });

    expect(editMask.querySelector('[data-edit-tool="brush"]')).toBeInTheDocument();
    expect(editMask.querySelector('[data-edit-tool="eraser"]')).toHaveAttribute(
      "stroke-width",
      "32",
    );
    expect(editMask.querySelector('[data-edit-tool="eraser"]')).toHaveAttribute(
      "vector-effect",
      "non-scaling-stroke",
    );

    fireEvent.click(within(infoPanel).getByRole("button", { name: "Отменить выделение" }));

    expect(editMask.querySelector('[data-edit-tool="brush"]')).toBeInTheDocument();
    expect(editMask.querySelector('[data-edit-tool="eraser"]')).not.toBeInTheDocument();
  });

  it("shows the brush size cursor for the brush and eraser", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const editSurface = screen.getByLabelText("Область редактирования изображения");
    const toolCursor = screen.getByTestId("file-edit-tool-cursor");

    expect(toolCursor).toHaveAttribute("data-visible", "false");

    fireEvent.pointerMove(editSurface, { clientX: 24, clientY: 30, pointerId: 1 });

    expect(toolCursor).toHaveAttribute("data-tool", "brush");
    expect(toolCursor).toHaveAttribute("data-visible", "true");
    expect(toolCursor).toHaveStyle({
      height: "32px",
      left: "24%",
      top: "30%",
      width: "32px",
    });

    fireEvent.click(within(infoPanel).getByRole("button", { name: "Ластик" }));

    expect(toolCursor).toHaveAttribute("data-tool", "eraser");
    expect(toolCursor).toHaveAttribute("data-visible", "true");

    fireEvent.pointerLeave(editSurface);

    expect(toolCursor).toHaveAttribute("data-visible", "false");
    expect(toolCursor).toHaveStyle({
      left: "24%",
      top: "30%",
    });
  });

  it("previews the brush size at the center of the image while it is adjusted", () => {
    vi.useFakeTimers();
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const slider = within(infoPanel).getByRole("slider", { name: "Размер кисти" });
    const sizePreview = screen.getByTestId("file-edit-brush-size-preview");

    expect(sizePreview).toHaveAttribute("data-visible", "false");

    fireEvent.change(slider, { target: { value: "64" } });

    expect(slider.parentElement?.style.getPropertyValue("--range-slider-fill")).toBe(
      "calc(77.7778% - 37.3333px + 24px)",
    );
    expect(sizePreview).toHaveAttribute("data-visible", "true");
    expect(sizePreview).toHaveStyle({
      height: "64px",
      left: "50%",
      top: "50%",
      width: "64px",
    });

    act(() => vi.advanceTimersByTime(700));

    expect(sizePreview).toHaveAttribute("data-visible", "false");
  });

  it("shows the brush size above the thumb while it changes and hides it after 600ms", () => {
    vi.useFakeTimers();
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

    const infoPanel = screen.getByRole("complementary");
    const slider = within(infoPanel).getByRole("slider", { name: "Размер кисти" });
    const valueBubble = screen.getByTestId("range-slider-value");

    expect(valueBubble).toHaveAttribute("data-visible", "false");

    fireEvent.change(slider, { target: { value: "64" } });

    expect(valueBubble).toHaveTextContent("64");
    expect(valueBubble).toHaveAttribute("data-visible", "true");

    act(() => vi.advanceTimersByTime(599));
    expect(valueBubble).toHaveAttribute("data-visible", "true");

    act(() => vi.advanceTimersByTime(1));
    expect(valueBubble).toHaveAttribute("data-visible", "false");
  });

  it("writes the selected button geometry directly to the moving indicator", () => {
    const makeRect = (left: number, width: number) => new DOMRect(left, 0, width, 44);
    const rectSpy = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function getBoundingClientRect(this: HTMLElement) {
        if (this.textContent === "Общая") return makeRect(100, 100);
        if (this.textContent === "Редактировать") return makeRect(500, 160);
        if (this.dataset.scrollAreaViewport === "true") return makeRect(100, 600);
        return makeRect(0, 0);
      });

    try {
      renderDialog();
      fireEvent.click(screen.getByRole("button", { name: "Редактировать" }));

      const indicator = screen.getByTestId("mode-switch-panel-indicator");
      expect(indicator).toHaveStyle({
        inlineSize: "160px",
        transform: "translate3d(400px, 0, 0)",
      });
    } finally {
      rectSpy.mockRestore();
    }
  });

  it("uses the provided icon for every preview tool", () => {
    renderDialog();

    const generalIcon = screen.getByRole("button", { name: "Общая" }).querySelector("img");
    const animateIcon = screen.getByRole("button", { name: "Оживить" }).querySelector("img");
    const enhanceIcon = screen.getByRole("button", { name: "Улучшить" }).querySelector("img");
    const removeBackgroundIcon = screen
      .getByRole("button", { name: "Удалить фон" })
      .querySelector("img");
    const editIcon = screen.getByRole("button", { name: "Редактировать" }).querySelector("img");
    expect(generalIcon).toHaveAttribute("src", "/assets/icons/ui/general-white.svg");
    expect(animateIcon).toHaveAttribute("src", "/assets/icons/ui/animate-white.svg");
    expect(enhanceIcon).toHaveAttribute("src", "/assets/icons/ui/enhance-white.svg");
    expect(removeBackgroundIcon).toHaveAttribute(
      "src",
      "/assets/icons/ui/remove-background-white.svg",
    );
    expect(editIcon).toHaveAttribute("src", "/assets/icons/ui/edit-white.svg");
    expect(screen.queryByTestId("file-tool-placeholder-icon")).not.toBeInTheDocument();
  });

  it("uses the platform share sheet when it is available", async () => {
    const share = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "share", {
      configurable: true,
      value: share,
    });

    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Поделиться файлом" }));

    await vi.waitFor(() => {
      expect(share).toHaveBeenCalledWith({
        title: job.prompt,
        url: new URL(`/web/v1/image-artifacts/${artifact.id}`, window.location.origin).toString(),
      });
    });
  });

  it("builds the recreate link from the selected file generation settings", () => {
    renderDialog();

    const recreate = screen.getByRole("link", { name: "Пересоздать" });
    const url = new URL(recreate.getAttribute("href")!, window.location.origin);

    expect(url.pathname).toBe("/ru/app/image");
    expect(url.searchParams.get("model")).toBe(job.model_id);
    expect(url.searchParams.get("prompt")).toBe(job.prompt);
    expect(url.searchParams.get("quality")).toBe(job.image_quality);
    expect(recreate.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/star-white.svg",
    );
  });

  it("renders every thumbnail and selects one directly", () => {
    const onSelect = vi.fn();
    renderDialog({ onSelect, selectedIndex: 1 });

    const thumbnails = screen.getAllByRole("button", { name: /^Выбрать файл:/ });
    expect(thumbnails).toHaveLength(3);
    expect(thumbnails[1]).toHaveAttribute("aria-current", "true");

    fireEvent.click(screen.getByRole("button", { name: `Выбрать файл: ${items[2]!.job.prompt}` }));
    expect(onSelect).toHaveBeenCalledWith(2);
  });

  it("distinguishes multiple artifacts from the same generation", () => {
    renderDialog({
      previewItems: [
        items[0]!,
        { artifact: { ...artifact, id: "50821e28-171f-4aa8-b4ba-8d345532471d" }, job },
      ],
    });

    expect(screen.getByRole("button", {
      name: `Выбрать файл: ${job.prompt} (1 из 2)`,
    })).toBeInTheDocument();
    expect(screen.getByRole("button", {
      name: `Выбрать файл: ${job.prompt} (2 из 2)`,
    })).toBeInTheDocument();
  });

  it("keeps unavailable thumbnails visible and skips them during navigation", () => {
    const onSelect = vi.fn();
    renderDialog({
      onSelect,
      previewItems: [
        items[0]!,
        { job: items[1]!.job, state: "loading" },
        { job: items[2]!.job, state: "unavailable" },
        {
          artifact: { ...artifact, id: "7cd1fce9-df71-4484-93cb-52d5672586a4" },
          job: { ...job, id: "cc4beef6-1606-434c-baa9-36165a164eb3", prompt: "Морской берег" },
        },
      ],
    });

    expect(screen.getByRole("button", { name: "Загружаем файл: Озеро в горах" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Файл недоступен: Туманный лес" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Следующий файл" }));
    expect(onSelect).toHaveBeenCalledWith(3);
  });

  it("does not open the preview when no file is ready", () => {
    renderDialog({
      previewItems: [
        { job: items[0]!.job, state: "loading" },
        { job: items[1]!.job, state: "unavailable" },
      ],
    });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("wraps on-screen and keyboard navigation while the dialog is open", () => {
    const onSelect = vi.fn();
    const view = renderDialog({ onSelect });

    fireEvent.click(screen.getByRole("button", { name: "Следующий файл" }));
    expect(onSelect).toHaveBeenLastCalledWith(1);

    fireEvent.keyDown(document, { key: "ArrowLeft" });
    expect(onSelect).toHaveBeenLastCalledWith(2);

    fireEvent.click(screen.getByRole("button", { name: "Предыдущий файл" }));
    expect(onSelect).toHaveBeenLastCalledWith(2);

    const editable = document.createElement("input");
    document.body.append(editable);
    const callCount = onSelect.mock.calls.length;
    fireEvent.keyDown(editable, { key: "ArrowRight" });
    expect(onSelect).toHaveBeenCalledTimes(callCount);
    editable.remove();

    view.unmount();
    fireEvent.keyDown(document, { key: "ArrowRight" });
    expect(onSelect).toHaveBeenCalledTimes(callCount);
  });

  it("shows selected-file prompt metadata and supplied action icons", () => {
    renderDialog();

    expect(screen.getByTestId("file-preview-info-viewport"))
      .toBe(screen.getByRole("complementary"));
    expect(screen.getByRole("complementary"))
      .toHaveAttribute("data-scroll-area-viewport", "true");
    expect(screen.getByRole("heading", { name: "Промпт для генерации" })).toBeInTheDocument();
    expect(screen.getByText(job.prompt)).toBeInTheDocument();
    expect(screen.getByText("1024 × 1536")).toBeInTheDocument();
    expect(screen.getByText("PNG")).toBeInTheDocument();
    expect(screen.getByText("42 Б")).toBeInTheDocument();
    expect(screen.getByText("2K")).toBeInTheDocument();
    expect(screen.getByText(new Intl.DateTimeFormat("ru-RU", {
      dateStyle: "medium",
      timeZone: "UTC",
    }).format(new Date(job.created_at)))).toBeInTheDocument();

    const copy = screen.getByRole("button", { name: "Копировать" });
    const download = screen.getByRole("link", { name: "Скачать файл" });
    const share = screen.getByRole("button", { name: "Поделиться файлом" });
    expect(copy.querySelector("img")).toHaveAttribute("src", "/assets/icons/ui/copy-white.svg");
    expect(download.querySelector("img")).toHaveAttribute("src", "/assets/icons/ui/download-white.svg");
    expect(share.querySelector("img")).toHaveAttribute("src", "/assets/icons/ui/repost-white.svg");
    expect(screen.getByRole("button", { name: "Оживить" })).not.toHaveAttribute("title");
    expect(screen.getByRole("button", { name: "Улучшить" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "Удалить фон" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "Редактировать" })).not.toBeDisabled();
  });

  it("copies the prompt and temporarily reports success", async () => {
    vi.useFakeTimers();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    renderDialog();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Копировать" }));
      await Promise.resolve();
    });

    expect(writeText).toHaveBeenCalledWith(job.prompt);
    expect(screen.getByRole("button", { name: "Скопировано" })).toBeInTheDocument();

    act(() => vi.advanceTimersByTime(2_000));
    expect(screen.getByRole("button", { name: "Копировать" })).toBeInTheDocument();
  });

  it("expands and collapses a prompt that exceeds five lines", () => {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(160);
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(100);
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
    expect(screen.getByRole("button", { name: "Свернуть" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Свернуть" }));
    expect(screen.getByRole("button", { name: "Показать ещё" })).toBeInTheDocument();
  });

  it("remeasures prompt overflow when the viewport changes", () => {
    let scrollHeight = 100;
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockImplementation(() => scrollHeight);
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(100);
    renderDialog();

    expect(screen.queryByRole("button", { name: "Показать ещё" })).toBeNull();

    scrollHeight = 160;
    fireEvent(window, new Event("resize"));
    expect(screen.getByRole("button", { name: "Показать ещё" })).toBeInTheDocument();
  });

  it("keeps keyboard focus inside the open dialog", () => {
    renderDialog();

    const firstControl = screen.getByRole("button", { name: "Закрыть предпросмотр" });
    const lastControl = screen.getByRole("button", { name: "Поделиться файлом" });

    firstControl.focus();
    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(lastControl).toHaveFocus();

    lastControl.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(firstControl).toHaveFocus();
  });

  it("uses a horizontal thumbnail rail on a narrow viewport", () => {
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({
      addEventListener: vi.fn(),
      matches: true,
      removeEventListener: vi.fn(),
    }));

    renderDialog();

    expect(screen.getByRole("navigation", { name: "Файлы для просмотра" })
      .closest("[data-orientation]"))
      .toHaveAttribute("data-orientation", "horizontal");
  });

  it("omits resolution metadata when dimensions are unavailable", () => {
    renderDialog({
      previewItems: [{
        artifact: { ...artifact, height: 0, width: 0 },
        job,
      }],
    });

    expect(screen.queryByText("Разрешение")).toBeNull();
  });
});
