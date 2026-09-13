import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
  useSearchParams: vi.fn(),
}));

vi.mock("@/features/models/image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn(),
}));

import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { ru } from "@/i18n/ru";
import type { ImageModelList } from "@/lib/web-api/contracts";
import { useRouter, useSearchParams } from "next/navigation";

import {
  useWorkspaceModelSelection,
  WorkspaceModelSelectionProvider,
} from "../WorkspaceModelSelection/WorkspaceModelSelection";
import { WorkspaceModelSelector } from "./WorkspaceModelSelector";

const catalogue: ImageModelList = {
  items: [
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 16, "2K": 60 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
    },
    {
      id: "gpt-image-2",
      name: "GPT Image 2",
      quality_options: ["1K"],
      price_by_quality: { "1K": 51 },
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "nano-banana-pro",
      name: "Nano Banana Pro",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 50, "2K": 80 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
    },
  ],
};

function SelectionControl() {
  const selection = useWorkspaceModelSelection();

  return (
    <button onClick={() => selection?.setSelectedModelId("gpt-image-2")} type="button">
      Select GPT outside
    </button>
  );
}

describe("WorkspaceModelSelector", () => {
  const push = vi.fn();

  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(useRouter).mockReturnValue({ push } as never);
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams() as never);
    vi.mocked(loadImageModelCatalog).mockResolvedValue(catalogue);
  });

  afterEach(() => {
    cleanup();
  });

  it("loads the safe catalogue once and opens a searchable image-model list", async () => {
    render(<WorkspaceModelSelector />);

    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });
    expect(loadImageModelCatalog).toHaveBeenCalledTimes(1);
    const chevron = trigger.querySelector('img[src="/assets/icons/ui/faq-arrow.svg"]');
    expect(chevron).toBeInTheDocument();
    expect(chevron).toHaveAttribute("width", "14");
    expect(chevron).toHaveAttribute("height", "8");
    expect(trigger.querySelector('img[src="/assets/icons/ui/chevron-down.svg"]')).not.toBeInTheDocument();
    expect(within(trigger).getByTestId("model-icon-fallback")).toBeInTheDocument();

    fireEvent.click(trigger);

    const dialog = screen.getByRole("dialog", { name: ru.modelSelector.dialogLabel });
    expect(dialog).toHaveAttribute("data-state", "open");
    const searchbox = screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel });
    expect(searchbox).toHaveFocus();
    expect(searchbox.parentElement?.querySelector(
      'img[src="/assets/icons/ui/search.svg"]',
    )).toBeInTheDocument();
    expect(searchbox.parentElement).not.toHaveTextContent("⌕");
    expect(screen.getByRole("heading", { name: ru.modelSelector.categories.images })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: /Nano Banana 2/ })).not.toHaveTextContent("✦");
    expect(within(dialog).getByRole("button", { name: /Nano Banana 2/ })).toHaveTextContent(
      "Быстрая генерация и редактирование изображений для повседневных задач",
    );
    const selectedOption = within(dialog).getByRole("button", { name: /Nano Banana 2/ });
    const unselectedOption = within(dialog).getByRole("button", { name: /GPT Image 2/ });
    expect(selectedOption).toHaveAttribute("aria-pressed", "true");
    expect(selectedOption.querySelector('[data-icon="check"]')).toBeNull();
    expect(unselectedOption).toHaveAttribute("aria-pressed", "false");
    expect(unselectedOption.querySelector('[data-icon="check"]')).toBeNull();
    expect(selectedOption).not.toHaveTextContent("●");
    expect(within(dialog).getAllByTestId("model-icon-fallback")).toHaveLength(3);
    const catalogueLink = screen.getByRole("link", { name: ru.modelSelector.openCatalogue });
    expect(catalogueLink).toHaveAttribute("href", "/app/models");
  });

  it("groups each available model once across the five ordered sections", async () => {
    render(<WorkspaceModelSelector />);
    fireEvent.click(await screen.findByRole("button", { name: new RegExp("Nano Banana 2") }));

    const dialog = screen.getByRole("dialog", { name: ru.modelSelector.dialogLabel });
    const headings = within(dialog).getAllByRole("heading", { level: 2 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "Популярные",
      "Изображения",
      "Текст",
      "Видео",
      "Аудио",
    ]);

    const popularSection = headings[0]?.closest("section");
    const imagesSection = headings[1]?.closest("section");
    expect(popularSection).not.toBeNull();
    expect(imagesSection).not.toBeNull();
    expect(within(popularSection as HTMLElement).getAllByRole("button").map((button) => button.textContent)).toEqual([
      expect.stringContaining("Nano Banana 2"),
      expect.stringContaining("GPT Image 2"),
    ]);
    expect(within(imagesSection as HTMLElement).getAllByRole("button").map((button) => button.textContent)).toEqual([
      expect.stringContaining("Nano Banana Pro"),
    ]);
    expect(within(dialog).getAllByRole("button", { name: /Nano Banana 2/ })).toHaveLength(1);
    expect(within(dialog).getAllByRole("button", { name: /GPT Image 2/ })).toHaveLength(1);
    expect(within(dialog).getAllByRole("button", { name: /Nano Banana Pro/ })).toHaveLength(1);
    expect(within(dialog).getAllByText("Скоро появятся")).toHaveLength(3);
  });

  it("filters locally, selects a model, closes, and navigates without a reload", async () => {
    render(<WorkspaceModelSelector />);
    fireEvent.click(await screen.findByRole("button", { name: new RegExp("Nano Banana 2") }));

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel }), {
      target: { value: "GPT" },
    });

    const dialog = screen.getByRole("dialog", { name: ru.modelSelector.dialogLabel });
    expect(within(dialog).queryByRole("button", { name: /Nano Banana 2/ })).toBeNull();
    const option = within(dialog).getByRole("button", { name: /GPT Image 2/ });
    expect(option).not.toHaveTextContent("✦");
    fireEvent.click(option);

    expect(push).toHaveBeenCalledExactlyOnceWith("/app/image?model=gpt-image-2");
    const closingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(closingDialog).toHaveAttribute("data-state", "closing");
    expect(closingDialog).toHaveAttribute("aria-hidden", "true");
    fireEvent.animationEnd(closingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    expect(document.getElementById("workspace-model-selector-dialog")).toBeNull();
    expect(screen.getByRole("button", { name: new RegExp("GPT Image 2") })).toHaveFocus();
  });

  it("reflects a model from a direct generator URL", async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams("model=gpt-image-2") as never);

    render(<WorkspaceModelSelector />);

    expect(await screen.findByRole("button", { name: new RegExp("GPT Image 2") })).toBeInTheDocument();
  });

  it("reflects a model changed by another workspace editor", async () => {
    render(
      <WorkspaceModelSelectionProvider>
        <WorkspaceModelSelector />
        <SelectionControl />
      </WorkspaceModelSelectionProvider>,
    );
    await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(screen.getByRole("button", { name: "Select GPT outside" }));

    expect(screen.getByRole("button", { name: new RegExp("GPT Image 2") })).toBeInTheDocument();
  });

  it("finishes the animated close after Escape or an outside press", async () => {
    render(
      <div>
        <WorkspaceModelSelector />
        <button type="button">Outside</button>
      </div>,
    );
    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: "Escape" });
    const escapeClosingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(escapeClosingDialog).toHaveAttribute("data-state", "closing");
    expect(escapeClosingDialog).toHaveAttribute("aria-hidden", "true");
    expect(trigger).toHaveFocus();
    fireEvent.animationEnd(escapeClosingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    expect(document.getElementById("workspace-model-selector-dialog")).toBeNull();

    fireEvent.click(trigger);
    fireEvent.pointerDown(screen.getByRole("button", { name: "Outside" }));
    const outsideClosingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(outsideClosingDialog).toHaveAttribute("data-state", "closing");
    fireEvent.animationEnd(outsideClosingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    await waitFor(() => {
      expect(document.getElementById("workspace-model-selector-dialog")).toBeNull();
    });
  });

  it("cancels an exit animation when the trigger reopens the selector", async () => {
    render(<WorkspaceModelSelector />);
    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(trigger);
    fireEvent.click(trigger);
    const closingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(closingDialog).toHaveAttribute("data-state", "closing");

    fireEvent.click(trigger);
    expect(closingDialog).toHaveAttribute("data-state", "open");
    fireEvent.animationEnd(closingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    expect(document.getElementById("workspace-model-selector-dialog")).toBe(closingDialog);
  });

  it("shows a truthful failure state and does not invent a model", async () => {
    vi.mocked(loadImageModelCatalog).mockRejectedValueOnce(new Error("offline"));

    render(<WorkspaceModelSelector />);

    expect(await screen.findByRole("button", { name: ru.modelSelector.unavailable })).toBeDisabled();
    expect(screen.queryByText("Nano Banana 2")).toBeNull();
  });
});
