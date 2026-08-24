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
  it("styles the whole model card as the interactive link", () => {
    expect(modelCardStylesheet).toMatch(
      /\.cardLink\s*\{[^}]*color:\s*inherit;[^}]*text-decoration:\s*none;/s,
    );
    expect(modelCardStylesheet).toMatch(/\.card\s*\{[^}]*cursor:\s*pointer;/s);
    expect(modelCardStylesheet).toMatch(/\.cardLink:hover \.card\s*\{/);
    expect(modelCardStylesheet).toMatch(/\.cardLink:focus-visible \.card\s*\{/);
    expect(modelCardStylesheet).not.toMatch(/\.card a\s*\{/);
  });

  it("establishes the catalog as an inline-size query container", () => {
    expect(stylesheet).toMatch(
      /\.catalog\s*\{[^}]*container-name:\s*models-catalog;[^}]*container-type:\s*inline-size;/s,
    );
  });

  it("keeps the desktop catalog narrow and symmetrically centered", () => {
    expect(stylesheet).toMatch(
      /\.catalog\s*\{[^}]*inline-size:\s*min\(100%,\s*64rem\);[^}]*margin-inline:\s*auto;/s,
    );
  });

  it("uses one non-overflowing column when the catalog container is narrow", () => {
    expect(stylesheet).toMatch(
      /@container models-catalog \(max-width: 52rem\) \{[\s\S]*?\.grid\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\);/,
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

  it("positions the shared model icon in the card artwork area", () => {
    expect(modelCardStylesheet).toMatch(/\.modelIcon\s*\{[^}]*grid-area:\s*spark;/s);
  });
});
