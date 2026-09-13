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
    ["src/features/models/ModelCard/ModelCard.module.css", "\\.card", "--color-panel"],
    ["src/features/files/FileCard/FileCard.module.css", "\\.card", "--color-surface"],
    ["src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css", "\\.card", "--color-surface"],
    ["src/features/account/ProfileIdentityCard/ProfileIdentityCard.module.css", "\\.card", "--color-surface"],
  ])("uses the assigned surface in %s", (path, selector, surface) => {
    expect(rule(read(path), selector)).toContain(`background: var(${surface})`);
  });

  it("keeps neutral hover and elevated states on the raised surface", () => {
    const selector = read("src/features/models/ModelCard/ModelCard.module.css");

    expect(selector).toMatch(
      /\.selectorCard:hover,[\s\S]*?\.selectorSelected\s*\{[^}]*background:\s*var\(--color-surface-raised\)/,
    );
  });
});
