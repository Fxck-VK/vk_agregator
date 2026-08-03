import { describe, expect, it } from "vitest";

import type { ImageModel } from "@/lib/web-api/contracts";

import { filterAndSortImageModels, filterImageModels, imageModelQualities } from "./model-filters";

const models: ImageModel[] = [
  {
    id: "nano-banana-2",
    name: "Nano Banana",
    quality_options: ["1K", "2K"],
    default_quality: "1K",
    supports_reference_image: true,
    max_reference_images: 1,
  },
  {
    id: "MODEL-ID",
    name: "Other Model",
    quality_options: ["4K"],
    default_quality: "4K",
    supports_reference_image: false,
    max_reference_images: 0,
  },
  {
    id: "third-model",
    name: "Third Model",
    quality_options: ["2K", "8K"],
    default_quality: "2K",
    supports_reference_image: true,
    max_reference_images: 2,
  },
];

describe("filterImageModels", () => {
  it("matches a trimmed query by model name and combines reference and quality filters", () => {
    expect(filterImageModels(models, { query: " banana ", referenceOnly: true, quality: "2K" })).toEqual([models[0]]);
  });

  it("matches a query by model id without applying optional filters", () => {
    expect(filterImageModels(models, { query: "model-id", referenceOnly: false, quality: null })).toEqual([models[1]]);
  });
});

describe("filterAndSortImageModels", () => {
  it("orders only matching models by name without mutating the cached catalog", () => {
    const unsortedModels = [models[2], models[0], models[1]];

    const result = filterAndSortImageModels(
      unsortedModels,
      { query: "", referenceOnly: false, quality: null },
      "name",
    );

    expect(result.map((model) => model.name)).toEqual(["Nano Banana", "Other Model", "Third Model"]);
    expect(unsortedModels.map((model) => model.name)).toEqual(["Third Model", "Nano Banana", "Other Model"]);
  });
});

describe("imageModelQualities", () => {
  it("returns distinct qualities in source order", () => {
    expect(imageModelQualities(models)).toEqual(["1K", "2K", "4K", "8K"]);
  });
});
