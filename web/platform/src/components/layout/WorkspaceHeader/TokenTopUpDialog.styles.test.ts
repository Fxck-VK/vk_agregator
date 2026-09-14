import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/WorkspaceHeader/TokenTopUpDialog.module.css"),
  "utf8",
);

describe("TokenTopUpDialog styles", () => {
  it("uses shared selection visuals with bright text and no local fill or selection mark", () => {
    expect(stylesheet).not.toContain("#1488ff");
    expect(stylesheet).not.toContain("--top-up-accent");
    const cardRule = stylesheet.match(/\.packageCard\s*\{[^}]*\}/s)?.[0] ?? "";
    expect(cardRule).toContain("--color-text: var(--panel-surface-text)");
    expect(cardRule).toContain("--color-text-muted: var(--panel-surface-text)");
    expect(cardRule).not.toMatch(/(?:background|border|border-radius|box-shadow):/);
    expect(stylesheet).not.toContain(".selectionMark");
    expect(stylesheet).toMatch(/\.priceLine mark\s*\{[^}]*background:\s*var\(--gradient-brand\);/s);
    expect(stylesheet).toMatch(/\.purchaseButton\s*\{[^}]*background:\s*var\(--gradient-brand\);/s);
  });

  it("keeps the purchase dialog at eighty percent of the viewport", () => {
    expect(stylesheet).toMatch(
      /\.dialog\s*\{[^}]*block-size:\s*80dvh;[^}]*min-block-size:\s*0;[^}]*overflow:\s*hidden;/s,
    );
    expect(stylesheet).toMatch(
      /\.packageList\s*\{[^}]*min-block-size:\s*0;/s,
    );
    expect(stylesheet).toMatch(/\.packageCard\s*\{[^}]*min-block-size:\s*0;/s);
    expect(stylesheet).not.toContain("min-block-size: 100dvh");
  });
});
