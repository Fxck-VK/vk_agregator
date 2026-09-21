import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"),
  "utf8",
);
const globalStylesheet = readFileSync(
  resolve(process.cwd(), "src/app/globals.css"),
  "utf8",
);

describe("AppShell floating panel gaps", () => {
  it("uses the sidebar itself as the lower layer beneath the rounded workspace", () => {
    expect(stylesheet).not.toMatch(/\.shell::before\s*\{/);
    expect(stylesheet).toMatch(
      /\.workspace\s*\{[^}]*position:\s*relative;[^}]*margin-inline-start:\s*var\(--sidebar-width\);/s,
    );
    expect(stylesheet).toMatch(
      /\.sidebar\s*\{[^}]*position:\s*absolute;[^}]*inline-size:\s*calc\(var\(--sidebar-width\) \+ var\(--radius-lg\)\);[^}]*padding:\s*0;[^}]*padding-inline-end:\s*var\(--radius-lg\);[^}]*background:\s*var\(--color-sidebar, var\(--color-panel\)\);/s,
    );
    expect(stylesheet).toMatch(
      /\.shell\[data-desktop-sidebar-collapsed="true"\] \.sidebar\s*\{[^}]*inline-size:\s*calc\(var\(--sidebar-collapsed-rail-width\) \+ var\(--radius-lg\)\);/s,
    );
  });

  it("keeps the workspace flush vertically and uses the global gap only at its trailing edge", () => {
    expect(globalStylesheet).toMatch(
      /:root\s*\{[^}]*--app-workspace-edge-gap:\s*0\.125rem;/s,
    );
    expect(stylesheet).toMatch(
      /\.shell\s*\{[^}]*--app-shell-edge-gap:\s*var\(--app-workspace-edge-gap\);/s,
    );
    expect(stylesheet).toMatch(
      /\.workspace\s*\{[^}]*block-size:\s*100dvh;[^}]*margin-block:\s*0;[^}]*margin-inline-end:\s*var\(--app-shell-edge-gap\);[^}]*border-start-start-radius:\s*var\(--radius-lg\);[^}]*border-end-start-radius:\s*var\(--radius-lg\);/s,
    );
    expect(stylesheet).toMatch(/\.workspaceScroller\s*\{[^}]*background:\s*var\(--color-workspace\);/s);
    expect(stylesheet).not.toMatch(/\.workspaceScroller\s*\{[^}]*margin-inline-end:/s);
    expect(stylesheet).toMatch(
      /@media \(width < 48rem\)\s*\{[\s\S]*?\.workspace\s*\{[^}]*margin:\s*0;/s,
    );
  });

});
