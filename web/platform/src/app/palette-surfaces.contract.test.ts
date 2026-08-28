import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

function rule(stylesheet: string, selector: string) {
  const match = stylesheet.match(new RegExp(`${selector}\\s*\\{([\\s\\S]*?)\\n\\}`));

  if (!match) {
    throw new Error(`Missing selector ${selector}`);
  }

  return match[1];
}

describe("palette surface roles", () => {
  it.each([
    ["src/features/models/ModelCard/ModelCard.module.css", "\\.card"],
    ["src/features/files/FileCard/FileCard.module.css", "\\.card"],
    ["src/features/workspace/FeaturedModels/FeaturedModels.module.css", "\\.card"],
    ["src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css", "\\.card"],
    ["src/features/account/ProfileIdentityCard/ProfileIdentityCard.module.css", "\\.card"],
  ])("uses the card surface in %s", (path, selector) => {
    expect(rule(read(path), selector)).toContain("background: var(--color-surface)");
  });

  it("keeps neutral hover and elevated states on the raised surface", () => {
    const selector = read(
      "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.module.css",
    );

    expect(selector).toMatch(
      /\.option:hover,[\s\S]*?\.optionSelected\s*\{[^}]*background:\s*var\(--color-surface-raised\)/,
    );
  });
});
