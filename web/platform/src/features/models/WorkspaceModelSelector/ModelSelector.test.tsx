import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";

import { ModelSelector, type ModelSelectorModel } from "./ModelSelector";

const taskModels: readonly ModelSelectorModel[] = [
  {
    category: "video",
    id: "video-generator",
    name: "Генератор видео",
  },
  {
    category: "video",
    id: "google-veo-3-1",
    name: "Google Veo 3.1",
  },
  {
    category: "video",
    id: "kling-3",
    name: "Kling 3.0",
  },
];

describe("ModelSelector", () => {
  afterEach(cleanup);

  it("exposes the panel variant on the shared selector root", () => {
    render(
      <ModelSelector
        models={taskModels}
        onSelect={vi.fn()}
        selectedModelId="video-generator"
        variant="panel"
      />,
    );

    expect(screen.getByRole("button", { name: /Генератор видео/ }).parentElement).toHaveAttribute(
      "data-variant",
      "panel",
    );
  });

  it.each([
    ["М".repeat(40), "М".repeat(40)],
    ["М".repeat(41), `${"М".repeat(39)}…`],
    ["🧠".repeat(41), `${"🧠".repeat(39)}…`],
  ])("limits the compact label while keeping the full accessible name: %s", (name, visibleName) => {
    render(<ModelSelector models={[{ ...taskModels[0], name }]} onSelect={vi.fn()} selectedModelId="video-generator" />);

    const trigger = screen.getByRole("button", { name: `Выбрана нейросеть ${name}. Открыть список` });
    expect(trigger.textContent).toBe(visibleName);
    fireEvent.click(trigger);
    expect(within(screen.getByRole("dialog")).getByText(name, { exact: true })).toBeInTheDocument();
  });

  it("keeps the complete name in the file editor panel variant", () => {
    const name = "Полное название модели для редактора фотографий длиннее сорока символов";
    render(<ModelSelector models={[{ ...taskModels[0], name }]} onSelect={vi.fn()} selectedModelId="video-generator" variant="panel" />);

    expect(screen.getByRole("button", { name: `Выбрана нейросеть ${name}. Открыть список` }).textContent).toBe(name);
  });

  it("uses the workspace selector view for a controlled task-specific model set", () => {
    const onSelect = vi.fn();

    render(
      <ModelSelector
        dialogLabel="Выбор нейросети для «Оживить»"
        models={taskModels}
        onSelect={onSelect}
        selectedModelId="video-generator"
        triggerAriaLabel={(name, isOpen) => (
          `Выбрана модель ${name}. ${isOpen ? "Закрыть" : "Открыть"} список`
        )}
      />,
    );

    const trigger = screen.getByRole("button", {
      name: "Выбрана модель Генератор видео. Открыть список",
    });
    fireEvent.click(trigger);

    const dialog = screen.getByRole("dialog", { name: "Выбор нейросети для «Оживить»" });
    expect(screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel })).toHaveFocus();
    expect(within(dialog).getAllByRole("heading", { level: 2 }).map((heading) => heading.textContent)).toEqual([
      "Популярные",
      "Изображения",
      "Текст",
      "Видео",
      "Аудио",
    ]);
    expect(within(dialog).getByRole("button", { name: /Генератор видео/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(within(dialog).getByRole("button", { name: /Kling 3.0/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel }), {
      target: { value: "Veo" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /Google Veo 3.1/ }));

    expect(onSelect).toHaveBeenCalledExactlyOnceWith(taskModels[1]);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
});
