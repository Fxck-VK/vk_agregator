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
    expect(trigger.querySelector('img[src="/assets/icons/ui/chevron-down.svg"]')).toBeInTheDocument();
    expect(within(trigger).getByTestId("model-icon-fallback")).toBeInTheDocument();

    fireEvent.click(trigger);

    const dialog = screen.getByRole("dialog", { name: ru.modelSelector.dialogLabel });
    expect(screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel })).toHaveFocus();
    expect(screen.getByRole("heading", { name: ru.modelSelector.imagesCategory })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: /Nano Banana 2/ })).toHaveTextContent("✦ 16");
    expect(within(dialog).getByRole("button", { name: /Nano Banana 2/ })).toHaveTextContent(
      ru.modelSelector.referenceDescription,
    );
    expect(within(dialog).getAllByTestId("model-icon-fallback")).toHaveLength(2);
    expect(screen.getByRole("link", { name: ru.modelSelector.openCatalogue })).toHaveAttribute("href", "/app/models");
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
    expect(option).toHaveTextContent("✦ 51");
    fireEvent.click(option);

    expect(push).toHaveBeenCalledExactlyOnceWith("/app/image?model=gpt-image-2");
    expect(screen.queryByRole("dialog", { name: ru.modelSelector.dialogLabel })).toBeNull();
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

  it("closes with Escape or an outside press and restores trigger focus", async () => {
    render(
      <div>
        <WorkspaceModelSelector />
        <button type="button">Outside</button>
      </div>,
    );
    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: ru.modelSelector.dialogLabel })).toBeNull();
    expect(trigger).toHaveFocus();

    fireEvent.click(trigger);
    fireEvent.pointerDown(screen.getByRole("button", { name: "Outside" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: ru.modelSelector.dialogLabel })).toBeNull());
  });

  it("shows a truthful failure state and does not invent a model", async () => {
    vi.mocked(loadImageModelCatalog).mockRejectedValueOnce(new Error("offline"));

    render(<WorkspaceModelSelector />);

    expect(await screen.findByRole("button", { name: ru.modelSelector.unavailable })).toBeDisabled();
    expect(screen.queryByText("Nano Banana 2")).toBeNull();
  });
});
