import { afterEach, describe, expect, it } from "vitest";

import {
  clearPendingConversationPrompt,
  readPendingConversationPrompt,
  savePendingConversationPrompt,
} from "./pending-conversation-prompt";

describe("pending conversation prompt", () => {
  afterEach(() => {
    window.sessionStorage.clear();
  });

  it("keeps a normalized prompt until its chat confirms that it is visible", () => {
    savePendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906", "  First stream prompt  ");

    expect(readPendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906")).toBe("First stream prompt");
    expect(readPendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906")).toBe("First stream prompt");

    clearPendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906");

    expect(readPendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906")).toBeNull();
  });
});
