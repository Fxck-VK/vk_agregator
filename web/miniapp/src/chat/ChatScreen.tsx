// src/chat/ChatScreen.tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Panel } from "@vkontakte/vkui";
import { Spinner } from "../ui/ui";
import { MessageBubble } from "./MessageBubble";
import { Composer } from "./Composer";
import { ChatList } from "./ChatList";
import { WorkflowMode } from "../workflow/WorkflowMode";
import { SettingsScreen } from "../settings/SettingsScreen";
import { loadThemeMode, watchThemeMode, type ThemeMode } from "../settings/theme";
import { modalityByOperation, uid, type Chat, type ChatMessage } from "./types";
import { loadAppTab, saveAppTab, type AppTab } from "../mode";
import {
  createChatMessage,
  createJob,
  createIdempotencyKey,
  getJob,
  listJobs,
  getBalance,
  isTerminal,
  listChatConversationMessages,
  listChatConversations,
  statusKind,
  errorLabel,
  apiUserMessage,
  resolveBotText,
  type ChatConversation,
  type ChatConversationMessage,
  type Job,
} from "../api/client";
import { haptic, type VkUser } from "../hooks/useBridge";
import { useChats } from "../hooks/useChats";
import neuroHubAvatar from "../assets/neurohub-avatar.png";

const POLL_MS = 2000;
const POLL_MAX = 90;
const CHAT_OPERATION = "text_generate";
const CHAT_MODEL_ID = "chatgpt";
const CHAT_ASSISTANT_NAME = "НейроХаб";

type SubmitRequest = {
  operation: string;
  modelId: string;
  chat?: boolean;
};

function tabTitle(tab: AppTab, activeChat?: Chat | null): { name: string; sub: string } {
  switch (tab) {
    case "create":
      return { name: "Создать", sub: "фото и видео" };
    case "settings":
      return { name: "Профиль", sub: "тема, баланс, история" };
    default:
      return { name: CHAT_ASSISTANT_NAME, sub: activeChat?.title ?? "НейроХаб диалог" };
  }
}

function chatIdForJob(jobId: string): string {
  return "job-" + jobId;
}

function titleForJob(job: Job): string {
  switch (job.operation) {
    case "text_generate":
      return "Текст · " + job.status;
    case "image_generate":
      return "Фото · " + job.status;
    case "video_generate":
      return "Видео · " + job.status;
    default:
      return "Генерация · " + job.status;
  }
}

function botMessageFromJob(job: Job): ChatMessage {
  const terminal = isTerminal(job.status);
  const failed = statusKind(job.status) === "failed";
  return {
    id: "b-" + job.id,
    role: "bot",
    jobId: job.id,
    operation: job.operation,
    status: job.status,
    pending: !terminal,
    error: failed ? errorLabel(job) : undefined,
    artifactIds: terminal && !failed ? job.output_artifact_ids : undefined,
    createdAt: job.created_at,
  };
}

function jobToMessages(job: Job): ChatMessage[] {
  const messages: ChatMessage[] = [];
  if (job.prompt) {
    messages.push({ id: "u-" + job.id, role: "user", text: job.prompt });
  }
  messages.push(botMessageFromJob(job));
  return messages;
}

function upsertJobChat(chats: Chat[], job: Job): Chat[] {
  const bot = botMessageFromJob(job);
  let updated = false;
  const next = chats.map((chat) => {
    const index = chat.messages.findIndex((msg) => msg.jobId === job.id);
    if (index === -1) return chat;
    updated = true;
    const messages = chat.messages.map((msg, i) =>
      i === index ? { ...msg, ...bot, text: msg.text } : msg,
    );
    return { ...chat, title: titleForJob(job), messages, updatedAt: Date.now() };
  });
  if (updated) return next;
  return [
    {
      id: chatIdForJob(job.id),
      title: titleForJob(job),
      createdAt: Date.parse(job.created_at) || Date.now(),
      updatedAt: Date.parse(job.updated_at) || Date.now(),
      messages: jobToMessages(job),
    },
    ...next,
  ];
}

