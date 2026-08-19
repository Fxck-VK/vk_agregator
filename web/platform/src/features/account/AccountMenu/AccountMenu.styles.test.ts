import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/account/AccountMenu/AccountMenu.module.css"),
  "utf8",
);

const menuRule = stylesheet.match(/\.menu \{([\s\S]*?)\n\}/)?.[1];
const triggerRule = stylesheet.match(/\.trigger \{([\s\S]*?)\n\}/)?.[1];

describe("AccountMenu styles", () => {
  it("keeps an upward-opening menu reachable on short viewports", () => {
    expect(menuRule).toContain("inset-block-end: calc(100% + var(--space-2))");
    expect(menuRule).toContain("max-block-size: calc(100dvh - 6rem)");
    expect(menuRule).toContain("overflow-y: auto");
    expect(menuRule).toContain("overscroll-behavior: contain");
  });

  it("keeps the rectangular account trigger transparent until interaction", () => {
    expect(triggerRule).toContain("grid-template-columns: 2.5rem minmax(0, 1fr) 1.5rem");
    expect(triggerRule).toContain("background: transparent");
    expect(stylesheet).toMatch(
      /\.trigger:hover,\s*\.trigger:focus-visible,\s*\.trigger\[data-open="true"\]\s*\{[^}]*background:\s*var\(--color-surface-raised\)/s,
    );
  });

  it("renders a fixed circular account icon without turning the trigger into a pill", () => {
    expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*inline-size:\s*2\.5rem;/s);
    expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*block-size:\s*2\.5rem;/s);
    expect(stylesheet).toMatch(/\.avatar\s*\{[^}]*border-radius:\s*50%;/s);
    expect(triggerRule).toContain("border-radius: var(--radius-lg)");
  });
});
