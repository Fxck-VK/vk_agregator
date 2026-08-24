import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"),
  "utf8",
);

describe("AppShell floating panel gaps", () => {
  it("uses the thin canvas gap around both floating panels", () => {
    expect(stylesheet).toMatch(
      /\.sidebar\s*\{[^}]*padding:\s*var\(--space-1\);/s,
    );
    expect(stylesheet).toMatch(
      /\.workspace\s*\{[^}]*block-size:\s*calc\(100dvh - var\(--space-1\) - var\(--space-1\)\);[^}]*margin-block:\s*var\(--space-1\);[^}]*margin-inline-end:\s*var\(--space-1\);/s,
    );
  });
});
