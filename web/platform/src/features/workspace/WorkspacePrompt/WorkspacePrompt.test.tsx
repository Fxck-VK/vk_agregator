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
import { readPendingConversationPrompt } from "@/features/conversations/pending-conversation-prompt";
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
    window.sessionStorage.clear();
  });

  it.each([
    { messageStatus: 201, jobStatus: "queued" },
    { messageStatus: 200, jobStatus: "provider_processing" },
    { messageStatus: 200, jobStatus: "succeeded" },
  ])("creates a normal chat and accepts a safe $messageStatus $jobStatus message response", async ({ messageStatus, jobStatus }) => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ ...queuedChatJob, status: jobStatus }, { status: messageStatus }));
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), {
      target: { value: "Помогите подготовить план" },
    });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    await vi.waitFor(() =>
      expect(push).toHaveBeenCalledWith("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906?refresh=1"),
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

  it("uses the existing create-then-message request sequence when Enter is pressed", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json(queuedChatJob, { status: 201 }));
    render(<WorkspacePrompt />);

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "Enter submission" } });
    fireEvent.keyDown(textarea, { key: "Enter" });

    await vi.waitFor(() => expect(push).toHaveBeenCalledWith("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906?refresh=1"));
    expect(readPendingConversationPrompt("d7c979f5-24e5-4f88-924b-a592d6e5a906")).toBe("Enter submission");
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
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
        body: JSON.stringify({ prompt: "Enter submission" }),
      },
    );
  });

  it.each([
    { messageStatus: 200, jobStatus: "queued" },
    { messageStatus: 201, jobStatus: "succeeded" },
    { messageStatus: 200, jobStatus: "failed_terminal" },
  ])("keeps the draft for an unsafe $messageStatus $jobStatus response", async ({ messageStatus, jobStatus }) => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ ...queuedChatJob, status: jobStatus }, { status: messageStatus }));
    render(<WorkspacePrompt />);

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "important draft" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.workspace.promptFailure);
    expect(textarea).toHaveValue("important draft");
    expect(push).not.toHaveBeenCalled();
  });

  it("reuses the conversation request key after a lost create response", async () => {
    vi.mocked(webBrowserMutation)
      .mockRejectedValueOnce(new Error("lost create response"))
      .mockResolvedValueOnce(Response.json(conversation, { status: 200 }))
      .mockResolvedValueOnce(Response.json(queuedChatJob, { status: 201 }));
    render(<WorkspacePrompt />);

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "same draft" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    await vi.waitFor(() => expect(push).toHaveBeenCalledTimes(1));
    const createCalls = vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path === "/web/v1/conversations");
    expect(createCalls).toHaveLength(2);
    expect(createCalls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      "c7c979f5-24e5-4f88-924b-a592d6e5a906",
      "c7c979f5-24e5-4f88-924b-a592d6e5a906",
    ]);
    expect(crypto.randomUUID).toHaveBeenCalledTimes(2);
  });

  it.each([402, 429])("does not create a second conversation when a known conversation returns %i", async (status) => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(new Response(null, { status }))
      .mockResolvedValueOnce(Response.json(queuedChatJob, { status: 201 }));
    render(<WorkspacePrompt />);

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "same draft" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    await vi.waitFor(() => expect(push).toHaveBeenCalledTimes(1));
    expect(vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path === "/web/v1/conversations")).toHaveLength(1);
    const messageCalls = vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path.includes("/messages"));
    expect(messageCalls).toHaveLength(2);
    expect(messageCalls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      "e7c979f5-24e5-4f88-924b-a592d6e5a906",
      "e7c979f5-24e5-4f88-924b-a592d6e5a906",
    ]);
    expect(crypto.randomUUID).toHaveBeenCalledTimes(2);
  });

  it("does not create a second conversation and reuses the message key after a lost message response", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockRejectedValueOnce(new Error("lost message response"))
      .mockResolvedValueOnce(Response.json({ ...queuedChatJob, status: "provider_processing" }, { status: 200 }));
    render(<WorkspacePrompt />);

    const textarea = screen.getByLabelText(ru.workspace.promptLabel);
    fireEvent.change(textarea, { target: { value: "same draft" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    await vi.waitFor(() => expect(push).toHaveBeenCalledTimes(1));
    expect(vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path === "/web/v1/conversations")).toHaveLength(1);
    const messageCalls = vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path.includes("/messages"));
    expect(messageCalls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      "e7c979f5-24e5-4f88-924b-a592d6e5a906",
      "e7c979f5-24e5-4f88-924b-a592d6e5a906",
    ]);
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

  it("does not route when the conversation payload is malformed", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ id: "not-a-uuid" }, { status: 201 }));
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Проверочный запрос" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.workspace.promptFailure);
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
    expect(push).not.toHaveBeenCalled();
  });

  it("does not route when the chat-job payload is malformed", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ job_id: "not-a-uuid", status: "queued" }, { status: 201 }));
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "Проверочный запрос" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.workspace.promptFailure);
    expect(push).not.toHaveBeenCalled();
  });

  it("keeps the draft and shows safe failure when the message request fails", async () => {
    const draft = "Текст нужно сохранить";
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockRejectedValueOnce(new Error("Unable to complete the request."));
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: draft } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.workspace.promptFailure);
    expect(screen.getByLabelText(ru.workspace.promptLabel)).toHaveValue(draft);
    expect(push).not.toHaveBeenCalled();
  });

  it("posts a trimmed prompt after successful whitespace-padded input", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json(queuedChatJob, { status: 201 }));
    render(<WorkspacePrompt />);

    fireEvent.change(screen.getByLabelText(ru.workspace.promptLabel), { target: { value: "  Проверочный запрос  " } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    await vi.waitFor(() => expect(push).toHaveBeenCalled());
    expect(webBrowserMutation).toHaveBeenNthCalledWith(
      2,
      "/web/v1/conversations/d7c979f5-24e5-4f88-924b-a592d6e5a906/messages",
      expect.objectContaining({ body: JSON.stringify({ prompt: "Проверочный запрос" }) }),
    );
  });
});
