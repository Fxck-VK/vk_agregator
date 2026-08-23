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

describe("Sidebar desktop collapse stylesheet", () => {
  it("floats the desktop sidebar above the workspace background", () => {
    expect(appShellStylesheet).toMatch(
      /\.sidebar\s*\{[^}]*padding:\s*var\(--space-2\);[^}]*background:\s*var\(--app-shell-canvas\);/s,
    );
    expect(stylesheet).toMatch(
      /\.panel\s*\{[^}]*block-size:\s*100%;[^}]*inline-size:\s*100%;[^}]*border-radius:\s*var\(--radius-lg\);/s,
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

  it("reserves a full icon rail with square active controls and hover tooltips", () => {
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-collapsed="true"\] \{[\s\S]*?inline-size: 100%;[\s\S]*?padding: var\(--space-3\) var\(--space-2\);/,
    );
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-collapsed="true"\] \[data-sidebar-conversation-list="true"\] \{[\s\S]*?inline-size: 100%;/,
    );
    expect(stylesheet).toMatch(
      /\.panel\[data-desktop-collapsed="true"\] \[data-sidebar-conversation-list="true"\] \{[\s\S]*?scrollbar-gutter: stable both-edges;/,
    );
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
});
