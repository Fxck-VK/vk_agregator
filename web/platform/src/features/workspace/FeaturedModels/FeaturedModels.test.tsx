import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/features/models/image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn(),
}));

import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import type { ImageModel } from "@/lib/web-api/contracts";

import { FeaturedModels } from "./FeaturedModels";

function createModel(index: number): ImageModel {
  return {
    id: `model-${index}`,
    name: `Модель ${index}`,
    quality_options: ["1K"],
    price_by_quality: { "1K": 20 + index },
    default_quality: "1K",
    supports_reference_image: false,
    max_reference_images: 0,
  };
}

describe("FeaturedModels", () => {
  beforeEach(() => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue({
      items: Array.from({ length: 6 }, (_, index) => createModel(index + 1)),
    });
  });

  it("reveals two more cards before offering the complete catalogue", async () => {
    render(<FeaturedModels />);

    expect(await screen.findAllByTestId("featured-model-card")).toHaveLength(4);
    expect(screen.getByRole("button", { name: "Показать ещё" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    expect(screen.queryByRole("link", { name: "Все нейросети" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));

    expect(screen.getAllByTestId("featured-model-card")).toHaveLength(6);
    expect(screen.queryByRole("button", { name: "Показать ещё" })).toBeNull();
    expect(screen.getByRole("link", { name: "Все нейросети" })).toHaveAttribute(
      "href",
      "/app/models",
    );
  });

  it("links directly to the catalogue when there are no hidden cards", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue({
      items: Array.from({ length: 4 }, (_, index) => createModel(index + 1)),
    });

    render(<FeaturedModels />);

    expect(await screen.findAllByTestId("featured-model-card")).toHaveLength(4);
    expect(screen.queryByRole("button", { name: "Показать ещё" })).toBeNull();
    expect(screen.getByRole("link", { name: "Все нейросети" })).toHaveAttribute(
      "href",
      "/app/models",
    );
  });
});
