import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/app/globals.css"), "utf8");

describe("global theme tokens", () => {
  it("defines explicit dark and approved light semantic palettes", () => {
    expect(stylesheet).toContain(':root[data-theme="dark"]');
    expect(stylesheet).toContain(':root[data-theme="light"]');
    expect(stylesheet).toContain("--color-background: #f6f7f9");
    expect(stylesheet).toContain("--color-surface: #ffffff");
    expect(stylesheet).toContain("--color-text: #171a21");
    expect(stylesheet).toContain("--color-text-muted: #667085");
    expect(stylesheet).toContain("color-scheme: light");
    expect(stylesheet).toContain("color-scheme: dark");
  });

  it("maps system preference to the light palette through the operating-system media query", () => {
    const systemLightMedia = stylesheet.match(/@media \(prefers-color-scheme: light\) \{([\s\S]*?)\n\}/)?.[1];

    expect(systemLightMedia).toBeDefined();
    expect(systemLightMedia ?? "").toContain(':root[data-theme="system"]');
    expect(systemLightMedia ?? "").toContain("--color-background: #f6f7f9");
    expect(systemLightMedia ?? "").toContain("color-scheme: light");
  });

  it("defines the shared public layout and interaction token contract", () => {
    expect(stylesheet).toContain("--container-narrow: 48rem");
    expect(stylesheet).toContain("--container-content: 66rem");
    expect(stylesheet).toContain("--container-wide: 76rem");
    expect(stylesheet).toContain("--font-size-label: 0.875rem");
    expect(stylesheet).toContain("--font-size-display: clamp(2.25rem, 6vw, 4.25rem)");
    expect(stylesheet).toContain("--line-height-tight: 1.08");
    expect(stylesheet).toContain("--radius-xl: 1.25rem");
    expect(stylesheet).toContain("--radius-pill: 999px");
    expect(stylesheet).toContain("--shadow-card:");
    expect(stylesheet).toContain("--shadow-floating:");
    expect(stylesheet).toContain("--header-height: 4.5rem");
    expect(stylesheet).toContain("--header-height-mobile: 4rem");
    expect(stylesheet).toContain("--opacity-disabled: 0.5");
    expect(stylesheet).toContain("--opacity-loading: 0.72");
  });

  it("defines status colors in dark, light, and system-light palettes", () => {
    const darkTheme = stylesheet.match(/:root\[data-theme="dark"\]\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
    const lightTheme = stylesheet.match(/:root\[data-theme="light"\]\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
    const systemLightMedia = stylesheet.match(/@media \(prefers-color-scheme: light\) \{([\s\S]*?)\n\}/)?.[1] ?? "";

    for (const palette of [darkTheme, lightTheme, systemLightMedia]) {
      expect(palette).toContain("--color-success:");
      expect(palette).toContain("--color-success-surface:");
      expect(palette).toContain("--color-warning:");
      expect(palette).toContain("--color-warning-surface:");
      expect(palette).toContain("--color-information:");
      expect(palette).toContain("--color-information-surface:");
    }
  });
});
