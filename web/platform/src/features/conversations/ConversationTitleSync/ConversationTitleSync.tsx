"use client";

import { useEffect } from "react";

import { parseConversationItem, type ConversationItem } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { startGenerationPolling } from "@/lib/generation-polling";

const retryDelaysMs = [1_000, 2_000, 4_000, 8_000, 15_000] as const;

type ConversationTitleSyncProps = {
  conversationId: string;
  fallbackTitle: string;
  onComplete?: () => void;
  onConversation: (conversation: ConversationItem) => void;
};

// A newly-created chat makes at most five short, sequential requests. It never
// refreshes the route or reloads the sidebar, so a title update cannot delay navigation.
export function ConversationTitleSync({
  conversationId,
  fallbackTitle,
  onComplete,
  onConversation,
}: ConversationTitleSyncProps) {
  useEffect(() => {
    const polling = startGenerationPolling({
      delayMs: attempt => retryDelaysMs[Math.min(attempt, retryDelaysMs.length - 1)],
      initialDelayMs: retryDelaysMs[0], maxAttempts: retryDelaysMs.length,
      onExhausted: onComplete,
      poll: async (signal) => {
        try {
          const response = await webBrowserFetch(
            `/web/v1/conversations/${conversationId}` as `/web/v1/${string}`, { signal },
          );
          if (signal.aborted) return false;
          if (response.status === 401 || response.status === 403 || response.status === 404) {
            onComplete?.();
            return false;
          }
          if (response.status === 200) {
            const conversation = parseConversationItem(await response.json());
            if (signal.aborted) return false;
            if (conversation.title.trim() !== "" && conversation.title.trim() !== fallbackTitle) {
              onConversation(conversation);
              onComplete?.();
              return false;
            }
          }
        } catch {
          // A transient read failure is retried on the existing bounded schedule.
        }
        return true;
      },
    });
    return polling.stop;
  }, [conversationId, fallbackTitle, onComplete, onConversation]);

  return null;
}
