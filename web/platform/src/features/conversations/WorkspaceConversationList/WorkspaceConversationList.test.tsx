import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import type { ConversationItem } from "@/lib/web-api/contracts";

import { WorkspaceConversationListProvider, useWorkspaceConversationList } from "./WorkspaceConversationList";

const accountAConversations: ConversationItem[] = [
  {
    id: "f9712bca-8d98-448d-b595-2a80bc9c2b1a",
    title: "Account A existing chat",
    created_at: "2026-08-02T09:00:00Z",
    updated_at: "2026-08-02T09:00:00Z",
  },
];

const createdConversation: ConversationItem = {
  id: "8e1a77d8-0a9c-45ef-ba5f-1c5cc6e1772b",
  title: "New chat",
  created_at: "2026-08-02T10:00:00Z",
  updated_at: "2026-08-02T10:00:00Z",
};

function ConversationListProbe({ conversation = createdConversation }: { conversation?: ConversationItem }) {
  const { conversations, upsertConversation } = useWorkspaceConversationList();

  return (
    <>
      <button onClick={() => upsertConversation(conversation)} type="button">Upsert</button>
      <output>{conversations.map(({ id, title }) => `${id}:${title}`).join(",")}</output>
    </>
  );
}

describe("WorkspaceConversationListProvider", () => {
  afterEach(() => cleanup());

  it("renders the supplied initial conversations", () => {
    render(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    expect(screen.getByRole("status")).toHaveTextContent(`${accountAConversations[0].id}:${accountAConversations[0].title}`);
  });

  it("prepends a returned conversation DTO", () => {
    render(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Upsert" }));

    expect(screen.getByRole("status")).toHaveTextContent(`${createdConversation.id}:${createdConversation.title},${accountAConversations[0].id}:${accountAConversations[0].title}`);
  });

  it("replaces a same-id conversation without creating a duplicate", () => {
    const updatedConversation = { ...createdConversation, title: "Renamed chat" };
    const rendered = render(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Upsert" }));
    rendered.rerender(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <ConversationListProbe conversation={updatedConversation} />
      </WorkspaceConversationListProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Upsert" }));

    expect(screen.getByRole("status")).toHaveTextContent(`${createdConversation.id}:Renamed chat,${accountAConversations[0].id}:${accountAConversations[0].title}`);
    expect(screen.getByRole("status").textContent?.match(new RegExp(createdConversation.id, "g"))).toHaveLength(1);
  });

  it("reconciles a changed server list", () => {
    const refreshedConversations = [{ ...createdConversation, title: "Server title" }];
    const rendered = render(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    rendered.rerender(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={refreshedConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    expect(screen.getByRole("status")).toHaveTextContent(`${createdConversation.id}:Server title`);
    expect(screen.getByRole("status")).not.toHaveTextContent(accountAConversations[0].id);
  });

  it("exposes the changed server list to children on their first rerender", () => {
    const refreshedConversations = [{ ...createdConversation, title: "Server title" }];
    const observedLists: string[][] = [];

    function RenderProbe() {
      const { conversations } = useWorkspaceConversationList();
      observedLists.push(conversations.map((conversation) => conversation.id));

      return null;
    }

    const rendered = render(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <RenderProbe />
      </WorkspaceConversationListProvider>,
    );
    observedLists.length = 0;

    rendered.rerender(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={refreshedConversations}>
        <RenderProbe />
      </WorkspaceConversationListProvider>,
    );

    expect(observedLists[0]).toEqual([createdConversation.id]);
  });

  it("resets to the next account's initial list", () => {
    const accountBConversations: ConversationItem[] = [{ ...createdConversation, title: "Account B chat" }];
    const rendered = render(
      <WorkspaceConversationListProvider accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c" initialConversations={accountAConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Upsert" }));
    rendered.rerender(
      <WorkspaceConversationListProvider accountId="b46c468b-114e-498e-93e3-4f90964a4f10" initialConversations={accountBConversations}>
        <ConversationListProbe />
      </WorkspaceConversationListProvider>,
    );

    expect(screen.getByRole("status")).toHaveTextContent(`${createdConversation.id}:Account B chat`);
    expect(screen.getByRole("status")).not.toHaveTextContent(accountAConversations[0].id);
  });
});
