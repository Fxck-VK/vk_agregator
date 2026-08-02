"use client";

import { useEffect } from "react";

import { parseConversationItem, type ConversationItem } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

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
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let request: AbortController | undefined;

    const complete = () => {
      if (active) {
        onComplete?.();
      }
    };

    const poll = async (attempt: number) => {
      if (!active) {
        return;
      }

      const controller = new AbortController();
      request = controller;
      let shouldStop = false;
      try {
        const response = await webBrowserFetch(
          `/web/v1/conversations/${conversationId}` as `/web/v1/${string}`,
          { signal: controller.signal },
        );
        if (!active || controller.signal.aborted) {
          return;
        }
        if (response.status === 401 || response.status === 403 || response.status === 404) {
          shouldStop = true;
          return;
        }
        if (response.status === 200) {
          const conversation = parseConversationItem(await response.json());
          if (conversation.title.trim() !== "" && conversation.title.trim() !== fallbackTitle) {
            onConversation(conversation);
            shouldStop = true;
          }
        }
      } catch {
        // A transient read failure is deliberately retried on this small, bounded schedule.
      } finally {
        if (request === controller) {
          request = undefined;
        }
        if (!active) {
          return;
        }
        if (shouldStop || attempt === retryDelaysMs.length - 1) {
          complete();
          return;
        }
        timer = setTimeout(() => void poll(attempt + 1), retryDelaysMs[attempt + 1]);
      }
    };

    timer = setTimeout(() => void poll(0), retryDelaysMs[0]);
    return () => {
      active = false;
      if (timer !== undefined) {
        clearTimeout(timer);
      }
      request?.abort();
    };
  }, [conversationId, fallbackTitle, onComplete, onConversation]);

  return null;
}
