import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/components/layout/AppShell/AppShell.module.css"), "utf8");

const workspaceRule = stylesheet.match(/\.workspace \{([\s\S]*?)\n\}/)?.[1];
const thumbRule = stylesheet.match(/\.workspace::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/)?.[1];

describe("AppShell workspace scrollbar", () => {
  it("uses a narrow dark custom scrollbar without native arrow buttons", () => {
    expect(workspaceRule).toContain("overflow-y: auto");
    expect(stylesheet).toMatch(
      /@supports \(-moz-appearance: none\) \{[\s\S]*\.workspace \{[\s\S]*scrollbar-width: thin;[\s\S]*scrollbar-color: var\(--color-border\) transparent;/,
    );
    expect(stylesheet).toContain(".workspace::-webkit-scrollbar");
    expect(stylesheet).toContain(".workspace::-webkit-scrollbar-track");
    expect(stylesheet).toContain(".workspace::-webkit-scrollbar-button");
    expect(stylesheet).toContain(".workspace::-webkit-scrollbar-thumb");
    expect(stylesheet).toContain("inline-size: 0.75rem");
    expect(stylesheet).toContain("display: none");
    expect(thumbRule).toContain("border-radius: 999px");
    expect(thumbRule).toContain("background-color: var(--color-border)");
    expect(thumbRule).toContain("background-clip: content-box");
  });
});
