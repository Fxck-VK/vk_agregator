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

import { WorkspacePrompt } from "./WorkspacePrompt";

const push = vi.fn();
const conversation = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  title: "",
  created_at: "2026-08-01T09:00:00Z",
  updated_at: "2026-08-01T09:00:00Z",
};
const queuedChatJob = {
  job_id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
  status: "queued",
};

describe("WorkspacePrompt", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ push } as never);
    vi.stubGlobal("crypto", {
      randomUUID: vi.fn()
        .mockReturnValueOnce("c7c979f5-24e5-4f88-924b-a592d6e5a906")
        .mockReturnValueOnce("e7c979f5-24e5-4f88-924b-a592d6e5a906"),
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it.each([201, 200])("creates a normal chat and accepts a safe %i message response", async (messageStatus) => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json(queuedChatJob, { status: messageStatus }));
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), {
      target: { value: "Помогите подготовить план" },
    });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    await vi.waitFor(() =>
      expect(push).toHaveBeenCalledWith("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906"),
    );
    expect(webBrowserMutation).toHaveBeenNthCalledWith(1, "/web/v1/conversations", {
      method: "POST",
      headers: { "X-Idempotency-Key": "c7c979f5-24e5-4f88-924b-a592d6e5a906" },
    });
    expect(webBrowserMutation).toHaveBeenNthCalledWith(
      2,
      "/web/v1/conversations/d7c979f5-24e5-4f88-924b-a592d6e5a906/messages",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": "e7c979f5-24e5-4f88-924b-a592d6e5a906",
        },
        body: JSON.stringify({ prompt: "Помогите подготовить план" }),
      },
    );
  });

  it("does not submit an empty prompt and keeps text after a recoverable failure", async () => {
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    expect(webBrowserMutation).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Сохраните этот текст" } });
    vi.mocked(webBrowserMutation).mockRejectedValueOnce(new Error("Unable to complete the request."));
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.workspace.promptFailure);
    expect(screen.getByLabelText(ru.workspace.promptLabel)).toHaveValue("Сохраните этот текст");
    expect(push).not.toHaveBeenCalled();
  });

  it("blocks a concurrent submission while creating the conversation", async () => {
    let settleRequest: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockReturnValue(
      new Promise<Response>((resolve) => {
        settleRequest = resolve;
      }),
    );
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Один запрос" } });
    const button = screen.getByRole("button", { name: ru.workspace.promptSubmit });
    fireEvent.click(button);
    fireEvent.click(button);

    expect(button).toBeDisabled();
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);

    settleRequest(new Response(null, { status: 500 }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.workspace.promptFailure);
  });
});
