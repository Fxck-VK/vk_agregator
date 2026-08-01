import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { webBrowserFetch } from "@/lib/web-api/browser";

import { ConversationHistory } from "./ConversationHistory";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";

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
    vi.clearAllMocks();
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

    fireEvent.click(screen.getByRole("button"));

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
});
