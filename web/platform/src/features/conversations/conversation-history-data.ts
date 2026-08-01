import "server-only";

import { z } from "zod";

import {
  parseConversationMessageList,
} from "@/lib/web-api/contracts";
import { webServerFetch } from "@/lib/web-api/server";
import {
  conversationHistoryPageLimit,
  type ConversationHistoryData,
} from "./conversation-history-contract";

const conversationIDSchema = z.string().uuid();

export type { ConversationHistoryData } from "./conversation-history-contract";

export async function loadConversationHistory(rawConversationID: string): Promise<ConversationHistoryData> {
  const parsedConversationID = conversationIDSchema.safeParse(rawConversationID);
  if (!parsedConversationID.success) {
    return { kind: "not_found" };
  }

  try {
    const response = await webServerFetch(
      `/web/v1/conversations/${parsedConversationID.data}/messages?limit=${conversationHistoryPageLimit}` as `/web/v1/${string}`,
    );
    if (response.status === 404) {
      return { kind: "not_found" };
    }
    if (response.status !== 200) {
      return { kind: "unavailable" };
    }
    const history = parseConversationMessageList(await response.json());
    return {
      kind: "ready",
      conversationId: parsedConversationID.data,
      messages: history.items,
      hasMoreBefore: history.has_more_before,
    };
  } catch {
    return { kind: "unavailable" };
  }
}
