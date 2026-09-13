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
const conversationsComponent = readFileSync(
  resolve(process.cwd(), "src/features/conversations/SidebarConversations/SidebarConversations.tsx"),
  "utf8",
);

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
    expect(conversationsComponent).toContain('from "@/components/ui/ScrollArea/ScrollArea"');
    expect(conversationsComponent).toContain('viewportAs="ul"');
    expect(conversationsComponent).toContain('"data-sidebar-conversation-list": "true"');
    expect(conversationsStylesheet).not.toMatch(/overflow-y:\s*(?:auto|scroll)/);
    expect(conversationsStylesheet).not.toContain("scrollbar-width:");
    expect(conversationsStylesheet).not.toContain("::-webkit-scrollbar");
  });
});
