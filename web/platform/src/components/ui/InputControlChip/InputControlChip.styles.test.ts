import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/InputControlChip/InputControlChip.module.css"),
  "utf8",
);

const rule = (selector: string) =>
  stylesheet.match(new RegExp(`${selector}\\s*\\{[^}]*\\}`, "s"))?.[0] ?? "";

describe("InputControlChip hover styles", () => {
  it("uses only the brand outline when a button or grouped control is hovered", () => {
    const hoverRule = rule(
      "\\.button:not\\(:disabled\\):hover,\\s*\\.group:hover",
    );

    expect(hoverRule).toContain("border-color: var(--color-accent)");
    expect(hoverRule).not.toContain("background:");
    expect(hoverRule).not.toMatch(/(?:^|[;{])\s*color\s*:/);
    expect(hoverRule).not.toContain("box-shadow:");
    expect(hoverRule).not.toContain("filter:");
    expect(hoverRule).not.toContain("transform:");
  });

  it("keeps an expanded button hollow with the same brand outline", () => {
    const expandedRule = rule('\\.button\\[aria-expanded="true"\\]');

    expect(expandedRule).toContain("border-color: var(--color-accent)");
    expect(expandedRule).not.toContain("background:");
  });
});
