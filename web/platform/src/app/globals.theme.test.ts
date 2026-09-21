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
      "--color-workspace: #0d1117",
      "--color-panel: #151b23",
      "--color-surface: #1a1b22",
      "--color-surface-raised: #1f232a",
      "--color-border: #2a2b35",
      "--color-text: var(--color-text-on-dark)",
      "--color-text-muted: var(--color-text-muted-on-dark)",
      "--color-brand-violet: #9a7cf5",
      "--color-brand-blue: #7c8ff7",
      "--color-brand-pink: #f09af0",
      "--color-focus: #a9cfff",
      "--color-text-on-accent: var(--color-text-on-light)",
      "--color-model-icon: var(--color-surface-light)",
    ]) {
      expect(darkTheme).toContain(token);
    }

    for (const token of [
      "--color-background: #edeae4",
      "--color-workspace: #faf8f4",
      "--color-panel: #fffdfa",
      "--color-sidebar: #f1eee8",
      "--color-model-icon-on-light: #9198a1",
      "--panel-surface-background: rgb(255 253 250 / 92%)",
      "--panel-surface-text: #29262e",
      "--panel-surface-color-scheme: light",
      "--color-surface: #f3f0ea",
      "--color-surface-raised: #ebe7e0",
      "--color-border: #d9d3ca",
      "--color-text: #29262e",
      "--color-text-muted: #635e69",
      "--color-brand-violet: #704cc4",
      "--color-brand-blue: #4b5fb5",
      "--color-brand-pink: #9f3e96",
      "--color-focus: #704cc4",
      "--color-text-on-accent: #ffffff",
      "--color-model-icon: var(--color-model-icon-on-light)",
    ]) {
      expect(lightTheme).toContain(token);
    }

    expect(stylesheet).toContain("--color-accent: var(--color-brand-violet)");
    expect(stylesheet).toContain("--color-accent-strong: var(--color-brand-blue)");
    expect(stylesheet).toContain("--color-text-on-light: #0d1117");
    expect(stylesheet).toContain("--color-text-on-dark: #f0f6fc");
    expect(stylesheet).toContain("--color-text-muted-on-dark: #9198a1");
    expect(stylesheet).toContain("--panel-surface-text: var(--color-text-on-dark)");
    expect(stylesheet).toContain("--panel-surface-text-muted: var(--color-text-muted-on-dark)");
    expect(stylesheet).toContain("--color-surface-light: #fff");
    expect(stylesheet).toContain("--color-qr-foreground: var(--color-text-on-light)");
    expect(stylesheet).toContain("--color-qr-background: var(--color-surface-light)");
    expect(stylesheet).toContain("--color-model-icon-on-light: #151b23");
    expect(stylesheet).toContain(
      "--gradient-brand: linear-gradient(120deg, #f29af3 0%, #b983f6 48%, #7c8ff7 100%)",
    );
  });

  it("maps system preference to the light palette through the operating-system media query", () => {
    const systemLightMedia = stylesheet.match(/@media \(prefers-color-scheme: light\) \{([\s\S]*?)\n\}/)?.[1];

    expect(systemLightMedia).toBeDefined();
    expect(systemLightMedia ?? "").toContain(':root[data-theme="system"]');
    expect(systemLightMedia ?? "").toContain("--color-background: #edeae4");
    expect(systemLightMedia ?? "").toContain("--color-workspace: #faf8f4");
    expect(systemLightMedia ?? "").toContain("--color-panel: #fffdfa");
    expect(systemLightMedia ?? "").toContain("--color-model-icon: var(--color-model-icon-on-light)");
    expect(systemLightMedia ?? "").toContain("--color-brand-violet: #704cc4");
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
    expect(stylesheet).toContain("--font-size-subsection: 1.5rem");
    expect(stylesheet).toContain("--line-height-subsection: 2rem");
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
    expect(stylesheet).toContain("--letter-spacing-display: -0.01em");
    expect(stylesheet).toContain("--letter-spacing-section: -0.01em");
    expect(stylesheet).toContain("--letter-spacing-subsection: 0");
    expect(stylesheet).toContain("--letter-spacing-interface: 0");
    expect(stylesheet).not.toContain("--letter-spacing-display: -0.03em");
    expect(stylesheet).not.toContain("--letter-spacing-section: -0.025em");
    expect(stylesheet).toContain("--font-sans: var(--font-geist-sans)");
    expect(stylesheet).toMatch(/body\s*\{[^}]*letter-spacing:\s*var\(--letter-spacing-interface\)/s);
    expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-display:\s*2rem/);
    expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-section:\s*1\.75rem/);
    expect(stylesheet).toMatch(/@media \(width < 48rem\)[\s\S]*--font-size-subsection:\s*1\.375rem/);
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

  it("uses the brand accent for text selection across the application", () => {
    const selectionRule = stylesheet.match(/::selection\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(selectionRule).toContain("background: var(--color-accent)");
    expect(selectionRule).toContain("color: #fff");
  });
});
