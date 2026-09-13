import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

function readStylesheet(path: string) {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

const globalStylesheet = readStylesheet("src/app/globals.css");
const composerStylesheet = readStylesheet("src/components/chat/ChatComposer/ChatComposer.module.css");
const modelSelectorStylesheet = readStylesheet(
  "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css",
);
const catalogToolbarStylesheet = readStylesheet(
  "src/features/models/ModelCatalogToolbar/ModelCatalogToolbar.module.css",
);
const filesToolbarStylesheet = readStylesheet(
  "src/features/files/FilesToolbar/FilesToolbar.module.css",
);

describe("shared input focus styles", () => {
  it("defines the shared focus treatment as a thin inset line", () => {
    expect(globalStylesheet).toMatch(
      /--input-focus-border-color:\s*color-mix\(in srgb,\s*var\(--color-accent\) 70%,\s*var\(--color-border\)\);/,
    );
    expect(globalStylesheet).toMatch(
      /--input-focus-ring:\s*inset 0 0 0 0\.0625rem var\(--input-focus-border-color\);/,
    );
  });

  it("replaces browser focus outlines with the shared internal treatment", () => {
    expect(globalStylesheet).toMatch(
      /:focus-visible,\s*:focus-visible \*\s*\{[^}]*outline:\s*none !important;/s,
    );
    expect(globalStylesheet).toMatch(
      /:where\(a, button, summary, \[role="button"\], \[tabindex\]:not\(\[tabindex="-1"\]\)\):focus-visible\s*\{[^}]*box-shadow:\s*var\(--input-focus-ring\);/s,
    );
    expect(globalStylesheet).toMatch(
      /:where\(input, textarea, select\):focus-visible\s*\{[^}]*border-color:\s*var\(--input-focus-border-color\);/s,
    );
  });

  it.each([
    { name: "composer", stylesheet: composerStylesheet, selector: ".surface:focus-within" },
    { name: "model selector", stylesheet: modelSelectorStylesheet, selector: ".searchRow:focus-within" },
    { name: "model catalog", stylesheet: catalogToolbarStylesheet, selector: ".searchField:focus-within" },
    { name: "files toolbar", stylesheet: filesToolbarStylesheet, selector: ".search input:focus-visible" },
  ])("applies the shared purple focus treatment to $name", ({ stylesheet, selector }) => {
    const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const rule = stylesheet.match(new RegExp(`${escapedSelector}\\s*\\{[^}]*\\}`, "s"))?.[0] ?? "";

    expect(rule).toContain("border-color: var(--input-focus-border-color)");
    expect(rule).toContain("box-shadow: var(--input-focus-ring)");
  });

  it("removes the nested blue outline from the model selector search input", () => {
    expect(modelSelectorStylesheet).not.toMatch(
      /\.search:focus-visible\s*\{[^}]*var\(--color-focus\)/s,
    );
  });
});
