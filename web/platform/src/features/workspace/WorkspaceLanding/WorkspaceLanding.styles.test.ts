import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/workspace/WorkspaceLanding/WorkspaceLanding.module.css"),
  "utf8",
);

describe("WorkspaceLanding background", () => {
  it("uses a flat workspace background without an accent glow", () => {
    const pageRule = stylesheet.match(/\.page\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(pageRule).toContain("background: var(--color-background)");
    expect(pageRule).not.toContain("radial-gradient");
  });
});

describe("WorkspaceLanding hero", () => {
  it("keeps the desktop heading compact and on one line", () => {
    const headingRule = stylesheet.match(/\.heroCopy h1\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(headingRule).toContain("font-size: clamp(2.25rem, 3.5vw, 3rem)");
    expect(headingRule).toContain("white-space: nowrap");
  });

  it("uses a compact desktop hero and restores heading wrapping on narrow screens", () => {
    const heroRule = stylesheet.match(/\.hero\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(heroRule).toContain("gap: clamp(var(--space-4), 2vw, var(--space-6))");
    expect(heroRule).toContain("min-block-size: min(40rem, calc(100dvh - 4.5rem))");
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\)[\s\S]*\.heroCopy h1\s*\{[^}]*white-space:\s*normal;/s,
    );
  });
});
