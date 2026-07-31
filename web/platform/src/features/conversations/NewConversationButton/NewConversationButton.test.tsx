import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { NewConversationButton } from "./NewConversationButton";

const push = vi.fn();
const conversation = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  title: "",
  created_at: "2026-07-31T09:00:00Z",
  updated_at: "2026-07-31T09:05:00Z",
};

describe("NewConversationButton", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ push } as never);
    vi.stubGlobal("crypto", {
      randomUUID: vi.fn().mockReturnValue("a2a006fc-4457-4bb5-bc4d-4f553d51766b"),
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it.each([200, 201])("uses a fresh UUID and navigates only after a parsed %i response", async (status) => {
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(conversation, { status }));
    render(<NewConversationButton />);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.createLabel }));

    await vi.waitFor(() =>
      expect(push).toHaveBeenCalledWith("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906"),
    );
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/conversations", {
      method: "POST",
      headers: { "X-Idempotency-Key": "a2a006fc-4457-4bb5-bc4d-4f553d51766b" },
    });
  });

  it("disables while pending so a second click cannot create a duplicate", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(
      new Promise<Response>((resolve) => {
        settleRequest = resolve;
      }),
    );
    render(<NewConversationButton />);

    const button = screen.getByRole("button", { name: ru.conversations.createLabel });
    fireEvent.click(button);
    fireEvent.click(button);

    expect(button).toBeDisabled();
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);

    settleRequest(new Response(null, { status: 500 }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.createFailure);
  });

  it.each([
    ["a missing CSRF failure", () => Promise.reject(new Error("Unable to complete the request."))],
    ["a rejected request", () => Promise.reject(new Error("untrusted backend detail"))],
    ["a non-success response", () => Promise.resolve(new Response(null, { status: 500 }))],
    ["invalid JSON", () => Promise.resolve(new Response("not JSON", { status: 201 }))],
  ])("shows neutral feedback without routing after %s", async (_caseName, request) => {
    vi.mocked(webBrowserMutation).mockImplementationOnce(request);
    render(<NewConversationButton />);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.createLabel }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.createFailure);
    expect(push).not.toHaveBeenCalled();
    expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });

  it("uses another UUID after a failed attempt", async () => {
    vi.mocked(globalThis.crypto.randomUUID)
      .mockReturnValueOnce("a2a006fc-4457-4bb5-bc4d-4f553d51766b")
      .mockReturnValueOnce("6fc25ee1-1f6f-47ac-b46f-f4b21c0ad5cd");
    vi.mocked(webBrowserMutation)
      .mockRejectedValueOnce(new Error("Unable to complete the request."))
      .mockResolvedValueOnce(new Response(null, { status: 500 }));
    render(<NewConversationButton />);

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.createLabel }));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.createLabel }));

    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(2));
    expect(webBrowserMutation).toHaveBeenNthCalledWith(1, "/web/v1/conversations", {
      method: "POST",
      headers: { "X-Idempotency-Key": "a2a006fc-4457-4bb5-bc4d-4f553d51766b" },
    });
    expect(webBrowserMutation).toHaveBeenNthCalledWith(2, "/web/v1/conversations", {
      method: "POST",
      headers: { "X-Idempotency-Key": "6fc25ee1-1f6f-47ac-b46f-f4b21c0ad5cd" },
    });
  });
});
