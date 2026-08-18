import { afterEach, describe, expect, it } from "vitest";

import {
  clearPendingConversationTitleSyncs,
  readPendingConversationTitleSync,
  savePendingConversationTitleSync,
} from "./pending-conversation-title-sync";

describe("pending conversation title sync", () => {
  afterEach(() => {
    window.sessionStorage.clear();
  });

  it("clears every pending title on logout without touching unrelated session data", () => {
    savePendingConversationTitleSync("d7c979f5-24e5-4f88-924b-a592d6e5a906", "First private title");
    savePendingConversationTitleSync("62d33e7f-7b0e-4a26-975b-41080b55d78d", "Second private title");
    window.sessionStorage.setItem("unrelated.preference", "keep");

    clearPendingConversationTitleSyncs();

    expect(readPendingConversationTitleSync("d7c979f5-24e5-4f88-924b-a592d6e5a906")).toBeNull();
    expect(readPendingConversationTitleSync("62d33e7f-7b0e-4a26-975b-41080b55d78d")).toBeNull();
    expect(window.sessionStorage.getItem("unrelated.preference")).toBe("keep");
  });
});

