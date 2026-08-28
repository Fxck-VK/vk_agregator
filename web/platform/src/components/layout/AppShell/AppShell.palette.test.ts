import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const read = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

describe("AppShell palette roles", () => {
  it("uses neutral outer, workspace, and panel layers", () => {
    const shell = read("src/components/layout/AppShell/AppShell.module.css");
    const sidebar = read("src/components/layout/Sidebar/Sidebar.module.css");
    const restoration = read(
      "src/features/session/SessionRestorationShell/SessionRestorationShell.module.css",
    );

    expect(shell.toLowerCase()).not.toContain("#9494f8");
    expect(shell).toMatch(/--app-shell-canvas:\s*var\(--color-background\)/);
    expect(shell).toMatch(/\.workspace\s*\{[^}]*background:\s*var\(--color-workspace\)/s);
    expect(sidebar).toMatch(/\.panel\s*\{[^}]*background:\s*var\(--color-panel\)/s);
    expect(restoration).toMatch(/\.sidebar\s*\{[^}]*background:\s*var\(--color-panel\)/s);
  });
});
