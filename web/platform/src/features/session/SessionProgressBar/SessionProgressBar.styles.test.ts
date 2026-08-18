import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/session/SessionProgressBar/SessionProgressBar.module.css"),
  "utf8",
);

describe("SessionProgressBar styles", () => {
  it("stays above the page without changing layout", () => {
    expect(stylesheet).toMatch(/\.track\s*\{[^}]*position:\s*fixed;/s);
    expect(stylesheet).toMatch(/\.track\s*\{[^}]*inset:\s*0 0 auto;/s);
    expect(stylesheet).toMatch(/\.track\s*\{[^}]*block-size:\s*0\.1875rem;/s);
    expect(stylesheet).toMatch(/\.track\s*\{[^}]*pointer-events:\s*none;/s);
  });

  it("animates only transforms and provides reduced motion", () => {
    expect(stylesheet).toMatch(/@keyframes session-progress[\s\S]*transform:/);
    expect(stylesheet).toMatch(/\.indicator\s*\{[^}]*animation:\s*session-progress/s);
    expect(stylesheet).toMatch(/background:\s*var\(--color-accent\)/);
    expect(stylesheet).toMatch(/@media \(prefers-reduced-motion:\s*reduce\)/);
  });
});
