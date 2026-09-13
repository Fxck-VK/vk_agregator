import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/MasonryGrid/MasonryGrid.module.css"),
  "utf8",
);

describe("MasonryGrid styles", () => {
  it("owns the shared Pinterest column layout", () => {
    const gridRule = stylesheet.match(/\.grid\s*\{([^}]*)\}/s)?.[1] ?? "";
    const itemRule = stylesheet.match(/\.grid\s*>\s*li\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(gridRule).toContain("column-width: 17rem");
    expect(gridRule).toContain("column-gap: var(--space-4)");
    expect(gridRule).toContain("margin: 0");
    expect(gridRule).toContain("padding: 0");
    expect(gridRule).toContain("list-style: none");
    expect(gridRule).not.toContain("grid-template-columns");
    expect(itemRule).toContain("inline-size: 100%");
    expect(itemRule).toContain("margin-block-end: var(--space-4)");
    expect(itemRule).toContain("break-inside: avoid");
  });
});
