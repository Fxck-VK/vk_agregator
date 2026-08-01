import { renderToStaticMarkup } from "react-dom/server";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("@/lib/web-api/server", () => ({ webServerFetch: vi.fn() }));

import { webServerFetch } from "@/lib/web-api/server";

import ConversationPage from "./page";

const conversationID = "d7c979f5-24e5-4f88-924b-a592d6e5a906";

function renderConversationPage(conversationId: string): Promise<string> {
  const page = ConversationPage as unknown as (props: {
    params: Promise<{ conversationId: string }>;
    searchParams: Promise<{ refresh?: string }>;
  }) => ReactElement | Promise<ReactElement>;

  return Promise.resolve(page({ params: Promise.resolve({ conversationId }), searchParams: Promise.resolve({}) })).then((element) => renderToStaticMarkup(element));
}

describe("ConversationPage", () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("loads the server-owned message history and renders only the safe message fields", async () => {
    vi.mocked(webServerFetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              id: "8d2cc78d-1a1c-447f-8bfe-5740c4f9ae95",
              seq: 1,
              role: "user",
              text: "Нарисуй ночной город",
              created_at: "2026-08-01T12:00:00Z",
            },
            {
              id: "1e0c07b8-c043-446b-b219-2825a1ca45f0",
              seq: 2,
              role: "assistant",
              text: "Готовлю результат.",
              created_at: "2026-08-01T12:00:01Z",
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const markup = await renderConversationPage(conversationID);

    expect(webServerFetch).toHaveBeenCalledWith(`/web/v1/conversations/${conversationID}/messages?limit=100`);
    expect(markup).toContain("Нарисуй ночной город");
    expect(markup).toContain("Готовлю результат.");
    expect(markup).not.toContain("conversation_id");
    expect(markup).not.toContain("job_id");
    expect(markup).not.toContain("token_count");
  });

  it("renders an explicit empty chat state", async () => {
    vi.mocked(webServerFetch).mockResolvedValue(
      new Response(JSON.stringify({ items: [] }), { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    const markup = await renderConversationPage(conversationID);

    expect(webServerFetch).toHaveBeenCalledWith(`/web/v1/conversations/${conversationID}/messages?limit=100`);
    expect(markup).toContain("В этом чате пока нет сообщений.");
  });

  it("renders a neutral unavailable state when the server denies access to the chat", async () => {
    vi.mocked(webServerFetch).mockResolvedValue(new Response(JSON.stringify({ error: "conversation not found" }), { status: 404 }));

    const markup = await renderConversationPage(conversationID);

    expect(webServerFetch).toHaveBeenCalledWith(`/web/v1/conversations/${conversationID}/messages?limit=100`);
    expect(markup).toContain("Этот чат недоступен.");
    expect(markup).not.toContain(conversationID);
  });

  it("does not fetch for an invalid route parameter", async () => {
    const markup = await renderConversationPage("not-a-uuid");

    expect(webServerFetch).not.toHaveBeenCalled();
    expect(markup).toContain("Этот чат недоступен.");
    expect(markup).not.toContain("not-a-uuid");
  });

  it("passes the explicit start-prompt refresh signal to conversation history", async () => {
    vi.mocked(webServerFetch).mockResolvedValue(
      new Response(JSON.stringify({ items: [] }), { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    const page = ConversationPage as unknown as (props: {
      params: Promise<{ conversationId: string }>;
      searchParams: Promise<{ refresh?: string }>;
    }) => ReactElement | Promise<ReactElement>;

    const element = await page({
      params: Promise.resolve({ conversationId: conversationID }),
      searchParams: Promise.resolve({ refresh: "1" }),
    });

    expect((element as ReactElement<{ initialRefresh?: boolean }>).props.initialRefresh).toBe(true);
  });
});
