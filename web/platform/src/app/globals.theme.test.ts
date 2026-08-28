import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/app/globals.css"), "utf8");

describe("global theme tokens", () => {
  it("defines the approved graphite and brand palettes", () => {
    const darkTheme = stylesheet.match(/:root\[data-theme="dark"\]\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
    const lightTheme = stylesheet.match(/:root\[data-theme="light"\]\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";

    for (const token of [
      "--color-background: #0c0c0f",
      "--color-workspace: #111217",
      "--color-panel: #15161c",
      "--color-surface: #1a1b22",
      "--color-surface-raised: #20212a",
      "--color-border: #2a2b35",
      "--color-text: #f5f5f7",
      "--color-text-muted: #9b9da8",
      "--color-brand-violet: #9a7cf5",
      "--color-brand-blue: #7c8ff7",
      "--color-brand-pink: #f09af0",
      "--color-focus: #a9cfff",
      "--color-text-on-accent: #111217",
    ]) {
      expect(darkTheme).toContain(token);
    }

    for (const token of [
      "--color-background: #f7f7fa",
      "--color-workspace: #ffffff",
      "--color-panel: #f3f3f7",
      "--color-surface: #eeeef4",
      "--color-surface-raised: #e8e8f0",
      "--color-border: #dedfe7",
      "--color-text: #17171b",
      "--color-text-muted: #6b6c76",
      "--color-brand-violet: #7563e6",
      "--color-brand-blue: #6678e6",
      "--color-brand-pink: #d56dd9",
      "--color-focus: #8475f0",
      "--color-text-on-accent: #ffffff",
    ]) {
      expect(lightTheme).toContain(token);
    }

    expect(stylesheet).toContain("--color-accent: var(--color-brand-violet)");
    expect(stylesheet).toContain("--color-accent-strong: var(--color-brand-blue)");
    expect(stylesheet).toContain(
      "--gradient-brand: linear-gradient(120deg, #f29af3 0%, #b983f6 48%, #7c8ff7 100%)",
    );
  });

  it("maps system preference to the light palette through the operating-system media query", () => {
    const systemLightMedia = stylesheet.match(/@media \(prefers-color-scheme: light\) \{([\s\S]*?)\n\}/)?.[1];

    expect(systemLightMedia).toBeDefined();
    expect(systemLightMedia ?? "").toContain(':root[data-theme="system"]');
    expect(systemLightMedia ?? "").toContain("--color-background: #f7f7fa");
    expect(systemLightMedia ?? "").toContain("--color-workspace: #ffffff");
    expect(systemLightMedia ?? "").toContain("--color-panel: #f3f3f7");
    expect(systemLightMedia ?? "").toContain("--color-brand-violet: #7563e6");
    expect(systemLightMedia ?? "").toContain("color-scheme: light");
  });

  it("defines the shared layout, typography, and interaction token contract", () => {
    expect(stylesheet).toContain("--container-narrow: 48rem");
    expect(stylesheet).toContain("--container-content: 66rem");
    expect(stylesheet).toContain("--container-wide: 76rem");
    expect(stylesheet).toContain("--font-size-display: 2.5rem");
    expect(stylesheet).toContain("--line-height-display: 2.75rem");
    expect(stylesheet).toContain("--font-size-section: 2rem");
    expect(stylesheet).toContain("--line-height-section: 2.375rem");
    expect(stylesheet).toContain("--font-size-supporting: 1.125rem");
    expect(stylesheet).toContain("--line-height-supporting: 1.6875rem");
    expect(stylesheet).toContain("--font-size-body: 1rem");
    expect(stylesheet).toContain("--line-height-body: 1.5rem");
    expect(stylesheet).toContain("--font-size-navigation: 0.9375rem");
    expect(stylesheet).toContain("--line-height-navigation: 1.375rem");
    expect(stylesheet).toContain("--font-size-ui: 0.875rem");
    expect(stylesheet).toContain("--line-height-ui: 1.25rem");
    expect(stylesheet).toContain("--font-size-caption: 0.8125rem");
    expect(stylesheet).toContain("--line-height-caption: 1.125rem");
    expect(stylesheet).toContain("--font-sans: var(--font-geist-sans)");
    expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-display:\s*2rem/);
    expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-section:\s*1\.75rem/);
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

  it("smoothly transitions theme-driven page colors while preserving reduced-motion support", () => {
    expect(stylesheet).toMatch(/html\s*\{[^}]*transition:\s*background-color var\(--motion-normal\);/s);
    expect(stylesheet).toMatch(
      /body\s*\{[^}]*transition:[^}]*background-color var\(--motion-normal\),[^}]*color var\(--motion-normal\);/s,
    );
    expect(stylesheet).toContain("@media (prefers-reduced-motion: reduce)");
    expect(stylesheet).toContain("transition-duration: 0.01ms !important");
  });
});
