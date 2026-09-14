import { describe, expect, it } from "vitest";

import type { ModelCatalogModel } from "./model-filters";

import { filterAndSortCatalogModels, filterCatalogModels } from "./model-filters";

const models: ModelCatalogModel[] = [
  {
    id: "nano-banana-2",
    name: "Nano Banana",
    categories: ["popular", "images"],
    quality_options: ["1K", "2K"],
    default_quality: "1K",
    supports_reference_image: true,
    max_reference_images: 1,
  },
  {
    id: "MODEL-ID",
    name: "Other Model",
    categories: ["text"],
    quality_options: ["4K"],
    default_quality: "4K",
    supports_reference_image: false,
    max_reference_images: 0,
  },
  {
    id: "third-model",
    name: "Third Model",
    categories: ["text", "study-work"],
    quality_options: ["2K", "8K"],
    default_quality: "2K",
    supports_reference_image: true,
    max_reference_images: 2,
  },
];

describe("filterCatalogModels", () => {
  it("matches a trimmed query by model name inside the selected server category", () => {
    expect(filterCatalogModels(models, { category: "images", query: " banana " })).toEqual([models[0]]);
  });

  it("matches a query by model id without inferring another category", () => {
    expect(filterCatalogModels(models, { category: "images", query: "model-id" })).toEqual([]);
    expect(filterCatalogModels(models, { category: "text", query: "model-id" })).toEqual([models[1]]);
  });
});

describe("filterAndSortCatalogModels", () => {
  it("orders only matching models by name without mutating the cached catalog", () => {
    const unsortedModels = [models[2], models[0], models[1]];

    const result = filterAndSortCatalogModels(
      unsortedModels,
      { category: "text", query: "" },
      "name",
    );

    expect(result.map((model) => model.name)).toEqual(["Other Model", "Third Model"]);
    expect(unsortedModels.map((model) => model.name)).toEqual(["Third Model", "Nano Banana", "Other Model"]);
  });
});
