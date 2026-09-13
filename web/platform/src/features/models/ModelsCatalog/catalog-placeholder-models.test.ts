import { describe, expect, it } from "vitest";

import { imageModelListSchema } from "@/lib/web-api/contracts";

import {
  catalogPlaceholderModels,
  isCatalogPlaceholderModel,
} from "./catalog-placeholder-models";

describe("catalog placeholder models", () => {
  it("provides exactly four schema-valid catalogue-only cards", () => {
    expect(catalogPlaceholderModels).toHaveLength(4);
    expect(imageModelListSchema.safeParse({ items: catalogPlaceholderModels }).success).toBe(true);
    expect(catalogPlaceholderModels.every(isCatalogPlaceholderModel)).toBe(true);
  });
});
