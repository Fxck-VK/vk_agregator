import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/account/AccountMenu/AccountMenu.module.css"),
  "utf8",
);

const menuRule = stylesheet.match(/\.menu \{([\s\S]*?)\n\}/)?.[1];

describe("AccountMenu styles", () => {
  it("keeps an upward-opening menu reachable on short viewports", () => {
    expect(menuRule).toContain("inset-block-end: calc(100% + var(--space-2))");
    expect(menuRule).toContain("max-block-size: calc(100dvh - 6rem)");
    expect(menuRule).toContain("overflow-y: auto");
    expect(menuRule).toContain("overscroll-behavior: contain");
  });
});
