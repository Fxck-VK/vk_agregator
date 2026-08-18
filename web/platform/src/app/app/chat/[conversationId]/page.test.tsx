import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader", () => ({
  ConversationHistoryLoader: vi.fn(() => null),
}));
vi.mock("@/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap", () => ({
  PendingConversationBootstrap: vi.fn(() => null),
}));

import { ConversationHistoryLoader } from "@/features/conversations/ConversationHistoryLoader/ConversationHistoryLoader";
import { PendingConversationBootstrap } from "@/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap";

import ConversationPage from "./page";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";

function renderConversationPage({
  routeConversationId = conversationId,
  refresh,
  pending,
}: {
  routeConversationId?: string;
  refresh?: string | string[];
  pending?: string | string[];
} = {}): Promise<ReactElement> {
  const page = ConversationPage as unknown as (props: {
    params: Promise<{ conversationId: string }>;
    searchParams: Promise<{ pending?: string | string[]; refresh?: string | string[] }>;
  }) => ReactElement | Promise<ReactElement>;

  return Promise.resolve(
    page({
      params: Promise.resolve({ conversationId: routeConversationId }),
      searchParams: Promise.resolve({ pending, refresh }),
    }),
  );
}

describe("ConversationPage", () => {
  it("renders a loader keyed by the route conversation ID", async () => {
    const element = await renderConversationPage();

    expect(element.type).toBe(ConversationHistoryLoader);
    expect(element.key).toBe(conversationId);
    expect(element.props).toMatchObject({ conversationId, initialRefresh: false });
  });

  it("passes the explicit start-prompt refresh signal to the client loader", async () => {
    const element = await renderConversationPage({ refresh: "1" });

    expect(element.props).toMatchObject({ conversationId, initialRefresh: true });
  });

  it("renders the optimistic bootstrap screen for a pending conversation route", async () => {
    const element = await renderConversationPage({ pending: "1" });

    expect(element.type).toBe(PendingConversationBootstrap);
    expect(element.key).toBe(conversationId);
    expect(element.props).toMatchObject({ conversationKey: conversationId });
  });
});
