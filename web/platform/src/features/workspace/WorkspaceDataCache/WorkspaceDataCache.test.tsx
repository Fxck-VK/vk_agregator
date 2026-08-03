import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ConversationHistoryData } from "@/features/conversations/conversation-history-data";
import type { ImageJobList } from "@/lib/web-api/contracts";

import {
  createWorkspaceDataCache,
  useWorkspaceDataCache,
  WorkspaceDataCacheProvider,
  type WorkspaceDataCache,
} from "./WorkspaceDataCache";

function readyHistory(conversationId: string): Extract<ConversationHistoryData, { kind: "ready" }> {
  return { kind: "ready", conversationId, messages: [], hasMoreBefore: false };
}

function WorkspaceDataCacheProbe({ onCache }: { onCache: (cache: WorkspaceDataCache) => void }) {
  onCache(useWorkspaceDataCache());

  return null;
}

describe("WorkspaceDataCache", () => {
  it("stores ready conversation histories only", () => {
    const cache = createWorkspaceDataCache();
    const ready = readyHistory("ready-conversation");

    cache.setConversationHistory(ready);
    cache.setConversationHistory({ kind: "not_found" });
    cache.setConversationHistory({ kind: "unavailable" });

    expect(cache.getConversationHistory(ready.conversationId)).toBe(ready);
    expect(cache.getConversationHistory("not-found-conversation")).toBeUndefined();
    expect(cache.getConversationHistory("unavailable-conversation")).toBeUndefined();
  });

  it("touches a history on read before evicting the oldest of nine histories", () => {
    const cache = createWorkspaceDataCache();
    const histories = Array.from({ length: 9 }, (_, index) => readyHistory(`conversation-${index + 1}`));

    histories.slice(0, 8).forEach((history) => cache.setConversationHistory(history));
    expect(cache.getConversationHistory(histories[0].conversationId)).toBe(histories[0]);
    cache.setConversationHistory(histories[8]);

    expect(cache.getConversationHistory(histories[1].conversationId)).toBeUndefined();
    expect(cache.getConversationHistory(histories[0].conversationId)).toBe(histories[0]);
    expect(cache.getConversationHistory(histories[8].conversationId)).toBe(histories[8]);
  });

  it("replaces the cached image files first page", () => {
    const cache = createWorkspaceDataCache();
    const firstPage: ImageJobList = { items: [], has_more: true, next_cursor: "first-page" };
    const refreshedFirstPage: ImageJobList = { items: [], has_more: false, next_cursor: null };

    cache.setImageFilesFirstPage(firstPage);
    expect(cache.getImageFilesFirstPage()).toBe(firstPage);

    cache.setImageFilesFirstPage(refreshedFirstPage);
    expect(cache.getImageFilesFirstPage()).toBe(refreshedFirstPage);
  });

  it("requires WorkspaceDataCacheProvider", () => {
    expect(() => render(<WorkspaceDataCacheProbe onCache={() => undefined} />)).toThrow(
      "useWorkspaceDataCache must be used within WorkspaceDataCacheProvider.",
    );
  });

  it("keeps one cache instance for the provider lifetime", () => {
    const observedCaches: WorkspaceDataCache[] = [];
    const onCache = (cache: WorkspaceDataCache) => observedCaches.push(cache);
    const rendered = render(
      <WorkspaceDataCacheProvider>
        <WorkspaceDataCacheProbe onCache={onCache} />
      </WorkspaceDataCacheProvider>,
    );

    rendered.rerender(
      <WorkspaceDataCacheProvider>
        <WorkspaceDataCacheProbe onCache={onCache} />
      </WorkspaceDataCacheProvider>,
    );

    expect(observedCaches).toHaveLength(2);
    expect(observedCaches[1]).toBe(observedCaches[0]);
  });
});