function jobIdsFromChats(chats: Chat[]): Set<string> {
  const ids = new Set<string>();
  for (const chat of chats) {
    for (const msg of chat.messages) {
      if (msg.jobId) ids.add(msg.jobId);
    }
  }
  return ids;
}

function isChatJob(job: Job): boolean {
  return job.operation === CHAT_OPERATION;
}

function promptForBot(messages: ChatMessage[], index: number): string {
  for (let i = index - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg.role === "user" && msg.text) return msg.text;
  }
  return "";
}

function upsertJob(jobs: Job[], job: Job): Job[] {
  const exists = jobs.some((item) => item.id === job.id);
  const next = exists ? jobs.map((item) => (item.id === job.id ? job : item)) : [job, ...jobs];
  return next.sort((a, b) => b.created_at.localeCompare(a.created_at));
}

function chatFromConversation(conversation: ChatConversation): Chat {
  const createdAt = Date.parse(conversation.created_at) || Date.now();
  const updatedAt = Date.parse(conversation.updated_at) || createdAt;
  return {
    id: conversation.id || "default",
    title: conversation.title || "НейроХаб диалог",
    preview: conversation.last_message_preview,
    createdAt,
    updatedAt,
    messages: [],
  };
}

function messageFromHistory(message: ChatConversationMessage): ChatMessage {
  return {
    id: message.id,
    role: message.role === "bot" ? "bot" : "user",
    text: message.text,
    jobId: message.job_id,
    createdAt: message.created_at,
  };
}

function isLocalDraftChat(chat: Chat): boolean {
  return (
    chat.messages.length === 0 ||
    chat.messages.some((msg) => msg.pending || Boolean(msg.error))
  );
}

function mergeBackendChats(prev: Chat[], backend: Chat[]): Chat[] {
  if (backend.length === 0) {
    return prev.length > 0 ? prev : [];
  }
  const byID = new Map<string, Chat>();
  for (const chat of backend) {
    byID.set(chat.id, chat);
  }
  for (const chat of prev) {
    const existing = byID.get(chat.id);
    if (existing) {
      const messages = chat.messages.length > 0 ? chat.messages : existing.messages;
      byID.set(chat.id, {
        ...existing,
        title: existing.title || chat.title,
        preview: existing.preview || chat.preview,
        messages,
        updatedAt: Math.max(existing.updatedAt, chat.updatedAt),
      });
      continue;
    }
    if (isLocalDraftChat(chat)) {
      byID.set(chat.id, chat);
    }
  }
  return Array.from(byID.values()).sort((a, b) => b.updatedAt - a.updatedAt);
}

function mergeHistoryMessages(current: ChatMessage[], history: ChatMessage[]): ChatMessage[] {
  const byID = new Map<string, ChatMessage>();
  for (const message of history) {
    byID.set(message.id, message);
  }
  for (const message of current) {
    if (message.pending || message.error) {
      byID.set(message.id, message);
      continue;
    }
    if (message.jobId && !history.some((item) => item.jobId === message.jobId && item.role === message.role)) {
      byID.set(message.id, message);
    }
  }
  return Array.from(byID.values()).sort((a, b) => {
    const at = Date.parse(a.createdAt ?? "") || 0;
    const bt = Date.parse(b.createdAt ?? "") || 0;
    return at - bt;
  });
}

function pollTargetForJob(chats: Chat[], job: Job): { chatId: string; botMsgId: string; missing: boolean } {
  for (const chat of chats) {
    const msg = chat.messages.find((item) => item.role === "bot" && item.jobId === job.id);
    if (msg) return { chatId: chat.id, botMsgId: msg.id, missing: false };
  }
  return { chatId: chatIdForJob(job.id), botMsgId: "b-" + job.id, missing: true };
}

