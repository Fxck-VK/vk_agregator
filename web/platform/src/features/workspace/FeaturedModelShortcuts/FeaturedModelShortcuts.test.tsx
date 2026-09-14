import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/features/models/generation-model-catalog", () => ({
  loadGenerationModelCatalog: vi.fn(),
}));

import { loadGenerationModelCatalog } from "@/features/models/generation-model-catalog";

import { FeaturedModelShortcuts } from "./FeaturedModelShortcuts";

const catalogue = {
  default_model_id: "chat-default",
  categoryErrors: {},
  items: [
    {
      id: "chat-default",
      name: "Loaded Chat Model",
      description: "Loaded text shortcut",
      category: "text",
      categories: ["popular", "text"],
    },
    {
      id: "nano / banana",
      name: "Nano / Banana",
      description: "Image one",
      category: "images",
      categories: ["images"],
      quality_options: ["1K"],
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 1,
    },
    {
      id: "second-model",
      name: "Second Model",
      description: "Image two",
      category: "images",
      categories: ["images"],
      quality_options: ["1K"],
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "third-model",
      name: "Third Model",
      description: "Image three",
      category: "images",
      categories: ["images"],
      quality_options: ["2K"],
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 2,
    },
    {
      id: "fourth-model",
      name: "Fourth Model",
      description: "Image four",
      category: "images",
      categories: ["images"],
      quality_options: ["4K"],
      default_quality: "4K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "fifth-model",
      name: "Fifth Model",
      description: "Image five",
      category: "images",
      categories: ["images"],
      quality_options: ["1K"],
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
  ],
} as Awaited<ReturnType<typeof loadGenerationModelCatalog>>;

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("FeaturedModelShortcuts", () => {
  it("selects the loaded chat shortcut plus the first four image models without links", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(catalogue);
    const onSelect = vi.fn();
    const onTextModelLoad = vi.fn();
    render(<FeaturedModelShortcuts onSelect={onSelect} onTextModelLoad={onTextModelLoad} selectedModelId="nano / banana" />);

    const shortcuts = await screen.findAllByTestId("featured-model-shortcut");

    expect(onTextModelLoad).toHaveBeenCalledWith(catalogue.items[0]);
    expect(shortcuts).toHaveLength(4);
    expect(shortcuts.map((shortcut) => shortcut.textContent)).toEqual([
      "Nano / Banana",
      "Second Model",
      "Third Model",
      "Fourth Model",
    ]);
    expect(screen.queryByText("Fifth Model")).toBeNull();
    expect(screen.queryByText("NeiroHub Chat")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Выбрать модель: Loaded Chat Model" })).toHaveAttribute("aria-pressed", "false");
    const button = screen.getByRole("button", { name: "Выбрать модель: Nano / Banana" });
    expect(button).not.toHaveAttribute("href");
    expect(button).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(button);
    expect(onSelect).toHaveBeenCalledWith(catalogue.items[1]);
    fireEvent.click(screen.getByRole("button", { name: "Выбрать модель: Loaded Chat Model" }));
    expect(onSelect).toHaveBeenLastCalledWith(catalogue.items[0]);
    expect(within(shortcuts[0]).getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(within(shortcuts[0]).queryByTestId("model-icon")).not.toBeInTheDocument();
  });

  it("renders four inert placeholders while the catalogue is loading", () => {
    vi.mocked(loadGenerationModelCatalog).mockReturnValue(new Promise(() => {}));
    render(<FeaturedModelShortcuts onSelect={vi.fn()} selectedModelId={null} />);

    expect(screen.getAllByTestId("featured-model-shortcut-skeleton")).toHaveLength(4);
    expect(screen.queryByTestId("featured-model-shortcut")).toBeNull();
  });

  it.each([
    ["a failed catalogue", () => Promise.reject(new Error("offline"))],
    ["an empty catalogue", () => Promise.resolve({ default_model_id: "", categoryErrors: {}, items: [] })],
  ])("renders no fake model shortcuts for %s", async (_caseName, load) => {
    vi.mocked(loadGenerationModelCatalog).mockImplementationOnce(load);
    render(<FeaturedModelShortcuts onSelect={vi.fn()} selectedModelId={null} />);

    await waitFor(() => expect(screen.queryAllByTestId("featured-model-shortcut-skeleton")).toHaveLength(0));
    expect(screen.queryByText("NeiroHub Chat")).not.toBeInTheDocument();
    expect(screen.queryByTestId("featured-model-shortcut")).toBeNull();
  });
});
