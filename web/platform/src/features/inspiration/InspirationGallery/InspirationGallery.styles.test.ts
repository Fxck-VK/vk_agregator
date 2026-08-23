import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/inspiration/InspirationExampleCard/InspirationExampleCard.module.css"),
  "utf8",
);

describe("InspirationGallery styles", () => {
  it("uses a fixed viewport dialog and a single-column mobile layout", () => {
    expect(stylesheet).toMatch(/\.overlay\s*\{[^}]*position:\s*fixed;/s);
    expect(stylesheet).toMatch(/\.overlay\s*\{[^}]*inset:\s*0;/s);
    expect(stylesheet).toMatch(/@media\s*\(width\s*<\s*60rem\)/);
    expect(stylesheet).toMatch(/@media\s*\(width\s*<\s*60rem\)[\s\S]*\.dialog\s*\{[^}]*grid-template-columns:\s*1fr;/s);
  });
});
