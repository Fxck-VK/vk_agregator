import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/Sidebar/Sidebar.module.css"),
  "utf8",
);

const thumbRule = stylesheet.match(
  /\.conversationsSlot::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/,
)?.[1];

const activeThumbRule = stylesheet.match(
  /\.conversationsSlot:hover::-webkit-scrollbar-thumb,\n\.conversationsSlot:focus-within::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/,
)?.[1];

describe("Sidebar scrollbar", () => {
  it("keeps the chat-list scrollbar compact, token-based, and free of system arrows", () => {
    expect(stylesheet).toContain("@supports (-moz-appearance: none)");
    expect(stylesheet).toMatch(
      /@supports \(-moz-appearance: none\) \{[\s\S]*scrollbar-width: thin;[\s\S]*scrollbar-color: var\(--color-border\) transparent;/,
    );
    expect(stylesheet).toContain(".conversationsSlot::-webkit-scrollbar");
    expect(stylesheet).toContain(".conversationsSlot::-webkit-scrollbar-track");
    expect(stylesheet).toContain(".conversationsSlot::-webkit-scrollbar-button");
    expect(stylesheet).toContain(".conversationsSlot::-webkit-scrollbar-thumb");
    expect(stylesheet).toContain("inline-size: 0.5rem");
    expect(stylesheet).toContain("border-radius: 999px");
    expect(stylesheet).toContain("background-clip: content-box");
    expect(stylesheet).toContain("background-color: var(--color-border)");
    expect(stylesheet).toContain("background-color: var(--color-accent)");
    expect(thumbRule).not.toContain("background:");
    expect(activeThumbRule).not.toContain("background:");
  });
});
