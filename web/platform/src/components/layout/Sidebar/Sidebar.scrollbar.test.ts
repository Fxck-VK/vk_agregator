import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/Sidebar/Sidebar.module.css"),
  "utf8",
);

const thumbRule = stylesheet.match(
  /\.scrollArea::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/,
)?.[1];

const activeThumbRule = stylesheet.match(
  /\.scrollArea:hover::-webkit-scrollbar-thumb,\n\.scrollArea:focus-within::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/,
)?.[1];

const scrollAreaRule = stylesheet.match(/\.scrollArea \{([\s\S]*?)\n\}/)?.[1];
const conversationsSlotRule = stylesheet.match(/\.conversationsSlot \{([\s\S]*?)\n\}/)?.[1];

describe("Sidebar scrollbar", () => {
  it("makes the shared navigation and chat area the only vertical scroll owner with a compact custom scrollbar", () => {
    expect(scrollAreaRule).toContain("flex: 1 1 auto");
    expect(scrollAreaRule).toContain("min-block-size: 0");
    expect(scrollAreaRule).toContain("overflow-y: auto");
    expect(conversationsSlotRule).toBeUndefined();
    expect(stylesheet.match(/overflow-y:\s*auto/g)).toHaveLength(1);
    expect(stylesheet).toContain("@supports (-moz-appearance: none)");
    expect(stylesheet).toMatch(
      /@supports \(-moz-appearance: none\) \{[\s\S]*scrollbar-width: thin;[\s\S]*scrollbar-color: var\(--color-border\) transparent;/,
    );
    expect(stylesheet).toContain(".scrollArea::-webkit-scrollbar");
    expect(stylesheet).toContain(".scrollArea::-webkit-scrollbar-track");
    expect(stylesheet).toContain(".scrollArea::-webkit-scrollbar-button");
    expect(stylesheet).toContain(".scrollArea::-webkit-scrollbar-thumb");
    expect(stylesheet).toContain("inline-size: 0.5rem");
    expect(stylesheet).toContain("border-radius: 999px");
    expect(stylesheet).toContain("background-clip: content-box");
    expect(stylesheet).toContain("background-color: var(--color-border)");
    expect(stylesheet).toContain("background-color: var(--color-accent)");
    expect(thumbRule).not.toContain("background:");
    expect(activeThumbRule).not.toContain("background:");
  });
});
