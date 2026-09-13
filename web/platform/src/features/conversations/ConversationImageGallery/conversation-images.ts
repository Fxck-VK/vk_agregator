import {
  parseConversationMessageList,
  type ConversationImage,
  type ConversationMessage,
} from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { conversationHistoryPageLimit } from "../conversation-history-contract";

export function collectConversationImages(messages: readonly ConversationMessage[]): ConversationImage[] {
  const images = new Map<string, ConversationImage>();
  for (const message of [...messages].sort((left, right) => left.seq - right.seq)) {
    if (message.role !== "assistant") continue;
    for (const image of message.images ?? []) {
      if (!images.has(image.artifact.id)) images.set(image.artifact.id, image);
    }
  }
  return [...images.values()];
}

export async function loadEarlierConversationImages(
  conversationID: string,
  beforeSeq: number,
  signal: AbortSignal,
): Promise<ConversationMessage[]> {
  const messages: ConversationMessage[] = [];
  let cursor = beforeSeq;
  while (!signal.aborted) {
    const response = await webBrowserFetch(
      `/web/v1/conversations/${conversationID}/messages?before_seq=${cursor}&limit=${conversationHistoryPageLimit}`,
      { signal },
    );
    if (response.status !== 200) throw new Error("Unable to load conversation images.");
    const page = parseConversationMessageList(await response.json());
    if (signal.aborted) throw new Error("Conversation image loading was cancelled.");
    const olderMessages = page.items.filter((message) => message.seq < cursor);
    messages.push(...olderMessages);
    if (!page.has_more_before) return messages;
    if (olderMessages.length === 0) throw new Error("Conversation cursor did not advance.");
    cursor = Math.min(...olderMessages.map((message) => message.seq));
  }
  throw new Error("Conversation image loading was cancelled.");
}
