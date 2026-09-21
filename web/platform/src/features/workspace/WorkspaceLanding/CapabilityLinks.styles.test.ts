import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheetPath = resolve(
  process.cwd(),
  "src/features/workspace/WorkspaceLanding/CapabilityLinks.module.css",
);
const stylesheet = existsSync(stylesheetPath) ? readFileSync(stylesheetPath, "utf8") : "";
const dividerRule = stylesheet.match(/\.divider\s*\{[^}]*\}/s)?.[0] ?? "";
const listRule = stylesheet.match(/\.list\s*\{[^}]*\}/s)?.[0] ?? "";
const linkRule = stylesheet.match(/\.link\s*\{[^}]*\}/s)?.[0] ?? "";
const iconRule = stylesheet.match(/\.link \.icon\s*\{[^}]*\}/s)?.[0] ?? "";

describe("CapabilityLinks layout", () => {
  it("matches the approved divider and three-by-two desktop link layout", () => {
    expect(dividerRule).toContain("grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr)");
    expect(stylesheet).toMatch(
      /\.divider::before,[\s\S]*\.divider::after\s*\{[^}]*content:\s*"";[^}]*background:\s*var\(--color-border\);/s,
    );
    expect(listRule).toContain("grid-template-columns: repeat(3, max-content)");
    expect(linkRule).toContain("background: transparent");
    expect(linkRule).toContain("border-radius: var(--radius-sm)");
    expect(iconRule).toContain("inline-size: 1.125rem");
    expect(iconRule).toContain("block-size: 1.125rem");
    expect(iconRule).toContain("background-color: currentColor");
    expect(iconRule).toContain("mask: var(--capability-icon) center / contain no-repeat");
    expect(stylesheet).not.toContain("chip-silhouette-dark.svg");
  });

  it("collapses the link grid without horizontal overflow on narrow screens", () => {
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\)[\s\S]*\.list\s*\{[^}]*grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\);/s,
    );
    expect(stylesheet).toMatch(
      /@media \(width < 36rem\)[\s\S]*\.list\s*\{[^}]*grid-template-columns:\s*1fr;/s,
    );
  });
});