export function ChatScreen({ user }: { user: VkUser }) {
  const {
    chats,
    activeChat,
    activeId,
    newChat,
    selectChat,
    deleteChat,
    clearChats,
    ensureActive,
    setMessages,
    setChats,
    setActiveId,
  } = useChats();

  const [balance, setBalance] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<AppTab>(() => loadAppTab());
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => loadThemeMode());
  const [jobs, setJobs] = useState<Job[]>([]);

  const scrollRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const pollingRef = useRef(new Set<string>());
  const pollTimersRef = useRef(new Map<string, number>());
  const seededRef = useRef(false);
  const submittingRef = useRef(false);
  const chatsRef = useRef(chats);

  useEffect(() => {
    chatsRef.current = chats;
  }, [chats]);

  const patchInChat = useCallback(
    (chatId: string, msgId: string, patch: Partial<ChatMessage>) => {
      setMessages(chatId, (prev) =>
        prev.map((m) => (m.id === msgId ? { ...m, ...patch } : m)),
      );
    },
    [setMessages],
  );

  const refreshBalance = useCallback(() => {
    getBalance().then(setBalance).catch(() => undefined);
  }, []);

  const loadConversationMessages = useCallback(
    async (chatId: string) => {
      if (!chatId || chatId.startsWith("job-")) return;
      const localChat = chatsRef.current.find((chat) => chat.id === chatId);
      if (localChat && localChat.messages.length === 0 && !localChat.preview) return;
      setHistoryLoading(true);
      try {
        const history = await listChatConversationMessages(chatId);
        if (!mountedRef.current) return;
        const messages = history.map(messageFromHistory);
        setChats((prev) =>
          prev.map((chat) =>
            chat.id === chatId
              ? {
                  ...chat,
                  messages: mergeHistoryMessages(chat.messages, messages),
                  preview: messages[messages.length - 1]?.text ?? chat.preview,
                }
              : chat,
          ),
        );
      } catch {
        /* keep already rendered messages on transient load errors */
      } finally {
        if (mountedRef.current) setHistoryLoading(false);
      }
    },
    [setChats],
  );

  const refreshConversations = useCallback(async () => {
    const conversations = await listChatConversations();
    if (!mountedRef.current) return;
    const backendChats = conversations.map(chatFromConversation);
    let nextChats: Chat[] = [];
    setChats((prev) => {
      const merged = mergeBackendChats(prev, backendChats);
      nextChats = merged.length > 0 ? merged : prev;
      return nextChats;
    });
    setActiveId((cur) => {
      if (cur && nextChats.some((chat) => chat.id === cur)) return cur;
      return nextChats[0]?.id ?? cur ?? null;
    });
  }, [setActiveId, setChats]);

  function changeTab(nextTab: AppTab) {
    setActiveTab(nextTab);
    saveAppTab(nextTab);
    setDrawerOpen(false);
  }

  useEffect(() => watchThemeMode(themeMode), [themeMode]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      pollTimersRef.current.forEach((timer) => window.clearTimeout(timer));
      pollTimersRef.current.clear();
      pollingRef.current.clear();
    };
  }, []);

  const poll = useCallback(
    async (chatId: string, botMsgId: string, jobId: string) => {
      const waitForNextPoll = () =>
        new Promise<void>((resolve) => {
          const timer = window.setTimeout(() => {
            pollTimersRef.current.delete(jobId);
            resolve();
          }, POLL_MS);
          pollTimersRef.current.set(jobId, timer);
        });

      for (let i = 0; i < POLL_MAX; i++) {
        if (!mountedRef.current) return;
        let job: Job;
        try {
          job = await getJob(jobId);
          setJobs((prev) => upsertJob(prev, job));
        } catch {
          if (i < POLL_MAX - 1) {
            await waitForNextPoll();
          }
          continue;
        }
        if (isTerminal(job.status)) {
          if (statusKind(job.status) === "failed") {
            patchInChat(chatId, botMsgId, {
              pending: false,
              status: job.status,
              error: errorLabel(job),
            });
            haptic("error");
          } else {
            const text = await resolveBotText(job);
            patchInChat(chatId, botMsgId, {
              pending: false,
              status: job.status,
              text,
              artifactIds: job.output_artifact_ids,
            });
            haptic("success");
            if (job.operation === CHAT_OPERATION) {
              void refreshConversations()
                .then(() => loadConversationMessages(chatId))
                .catch(() => undefined);
            }
          }
          refreshBalance();
          return;
        }
        patchInChat(chatId, botMsgId, { status: job.status });
        if (i < POLL_MAX - 1) {
          await waitForNextPoll();
        }
      }
      if (mountedRef.current) {
        patchInChat(chatId, botMsgId, {
          pending: false,
          error: "Превышено время ожидания",
        });
      }
    },
    [loadConversationMessages, patchInChat, refreshBalance, refreshConversations],
  );

  const startPoll = useCallback(
    (chatId: string, botMsgId: string, jobId: string) => {
      if (pollingRef.current.has(jobId)) return;
      pollingRef.current.add(jobId);
      void poll(chatId, botMsgId, jobId).finally(() => {
        pollingRef.current.delete(jobId);
      });
    },
    [poll],
  );

  function runSubmit(text: string, request?: SubmitRequest): boolean {
    if (submittingRef.current) return false;
    submittingRef.current = true;
    setSubmitting(true);
    void submitJob(text, request).finally(() => {
      submittingRef.current = false;
      if (mountedRef.current) setSubmitting(false);
    });
    return true;
  }

  function handleSend(text: string): boolean {
    return runSubmit(text, { operation: CHAT_OPERATION, modelId: CHAT_MODEL_ID, chat: true });
  }

  function handleRetry(msg: ChatMessage, prompt: string): void {
    if (!prompt) return;
    const operation = msg.operation ?? CHAT_OPERATION;
    const fallbackModel = modalityByOperation(operation).models[0]?.id ?? CHAT_MODEL_ID;
    runSubmit(prompt, {
      operation,
      modelId: msg.model ?? fallbackModel,
      chat: operation === CHAT_OPERATION,
    });
  }

  async function submitJob(
    text: string,
    request?: SubmitRequest,
  ): Promise<Job | null> {
    const operation = request?.operation ?? CHAT_OPERATION;
    const selectedModel = request?.modelId ?? CHAT_MODEL_ID;
    const isChat = request?.chat === true;
    const chatId = isChat ? ensureActive() : "";
    const botId = "b-" + uid();
    const idempotencyKey = createIdempotencyKey();
    if (isChat) {
      setMessages(chatId, (prev) => [
        ...prev,
        { id: "u-" + uid(), role: "user", text },
        {
          id: botId,
          role: "bot",
          operation,
          model: selectedModel,
          pending: true,
          status: "received",
        },
      ]);
    }
    haptic("light");
    try {
      const job = isChat
        ? await createChatMessage(
            { prompt: text, conversation_id: chatId },
            { idempotencyKey },
          )
        : await createJob(
            { operation, prompt: text, model_id: selectedModel },
            { idempotencyKey },
          );
      patchInChat(chatId, botId, {
        jobId: job.id,
        status: job.status,
        createdAt: job.created_at,
      });
      setJobs((prev) => upsertJob(prev, job));
      refreshBalance();
      startPoll(chatId, botId, job.id);
      return job;
    } catch (e) {
      patchInChat(chatId, botId, {
        pending: false,
        error: apiUserMessage(e),
      });
      haptic("error");
      return null;
    }
  }

  // Первый запуск: баланс + восстановление активных и локально отмеченных задач.
  useEffect(() => {
    refreshBalance();
    void refreshConversations().catch(() => undefined);
    if (seededRef.current) return;
    seededRef.current = true;
    listJobs()
      .then((jobs) => {
        if (!mountedRef.current) return;
        const sorted = [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at));
        setJobs(sorted);
        const localJobIds = jobIdsFromChats(chats);
        const restored = sorted.filter((job) => !isTerminal(job.status) || localJobIds.has(job.id));
        const restoredChatJobs = restored.filter(isChatJob);
        if (restoredChatJobs.length > 0) {
          setChats((prev) =>
            restoredChatJobs.reduceRight((next, job) => upsertJobChat(next, job), prev),
          );
          setActiveId((cur) => cur ?? chatIdForJob(restoredChatJobs[0].id));
        }
        for (const job of restored) {
          const target = pollTargetForJob(chats, job);
          if (!isTerminal(job.status)) {
            startPoll(target.chatId, target.botMsgId, job.id);
          } else if (statusKind(job.status) === "done" && job.operation === "text_generate") {
            void resolveBotText(job).then((text) => {
              if (text && mountedRef.current) {
                patchInChat(target.chatId, target.botMsgId, { text });
              }
            });
          }
        }
      })
      .catch(() => undefined)
      .finally(() => {
        if (mountedRef.current) setLoading(false);
      });
  }, [refreshBalance, refreshConversations, setChats, setActiveId, startPoll, patchInChat]);

  useEffect(() => {
    if (activeTab !== "chat" || !activeId) return;
    void loadConversationMessages(activeId);
  }, [activeTab, activeId, loadConversationMessages]);

  useEffect(() => {
    const pending = jobs.filter((job) => !isTerminal(job.status));
    if (pending.length === 0) return;

    const missingChats: Job[] = [];
    for (const job of pending) {
      const target = pollTargetForJob(chats, job);
      if (target.missing && isChatJob(job)) missingChats.push(job);
      startPoll(target.chatId, target.botMsgId, job.id);
    }

    if (missingChats.length > 0) {
      setChats((prev) =>
        missingChats.reduceRight((next, job) => upsertJobChat(next, job), prev),
      );
    }
  }, [jobs, chats, setChats, startPoll]);

  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [activeChat?.messages]);

  const messages = activeChat?.messages ?? [];
  const empty =
    !loading &&
    !historyLoading &&
    messages.length === 0 &&
    !activeChat?.preview;
  const header = tabTitle(activeTab, activeChat);

  return (
    <Panel id="miniapp-root-panel" className="chat-panel" mode="plain">
      <div className="chat">
      <ChatList
        chats={chats}
        activeId={activeId}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onSelect={(id) => {
          selectChat(id);
          setDrawerOpen(false);
        }}
        onNew={() => {
          newChat();
          setDrawerOpen(false);
        }}
        onDelete={deleteChat}
        onClearHistory={clearChats}
      />

      {activeTab === "chat" && (
        <header className="chat__header">
          <button
            type="button"
            className="chat__history-btn"
            aria-label="История диалогов"
            onClick={() => setDrawerOpen(true)}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M3 12a9 9 0 1 0 3-6.7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              <path d="M3 3v6h6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
          <div className="chat__presence">
            <div className="chat__presence-avatar">
              <img src={neuroHubAvatar} alt="" />
              <span className="chat__presence-dot" aria-hidden="true" />
            </div>
            <div>
              <div className="chat__presence-name">{header.name}</div>
              <div
                className={
                  "chat__presence-status " + (submitting ? "is-typing" : "is-online")
                }
              >
                {submitting ? "думает..." : "онлайн"}
              </div>
            </div>
          </div>
          <span className="chat__ai-badge" aria-hidden="true">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2l2.2 6.8H21l-5.5 4 2.1 6.7L12 15.8 6.4 19.5l2.1-6.7L3 8.8h6.8L12 2z" />
            </svg>
            AI
          </span>
        </header>
      )}

      <section className={"app-tab-panel" + (activeTab === "chat" ? " is-active" : "")} aria-hidden={activeTab !== "chat"}>
          <div className="chat__scroll" ref={scrollRef}>
            {(loading || (historyLoading && messages.length === 0 && Boolean(activeChat?.preview))) && (
              <div className="splash">
                <Spinner />
              </div>
            )}
            {empty && (
              <div className="greeting">
                <span className="greeting__avatar avatar--bot" aria-hidden="true">
                  <img className="avatar__img" src={neuroHubAvatar} alt="" />
                </span>
                <h1 className="greeting__title">Привет, {user.firstName}!</h1>
                <p className="greeting__text">
                  Напишите сообщение — НейроХаб ответит в этом диалоге с учетом последних реплик.
                </p>
              </div>
            )}
            {messages.map((m, index) =>
              m.role === "bot" ? (
                <MessageBubble
                  key={m.id}
                  msg={m}
                  userName={user.name}
                  userAvatar={user.avatar}
                  botName={CHAT_ASSISTANT_NAME}
                  onRetry={() => handleRetry(m, promptForBot(messages, index))}
                />
              ) : (
                <MessageBubble key={m.id} msg={m} userName={user.name} userAvatar={user.avatar} />
              ),
            )}
          </div>

          <Composer
            onDraftChange={() => undefined}
            onSend={handleSend}
            disabled={loading || submitting}
          />
      </section>

      <section className={"app-tab-panel app-tab-panel--create" + (activeTab === "create" ? " is-active" : "")} aria-hidden={activeTab !== "create"}>
        <WorkflowMode
          user={user}
          jobs={jobs}
          chats={chats}
          loading={loading}
          submitting={submitting}
          onCreateJob={submitJob}
        />
      </section>

      <section className={"app-tab-panel app-tab-panel--settings" + (activeTab === "settings" ? " is-active" : "")} aria-hidden={activeTab !== "settings"}>
        <SettingsScreen
          themeMode={themeMode}
          balance={balance}
          jobs={jobs}
          loading={loading}
          onThemeModeChange={setThemeMode}
          onClearLocalHistory={clearChats}
          onRefreshBalance={refreshBalance}
        />
      </section>

      <nav className="nh-tabbar" aria-label="Навигация">
        {(
          [
            {
              id: "create" as AppTab,
              label: "Создать",
              icon: (
                <svg className="nh-tabbar__icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M12 2l2.2 6.8H21l-5.5 4 2.1 6.7L12 15.8 6.4 19.5l2.1-6.7L3 8.8h6.8L12 2z" stroke="currentColor" strokeWidth="1.8" />
                </svg>
              ),
            },
            {
              id: "chat" as AppTab,
              label: "Чат",
              icon: (
                <svg className="nh-tabbar__icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M4 5h16a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H9l-5 4V7a2 2 0 0 1 2-2z" stroke="currentColor" strokeWidth="1.8" />
                </svg>
              ),
            },
            {
              id: "settings" as AppTab,
              label: "Профиль",
              icon: (
                <svg className="nh-tabbar__icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <circle cx="12" cy="8" r="4" stroke="currentColor" strokeWidth="1.8" />
                  <path d="M5 20c1.5-4 12.5-4 14 0" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
                </svg>
              ),
            },
          ] as const
        ).map((tab) => (
          <button
            key={tab.id}
            type="button"
            className={"nh-tabbar__btn" + (activeTab === tab.id ? " is-active" : "")}
            aria-label={tab.label}
            aria-current={activeTab === tab.id ? "page" : undefined}
            onClick={() => changeTab(tab.id)}
          >
            <span className="nh-tabbar__icon-wrap">{tab.icon}</span>
            <span className="nh-tabbar__label">{tab.label}</span>
          </button>
        ))}
      </nav>
      </div>
    </Panel>
  );
}
