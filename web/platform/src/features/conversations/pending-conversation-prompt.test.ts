import { afterEach, describe, expect, it } from "vitest";

import {
  clearPendingConversationPrompts,
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

  it("clears every pending prompt on logout without touching unrelated session data", () => {
    savePendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906", "First private prompt");
    savePendingConversationPrompt("62d33e7f-7b0e-4a26-975b-41080b55d78d", "Second private prompt");
    window.sessionStorage.setItem("unrelated.preference", "keep");

    clearPendingConversationPrompts();

    expect(readPendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906")).toBeNull();
    expect(readPendingConversationPrompt("62d33e7f-7b0e-4a26-975b-41080b55d78d")).toBeNull();
    expect(window.sessionStorage.getItem("unrelated.preference")).toBe("keep");
  });
});
