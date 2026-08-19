import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/layout/Sidebar/Sidebar.module.css"),
  "utf8",
);
const conversationsStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/conversations/SidebarConversations/SidebarConversations.module.css"),
  "utf8",
);

const thumbRule = conversationsStylesheet.match(
  /\.list::-webkit-scrollbar-thumb \{([\s\S]*?)\n\}/,
)?.[1];

const activeThumbRule = conversationsStylesheet.match(
  /\.list:hover::-webkit-scrollbar-thumb,\r?\n\.list:focus-within::-webkit-scrollbar-thumb \{([\s\S]*?)\r?\n\}/,
)?.[1];

const scrollAreaRule = stylesheet.match(/\.scrollArea \{([\s\S]*?)\n\}/)?.[1];
const conversationsSlotRule = stylesheet.match(/\.conversationsSlot \{([\s\S]*?)\n\}/)?.[1];
const listRule = conversationsStylesheet.match(/\.list \{([\s\S]*?)\n\}/)?.[1];

describe("Sidebar scrollbar", () => {
  it("keeps navigation fixed and makes only an overflowing chat list scrollable", () => {
    expect(scrollAreaRule).toContain("flex: 1 1 auto");
    expect(scrollAreaRule).toContain("min-block-size: 0");
    expect(scrollAreaRule).toContain("overflow: hidden");
    expect(scrollAreaRule).not.toContain("overflow-y: auto");
    expect(conversationsSlotRule).toContain("flex: 1 1 auto");
    expect(conversationsSlotRule).toContain("min-block-size: 0");
    expect(conversationsSlotRule).toContain("overflow: hidden");
    expect(listRule).toContain("min-block-size: 0");
    expect(listRule).toContain("overflow-y: auto");
    expect(conversationsStylesheet.match(/overflow-y:\s*auto/g)).toHaveLength(1);
    expect(conversationsStylesheet).toContain("@supports (-moz-appearance: none)");
    expect(conversationsStylesheet).toMatch(
      /@supports \(-moz-appearance: none\) \{[\s\S]*scrollbar-width: thin;[\s\S]*scrollbar-color: transparent transparent;[\s\S]*\.list:hover[\s\S]*scrollbar-color: var\(--color-accent\) transparent;/,
    );
    expect(conversationsStylesheet).toContain(".list::-webkit-scrollbar");
    expect(conversationsStylesheet).toContain(".list::-webkit-scrollbar-track");
    expect(conversationsStylesheet).toContain(".list::-webkit-scrollbar-button");
    expect(conversationsStylesheet).toContain(".list::-webkit-scrollbar-thumb");
    expect(conversationsStylesheet).toContain("inline-size: 0.5rem");
    expect(conversationsStylesheet).toContain("border-radius: 999px");
    expect(conversationsStylesheet).toContain("background-clip: content-box");
    expect(thumbRule).toContain("background-color: transparent");
    expect(activeThumbRule).toContain("background-color: var(--color-accent)");
    expect(thumbRule).not.toContain("background:");
    expect(activeThumbRule).toBeDefined();
    expect(activeThumbRule).not.toContain("background:");
  });
});
