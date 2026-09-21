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
    ["src/features/files/FileCard/FileCard.module.css", "\\.card", "--color-panel"],
    ["src/features/account/ProfileBalanceCard/ProfileBalanceCard.module.css", "\\.card", "--color-panel"],
    ["src/features/account/ProfileIdentityCard/ProfileIdentityCard.module.css", "\\.card", "--color-panel"],
  ])("uses the assigned surface in %s", (path, selector, surface) => {
    expect(rule(read(path), selector)).toContain(`background: var(${surface})`);
  });

  it("keeps selectable cards transparent and shares their brand outline", () => {
    const selector = read("src/features/models/ModelCard/ModelCard.module.css");

    const shared = read("src/components/ui/selectable-control.module.css");
    const source = read("src/features/models/ModelCard/ModelCard.tsx");
    expect(source).toContain("selectableStyles.control");
    expect(shared).toContain("background: transparent");
    expect(shared).toContain("border-color: var(--color-accent)");
    expect(selector).not.toMatch(/\.selectorCard:hover/);
  });
});
