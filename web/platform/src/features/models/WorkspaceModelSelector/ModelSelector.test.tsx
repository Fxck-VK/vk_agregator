import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";
import previewCatalog from "@/features/session/model-catalog.preview.json";
import { parseModelCatalog, projectChatModelCatalog } from "../model-catalog-contract";

import { ModelSelector, type ModelSelectorModel } from "./ModelSelector";

const taskModels: readonly ModelSelectorModel[] = [
  {
    categories: ["popular", "video-audio"],
    category: "video",
    id: "video-generator",
    name: "Генератор видео",
  },
  {
    categories: ["popular", "video-audio"],
    category: "video",
    id: "google-veo-3-1",
    name: "Google Veo 3.1",
  },
  {
    categories: ["popular", "video-audio"],
    category: "video",
    id: "kling-3",
    name: "Kling 3.0",
  },
];

function popularOptions(dialog = screen.getByRole("dialog")) {
  return within(within(dialog).getByRole("region", { name: "Популярные" }));
}

describe("ModelSelector", () => {
  afterEach(cleanup);

  it("keeps native and application capabilities in the shared model picker", () => {
    const text = projectChatModelCatalog(parseModelCatalog(previewCatalog)).items.find((model) => model.id === "gpt_5_5")!;
    const model: ModelSelectorModel = { ...text, category: "text" };
    render(<ModelSelector models={[model]} onSelect={vi.fn()} selectedModelId={model.id} />);
    fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
    fireEvent.click(screen.getByText("Возможности модели"));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByText("В этом интерфейсе")).toBeVisible();
    expect(dialog.getByText("В API провайдера")).toBeVisible();
  });

  it("keeps descriptions inline by default, including on hover", () => {
    const model = { ...taskModels[0], description: "Описание возможностей модели" };
    render(<ModelSelector models={[model]} onSelect={vi.fn()} selectedModelId={model.id} />);
    fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
    const option = popularOptions().getByRole("button", { name: /Генератор видео/ });
    expect(within(option).getByText(model.description)).toBeVisible();
    fireEvent.pointerEnter(option, { pointerType: "mouse" });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("shows tooltip descriptions outside the scrolling dialog without changing selection", () => {
    const model = { ...taskModels[0], description: "Описание возможностей модели" };
    const onSelect = vi.fn();
    render(<ModelSelector descriptionMode="tooltip" models={[model]} onSelect={onSelect} selectedModelId={model.id} renderInPortal />);
    fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
    const dialog = screen.getByRole("dialog");
    const option = popularOptions(dialog).getByRole("button", { name: model.name });
    expect(option).toHaveAccessibleDescription(model.description);
    expect(within(option).getByText(model.description)).not.toBeVisible();
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.pointerEnter(option, { pointerType: "mouse" });
    const tooltip = screen.getByRole("tooltip");
    expect(tooltip).toHaveTextContent(model.description);
    expect(dialog).not.toContainElement(tooltip);
    expect(onSelect).not.toHaveBeenCalled();
    fireEvent.pointerLeave(option, { pointerType: "mouse" });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.focus(option);
    expect(screen.getByRole("tooltip")).toHaveTextContent(model.description);
    fireEvent.scroll(within(dialog).getByRole("region", { name: ru.modelSelector.feedLabel }));
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    fireEvent.blur(option);
    fireEvent.focus(option);
    expect(screen.getByRole("tooltip")).toBeInTheDocument();
    fireEvent.click(option);
    expect(onSelect).toHaveBeenCalledExactlyOnceWith(model);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("dismisses a model description when filtering or closing the selector", () => {
    const model = { ...taskModels[0], description: "Описание возможностей модели" };
    render(<ModelSelector descriptionMode="tooltip" models={[model]} onSelect={vi.fn()} selectedModelId={model.id} />);
    const trigger = screen.getByRole("button", { name: /Выбрана нейросеть/ });
    fireEvent.click(trigger);
    fireEvent.pointerEnter(popularOptions().getByRole("button", { name: model.name }));
    expect(screen.getByRole("tooltip")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "missing" } });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "" } });
    fireEvent.focus(popularOptions().getByRole("button", { name: model.name }));
    expect(screen.getByRole("tooltip")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("shows only the latest description when pointer hover follows keyboard focus", () => {
    render(<ModelSelector descriptionMode="tooltip" models={taskModels} onSelect={vi.fn()} selectedModelId={taskModels[0].id} />);
    fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
    const dialog = screen.getByRole("dialog");
    fireEvent.focus(popularOptions(dialog).getByRole("button", { name: taskModels[0].name }));
    fireEvent.pointerEnter(popularOptions(dialog).getByRole("button", { name: taskModels[1].name }));
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    expect(screen.getByRole("tooltip")).toHaveTextContent(taskModels[1].name);
  });

  it("keeps categories available for an empty search, supports keyboard selection and isolates instances", () => {
    render(<>
      <ModelSelector models={taskModels} onSelect={vi.fn()} selectedModelId="video-generator" />
      <ModelSelector models={taskModels} onSelect={vi.fn()} selectedModelId="kling-3" />
    </>);
    const triggers = screen.getAllByRole("button", { name: /Выбрана нейросеть/ });
    fireEvent.click(triggers[0]);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "missing" } });
    const popular = screen.getByRole("button", { name: "Популярные" });
    expect(within(screen.getByRole("toolbar")).getAllByRole("button")).toHaveLength(2);
    fireEvent.keyDown(popular, { key: "ArrowRight" });
    const video = screen.getByRole("button", { name: "Видео и аудио" });
    expect(video).toHaveFocus();
    expect(video).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("status")).toHaveTextContent(ru.modelSelector.empty);
    const firstPanelId = screen.getByRole("region", { name: ru.modelSelector.feedLabel }).id;
    fireEvent.click(triggers[1]);
    expect(screen.getAllByRole("region", { name: ru.modelSelector.feedLabel }).at(-1)?.id).not.toBe(firstPanelId);
  });

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
    expect(popularOptions().getByText(name, { exact: true })).toBeInTheDocument();
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
    expect(within(within(dialog).getByRole("toolbar")).getAllByRole("button").map((button) => button.textContent)).toEqual(
      ["Популярные", "Видео и аудио"],
    );
    expect(popularOptions(dialog).getByRole("button", { name: /Генератор видео/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(popularOptions(dialog).getByRole("button", { name: /Kling 3.0/ })).toHaveAttribute(
      "aria-pressed",
      "false",
    );

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel }), {
      target: { value: "Veo" },
    });
    fireEvent.click(popularOptions(dialog).getByRole("button", { name: /Google Veo 3.1/ }));

    expect(onSelect).toHaveBeenCalledExactlyOnceWith(taskModels[1]);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
});
