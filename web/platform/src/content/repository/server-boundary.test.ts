import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const rawContentModules = [
  "../data/models.ts",
  "../data/tools.ts",
  "../data/articles.ts",
  "../data/content.ts",
] as const;

describe("raw editorial content boundary", () => {
  it.each(rawContentModules)("keeps %s server-only", (modulePath) => {
    const source = readFileSync(fileURLToPath(new URL(modulePath, import.meta.url)), "utf8");

    expect(source).toMatch(/^import "server-only";/);
  });
});
