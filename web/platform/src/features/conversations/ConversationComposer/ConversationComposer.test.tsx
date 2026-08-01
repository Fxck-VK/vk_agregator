import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { ConversationComposer } from "./ConversationComposer";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const firstKey = "11111111-1111-4111-8111-111111111111";
const secondKey = "22222222-2222-4222-8222-222222222222";
const queuedJob = {
  job_id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
  status: "queued",
};

describe("ConversationComposer", () => {
  beforeEach(() => {
    vi.stubGlobal("crypto", {
      randomUUID: vi.fn().mockReturnValueOnce(firstKey).mockReturnValueOnce(secondKey),
    });
  });

  afterEach(() => {
    cleanup();
    vi.resetAllMocks();
    vi.unstubAllGlobals();
  });

  it("uses the exact NeiroHub question placeholder", () => {
    render(<ConversationComposer conversationId={conversationId} onAccepted={vi.fn()} />);

    expect(screen.getByLabelText(ru.conversations.composerLabel)).toHaveAttribute(
      "placeholder",
      "Задайте вопрос NeiroHub",
    );
  });

  it("submits a non-empty draft when Enter is pressed", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    const onAccepted = vi.fn();
    render(<ConversationComposer conversationId={conversationId} onAccepted={onAccepted} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Вопрос с клавиатуры" } });
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter" });
    fireEvent(textarea, event);

    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(1));
    await vi.waitFor(() => expect(onAccepted).toHaveBeenCalledTimes(1));
    expect(event.defaultPrevented).toBe(true);
  });

  it("leaves Shift+Enter to the textarea without submitting", () => {
    render(<ConversationComposer conversationId={conversationId} onAccepted={vi.fn()} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Первая строка" } });
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key: "Enter", shiftKey: true });
    fireEvent(textarea, event);

    expect(event.defaultPrevented).toBe(false);
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("shows an accessible three-dot waiting status only while awaiting a response", () => {
    const { rerender } = render(
      <ConversationComposer conversationId={conversationId} isAwaitingResponse onAccepted={vi.fn()} />,
    );

    const waitingStatus = screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse });
    expect(waitingStatus).toHaveAttribute("aria-live", "polite");
    expect(waitingStatus.querySelectorAll('[aria-hidden="true"]')).toHaveLength(3);

    rerender(<ConversationComposer conversationId={conversationId} isAwaitingResponse={false} onAccepted={vi.fn()} />);

    expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull();
  });

  it.each([
    { httpStatus: 201, jobStatus: "queued" },
    { httpStatus: 200, jobStatus: "provider_processing" },
    { httpStatus: 200, jobStatus: "succeeded" },
  ])("posts text to its conversation and accepts a safe $httpStatus $jobStatus response", async ({ httpStatus, jobStatus }) => {
    const onAccepted = vi.fn();
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(
      Response.json({ ...queuedJob, status: jobStatus }, { status: httpStatus }),
    );
    render(<ConversationComposer conversationId={conversationId} onAccepted={onAccepted} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), {
      target: { value: "  Продолжи диалог  " },
    });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    await vi.waitFor(() => expect(onAccepted).toHaveBeenCalledTimes(1));
    expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/conversations/${conversationId}/messages`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Idempotency-Key": firstKey,
      },
      body: JSON.stringify({ prompt: "Продолжи диалог" }),
    });
    expect(screen.getByLabelText(ru.conversations.composerLabel)).toHaveValue("");
    expect(screen.queryByText("Сообщение принято. Ответ появится в этом чате.")).toBeNull();
  });

  it("retains a failed draft and reuses its idempotency key for an exact normalized retry", async () => {
    vi.mocked(webBrowserMutation)
      .mockRejectedValueOnce(new Error("private backend detail"))
      .mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    const onAccepted = vi.fn();
    render(<ConversationComposer conversationId={conversationId} onAccepted={onAccepted} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "  Сохрани черновик  " } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.composerFailure);
    expect(screen.queryByText("private backend detail")).toBeNull();
    expect(textarea).toHaveValue("  Сохрани черновик  ");

    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    await vi.waitFor(() => expect(onAccepted).toHaveBeenCalledTimes(1));
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
    expect(vi.mocked(webBrowserMutation).mock.calls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      firstKey,
      firstKey,
    ]);
    expect(crypto.randomUUID).toHaveBeenCalledTimes(1);
  });

  it("invalidates a failed retry key when the normalized draft changes", async () => {
    vi.mocked(webBrowserMutation)
      .mockRejectedValueOnce(new Error("lost response"))
      .mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    render(<ConversationComposer conversationId={conversationId} onAccepted={vi.fn()} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Первый текст" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
    await screen.findByRole("alert");

    fireEvent.change(textarea, { target: { value: "Другой текст" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(2));
    expect(vi.mocked(webBrowserMutation).mock.calls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      firstKey,
      secondKey,
    ]);
  });

  it.each([
    ["failed status", new Response(null, { status: 503 })],
    ["malformed payload", Response.json({ job_id: "not-a-uuid", status: "queued" }, { status: 201 })],
    ["unsafe created status", Response.json({ ...queuedJob, status: "succeeded" }, { status: 201 })],
    ["unsafe replay status", Response.json({ ...queuedJob, status: "failed_terminal" }, { status: 200 })],
  ])("keeps the draft and reports a safe error for a %s", async (_caseName, response) => {
    const onAccepted = vi.fn();
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(response);
    render(<ConversationComposer conversationId={conversationId} onAccepted={onAccepted} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), { target: { value: "Важный текст" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.composerFailure);
    expect(screen.getByLabelText(ru.conversations.composerLabel)).toHaveValue("Важный текст");
    expect(onAccepted).not.toHaveBeenCalled();
  });
});
