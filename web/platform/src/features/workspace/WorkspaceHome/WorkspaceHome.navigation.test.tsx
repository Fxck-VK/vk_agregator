import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("../WorkspacePrompt/WorkspacePrompt", () => ({ WorkspacePrompt: () => <input aria-label="Chat draft" /> }));
vi.mock("../WorkspacePrompt/NewChatPrompt", () => ({ NewChatPrompt: () => <input aria-label="Chat draft" /> }));
vi.mock("../WorkspaceLanding/WorkspaceLanding", () => ({ WorkspaceLanding: () => null }));
vi.mock("@/features/inspiration/InspirationGallery/InspirationGallery", () => ({ InspirationGallery: () => null }));

import { clearConversationCache } from "@/features/conversations/conversation-list-cache";
import { WorkspaceConversationListProvider, useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { WorkspaceHome } from "./WorkspaceHome";

const emptySeed: [] = [];
const conversation = {
  id: "f9712bca-8d98-448d-b595-2a80bc9c2b1a",
  title: "Previously loaded chat",
  created_at: "2026-08-02T09:00:00Z",
  updated_at: "2026-08-02T09:00:00Z",
};

function ListProbe() {
  const { conversations, failed } = useWorkspaceConversationList();
  return <output data-testid="chat-list" data-failed={failed}>{conversations.map(item => item.title).join(",")}</output>;
}

function workspace(section: "chats" | "models", deferred = true) {
  return (
    <WorkspaceConversationListProvider accountId="navigation-account" initialConversations={emptySeed} deferred={deferred}>
      <ListProbe />
      <WorkspaceHome section={section} />
    </WorkspaceConversationListProvider>
  );
}

describe("chat screen return", () => {
  afterEach(() => { cleanup(); clearConversationCache(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

  it("shows the loaded list immediately and revalidates on return without replacing the draft", async () => {
    let finish!: (response: Response) => void;
    const send = vi.fn()
      .mockResolvedValueOnce(Response.json({ items: [conversation] }))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { finish = resolve; }));
    vi.stubGlobal("fetch", send);
    const view = render(workspace("models"));
    await waitFor(() => expect(screen.getByTestId("chat-list")).toHaveTextContent(conversation.title));
    view.rerender(workspace("chats"));
    expect(screen.getByTestId("chat-list")).toHaveTextContent(conversation.title);
    await waitFor(() => expect(send).toHaveBeenCalledTimes(2));
    const draft = screen.getByLabelText("Chat draft");
    fireEvent.change(draft, { target: { value: "Keep my draft" } });
    await act(async () => { finish(Response.json({ items: [{ ...conversation, title: "Updated chat" }] })); });
    expect(screen.getByTestId("chat-list")).toHaveTextContent("Updated chat");
    expect(screen.getByLabelText("Chat draft")).toBe(draft);
    expect(draft).toHaveValue("Keep my draft");
  });

  it("retains the list when a return refresh fails", async () => {
    const send = vi.fn()
      .mockResolvedValueOnce(Response.json({ items: [conversation] }))
      .mockResolvedValueOnce(new Response(null, { status: 503 }));
    vi.stubGlobal("fetch", send);
    const view = render(workspace("models"));
    await waitFor(() => expect(screen.getByTestId("chat-list")).toHaveTextContent(conversation.title));
    view.rerender(workspace("chats"));
    await waitFor(() => expect(screen.getByTestId("chat-list")).toHaveAttribute("data-failed", "true"));
    expect(screen.getByTestId("chat-list")).toHaveTextContent(conversation.title);
  });

  it("coalesces entry with an initial pending read and leaves standalone seeded screens alone", async () => {
    const send = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal("fetch", send);
    const view = render(workspace("chats"));
    await act(async () => undefined);
    expect(send).toHaveBeenCalledTimes(1);
    view.unmount();
    send.mockClear();
    render(workspace("chats", false));
    await act(async () => undefined);
    expect(send).not.toHaveBeenCalled();
  });

  it.each(["hidden", "offline"])("does not add a return read while %s", async condition => {
    const send = vi.fn(async () => Response.json({ items: [conversation] }));
    vi.stubGlobal("fetch", send);
    const view = render(workspace("models"));
    await waitFor(() => expect(screen.getByTestId("chat-list")).toHaveTextContent(conversation.title));
    if (condition === "hidden") vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    else vi.spyOn(navigator, "onLine", "get").mockReturnValue(false);
    view.rerender(workspace("chats"));
    await act(async () => undefined);
    expect(send).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("chat-list")).toHaveTextContent(conversation.title);
  });
});
