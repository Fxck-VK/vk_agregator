import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(
    process.cwd(),
    "src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.module.css",
  ),
  "utf8",
);

function getRule(selector: string) {
  return stylesheet.match(new RegExp(`${selector}\\s*\\{(?<body>[\\s\\S]*?)\\}`))?.groups?.body ?? "";
}

describe("FeaturedModelShortcuts styles", () => {
  it("keeps the featured model artwork frameless without changing its size", () => {
    const iconRule = getRule("\\.icon");
    const hoverRule = getRule("\\.shortcut:hover \\.icon");

    expect(iconRule).toContain("inline-size: 3.5rem");
    expect(iconRule).toContain("block-size: 3.5rem");
    expect(iconRule).not.toMatch(/\bborder(?:-radius)?:/);
    expect(iconRule).not.toContain("background:");
    expect(hoverRule).toContain("translate: 0 -0.2rem");
    expect(hoverRule).not.toContain("border-color:");
  });
});
