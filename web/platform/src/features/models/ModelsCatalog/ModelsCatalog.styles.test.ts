import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelsCatalog/ModelsCatalog.module.css"),
  "utf8",
);

describe("ModelsCatalog responsive styles", () => {
  it("uses a single non-overflowing column at the small-screen breakpoint", () => {
    expect(stylesheet).toMatch(
      /@media \(max-width: 42rem\) \{[\s\S]*?\.grid\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\);/,
    );
  });
});
