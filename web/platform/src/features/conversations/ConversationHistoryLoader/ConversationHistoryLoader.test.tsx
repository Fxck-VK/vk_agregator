import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import {
  useWorkspaceDataCache,
  WorkspaceDataCacheProvider,
  type WorkspaceDataCache,
} from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { ru } from "@/i18n/ru";
import { webBrowserFetch } from "@/lib/web-api/browser";

import { ConversationHistoryLoader } from "./ConversationHistoryLoader";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";

const cachedHistory = {
  kind: "ready" as const,
  conversationId,
  hasMoreBefore: false,
  messages: [
    {
      id: "11111111-1111-4111-8111-111111111111",
      seq: 1,
      role: "user" as const,
      text: "cached private message",
      rating: null,
      created_at: "2026-08-01T12:00:00Z",
    },
  ],
};

function WorkspaceDataCacheSeed({ history }: { history?: typeof cachedHistory }) {
  const cache = useWorkspaceDataCache();

  if (history !== undefined) {
    cache.setConversationHistory(history);
  }

  return null;
}

function WorkspaceDataCacheProbe({ onCache }: { onCache: (cache: WorkspaceDataCache) => void }) {
  onCache(useWorkspaceDataCache());

  return null;
}

function renderLoader({
  cacheHistory,
  loaderConversationId = conversationId,
  onCache = () => undefined,
}: {
  cacheHistory?: typeof cachedHistory;
  loaderConversationId?: string;
  onCache?: (cache: WorkspaceDataCache) => void;
} = {}) {
  return render(
    <WorkspaceDataCacheProvider>
      <WorkspaceDataCacheSeed history={cacheHistory} />
      <WorkspaceDataCacheProbe onCache={onCache} />
      <ConversationHistoryLoader conversationId={loaderConversationId} />
    </WorkspaceDataCacheProvider>,
  );
}

describe("ConversationHistoryLoader", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders a cached message while revalidation is pending", () => {
    vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise<Response>(() => {}));

    renderLoader({ cacheHistory: cachedHistory });

    expect(screen.getByText("cached private message")).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledWith(
      `/web/v1/conversations/${conversationId}/messages?limit=100`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("replaces cached messages when revalidation returns ready history", async () => {
    let resolveRevalidation: (response: Response) => void = () => {};
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRevalidation = resolve;
      }),
    );

    renderLoader({ cacheHistory: cachedHistory });

    expect(screen.getByText("cached private message")).toBeInTheDocument();
    resolveRevalidation(
      Response.json({
        items: [
          {
            id: "22222222-2222-4222-8222-222222222222",
            seq: 2,
            role: "assistant",
            text: "fresh private message",
            created_at: "2026-08-01T12:00:01Z",
          },
        ],
      }),
    );

    expect(await screen.findByText("fresh private message")).toBeInTheDocument();
    expect(screen.queryByText("cached private message")).not.toBeInTheDocument();
  });

  it("does not cache a not-found history", async () => {
    let observedCache: WorkspaceDataCache | undefined;
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 404 }));

    renderLoader({ onCache: (cache) => { observedCache = cache; } });

    await screen.findByRole("status");
    expect(observedCache?.getConversationHistory(conversationId)).toBeUndefined();
  });

  it("keeps cached history visible when a background revalidation fails", async () => {
    let resolveRevalidation: (response: Response) => void = () => {};
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRevalidation = resolve;
      }),
    );

    renderLoader({ cacheHistory: cachedHistory });
    expect(screen.getByText("cached private message")).toBeInTheDocument();

    await act(async () => {
      resolveRevalidation(new Response(null, { status: 503 }));
      await Promise.resolve();
    });

    expect(screen.getByText("cached private message")).toBeInTheDocument();
    expect(screen.queryByText(ru.conversations.historyLoadFailure)).not.toBeInTheDocument();
  });

  it("shows an unavailable state on a cold revalidation failure", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 }));

    renderLoader();

    expect(await screen.findByText(ru.conversations.historyLoadFailure)).toBeInTheDocument();
  });

  it("evicts cached history when the server returns not found", async () => {
    let observedCache: WorkspaceDataCache | undefined;
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 404 }));

    renderLoader({ cacheHistory: cachedHistory, onCache: (cache) => { observedCache = cache; } });

    expect(await screen.findByText(ru.conversations.historyUnavailable)).toBeInTheDocument();
    expect(observedCache?.getConversationHistory(conversationId)).toBeUndefined();
  });

  it("does not fetch an invalid conversation UUID", async () => {
    renderLoader({ loaderConversationId: "not-a-uuid" });

    await screen.findByRole("status");
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
});
