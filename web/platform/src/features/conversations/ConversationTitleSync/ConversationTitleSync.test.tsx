import { act, cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { webBrowserFetch } from "@/lib/web-api/browser";

import { ConversationTitleSync } from "./ConversationTitleSync";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const fallbackTitle = "Первый вопрос пользователя";
const metadata = {
  id: conversationId,
  created_at: "2026-08-02T10:00:00Z",
  updated_at: "2026-08-02T10:00:00Z",
};

describe("ConversationTitleSync", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("polls one fresh conversation sequentially and only publishes a changed semantic title", async () => {
    vi.useFakeTimers();
    const onConversation = vi.fn();
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ ...metadata, title: fallbackTitle }))
      .mockResolvedValueOnce(Response.json({ ...metadata, title: "План запуска продукта" }));

    render(
      <ConversationTitleSync
        conversationId={conversationId}
        fallbackTitle={fallbackTitle}
        onConversation={onConversation}
      />,
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000);
    });
    expect(webBrowserFetch).toHaveBeenNthCalledWith(
      1,
      `/web/v1/conversations/${conversationId}`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(onConversation).not.toHaveBeenCalled();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000);
    });
    expect(webBrowserFetch).toHaveBeenNthCalledWith(
      2,
      `/web/v1/conversations/${conversationId}`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(onConversation).toHaveBeenCalledWith({ ...metadata, title: "План запуска продукта" });
    expect(vi.mocked(webBrowserFetch)).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    expect(vi.mocked(webBrowserFetch)).toHaveBeenCalledTimes(2);
  });

  it("cancels its one in-flight request when the active chat unmounts", async () => {
    vi.useFakeTimers();
    let requestSignal: AbortSignal | undefined;
    vi.mocked(webBrowserFetch).mockImplementation((_path, init) => {
      requestSignal = init?.signal ?? undefined;
      return new Promise<Response>(() => {});
    });
    const { unmount } = render(
      <ConversationTitleSync
        conversationId={conversationId}
        fallbackTitle={fallbackTitle}
        onConversation={vi.fn()}
      />,
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000);
    });
    unmount();

    expect(requestSignal?.aborted).toBe(true);
  });
});
