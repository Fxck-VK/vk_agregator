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
  it("uses exact complementary narrow and wide breakpoints for the desktop collapse state", () => {
    expect(stylesheet).not.toContain("max-width: 47.99rem");
    expect(appShellStylesheet).not.toContain("max-width: 47.99rem");
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.desktopTrigger \{[\s\S]*?display: none;/,
    );
    expect(stylesheet).toMatch(
      /@media \(min-width: 48rem\) \{[\s\S]*?\.panel\[data-desktop-collapsed="true"\] \{[\s\S]*?transform: translateX\(-105%\);/,
    );
    expect(appShellStylesheet).toMatch(
      /@media \(width < 48rem\) \{[\s\S]*?\.workspace \{[\s\S]*?margin-inline-start: 0;/,
    );
    expect(appShellStylesheet).toMatch(
      /@media \(min-width: 48rem\) \{[\s\S]*?\.shell\[data-desktop-sidebar-collapsed="true"\] \.workspace \{[\s\S]*?margin-inline-start: var\(--sidebar-collapsed-rail-width\);/,
    );
    expect(stylesheet).toMatch(
      /\.desktopTrigger\[data-desktop-collapsed="true"\] svg \{[\s\S]*?transform: rotate\(180deg\);/,
    );
  });

  it("reserves a collapsed desktop rail for the fixed sidebar trigger", () => {
    expect(stylesheet).toMatch(
      /\.desktopTrigger \{[\s\S]*?inline-size: 2\.25rem;/,
    );
    expect(stylesheet).toMatch(
      /\.desktopTrigger\[data-desktop-collapsed="true"\] \{[\s\S]*?inset-inline-start: var\(--space-3\);/,
    );
    expect(appShellStylesheet).toMatch(
      /\.shell \{[\s\S]*?--sidebar-collapsed-rail-width: calc\(var\(--space-3\) \+ 2\.25rem\);/,
    );
  });
});
