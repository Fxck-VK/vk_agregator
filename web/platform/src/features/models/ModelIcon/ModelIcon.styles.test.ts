import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelIcon/ModelIcon.module.css"),
  "utf8",
);

describe("ModelIcon theme styles", () => {
  it("uses the white placeholder in dark mode and the dark placeholder in light mode", () => {
    expect(stylesheet).toMatch(
      /\.fallback\s*\{[^}]*background-image:\s*var\(--model-icon-fallback-dark\)/s,
    );
    expect(stylesheet).toMatch(
      /:global\(:root\[data-theme="light"\]\) \.fallback\s*\{[^}]*background-image:\s*var\(--model-icon-fallback-light\)/s,
    );
    expect(stylesheet).toMatch(
      /@media \(prefers-color-scheme: light\)[\s\S]*:global\(:root\[data-theme="system"\]\) \.fallback\s*\{[^}]*background-image:\s*var\(--model-icon-fallback-light\)/s,
    );
  });
});
