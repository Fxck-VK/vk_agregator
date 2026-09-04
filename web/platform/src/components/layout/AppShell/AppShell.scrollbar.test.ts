import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"), "utf8");
const component = readFileSync(resolve(process.cwd(), "src/components/layout/AppShell/AppShell.tsx"), "utf8");

const shellRule = stylesheet.match(/\.shell \{([\s\S]*?)\n\}/)?.[1];
const sidebarRule = stylesheet.match(/\.sidebar \{([\s\S]*?)\n\}/)?.[1];
const workspaceRule = stylesheet.match(/\.workspace \{([\s\S]*?)\n\}/)?.[1];
const workspaceScrollerRule = stylesheet.match(/\.workspaceScroller \{([\s\S]*?)\n\}/)?.[1];
const trackRule = stylesheet.match(/\.workspaceScroller::-webkit-scrollbar-track \{([\s\S]*?)\n\}/)?.[1];
const thumbRule = stylesheet.match(/\.workspaceScroller::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/)?.[1];

describe("AppShell workspace scrollbar", () => {
  it("locks document scrolling only while the app shell is mounted", () => {
    expect(component).toContain("data-app-shell");
    expect(stylesheet).toContain(":global(html:has(body > [data-app-shell]))");
    expect(stylesheet).toContain(":global(body:has(> [data-app-shell]))");
    expect(stylesheet).toMatch(
      /:global\(html:has\(body > \[data-app-shell\]\)\),[\s\S]*:global\(body:has\(> \[data-app-shell\]\)\)\s*\{[^}]*block-size:\s*100%;[^}]*overflow:\s*hidden;/s,
    );
  });

  it("keeps the native workspace scroller as the only scroll owner", () => {
    expect(component).toContain("className={styles.workspaceScroller}");
    expect(workspaceRule).toContain("overflow: hidden");
    expect(workspaceScrollerRule).toContain("overflow-y: auto");
    expect(workspaceScrollerRule).toContain("background: var(--color-background)");
    expect(workspaceScrollerRule).not.toContain("margin-inline-end");
  });

  it("uses a rounded floating thumb with transparent tracks and no native arrow buttons", () => {
    expect(workspaceScrollerRule).toContain("overflow-y: auto");
    expect(stylesheet).toMatch(
      /@supports \(-moz-appearance: none\) \{[\s\S]*\.workspaceScroller \{[\s\S]*scrollbar-width: thin;[\s\S]*scrollbar-color: var\(--color-border\) transparent;/,
    );
    expect(stylesheet).toContain(".workspaceScroller::-webkit-scrollbar");
    expect(stylesheet).toContain(".workspaceScroller::-webkit-scrollbar-track");
    expect(stylesheet).toContain(".workspaceScroller::-webkit-scrollbar-button");
    expect(stylesheet).toContain(".workspaceScroller::-webkit-scrollbar-thumb");
    expect(stylesheet).toContain("inline-size: 0.75rem");
    expect(stylesheet).toContain("display: none");
    expect(trackRule).toContain("margin-block-start: calc(var(--space-8) + var(--space-3))");
    expect(trackRule).toContain("background: transparent");
    expect(thumbRule).toContain("border-radius: 999px");
    expect(thumbRule).toContain("background-color: var(--color-border)");
    expect(thumbRule).toContain("background-clip: content-box");
    expect(stylesheet).not.toContain("var(--color-text-muted)");
  });
});

describe("AppShell workspace surface", () => {
  it("uses the approved graphite canvas behind both floating panels", () => {
    expect(shellRule).toContain("--app-shell-canvas: var(--color-background)");
    expect(shellRule).toContain("background: var(--app-shell-canvas)");
    expect(sidebarRule).toContain("background: var(--app-shell-canvas)");
  });

  it("renders the desktop workspace as a floating panel matching the sidebar", () => {
    expect(shellRule).toContain("--app-shell-edge-gap: 0.125rem");
    expect(workspaceRule).toContain(
      "block-size: calc(100dvh - var(--app-shell-edge-gap) - var(--app-shell-edge-gap))",
    );
    expect(workspaceRule).toContain("margin-block: var(--app-shell-edge-gap)");
    expect(workspaceRule).toContain("margin-inline-end: var(--app-shell-edge-gap)");
    expect(workspaceRule).toContain("border-radius: var(--radius-lg)");
    expect(workspaceRule).toContain("background: var(--color-workspace)");
    expect(workspaceRule).toContain("box-shadow: var(--shadow-card)");
  });

  it("removes the floating-panel spacing and rounding on mobile", () => {
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.workspace \{[\s\S]*?block-size: 100dvh;[\s\S]*?margin: 0;[\s\S]*?border-radius: 0;/,
    );
  });
});
