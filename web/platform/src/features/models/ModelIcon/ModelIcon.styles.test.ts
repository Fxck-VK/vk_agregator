import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/models/ModelIcon/ModelIcon.module.css"),
  "utf8",
);

describe("ModelIcon theme styles", () => {
  it("colors the theme silhouettes through a shared token without inline styles", () => {
    expect(stylesheet).toMatch(
      /\.fallback\s*\{[^}]*mask-image:\s*url\("\/assets\/images\/models\/chip-silhouette\.svg"\)/s,
    );
    expect(stylesheet).toMatch(
      /:global\(:root\[data-theme="light"\]\) \.fallback\s*\{[^}]*mask-image:\s*url\("\/assets\/images\/models\/chip-silhouette-dark\.svg"\)/s,
    );
    expect(stylesheet).toMatch(
      /@media \(prefers-color-scheme: light\)[\s\S]*:global\(:root\[data-theme="system"\]\) \.fallback\s*\{[^}]*mask-image:\s*url\("\/assets\/images\/models\/chip-silhouette-dark\.svg"\)/s,
    );
    expect(stylesheet).not.toContain("--model-icon-fallback");
    expect(stylesheet).toContain("background-color: var(--color-model-icon)");
    expect(stylesheet).toContain("mask-mode: alpha");
  });
});
