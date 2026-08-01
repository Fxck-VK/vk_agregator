import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
  webBrowserMutation: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

import { ConversationHistory } from "./ConversationHistory";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const queuedJob = {
  job_id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
  status: "queued",
};

const initialHistory = {
  kind: "ready" as const,
  conversationId,
  hasMoreBefore: true,
  messages: [
    {
      id: "11111111-1111-4111-8111-111111111111",
      seq: 102,
      role: "user" as const,
      text: "message 102",
      created_at: "2026-08-01T12:00:00Z",
    },
    {
      id: "22222222-2222-4222-8222-222222222222",
      seq: 103,
      role: "assistant" as const,
      text: "message 103",
      created_at: "2026-08-01T12:00:01Z",
    },
  ],
};

describe("ConversationHistory", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("prepends a bounded older page and keeps its next cursor on the first loaded message", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(
      Response.json({
        items: [
          {
            id: "99999999-9999-4999-8999-999999999999",
            seq: 100,
            role: "user",
            text: "message 100",
            created_at: "2026-08-01T11:59:58Z",
          },
          {
            id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
            seq: 101,
            role: "assistant",
            text: "message 101",
            created_at: "2026-08-01T11:59:59Z",
          },
        ],
        has_more_before: true,
      }),
    );

    render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.historyLoadEarlier }));

    await vi.waitFor(() =>
      expect(webBrowserFetch).toHaveBeenCalledWith(
        `/web/v1/conversations/${conversationId}/messages?before_seq=102&limit=100`,
      ),
    );
    await screen.findByText("message 100");
    expect(screen.getAllByRole("listitem").map((item) => item.textContent)).toEqual([
      expect.stringContaining("message 100"),
      expect.stringContaining("message 101"),
      expect.stringContaining("message 102"),
      expect.stringContaining("message 103"),
    ]);
  });

  it("resets local paging state when the user opens a different chat", () => {
    const { rerender } = render(<ConversationHistory history={initialHistory as never} />);

    rerender(
      <ConversationHistory
        history={{
          kind: "ready",
          conversationId: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
          hasMoreBefore: false,
          messages: [
            {
              id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
              seq: 1,
              role: "user",
              text: "message from another chat",
              created_at: "2026-08-01T12:01:00Z",
            },
          ],
        } as never}
      />,
    );

    expect(screen.getByText("message from another chat")).toBeTruthy();
    expect(screen.queryByText("message 102")).toBeNull();
  });

  it("does not poll for newer messages before a user message is accepted", () => {
    render(<ConversationHistory history={initialHistory as never} />);

    expect(webBrowserFetch).not.toHaveBeenCalled();
  });

  it("keeps the composer disabled until the accepted message refresh observes its assistant reply", async () => {
    let resolveRefresh: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRefresh = resolve;
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Первый запрос" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    await vi.waitFor(() => expect(webBrowserFetch).toHaveBeenCalledTimes(1));
    expect(textarea).toBeDisabled();
    expect(screen.getByRole("button", { name: ru.conversations.composerSubmit })).toBeDisabled();
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);

    resolveRefresh(
      Response.json({
        items: [
          {
            id: "66666666-6666-4666-8666-666666666666",
            seq: 104,
            role: "assistant",
            text: "ответ на первый запрос",
            created_at: "2026-08-01T12:00:05Z",
          },
        ],
      }),
    );

    await screen.findByText("ответ на первый запрос");
    await vi.waitFor(() => expect(textarea).not.toBeDisabled());
    fireEvent.change(textarea, { target: { value: "Второй запрос" } });
    expect(screen.getByRole("button", { name: ru.conversations.composerSubmit })).toBeEnabled();
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
  });

  it("appends strictly newer records in order and deduplicates after an accepted send", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(
      Response.json({
        items: [
          initialHistory.messages[1],
          {
            id: "33333333-3333-4333-8333-333333333333",
            seq: 104,
            role: "user",
            text: "message 104",
            created_at: "2026-08-01T12:00:02Z",
          },
          {
            id: "33333333-3333-4333-8333-333333333333",
            seq: 104,
            role: "user",
            text: "message 104",
            created_at: "2026-08-01T12:00:02Z",
          },
          {
            id: "44444444-4444-4444-8444-444444444444",
            seq: 105,
            role: "assistant",
            text: "message 105",
            created_at: "2026-08-01T12:00:03Z",
          },
        ],
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), { target: { value: "Продолжить" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    await screen.findByText("message 105");
    expect(webBrowserFetch).toHaveBeenCalledWith(
      `/web/v1/conversations/${conversationId}/messages?after_seq=103&limit=100`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(screen.getAllByRole("listitem").map((item) => item.textContent)).toEqual([
      expect.stringContaining("message 102"),
      expect.stringContaining("message 103"),
      expect.stringContaining("message 104"),
      expect.stringContaining("message 105"),
    ]);
    expect(screen.getAllByText("message 104")).toHaveLength(1);
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });

  it("keeps polling after a neutral refresh failure and can still observe completion", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch)
      .mockRejectedValueOnce(new Error("private backend detail"))
      .mockResolvedValueOnce(
        Response.json({
          items: [
            {
              id: "55555555-5555-4555-8555-555555555555",
              seq: 104,
              role: "assistant",
              text: "message after retry",
              created_at: "2026-08-01T12:00:04Z",
            },
          ],
        }),
      );
    render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), { target: { value: "Продолжить" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(screen.getByText(ru.conversations.refreshDelayed)).toBeTruthy();
    expect(screen.queryByText("private backend detail")).toBeNull();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000);
    });

    expect(screen.getByText("message after retry")).toBeTruthy();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("never overlaps polling requests", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockImplementationOnce(
      (_path, init) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
        }),
    );
    const { unmount } = render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), { target: { value: "Продолжить" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("stops polling at fifteen attempts and never fetches after the thirty-second deadline", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), { target: { value: "Продолжить" } });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);

    for (let attempt = 1; attempt < 15; attempt += 1) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2_000);
      });
    }
    expect(webBrowserFetch).toHaveBeenCalledTimes(15);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(62_000);
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(15);
  });
});
