import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelsCatalog/ModelsCatalog.module.css"),
  "utf8",
);
const modelCardStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelCard/ModelCard.module.css"),
  "utf8",
);
const toolbarStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css"),
  "utf8",
);

describe("ModelsCatalog responsive styles", () => {
  it("uses the global text token for CTAs on the accent surface", () => {
    expect(modelCardStylesheet).toMatch(
      /\.card a\s*\{[^}]*background:\s*var\(--color-accent\);[^}]*color:\s*var\(--color-text\);/s,
    );
  });

  it("uses a single non-overflowing column at the small-screen breakpoint", () => {
    expect(stylesheet).toMatch(
      /@media \(max-width: 42rem\) \{[\s\S]*?\.grid\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\);/,
    );
  });

  it("keeps the category pills horizontally scrollable", () => {
    expect(toolbarStylesheet).toMatch(
      /\.categoryList\s*\{[^}]*overflow-x:\s*auto;[^}]*scrollbar-width:\s*thin;/s,
    );
  });

  it("uses exactly two catalogue columns on desktop", () => {
    expect(stylesheet).toMatch(
      /\.grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s,
    );
  });

  it("makes the search field span the available width", () => {
    expect(toolbarStylesheet).toMatch(/\.searchField\s*\{[^}]*inline-size:\s*100%;/s);
  });

  it("renders the model-card sparkle as decorative CSS content", () => {
    expect(modelCardStylesheet).toMatch(/\.card::before\s*\{[^}]*content:\s*["'][^"']+["'];/s);
  });
});
