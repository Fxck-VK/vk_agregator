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
  it("stays sticky inside the workspace without covering the sidebar overlay", () => {
    const headerZIndex = Number(headerStylesheet.match(/\.header\s*\{[^}]*z-index:\s*(\d+);/s)?.[1]);
    const sidebarZIndex = Number(appShellStylesheet.match(/\.sidebar\s*\{[^}]*z-index:\s*(\d+);/s)?.[1]);

    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*position:\s*sticky;/s);
    expect(headerStylesheet).toMatch(/\.header\s*\{[^}]*inset-block-start:\s*0;/s);
    expect(headerZIndex).toBeLessThan(sidebarZIndex);
  });
});
