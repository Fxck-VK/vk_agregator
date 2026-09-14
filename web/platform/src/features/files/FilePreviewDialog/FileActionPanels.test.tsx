import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { FileAnimationPanel } from "./FileAnimationPanel";
import { FileEditorPanel, useFileEditorController } from "./FileEditorPanel";
import { makeModelCatalogFixture } from "@/test/model-catalog";
import { parseModelCatalog } from "@/features/models/model-catalog-contract";
import {
  resetFileActionModelCatalogLoaderForTests,
  setFileActionModelCatalogLoaderForTests,
  type WorkspaceModelCatalog,
} from "./file-action-models";

const emptyCatalog: WorkspaceModelCatalog = {
  default_model_id: "",
  items: [],
  schema_version: 1,
};

function TestEditorPanel() {
  const controller = useFileEditorController();
  return <FileEditorPanel controller={controller} />;
}

describe("file action panels", () => {
  it("opens and selects a real catalog-backed editor model", async () => {
    const catalog = parseModelCatalog(makeModelCatalogFixture({ images: ["First", "Second"].map(name => ({
      id: name.toLowerCase(), name, quality_options: ["1K"], default_quality: "1K",
      supports_reference_image: false, max_reference_images: 0,
    })) }));
    for (const model of catalog.items) {
      model.operations[0].id = "edit";
      model.operations[0].inputs.images = { support: "supported", enabled: true, max_count: 1 };
    }
    setFileActionModelCatalogLoaderForTests(async () => catalog);
    render(<TestEditorPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "Выбрана модель First. Открыть список" }));
    const choices = screen.getAllByRole("button", { name: /^Second/ });
    expect(choices).toHaveLength(2); // The server lists this model in popular and images.
    fireEvent.click(choices[0]);
    expect(screen.getByRole("button", { name: "Выбрана модель Second. Открыть список" })).toBeVisible();
  });
  afterEach(() => {
    cleanup();
    resetFileActionModelCatalogLoaderForTests();
  });

  it("keeps the animation action disabled when no catalog operation supports file input", async () => {
    setFileActionModelCatalogLoaderForTests(async () => emptyCatalog);

    render(<FileAnimationPanel />);

    const unavailableAction = await screen.findByRole("button", { name: "Оживить недоступно" });
    expect(unavailableAction).toBeDisabled();
    expect(screen.getByRole("button", { name: "Нейросети временно недоступны" })).toBeDisabled();
    expect(screen.queryByText("Генератор видео")).toBeNull();
    expect(screen.queryByText("Google Veo 3.1")).toBeNull();
  });

  it("keeps local editor tools usable while disabling catalog-backed edit submission", async () => {
    setFileActionModelCatalogLoaderForTests(async () => emptyCatalog);

    render(<TestEditorPanel />);

    const panel = screen.getByRole("region", { name: "Настройки редактирования" });
    expect(within(panel).getByRole("button", { name: "Кисть" })).toHaveAttribute("aria-pressed", "true");
    expect(within(panel).getByRole("slider", { name: "Размер кисти" })).toHaveValue("32");

    const unavailableSubmit = await within(panel).findByRole("button", {
      name: "Редактировать недоступно",
    });
    expect(unavailableSubmit).toBeDisabled();
    expect(within(panel).getByRole("button", {
      name: "Нейросети временно недоступны",
    })).toBeDisabled();
    expect(within(panel).queryByText("Nano Banana Pro")).toBeNull();
    expect(within(panel).queryByRole("button", { name: /Разрешение:/ })).toBeNull();
    expect(within(panel).queryByRole("button", { name: /Соотношение сторон:/ })).toBeNull();
  });
});
