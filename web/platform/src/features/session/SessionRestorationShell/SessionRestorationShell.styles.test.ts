import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(
    process.cwd(),
    "src/features/session/SessionRestorationShell/SessionRestorationShell.module.css",
  ),
  "utf8",
);

describe("SessionRestorationShell styles", () => {
  it("preserves the desktop workspace geometry", () => {
    expect(stylesheet).toMatch(/\.shell\s*\{[^}]*min-block-size:\s*100dvh;/s);
    expect(stylesheet).toMatch(
      /\.sidebar\s*\{[^}]*inline-size:\s*var\(--sidebar-width\);/s,
    );
    expect(stylesheet).toMatch(
      /\.workspace\s*\{[^}]*margin-inline-start:\s*var\(--sidebar-width\);/s,
    );
    expect(stylesheet).toMatch(
      /\.header\s*\{[^}]*block-size:\s*var\(--header-height\);/s,
    );
  });

  it("uses a content-only mobile shell and respects reduced motion", () => {
    expect(stylesheet).toMatch(
      /@media \(max-width:\s*48rem\)[\s\S]*\.sidebar\s*\{[^}]*display:\s*none;/s,
    );
    expect(stylesheet).toMatch(
      /@media \(max-width:\s*48rem\)[\s\S]*\.workspace\s*\{[^}]*margin-inline-start:\s*0;/s,
    );
    expect(stylesheet).toMatch(/@media \(prefers-reduced-motion:\s*reduce\)/);
  });
});
