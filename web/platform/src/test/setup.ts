import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
});

afterEach(async () => {
  const cache = await import("@/features/models/model-catalog-cache");
  cache.resetModelCatalogCacheForTests?.();
});
