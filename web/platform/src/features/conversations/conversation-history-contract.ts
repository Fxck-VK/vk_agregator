import type { ConversationMessage } from "@/lib/web-api/contracts";

export const conversationHistoryPageLimit = 100;

export type ConversationHistoryData =
  | {
      kind: "ready";
      conversationId: string;
      messages: ConversationMessage[];
      hasMoreBefore: boolean;
    }
  | { kind: "not_found" }
  | { kind: "unavailable" };
