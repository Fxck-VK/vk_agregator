import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// jsdom does not implement Web Locks. Keep tests on the browser coordination path.
Object.defineProperty(navigator, "locks", { configurable: true, value: {
  request: async (_name: string, _options: unknown, callback: () => unknown) => callback(),
} });

afterEach(() => {
  cleanup();
});

afterEach(async () => {
  const cache = await import("@/features/models/model-catalog-cache");
  cache.resetModelCatalogCacheForTests?.();
});
