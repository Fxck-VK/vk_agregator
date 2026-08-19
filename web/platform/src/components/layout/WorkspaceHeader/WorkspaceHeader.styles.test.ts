import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const headerStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css"),
  "utf8",
);
const appShellStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"),
  "utf8",
);

describe("WorkspaceHeader styles", () => {
  it("floats interactive controls over workspace content without drawing a header strip", () => {
    const headerZIndex = Number(headerStylesheet.match(/\.header\s*\{[^}]*z-index:\s*(\d+);/s)?.[1]);
    const sidebarZIndex = Number(appShellStylesheet.match(/\.sidebar\s*\{[^}]*z-index:\s*(\d+);/s)?.[1]);

    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*position:\s*sticky;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*inset-block-start:\s*0;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*block-size:\s*0;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*min-block-size:\s*0;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*padding:\s*0;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*background:\s*transparent;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*pointer-events:\s*none;/s);
    expect(headerStylesheet).toMatch(
      /\.leading,\s*\.trailing\s*\{[^}]*position:\s*absolute;[^}]*pointer-events:\s*auto;/s,
    );
    expect(headerZIndex).toBeLessThan(sidebarZIndex);
  });
});
