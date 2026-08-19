import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

function readStylesheet(path: string) {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

function getClassBlock(stylesheet: string, className: string) {
  const match = stylesheet.match(new RegExp(`\\.${className}\\s*\\{([\\s\\S]*?)\\n\\}`));

  if (!match) {
    throw new Error(`CSS class .${className} was not found`);
  }

  return match[1];
}

describe("workspace dividers", () => {
  it("keeps the sidebar and workspace content on one continuous surface", () => {
    const stylesheet = readStylesheet("src/components/layout/Sidebar/Sidebar.module.css");

    expect(getClassBlock(stylesheet, "panel")).not.toContain("border-inline-end");
  });

  it("does not draw a divider between chats and the account control", () => {
    const stylesheet = readStylesheet("src/components/layout/Sidebar/Sidebar.module.css");

    expect(getClassBlock(stylesheet, "accountSlot")).not.toContain("border-block-start");
  });

  it("does not draw a divider under the sticky workspace header", () => {
    const stylesheet = readStylesheet(
      "src/components/layout/WorkspaceHeader/WorkspaceHeader.module.css",
    );

    expect(getClassBlock(stylesheet, "header")).not.toContain("border-block-end");
  });

  it("does not draw a divider above the conversation composer", () => {
    const stylesheet = readStylesheet(
      "src/features/conversations/ConversationComposer/ConversationComposer.module.css",
    );

    expect(getClassBlock(stylesheet, "dock")).not.toContain("border-block-start");
  });
});
