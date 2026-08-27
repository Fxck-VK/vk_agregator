import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/features/models/image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn(),
}));

import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";

import { FeaturedModelShortcuts } from "./FeaturedModelShortcuts";

const catalogue = {
  items: [
    {
      id: "nano / banana",
      name: "Nano / Banana",
      quality_options: ["1K"],
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 1,
    },
    {
      id: "second-model",
      name: "Second Model",
      quality_options: ["1K"],
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "third-model",
      name: "Third Model",
      quality_options: ["2K"],
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 2,
    },
    {
      id: "fourth-model",
      name: "Fourth Model",
      quality_options: ["4K"],
      default_quality: "4K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "fifth-model",
      name: "Fifth Model",
      quality_options: ["1K"],
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
  ],
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("FeaturedModelShortcuts", () => {
  it("links the first four available models and uses the shared fallback artwork", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue(catalogue);
    render(<FeaturedModelShortcuts />);

    const shortcuts = await screen.findAllByTestId("featured-model-shortcut");

    expect(shortcuts).toHaveLength(4);
    expect(shortcuts.map((shortcut) => shortcut.textContent)).toEqual([
      "Nano / Banana",
      "Second Model",
      "Third Model",
      "Fourth Model",
    ]);
    expect(screen.queryByText("Fifth Model")).toBeNull();
    expect(screen.getByRole("link", { name: "Открыть генератор: Nano / Banana" })).toHaveAttribute(
      "href",
      "/app/image?model=nano%20%2F%20banana",
    );
    expect(within(shortcuts[0]).getByTestId("model-icon")).toHaveAttribute(
      "src",
      expect.stringContaining("default-model-87465de8.png"),
    );
  });

  it("uses supplied artwork for a matching model id", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue(catalogue);
    render(<FeaturedModelShortcuts artworkByModelId={{ "nano / banana": "/assets/images/models/custom.png" }} />);

    const shortcut = await screen.findByRole("link", { name: "Открыть генератор: Nano / Banana" });

    expect(within(shortcut).getByTestId("model-icon")).toHaveAttribute(
      "src",
      expect.stringContaining("custom.png"),
    );
  });

  it("renders four inert placeholders while the catalogue is loading", () => {
    vi.mocked(loadImageModelCatalog).mockReturnValue(new Promise(() => {}));
    render(<FeaturedModelShortcuts />);

    expect(screen.getAllByTestId("featured-model-shortcut-skeleton")).toHaveLength(4);
    expect(screen.queryByTestId("featured-model-shortcut")).toBeNull();
  });

  it.each([
    ["a failed catalogue", () => Promise.reject(new Error("offline"))],
    ["an empty catalogue", () => Promise.resolve({ items: [] })],
  ])("renders no fake model shortcuts for %s", async (_caseName, load) => {
    vi.mocked(loadImageModelCatalog).mockImplementationOnce(load);
    render(<FeaturedModelShortcuts />);

    await waitFor(() => expect(screen.queryAllByTestId("featured-model-shortcut-skeleton")).toHaveLength(0));
    expect(screen.queryByTestId("featured-model-shortcut")).toBeNull();
  });
});
