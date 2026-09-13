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
const toolbarSource = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.tsx"),
  "utf8",
);
const modeSwitchPanelStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModeSwitchPanel/ModeSwitchPanel.module.css"),
  "utf8",
);
const selectableStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/selectable-control.module.css"),
  "utf8",
);
const workspaceHeaderStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css"),
  "utf8",
);
const modelSelectorStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css"),
  "utf8",
);

describe("ModelsCatalog responsive styles", () => {
  it("styles the whole model card as the interactive link", () => {
    expect(modelCardStylesheet).toMatch(
      /\.cardLink\s*\{[^}]*color:\s*inherit;[^}]*text-decoration:\s*none;/s,
    );
    expect(modelCardStylesheet).toMatch(/\.card\s*\{[^}]*cursor:\s*pointer;/s);
    expect(modelCardStylesheet).toMatch(
      /\.card\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-border\);/s,
    );
    expect(modelCardStylesheet).toMatch(
      /\.cardLink:not\(\.placeholder\):hover \.card\s*\{[^}]*border-color:\s*var\(--color-accent\);[^}]*transform:\s*translateY\(-0\.15rem\);/s,
    );
    expect(modelCardStylesheet).not.toMatch(
      /\.cardLink:not\(\.placeholder\):hover \.card\s*\{[^}]*background:/s,
    );
    expect(modelCardStylesheet).not.toMatch(
      /\.cardLink:not\(\.placeholder\):hover \.card\s*\{[^}]*box-shadow:/s,
    );
    expect(modelCardStylesheet).toMatch(
      /\.placeholder \.card\s*\{[^}]*cursor:\s*default;/s,
    );
    expect(modelCardStylesheet).toMatch(/\.cardLink:focus-visible \.card\s*\{/);
    expect(modelCardStylesheet).not.toMatch(/\.card a\s*\{/);
  });

  it("establishes the catalog as an inline-size query container", () => {
    expect(stylesheet).toMatch(
      /\.catalog\s*\{[^}]*container-name:\s*models-catalog;[^}]*container-type:\s*inline-size;/s,
    );
  });

  it("lets the shared workspace page frame own the catalog width", () => {
    expect(stylesheet).toMatch(
      /\.catalog\s*\{[^}]*inline-size:\s*100%;[^}]*padding-block-end:\s*var\(--space-8\);/s,
    );
    expect(stylesheet).not.toMatch(/\.catalog\s*\{[^}]*margin-inline:/s);
  });

  it("uses one non-overflowing column when the catalog container is narrow", () => {
    expect(stylesheet).toMatch(
      /@container models-catalog \(max-width: 42rem\) \{[\s\S]*?\.grid\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\);/,
    );
  });

  it("uses the shared horizontally scrollable mode panel for categories", () => {
    expect(modeSwitchPanelStylesheet).toMatch(
      /\.root\s*\{[^}]*inline-size:\s*fit-content;[^}]*max-inline-size:\s*100%;/s,
    );
    expect(modeSwitchPanelStylesheet).toMatch(
      /\.viewport\s*\{[^}]*display:\s*flex;[^}]*justify-content:\s*safe center;/s,
    );
    expect(toolbarSource).toContain('from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel"');
    expect(toolbarSource).toContain("<ModeSwitchPanel");
    expect(toolbarSource).toContain("className={styles.categoryPanel}");
    expect(toolbarSource).toContain('semantics="tabs"');
    expect(toolbarSource).not.toContain('from "@/components/ui/ScrollArea/ScrollArea"');
    expect(toolbarStylesheet).not.toMatch(/\.(?:categoryScroll|categoryList|category)\b/);
    expect(toolbarStylesheet).not.toMatch(/overflow-x:\s*auto/);
    expect(toolbarStylesheet).not.toContain("scrollbar-width:");
    expect(toolbarStylesheet).not.toContain("::-webkit-scrollbar");
    expect(toolbarStylesheet).toMatch(
      /\.categoryPanel\s*\{[^}]*inline-size:\s*100%;/s,
    );
  });

  it("inherits active and hover outlines from the shared mode panel", () => {
    expect(selectableStylesheet).toMatch(
      /\.control:where\([^}]*?\)\s*\{[^}]*border-color:\s*var\(--color-accent\);/s,
    );
    expect(modeSwitchPanelStylesheet).toMatch(
      /\.indicator\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-accent\);[^}]*background:\s*transparent;/s,
    );
    expect(selectableStylesheet).not.toMatch(
      /\.control:where\([^}]*?\)\s*\{[^}]*box-shadow:/s,
    );
  });

  it("keeps matching twenty-pixel gaps around the search field", () => {
    expect(workspaceHeaderStylesheet).toMatch(
      /\.header\s*\{[^}]*padding:\s*var\(--space-3\) clamp\(var\(--space-4\), 4vw, var\(--space-6\)\);/s,
    );
    expect(modelSelectorStylesheet).toMatch(
      /\.trigger\s*\{[^}]*min-block-size:\s*2\.75rem;/s,
    );
    expect(toolbarStylesheet).toMatch(
      /\.toolbar\s*\{[^}]*position:\s*sticky;[^}]*z-index:\s*9;[^}]*inset-block-start:\s*calc\(var\(--space-3\) \+ 2\.75rem \+ var\(--space-5\) - var\(--space-2\)\);/s,
    );
    expect(toolbarStylesheet).toMatch(/\.controls\s*\{[^}]*gap:\s*var\(--space-5\);/s);
    expect(toolbarStylesheet).toMatch(
      /\.toolbar\s*\{[^}]*padding-block:\s*var\(--space-2\);[^}]*background:\s*transparent;/s,
    );
    expect(toolbarStylesheet).not.toContain("var(--header-height)");
    expect(toolbarStylesheet).not.toContain("var(--header-height-mobile)");
    expect(toolbarStylesheet).not.toMatch(/\.toolbar::before\s*\{/);
  });

  it("uses exactly two catalogue columns on desktop", () => {
    expect(stylesheet).toMatch(
      /\.grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);[^}]*gap:\s*var\(--space-5\);/s,
    );
  });

  it("uses the approved compact shared-card composition", () => {
    expect(modelCardStylesheet).toMatch(
      /\.card\s*\{[^}]*grid-template-rows:\s*auto 1fr;[^}]*gap:\s*var\(--space-3\);[^}]*min-block-size:\s*11\.5rem;/s,
    );
    expect(modelCardStylesheet).toMatch(
      /\.cardTop\s*\{[^}]*display:\s*flex;[^}]*justify-content:\s*space-between;/s,
    );
    expect(modelCardStylesheet).toMatch(/\.copy\s*\{[^}]*display:\s*grid;/s);
    expect(modelCardStylesheet).not.toMatch(/\.qualities\s*\{/);
    expect(modelCardStylesheet).not.toMatch(/\.reference\s*\{/);
  });

  it("makes the search field span the available width", () => {
    expect(toolbarStylesheet).toMatch(/\.searchField\s*\{[^}]*inline-size:\s*100%;/s);
  });

  it("centers the search icon against the input text", () => {
    expect(toolbarStylesheet).toMatch(
      /\.searchIcon\s*\{[^}]*inset-block-start:\s*50%;[^}]*transform:\s*translateY\(-50%\);/s,
    );
    expect(toolbarStylesheet).toMatch(
      /\.searchIcon\s*\{[^}]*inline-size:\s*1\.5rem;[^}]*block-size:\s*1\.5rem;/s,
    );
  });

  it("keeps the shared model icon and price in one top row", () => {
    expect(modelCardStylesheet).toMatch(/\.cardTop\s*\{[^}]*align-items:\s*center;/s);
    expect(modelCardStylesheet).toMatch(/\.price\s*\{[^}]*white-space:\s*nowrap;/s);
  });
});
