import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ usePathname: vi.fn(), useRouter: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn() }));

import { usePathname, useRouter } from "next/navigation";

import { savePendingConversationBootstrap } from "@/features/conversations/pending-conversation-bootstrap";
import { SidebarConversations } from "@/features/conversations/SidebarConversations/SidebarConversations";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { PendingConversationBootstrap } from "./PendingConversationBootstrap";

const replace = vi.fn();
const conversationIdempotencyId = "c7c979f5-24e5-4f88-924b-a592d6e5a906";
const messageIdempotencyId = "e7c979f5-24e5-4f88-924b-a592d6e5a906";
const serverConversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const accountId = "0ce06a6a-16d8-4b16-b9df-5e63175a4a0c";
const conversation = {
  id: serverConversationId,
  title: "",
  created_at: "2026-08-01T09:00:00Z",
  updated_at: "2026-08-01T09:00:00Z",
};
const job = { job_id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b", status: "queued" };

function seedIntent() {
  savePendingConversationBootstrap({
    conversationKey: conversationIdempotencyId,
    messageKey: messageIdempotencyId,
    prompt: "Первый вопрос",
  });
}

function renderPending() {
  return render(
    <WorkspaceConversationListProvider accountId={accountId} initialConversations={[]}>
      <PendingConversationBootstrap conversationKey={conversationIdempotencyId} />
      <SidebarConversations />
    </WorkspaceConversationListProvider>,
  );
}

describe("PendingConversationBootstrap", () => {
  beforeEach(() => {
    vi.mocked(usePathname).mockReturnValue(`/app/chat/${conversationIdempotencyId}`);
    vi.mocked(useRouter).mockReturnValue({ replace } as never);
    seedIntent();
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    window.sessionStorage.clear();
  });

  it("sends image settings in the new conversation and stores its public job for refresh recovery", async () => {
    savePendingConversationBootstrap({conversationKey:conversationIdempotencyId,messageKey:messageIdempotencyId,prompt:"Кадр",modelId:"nano_banana_2",generationOptions:{image_quality:"2K",output_count:2,aspect_ratio:"9:16"}});
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(conversation,{status:201})).mockResolvedValueOnce(Response.json(job,{status:201}));
    renderPending();
    await vi.waitFor(()=>expect(replace).toHaveBeenCalledWith(`/app/chat/${serverConversationId}?refresh=1`));
    const [path,init] = vi.mocked(webBrowserMutation).mock.calls[1];
    expect(path).toBe(`/web/v1/conversations/${serverConversationId}/messages`);
    expect(JSON.parse(init.body as string)).toEqual({prompt:"Кадр",model_id:"nano_banana_2",image_quality:"2K",output_count:2,aspect_ratio:"9:16"});
    expect(JSON.parse(window.sessionStorage.getItem(`neirohub:conversation-pending-media:${serverConversationId}`)!)).toMatchObject({jobID:job.job_id,baselineSeq:0});
  });

  it("shows the first message and typing indicator before create resolves", async () => {
    vi.mocked(webBrowserMutation).mockReturnValueOnce(new Promise<Response>(() => {}));
    renderPending();

    expect(screen.queryByText("Диалог", { exact: true })).toBeNull();
    expect(screen.queryByRole("heading", { name: ru.conversations.historyTitle })).toBeNull();
    expect(screen.getAllByText("Первый вопрос")[0]).toBeVisible();
    expect(screen.queryByText(ru.conversations.userRole)).toBeNull();
    expect(screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeVisible();
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(1));
    expect(replace).not.toHaveBeenCalled();
  });

  it("creates the conversation and first message in the background, then replaces the route", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockResolvedValueOnce(Response.json(job, { status: 201 }));
    renderPending();

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith(`/app/chat/${serverConversationId}?refresh=1`));
    expect(webBrowserMutation).toHaveBeenNthCalledWith(1, "/web/v1/conversations", {
      method: "POST",
      headers: { "X-Idempotency-Key": conversationIdempotencyId },
    });
    expect(webBrowserMutation).toHaveBeenNthCalledWith(2, `/web/v1/conversations/${serverConversationId}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Idempotency-Key": messageIdempotencyId },
      body: JSON.stringify({ prompt: "Первый вопрос" }),
    });
  });

  it("keeps the chosen model and idempotency key on first-message retries and subsequent turns", async () => {
    savePendingConversationBootstrap({ conversationKey: conversationIdempotencyId, messageKey: messageIdempotencyId, prompt: "Первый вопрос", modelId: "gpt_5_5" });
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockRejectedValueOnce(new Error("timeout"))
      .mockResolvedValueOnce(Response.json(job, { status: 201 }));
    renderPending();
    await screen.findByRole("alert");
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.messageRetryLabel }));
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith(`/app/chat/${serverConversationId}?refresh=1`));
    const calls = vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path.endsWith("/messages"));
    expect(calls).toHaveLength(2);
    for (const [, init] of calls) {
      expect(JSON.parse(init.body as string)).toEqual({ prompt: "Первый вопрос", model_id: "gpt_5_5" });
      expect(new Headers(init.headers).get("X-Idempotency-Key")).toBe(messageIdempotencyId);
    }
    expect(window.sessionStorage.getItem(`neirohub:conversation-model:${serverConversationId}`)).toBe("gpt_5_5");
  });

  it("shows a local failure and retries with the same keys", async () => {
    vi.mocked(webBrowserMutation)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(Response.json(conversation, { status: 200 }))
      .mockResolvedValueOnce(Response.json(job, { status: 201 }));
    renderPending();

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.messageNotSent);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.messageRetryLabel }));

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith(`/app/chat/${serverConversationId}?refresh=1`));
    const createCalls = vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path === "/web/v1/conversations");
    expect(createCalls).toHaveLength(2);
    expect(createCalls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      conversationIdempotencyId,
      conversationIdempotencyId,
    ]);
  });

  it("does not create a second conversation when retrying a failed first message", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json(conversation, { status: 201 }))
      .mockRejectedValueOnce(new Error("message timeout"))
      .mockResolvedValueOnce(Response.json(job, { status: 201 }));
    renderPending();

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.messageNotSent);
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.messageRetryLabel }));

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith(`/app/chat/${serverConversationId}?refresh=1`));
    expect(vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path === "/web/v1/conversations")).toHaveLength(1);
    const messageCalls = vi.mocked(webBrowserMutation).mock.calls.filter(([path]) => path.includes("/messages"));
    expect(messageCalls).toHaveLength(2);
    expect(messageCalls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      messageIdempotencyId,
      messageIdempotencyId,
    ]);
  });
});
