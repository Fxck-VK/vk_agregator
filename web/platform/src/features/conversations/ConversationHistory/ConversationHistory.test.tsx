import { StrictMode } from "react";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
  webBrowserMutation: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { savePendingConversationPrompt } from "@/features/conversations/pending-conversation-prompt";
import { savePendingConversationTitleSync } from "@/features/conversations/pending-conversation-title-sync";
import { WorkspaceConversationListProvider, useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

import { ConversationHistory } from "./ConversationHistory";

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const queuedJob = {
  job_id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
  status: "queued",
};

function WorkspaceConversationTitleProbe() {
  const { conversations } = useWorkspaceConversationList();

  return <output data-testid="workspace-conversation-titles">{conversations.map((conversation) => conversation.title).join(",")}</output>;
}

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
    document.querySelector("main[data-testid=\"workspace-scroll-region\"]")?.remove();
    window.sessionStorage.clear();
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders conversation messages without visible role labels", () => {
    render(<ConversationHistory history={initialHistory as never} />);

    expect(screen.queryByText("Диалог", { exact: true })).toBeNull();
    expect(screen.queryByRole("heading", { name: ru.conversations.historyTitle })).toBeNull();
    const messageItems = within(screen.getByRole("list")).getAllByRole("listitem");

    expect(within(messageItems[0]).queryByText(ru.conversations.userRole)).toBeNull();
    expect(within(messageItems[1]).queryByText(ru.conversations.assistantRole)).toBeNull();
    expect(within(messageItems[1]).getByText("message 103")).toBeVisible();
  });

  it("renders Markdown structure only for assistant messages", () => {
    const markdownHistory = {
      ...initialHistory,
      messages: [
        { ...initialHistory.messages[0], text: "**Текст пользователя**" },
        {
          ...initialHistory.messages[1],
          text: "## Ответ\n\n1. **Первый пункт**\n2. Второй пункт",
        },
      ],
    };

    render(<ConversationHistory history={markdownHistory as never} />);

    const [userMessage, assistantMessage] = Array.from(screen.getAllByRole("list")[0].children);
    expect(within(userMessage as HTMLElement).getByText("**Текст пользователя**")).toBeVisible();
    expect(within(userMessage as HTMLElement).queryByText("Текст пользователя")).toBeNull();
    expect(within(assistantMessage as HTMLElement).getByRole("heading", { level: 2, name: "Ответ" })).toBeVisible();
    expect(within(assistantMessage as HTMLElement).getAllByRole("listitem")).toHaveLength(2);
  });

  it("copies the exact text for user and assistant messages while keeping role-specific actions", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<ConversationHistory history={initialHistory as never} />);

    const messageItems = within(screen.getByRole("list")).getAllByRole("listitem");
    const userMessage = within(messageItems[0]);
    const assistantMessage = within(messageItems[1]);

    fireEvent.click(userMessage.getByRole("button", { name: "Копировать сообщение" }));

    await vi.waitFor(() => expect(writeText).toHaveBeenCalledWith("message 102"));
    fireEvent.click(assistantMessage.getByRole("button", { name: "Копировать сообщение" }));
    await vi.waitFor(() => expect(writeText).toHaveBeenLastCalledWith("message 103"));

    expect(userMessage.queryByRole("button", { name: "Лайк" })).toBeNull();
    expect(userMessage.queryByRole("button", { name: "Дизлайк" })).toBeNull();
    expect(assistantMessage.queryByRole("button", { name: "Пересоздать сообщение" })).toBeNull();
  });

  it("shows NeiroHub copy feedback and restores the copy action after two seconds", async () => {
    vi.useFakeTimers();
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<ConversationHistory history={initialHistory as never} />);

    const userMessage = within(within(screen.getByRole("list")).getAllByRole("listitem")[0]);
    const copyButton = userMessage.getByRole("button", { name: ru.conversations.copyMessage });

    expect(copyButton).not.toHaveAttribute("title");
    expect(copyButton).toHaveAttribute("data-tooltip", ru.conversations.copyMessage);
    expect(copyButton.querySelector('[data-icon="copy"]')).not.toBeNull();

    await act(async () => {
      fireEvent.click(copyButton);
      await Promise.resolve();
    });

    expect(writeText).toHaveBeenCalledWith("message 102");
    expect(copyButton).toHaveAccessibleName("Скопировано");
    expect(copyButton).toHaveAttribute("data-tooltip", "Скопировано");
    expect(copyButton.querySelector('[data-icon="check"]')).not.toBeNull();

    act(() => {
      vi.advanceTimersByTime(2_000);
    });

    expect(copyButton).toHaveAccessibleName(ru.conversations.copyMessage);
    expect(copyButton).toHaveAttribute("data-tooltip", ru.conversations.copyMessage);
    expect(copyButton.querySelector('[data-icon="copy"]')).not.toBeNull();
  });

  it("uses the NeiroHub tooltip contract for every message action", () => {
    render(<ConversationHistory history={initialHistory as never} />);

    const messageItems = within(screen.getByRole("list")).getAllByRole("listitem");
    const userMessage = within(messageItems[0]);
    const assistantMessage = within(messageItems[1]);
    const actions = [
      userMessage.getByRole("button", { name: ru.conversations.copyMessage }),
      userMessage.getByRole("button", { name: ru.conversations.recreateMessage }),
      assistantMessage.getByRole("button", { name: ru.conversations.likeMessage }),
      assistantMessage.getByRole("button", { name: ru.conversations.dislikeMessage }),
    ];
    const labels = [
      ru.conversations.copyMessage,
      ru.conversations.recreateMessage,
      ru.conversations.likeMessage,
      ru.conversations.dislikeMessage,
    ];

    actions.forEach((button, index) => {
      expect(button).toHaveAttribute("data-tooltip", labels[index]);
      expect(button).not.toHaveAttribute("title");
    });
  });

  it("keeps like and dislike mutually exclusive and clears a repeated rating", () => {
    render(<ConversationHistory history={initialHistory as never} />);

    const assistantMessage = within(within(screen.getByRole("list")).getAllByRole("listitem")[1]);
    const likeButton = assistantMessage.getByRole("button", { name: "Лайк" });
    const dislikeButton = assistantMessage.getByRole("button", { name: "Дизлайк" });

    expect(likeButton).toHaveAttribute("aria-pressed", "false");
    expect(dislikeButton).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(likeButton);
    expect(likeButton).toHaveAttribute("aria-pressed", "true");
    expect(dislikeButton).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(dislikeButton);
    expect(likeButton).toHaveAttribute("aria-pressed", "false");
    expect(dislikeButton).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(dislikeButton);
    expect(likeButton).toHaveAttribute("aria-pressed", "false");
    expect(dislikeButton).toHaveAttribute("aria-pressed", "false");
  });

  it("moves a user message into the draft without focusing the composer or sending it", () => {
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Черновик, который будет заменён" } });
    const userMessage = within(screen.getByRole("list")).getAllByRole("listitem")[0];
    const recreateButton = within(userMessage).getByRole("button", {
      name: "Пересоздать сообщение",
    });
    recreateButton.focus();
    fireEvent.click(recreateButton);

    expect(screen.getByLabelText(ru.conversations.composerLabel)).toHaveValue("message 102");
    expect(document.activeElement).toBe(recreateButton);
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("smoothly scrolls the workspace after an accepted Enter send", async () => {
    const { region, scrollTo } = addWorkspaceScrollRegion();
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise<Response>(() => {}));
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Pending stream prompt" } });
    fireEvent.keyDown(textarea, { key: "Enter" });

    await screen.findByText("Pending stream prompt");
    await vi.waitFor(() =>
      expect(scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: region.scrollHeight }),
    );
  });

  it("keeps the workspace position when a polling reply arrives above the bottom", async () => {
    const { region, scrollTo, setScrollHeight } = addWorkspaceScrollRegion();
    let resolveRefresh: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRefresh = resolve;
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Pending stream prompt" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
    await screen.findByText("Pending stream prompt");
    await vi.waitFor(() =>
      expect(scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: region.scrollHeight }),
    );
    scrollTo.mockClear();

    region.scrollTop = 100;
    fireEvent.scroll(region);
    setScrollHeight(1_800);
    resolveRefresh(
      Response.json({
        items: [
          {
            id: "77777777-7777-4777-8777-777777777777",
            seq: 104,
            role: "assistant",
            text: "assistant reply while scrolled away",
            created_at: "2026-08-01T12:00:04Z",
          },
        ],
      }),
    );

    await screen.findByText("assistant reply while scrolled away");
    expect(scrollTo).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
  });

  it("follows a polling reply while the workspace is already at the bottom", async () => {
    const { region, scrollTo, setScrollHeight } = addWorkspaceScrollRegion();
    let resolveRefresh: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRefresh = resolve;
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), { target: { value: "Pending stream prompt" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));
    await screen.findByText("Pending stream prompt");
    await vi.waitFor(() =>
      expect(scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: region.scrollHeight }),
    );
    scrollTo.mockClear();

    setScrollHeight(1_800);
    resolveRefresh(
      Response.json({
        items: [
          {
            id: "77777777-7777-4777-8777-777777777777",
            seq: 104,
            role: "assistant",
            text: "assistant reply while following latest",
            created_at: "2026-08-01T12:00:04Z",
          },
        ],
      }),
    );

    await screen.findByText("assistant reply while following latest");
    await vi.waitFor(() =>
      expect(scrollTo).toHaveBeenCalledWith({ behavior: "smooth", top: region.scrollHeight }),
    );
    region.scrollTop = 1_400;
    fireEvent.scroll(region);
    expect(screen.queryByRole("button", { name: ru.conversations.scrollToLatest })).toBeNull();
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

  it("starts a bounded initial refresh from the current last sequence and appends a delayed first reply", async () => {
    vi.useFakeTimers();
    const firstPromptHistory = {
      kind: "ready" as const,
      conversationId,
      hasMoreBefore: false,
      messages: [initialHistory.messages[0]],
    };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [] }))
      .mockResolvedValueOnce(
        Response.json({
          items: [
            {
              id: "77777777-7777-4777-8777-777777777777",
              seq: 103,
              role: "assistant",
              text: "delayed first reply",
              created_at: "2026-08-01T12:00:02Z",
            },
          ],
        }),
      );

    render(<ConversationHistory history={firstPromptHistory as never} initialRefresh />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(webBrowserFetch).toHaveBeenNthCalledWith(
      1,
      `/web/v1/conversations/${conversationId}/messages?after_seq=102&limit=100`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000);
    });

    expect(screen.getByText("delayed first reply")).toBeTruthy();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("starts an initial refresh from zero when server-rendered history is empty", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({ items: [] }));

    render(
      <ConversationHistory
        history={{ kind: "ready", conversationId, hasMoreBefore: false, messages: [] } as never}
        initialRefresh
      />,
    );

    await vi.waitFor(() =>
      expect(webBrowserFetch).toHaveBeenCalledWith(
        `/web/v1/conversations/${conversationId}/messages?after_seq=0&limit=100`,
        expect.objectContaining({ signal: expect.any(AbortSignal) }),
      ),
    );
  });

  it("does not start an initial refresh when the server-rendered history already has an assistant reply", () => {
    render(<ConversationHistory history={initialHistory as never} initialRefresh />);

    expect(webBrowserFetch).not.toHaveBeenCalled();
    expect(screen.getByLabelText(ru.conversations.composerLabel)).not.toBeDisabled();
  });

  it("places a pending assistant indicator below the optimistic user message and replaces it with server history", async () => {
    let resolveRefresh: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRefresh = resolve;
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Pending stream prompt" } });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    const messageList = screen.getByRole("list");
    await within(messageList).findByText("Pending stream prompt");
    const messageItems = within(messageList).getAllByRole("listitem");
    expect(messageItems.at(-2)).toHaveTextContent("Pending stream prompt");
    expect(messageItems.at(-1)).toHaveAttribute("data-chat-pending", "assistant");
    expect(
      within(messageItems.at(-1) as HTMLElement).getByRole("status", { name: ru.conversations.composerAwaitingResponse }),
    ).toBeTruthy();

    resolveRefresh(
      Response.json({
        items: [
          {
            id: "66666666-6666-4666-8666-666666666666",
            seq: 104,
            role: "user",
            text: "Pending stream prompt",
            created_at: "2026-08-01T12:00:04Z",
          },
          {
            id: "77777777-7777-4777-8777-777777777777",
            seq: 105,
            role: "assistant",
            text: "assistant completion",
            created_at: "2026-08-01T12:00:05Z",
          },
        ],
      }),
    );

    await screen.findByText("assistant completion");
    expect(screen.getAllByText("Pending stream prompt")).toHaveLength(1);
    expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull();
  });

  it("shows typing dots in the circular dock control until the assistant reply completes", async () => {
    const workspaceScroll = addWorkspaceScrollRegion();
    let resolveRefresh: (response: Response) => void = () => {};
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        resolveRefresh = resolve;
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Покажи индикатор" } });
    fireEvent.keyDown(textarea, { key: "Enter" });

    await vi.waitFor(() => {
      expect(screen.getAllByRole("status", { name: ru.conversations.composerAwaitingResponse })).toHaveLength(2);
    });
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();

    resolveRefresh(
      Response.json({
        items: [
          {
            id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
            seq: 104,
            role: "assistant",
            text: "Готовый ответ",
            created_at: "2026-08-01T12:00:05Z",
          },
        ],
      }),
    );

    await screen.findByText("Готовый ответ");
    workspaceScroll.region.scrollTop = 100;
    fireEvent.scroll(workspaceScroll.region);

    expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull();
    expect(screen.getByRole("button", { name: ru.conversations.scrollToLatest })).toBeVisible();
  });

  it("renders a sent turn immediately and retries an in-place failure with the same idempotency key", async () => {
    const idempotencyKey = "55555555-5555-4555-8555-555555555555";
    let rejectMutation: (reason?: unknown) => void = () => {};
    vi.stubGlobal("crypto", { randomUUID: vi.fn().mockReturnValue(idempotencyKey) });
    vi.mocked(webBrowserMutation)
      .mockReturnValueOnce(new Promise<Response>((_resolve, reject) => { rejectMutation = reject; }))
      .mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise<Response>(() => {}));
    render(<ConversationHistory history={initialHistory as never} />);

    const textarea = screen.getByLabelText(ru.conversations.composerLabel);
    fireEvent.change(textarea, { target: { value: "Оптимистичный вопрос" } });
    fireEvent.keyDown(textarea, { key: "Enter" });

    expect(textarea).toHaveValue("");
    expect(screen.getByText("Оптимистичный вопрос")).toBeVisible();
    expect(screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeVisible();

    rejectMutation(new Error("network detail"));

    expect(await screen.findByText("Не отправлено")).toBeVisible();
    expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));

    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeVisible();
    expect(vi.mocked(webBrowserMutation).mock.calls.map(([, init]) => new Headers(init.headers).get("X-Idempotency-Key"))).toEqual([
      idempotencyKey,
      idempotencyKey,
    ]);
    expect(crypto.randomUUID).toHaveBeenCalledTimes(1);
  });

  it("shows the first pending prompt in the stream after the workspace opens its new chat", async () => {
    vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise<Response>(() => {}));
    savePendingConversationPrompt(conversationId, "First workspace prompt");

    render(
      <ConversationHistory
        history={{ kind: "ready", conversationId, hasMoreBefore: false, messages: [] } as never}
        initialRefresh
      />,
    );

    const messageList = await screen.findByRole("list");
    expect(await within(messageList).findByText("First workspace prompt")).toBeTruthy();
    expect(within(messageList).getAllByRole("listitem").at(-1)).toHaveAttribute("data-chat-pending", "assistant");
  });

  it("restores the pending fallback title into the workspace list after a page reload", () => {
    savePendingConversationTitleSync(conversationId, "First workspace prompt");

    render(
      <WorkspaceConversationListProvider
        accountId="0ce06a6a-16d8-4b16-b9df-5e63175a4a0c"
        initialConversations={[
          {
            id: conversationId,
            title: "",
            created_at: "2026-08-01T12:00:00Z",
            updated_at: "2026-08-01T12:00:00Z",
          },
        ]}
      >
        <ConversationHistory history={{ kind: "ready", conversationId, hasMoreBefore: false, messages: [] } as never} />
        <WorkspaceConversationTitleProbe />
      </WorkspaceConversationListProvider>,
    );

    expect(screen.getByTestId("workspace-conversation-titles")).toHaveTextContent("First workspace prompt");
  });

  it("does not duplicate the first prompt when the server already rendered it", async () => {
    vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise<Response>(() => {}));
    savePendingConversationPrompt(conversationId, "Already persisted prompt");

    render(
      <ConversationHistory
        history={{
          kind: "ready",
          conversationId,
          hasMoreBefore: false,
          messages: [
            {
              id: "88888888-8888-4888-8888-888888888888",
              seq: 1,
              role: "user",
              text: "Already persisted prompt",
              created_at: "2026-08-01T12:00:00Z",
            },
          ],
        } as never}
        initialRefresh
      />,
    );

    await act(async () => {
      await Promise.resolve();
    });

    const messageList = screen.getByRole("list");
    expect(within(messageList).getAllByText("Already persisted prompt")).toHaveLength(1);
    expect(within(messageList).getAllByRole("listitem").at(-1)).toHaveAttribute("data-chat-pending", "assistant");
  });

  it("keeps the first pending prompt through Strict Mode effect replay", async () => {
    vi.mocked(webBrowserFetch).mockReturnValue(new Promise<Response>(() => {}));
    savePendingConversationPrompt(conversationId, "Strict Mode prompt");

    render(
      <StrictMode>
        <ConversationHistory
          history={{ kind: "ready", conversationId, hasMoreBefore: false, messages: [] } as never}
          initialRefresh
        />
      </StrictMode>,
    );

    const messageList = await screen.findByRole("list");
    expect(await within(messageList).findByText("Strict Mode prompt")).toBeTruthy();
  });

  it("places the optimistic prompt after server messages that arrive while it is pending", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json(queuedJob, { status: 201 }));
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(
      Response.json({
        items: [
          {
            id: "99999999-9999-4999-8999-999999999999",
            seq: 104,
            role: "user",
            text: "Earlier server message",
            created_at: "2026-08-01T12:00:04Z",
          },
        ],
      }),
    );
    render(<ConversationHistory history={initialHistory as never} />);

    fireEvent.change(screen.getByLabelText(ru.conversations.composerLabel), {
      target: { value: "My pending prompt" },
    });
    fireEvent.click(screen.getByRole("button", { name: ru.conversations.composerSubmit }));

    await screen.findByText("Earlier server message");
    const messageItems = within(screen.getByRole("list")).getAllByRole("listitem");
    const itemTexts = messageItems.map((item) => item.textContent);
    expect(itemTexts.findIndex((text) => text?.includes("Earlier server message") ?? false)).toBeLessThan(
      itemTexts.findIndex((text) => text?.includes("My pending prompt") ?? false),
    );
    expect(messageItems.at(-1)).toHaveAttribute("data-chat-pending", "assistant");
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
    expect(screen.getByRole("status", { name: ru.conversations.composerAwaitingResponse })).toHaveAttribute(
      "aria-live",
      "polite",
    );
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
    await vi.waitFor(() => expect(screen.queryByRole("status", { name: ru.conversations.composerAwaitingResponse })).toBeNull());
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
            text: "Продолжить",
            created_at: "2026-08-01T12:00:02Z",
          },
          {
            id: "33333333-3333-4333-8333-333333333333",
            seq: 104,
            role: "user",
            text: "Продолжить",
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
      expect.stringContaining("Продолжить"),
      expect.stringContaining("message 105"),
    ]);
    expect(screen.getAllByText("Продолжить")).toHaveLength(1);
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

function addWorkspaceScrollRegion() {
  const region = document.createElement("main");
  const scrollTo = vi.fn();
  region.dataset.testid = "workspace-scroll-region";
  Object.defineProperties(region, {
    clientHeight: { configurable: true, value: 400 },
    scrollHeight: { configurable: true, writable: true, value: 1_600 },
    scrollTo: { configurable: true, value: scrollTo },
    scrollTop: { configurable: true, writable: true, value: 1_200 },
  });
  document.body.append(region);

  return {
    region,
    scrollTo,
    setScrollHeight: (scrollHeight: number) => {
      Object.defineProperty(region, "scrollHeight", { configurable: true, value: scrollHeight });
    },
  };
}
