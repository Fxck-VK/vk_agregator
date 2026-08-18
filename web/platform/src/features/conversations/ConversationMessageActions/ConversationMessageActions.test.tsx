import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn() }));

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { ConversationMessageActions } from "./ConversationMessageActions";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const messageId = "22222222-2222-4222-8222-222222222222";

function renderAssistant(initialRating: "like" | "dislike" | null = null) {
  return render(
    <ConversationMessageActions
      conversationId={conversationId}
      initialRating={initialRating}
      kind="assistant"
      messageId={messageId}
      messageText="Assistant answer"
    />,
  );
}

describe("ConversationMessageActions assistant rating", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("starts from the persisted rating and changes optimistically", () => {
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise<Response>(() => {}));
    renderAssistant("like");

    const like = screen.getByRole("button", { name: ru.conversations.likeMessage });
    const dislike = screen.getByRole("button", { name: ru.conversations.dislikeMessage });
    expect(like).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(dislike);

    expect(like).toHaveAttribute("aria-pressed", "false");
    expect(dislike).toHaveAttribute("aria-pressed", "true");
    expect(webBrowserMutation).toHaveBeenCalledWith(
      `/web/v1/conversations/${conversationId}/messages/${messageId}/rating`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rating: "dislike" }),
      },
    );
  });

  it("rolls the latest optimistic change back when persistence fails", async () => {
    vi.mocked(webBrowserMutation).mockRejectedValueOnce(new Error("offline"));
    renderAssistant("like");

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.dislikeMessage }));
    await vi.waitFor(() => expect(screen.getByRole("button", { name: ru.conversations.likeMessage })).toHaveAttribute("aria-pressed", "true"));
    expect(screen.getByRole("button", { name: ru.conversations.dislikeMessage })).toHaveAttribute("aria-pressed", "false");
  });

  it("serializes rapid changes so an older response cannot overwrite the latest click", async () => {
    let settleFirst: (response: Response) => void = () => {};
    let settleSecond: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation)
      .mockReturnValueOnce(new Promise<Response>((resolve) => { settleFirst = resolve; }))
      .mockReturnValueOnce(new Promise<Response>((resolve) => { settleSecond = resolve; }));
    renderAssistant();

    const like = screen.getByRole("button", { name: ru.conversations.likeMessage });
    const dislike = screen.getByRole("button", { name: ru.conversations.dislikeMessage });
    fireEvent.click(like);
    fireEvent.click(dislike);

    expect(dislike).toHaveAttribute("aria-pressed", "true");
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);

    await act(async () => settleFirst(Response.json({ rating: "like" }, { status: 200 })));
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(2));
    expect(dislike).toHaveAttribute("aria-pressed", "true");

    await act(async () => settleSecond(Response.json({ rating: "dislike" }, { status: 200 })));
    await vi.waitFor(() => expect(dislike).toHaveAttribute("aria-pressed", "true"));
  });

  it("sends null when the selected rating is clicked again", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ rating: null }, { status: 200 }));
    renderAssistant("like");

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.likeMessage }));

    expect(screen.getByRole("button", { name: ru.conversations.likeMessage })).toHaveAttribute("aria-pressed", "false");
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledWith(
      `/web/v1/conversations/${conversationId}/messages/${messageId}/rating`,
      expect.objectContaining({ body: JSON.stringify({ rating: null }) }),
    ));
  });
});
