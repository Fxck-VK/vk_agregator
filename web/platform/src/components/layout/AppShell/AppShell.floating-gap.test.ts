import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"),
  "utf8",
);

describe("AppShell floating panel gaps", () => {
  it("uses a reduced edge gap without changing the scroller inset", () => {
    expect(stylesheet).toMatch(
      /\.shell\s*\{[^}]*--app-shell-edge-gap:\s*0\.125rem;/s,
    );
    expect(stylesheet).toMatch(
      /\.sidebar\s*\{[^}]*padding:\s*var\(--app-shell-edge-gap\);/s,
    );
    expect(stylesheet).toMatch(
      /\.workspace\s*\{[^}]*block-size:\s*calc\(100dvh - var\(--app-shell-edge-gap\) - var\(--app-shell-edge-gap\)\);[^}]*margin-block:\s*var\(--app-shell-edge-gap\);[^}]*margin-inline-end:\s*var\(--app-shell-edge-gap\);/s,
    );
    expect(stylesheet).toMatch(
      /\.workspaceScroller\s*\{[^}]*margin-inline-end:\s*var\(--space-1\);/s,
    );
  });
});
