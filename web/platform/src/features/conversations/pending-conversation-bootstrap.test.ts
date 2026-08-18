import { afterEach, describe, expect, it } from "vitest";

import {
  clearPendingConversationBootstrap,
  clearPendingConversationBootstraps,
  readPendingConversationBootstrap,
  savePendingConversationBootstrap,
  updatePendingConversationBootstrap,
} from "./pending-conversation-bootstrap";

const pendingID = "c7c979f5-24e5-4f88-924b-a592d6e5a906";
const messageKey = "e7c979f5-24e5-4f88-924b-a592d6e5a906";
const canonicalID = "d7c979f5-24e5-4f88-924b-a592d6e5a906";

describe("pending conversation bootstrap", () => {
  afterEach(() => window.sessionStorage.clear());

  it("normalizes and retains the retry-safe creation intent", () => {
    savePendingConversationBootstrap({
      conversationKey: pendingID,
      messageKey,
      prompt: "  Build the launch plan  ",
    });

    expect(readPendingConversationBootstrap(pendingID)).toEqual({
      conversationKey: pendingID,
      messageKey,
      prompt: "Build the launch plan",
    });

    updatePendingConversationBootstrap(pendingID, { conversationId: canonicalID });

    expect(readPendingConversationBootstrap(pendingID)).toEqual({
      conversationKey: pendingID,
      conversationId: canonicalID,
      messageKey,
      prompt: "Build the launch plan",
    });
  });

  it("rejects malformed or mismatched browser data", () => {
    window.sessionStorage.setItem(
      `neirohub.pending-conversation-bootstrap:${pendingID}`,
      JSON.stringify({ conversationKey: canonicalID, messageKey, prompt: "mismatch" }),
    );

    expect(readPendingConversationBootstrap(pendingID)).toBeNull();
    expect(window.sessionStorage.getItem(`neirohub.pending-conversation-bootstrap:${pendingID}`)).toBeNull();
  });

  it("clears one or every private bootstrap without touching unrelated data", () => {
    savePendingConversationBootstrap({ conversationKey: pendingID, messageKey, prompt: "First" });
    savePendingConversationBootstrap({ conversationKey: canonicalID, messageKey: pendingID, prompt: "Second" });
    window.sessionStorage.setItem("unrelated.preference", "keep");

    clearPendingConversationBootstrap(pendingID);
    expect(readPendingConversationBootstrap(pendingID)).toBeNull();
    expect(readPendingConversationBootstrap(canonicalID)?.prompt).toBe("Second");

    clearPendingConversationBootstraps();
    expect(readPendingConversationBootstrap(canonicalID)).toBeNull();
    expect(window.sessionStorage.getItem("unrelated.preference")).toBe("keep");
  });
});
