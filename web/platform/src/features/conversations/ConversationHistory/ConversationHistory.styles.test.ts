import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/conversations/ConversationHistory/ConversationHistory.module.css"),
  "utf8",
);

function getClassBlock(className: string) {
  const match = stylesheet.match(new RegExp(`\\.${className}\\s*\\{([\\s\\S]*?)\\n\\}`));

  if (!match) {
    throw new Error(`CSS class .${className} was not found`);
  }

  return match[1];
}

describe("ConversationHistory message surfaces", () => {
  it("keeps user prompts compact and renders assistant replies directly on the page", () => {
    const userMessage = getClassBlock("userMessage");
    const assistantMessage = getClassBlock("assistantMessage");

    expect(userMessage).toContain("justify-self: end");
    expect(userMessage).toContain("inline-size: fit-content");
    expect(userMessage).toContain("max-inline-size:");
    expect(assistantMessage).toContain("border: 0");
    expect(assistantMessage).toContain("padding: 0");
    expect(assistantMessage).toContain("background: transparent");
  });
});
