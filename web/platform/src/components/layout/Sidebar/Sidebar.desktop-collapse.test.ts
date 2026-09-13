import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/Sidebar/Sidebar.module.css"),
  "utf8",
);
const appShellStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"),
  "utf8",
);
const tooltipStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/Tooltip/Tooltip.module.css"),
  "utf8",
);

describe("Sidebar desktop collapse stylesheet", () => {
  it("keeps the sidebar content full-height while its surface continues beneath the workspace", () => {
    expect(appShellStylesheet).toMatch(
      /\.sidebar\s*\{[^}]*position:\s*absolute;[^}]*inline-size:\s*calc\(var\(--sidebar-width\) \+ var\(--radius-lg\)\);[^}]*padding:\s*0;[^}]*padding-inline-end:\s*var\(--radius-lg\);[^}]*background:\s*var\(--color-panel\);/s,
    );
    expect(stylesheet).toMatch(
      /\.panel\s*\{[^}]*block-size:\s*100%;[^}]*inline-size:\s*100%;[^}]*border-radius:\s*0;/s,
    );
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\)[\s\S]*\.panel\s*\{[^}]*border-radius:\s*0;/s,
    );
  });

  it("uses complementary breakpoints without hiding the collapsed desktop rail", () => {
    expect(stylesheet).not.toContain("max-width: 47.99rem");
    expect(appShellStylesheet).not.toContain("max-width: 47.99rem");
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.desktopTrigger \{[\s\S]*?display: none;/,
    );
    expect(stylesheet).not.toMatch(
      /@media \(min-width: 48rem\) \{[\s\S]*?\.panel\[data-desktop-collapsed="true"\] \{[\s\S]*?transform: translateX\(-105%\);/,
    );
    expect(appShellStylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.workspace \{[\s\S]*?margin: 0;/,
    );
    expect(appShellStylesheet).toMatch(
      /@media \(min-width: 48rem\) \{[\s\S]*?\.shell\[data-desktop-sidebar-collapsed="true"\] \.workspace \{[\s\S]*?margin-inline-start: var\(--sidebar-collapsed-rail-width\);/,
    );
  });

  it("animates the desktop rail and workspace together without ignoring reduced motion", () => {
    expect(appShellStylesheet).toMatch(
      /\.shell\s*\{[^}]*--sidebar-motion-duration:\s*400ms;[^}]*--sidebar-motion-easing:\s*cubic-bezier\(0\.4, 0, 0\.2, 1\);/s,
    );
    expect(appShellStylesheet).toMatch(
      /\.sidebar\s*\{[^}]*transition:\s*inline-size var\(--sidebar-motion-duration\) var\(--sidebar-motion-easing\);/s,
    );
    expect(appShellStylesheet).toMatch(
      /\.workspace\s*\{[^}]*transition:\s*margin-inline-start var\(--sidebar-motion-duration\) var\(--sidebar-motion-easing\);/s,
    );
    expect(stylesheet).toMatch(
      /\.panel\s*\{[^}]*transition:[^;]*padding var\(--sidebar-motion-duration\) var\(--sidebar-motion-easing\)/s,
    );
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-transition="out"\][\s\S]*animation:\s*sidebarContentOut 100ms ease-in both;/,
    );
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-transition="in"\][\s\S]*animation:\s*sidebarContentIn var\(--sidebar-motion-duration\) var\(--sidebar-motion-easing\) both;/,
    );
    expect(appShellStylesheet).toMatch(
      /@media \(prefers-reduced-motion: reduce\)\s*\{[^}]*\.sidebar,[^}]*\.workspace\s*\{[^}]*transition:\s*none;/s,
    );
  });

  it("reserves a full icon rail with square active controls and hover tooltips", () => {
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-collapsed="true"\] \{[\s\S]*?inline-size: 100%;[\s\S]*?padding: var\(--space-3\) var\(--space-2\);/,
    );
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-collapsed="true"\] \[data-sidebar-conversation-list="true"\] \{[\s\S]*?inline-size: 100%;/,
    );
    expect(stylesheet).not.toContain("scrollbar-gutter:");
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-collapsed="true"\] \.navigationList a\[aria-current="page"\] \{[\s\S]*?border-radius: var\(--radius-sm\);/,
    );
    expect(appShellStylesheet).toMatch(
      /\.shell \{[\s\S]*?--sidebar-collapsed-rail-width: 5\.5rem;/,
    );
    expect(stylesheet).toMatch(
      /\.railTooltip \{[\s\S]*?position: fixed;[\s\S]*?inset-inline-start: calc\(var\(--sidebar-collapsed-rail-width\) \+ var\(--space-2\)\);/,
    );
    expect(stylesheet).toMatch(
      /\.collapsedBrandControl:hover \.brandChip[\s\S]*?opacity: 0;/,
    );
  });

  it("keeps the collapsed toggle stable when browser zoom changes during hover", () => {
    expect(stylesheet).toMatch(
      /\.collapsedBrandControl\s*\{[^}]*contain:\s*paint;[^}]*overflow:\s*hidden;/s,
    );
    expect(stylesheet).toMatch(
      /\.collapsedBrandControl \.brandChip,\s*\.collapsedBrandControl \.expandMark\s*\{[^}]*inset:\s*0;[^}]*margin:\s*auto;[^}]*transition:\s*opacity var\(--motion-fast\);/s,
    );
    expect(stylesheet).toMatch(
      /\.desktopChevron\s*\{[^}]*inline-size:\s*0\.875rem;[^}]*block-size:\s*auto;/s,
    );
    expect(stylesheet).toMatch(/\.collapseMark\s*\{[^}]*transform:\s*rotate\(90deg\);/s);
    expect(stylesheet).toMatch(
      /\.collapsedBrandControl \.expandMark\s*\{[^}]*transform:\s*rotate\(-90deg\);/s,
    );
    expect(stylesheet).not.toMatch(/\.railTooltip\s*\{[^}]*backdrop-filter:/s);
    expect(tooltipStylesheet).toMatch(
      /\.bubble\s*\{[^}]*inline-size:\s*max-content;[^}]*overflow-wrap:\s*anywhere;/s,
    );
  });
});
