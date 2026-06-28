// src/chat/ChatScreen.tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Panel } from "@vkontakte/vkui";
import { Spinner } from "../ui/ui";
import { MessageBubble } from "./MessageBubble";
import { Composer } from "./Composer";
import { WorkflowMode } from "../workflow/WorkflowMode";
import { SettingsScreen } from "../settings/SettingsScreen";
import { loadThemeMode, watchThemeMode, type ThemeMode } from "../settings/theme";
import { modalityByOperation, uid, type Chat, type ChatMessage } from "./types";
import { cleanupLegacyChatStorage, defaultThread } from "./store";
import { loadAppTab, saveAppTab, type AppTab } from "../mode";
import {
  createChatMessage,
  createJob,
  createIdempotencyKey,
  getJob,
  listJobs,
  getBalance,
  isTerminal,
  listChatMessages,
  statusKind,
  errorLabel,
  apiUserMessage,
  resolveBotText,
  type ChatConversationMessage,
  type Job,
} from "../api/client";
import { haptic, type VkUser } from "../hooks/useBridge";
import neuroHubAvatar from "../assets/neurohub-avatar.png";

const POLL_MS = 1200;
const POLL_MAX = 90;
const CHAT_OPERATION = "text_generate";
const CHAT_MODEL_ID = "chatgpt";
const CHAT_ASSISTANT_NAME = "НейроХаб";
const DEFAULT_CHAT_ID = "default";

type SubmitRequest = {
  operation: string;
  modelId?: string;
  videoRouteAlias?: string;
  imageQuality?: string;
  chat?: boolean;
  referenceArtifactIds?: string[];
  durationSec?: number;
};

function tabTitle(tab: AppTab): { name: string; sub: string } {
  switch (tab) {
    case "create":
      return { name: "Создать", sub: "фото и видео" };
    case "settings":
      return { name: "Профиль", sub: "тема, баланс, история" };
    default:
      return {
        name: CHAT_ASSISTANT_NAME,
        sub: "НейроХаб диалог",
      };
  }
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

function messageFromHistory(message: ChatConversationMessage): ChatMessage {
  return {
    id: message.id,
    role: message.role === "bot" ? "bot" : "user",
    text: message.text,
    jobId: message.job_id,
    seq: message.seq,
    createdAt: message.created_at,
  };
}

function compareChatMessages(a: ChatMessage, b: ChatMessage): number {
  const aSeq = a.seq ?? Number.NaN;
  const bSeq = b.seq ?? Number.NaN;
  if (Number.isFinite(aSeq) && Number.isFinite(bSeq) && aSeq !== bSeq) {
    return aSeq - bSeq;
  }
  const at = Date.parse(a.createdAt ?? "") || 0;
  const bt = Date.parse(b.createdAt ?? "") || 0;
  if (at !== bt) return at - bt;
  if (a.role !== b.role) return a.role === "user" ? -1 : 1;
  return 0;
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
    const hasHistoryTwin = history.some(
      (item) =>
        item.jobId === message.jobId &&
        item.role === message.role &&
        (message.role !== "user" || item.text === message.text || !message.text),
    );
    if (message.jobId && !hasHistoryTwin) {
      byID.set(message.id, message);
    }
  }
  return Array.from(byID.values()).sort(compareChatMessages);
}

const EARLY_CHAT_TEXT_STATUSES = new Set([
  "provider_succeeded",
  "postprocessing",
  "result_ready",
  "delivering",
]);

async function earlyChatBotText(job: Job): Promise<string | undefined> {
  if (job.operation !== CHAT_OPERATION) return undefined;
  if (!EARLY_CHAT_TEXT_STATUSES.has(job.status)) return undefined;
  try {
    const history = await listChatMessages();
    const bot = history.find((item) => item.job_id === job.id && item.role === "bot");
    const text = bot?.text?.trim();
    return text || undefined;
  } catch {
    return undefined;
  }
}

export function ChatScreen({ user }: { user: VkUser }) {
  const [chat, setChat] = useState<Chat>(() => defaultThread());
  const [balance, setBalance] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [activeTab, setActiveTab] = useState<AppTab>(() => loadAppTab());
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => loadThemeMode());
  const [jobs, setJobs] = useState<Job[]>([]);
  const [workflowJobRequest, setWorkflowJobRequest] = useState<Job | null>(null);
  const chats = [chat];
  const activeChat = chat;
  const activeId = chat.id;

  const scrollRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const pollingRef = useRef(new Set<string>());
  const pollTimersRef = useRef(new Map<string, number>());
  const seededRef = useRef(false);
  const submittingRef = useRef(false);

  const setMessages = useCallback((updater: (prev: ChatMessage[]) => ChatMessage[]) => {
    setChat((prev) => {
      const messages = updater(prev.messages);
      return { ...prev, messages, updatedAt: Date.now() };
    });
  }, []);

  const patchInChat = useCallback(
    (chatId: string, msgId: string, patch: Partial<ChatMessage>) => {
      if (chatId !== DEFAULT_CHAT_ID) return;
      setMessages((prev) =>
        prev.map((m) => (m.id === msgId ? { ...m, ...patch } : m)),
      );
    },
    [setMessages],
  );

  const refreshBalance = useCallback(() => {
    getBalance().then(setBalance).catch(() => undefined);
  }, []);

  const loadConversationMessages = useCallback(
    async () => {
      setHistoryLoading(true);
      try {
        const history = await listChatMessages();
        if (!mountedRef.current) return;
        const messages = history.map(messageFromHistory);
        setChat((prev) => ({
          ...prev,
          messages: mergeHistoryMessages(prev.messages, messages),
          preview: messages[messages.length - 1]?.text ?? prev.preview,
          updatedAt: Date.now(),
        }));
      } catch {
        if (!mountedRef.current) return;
        /* keep already rendered messages on transient load errors */
      } finally {
        if (mountedRef.current) setHistoryLoading(false);
      }
    },
    [],
  );

  const changeTab = useCallback((nextTab: AppTab) => {
    if (nextTab !== activeTab && document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    setActiveTab(nextTab);
    saveAppTab(nextTab);
  }, [activeTab]);

  const clearWorkflowJobRequest = useCallback(() => setWorkflowJobRequest(null), []);

  const handleHistoryJobClick = useCallback(
    (job: Job) => {
      if (job.operation === CHAT_OPERATION) {
        changeTab("chat");
        void loadConversationMessages();
        return;
      }
      if (job.operation === "image_generate" || job.operation === "video_generate") {
        const latest = jobs.find((item) => item.id === job.id) ?? job;
        setWorkflowJobRequest(latest);
        changeTab("create");
      }
    },
    [changeTab, jobs, loadConversationMessages],
  );

  useEffect(() => watchThemeMode(themeMode), [themeMode]);

  useEffect(() => {
    mountedRef.current = true;
    const pollTimers = pollTimersRef.current;
    const polling = pollingRef.current;
    return () => {
      mountedRef.current = false;
      pollTimers.forEach((timer) => window.clearTimeout(timer));
      pollTimers.clear();
      polling.clear();
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
            const text = (await resolveBotText(job)) ?? (await earlyChatBotText(job));
            patchInChat(chatId, botMsgId, {
              pending: false,
              status: job.status,
              ...(text ? { text } : {}),
              artifactIds: job.output_artifact_ids,
            });
            haptic("success");
            if (job.operation === CHAT_OPERATION) {
              void loadConversationMessages().catch(() => undefined);
            }
          }
          refreshBalance();
          return;
        }
        const earlyText = await earlyChatBotText(job);
        if (earlyText) {
          patchInChat(chatId, botMsgId, {
            pending: false,
            status: job.status,
            text: earlyText,
            artifactIds: job.output_artifact_ids,
          });
        } else {
          patchInChat(chatId, botMsgId, { status: job.status });
        }
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
    [loadConversationMessages, patchInChat, refreshBalance],
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
    const selectedVideoRouteAlias = request?.videoRouteAlias;
    const isChat = request?.chat === true;
    const chatId = isChat ? DEFAULT_CHAT_ID : "";
    const userMsgId = "u-" + uid();
    const botId = "b-" + uid();
    const idempotencyKey = createIdempotencyKey();
    if (isChat) {
      const sentAt = new Date().toISOString();
      setMessages((prev) => [
        ...prev,
        { id: userMsgId, role: "user", text, createdAt: sentAt },
        {
          id: botId,
          role: "bot",
          operation,
          model: selectedModel,
          pending: true,
          status: "received",
          createdAt: sentAt,
        },
      ]);
    }
    haptic("light");
    try {
      const job = isChat
        ? await createChatMessage(
            { prompt: text },
            { idempotencyKey },
          )
        : await createJob(
            {
              operation,
              prompt: text,
              model_id: !isChat && operation === "video_generate" ? undefined : selectedModel,
              video_route_alias:
                !isChat && operation === "video_generate" ? selectedVideoRouteAlias : undefined,
              image_quality:
                !isChat && operation === "image_generate" ? request?.imageQuality : undefined,
              reference_artifact_ids:
                !isChat &&
                (operation === "image_generate" || operation === "video_generate") &&
                request?.referenceArtifactIds &&
                request.referenceArtifactIds.length > 0
                  ? request.referenceArtifactIds
                  : undefined,
              duration_sec:
                !isChat && operation === "video_generate" && request?.durationSec !== undefined
                  ? request.durationSec
                  : undefined,
            },
            { idempotencyKey },
          );
      patchInChat(chatId, userMsgId, { jobId: job.id });
      patchInChat(chatId, botId, {
        jobId: job.id,
        status: job.status,
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
      if (!isChat) throw e;
      return null;
    }
  }

  // Первый запуск: баланс + готовая история чата и активные media jobs.
  useEffect(() => {
    let cancelled = false;

    const finishLoading = () => {
      if (!cancelled && mountedRef.current) setLoading(false);
    };

    refreshBalance();
    cleanupLegacyChatStorage();
    void loadConversationMessages().catch(() => undefined);

    if (!seededRef.current) {
      seededRef.current = true;
      listJobs()
      .then((jobs) => {
        if (!mountedRef.current || cancelled) return;
        const sorted = [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at));
        setJobs(sorted);
        for (const job of sorted) {
          if (job.operation === CHAT_OPERATION || isTerminal(job.status)) continue;
          startPoll("", "b-" + job.id, job.id);
        }
      })
      .catch(() => undefined)
      .finally(finishLoading);
    } else {
      finishLoading();
    }

    return () => {
      cancelled = true;
      seededRef.current = false;
    };
    // Bootstrap once per mount; callback identities must not retrigger init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (activeTab !== "chat") return;
    void loadConversationMessages();
  }, [activeTab, loadConversationMessages]);

  useEffect(() => {
    const pending = jobs.filter((job) => !isTerminal(job.status));
    if (pending.length === 0) return;

    for (const job of pending) {
      if (job.operation === CHAT_OPERATION) continue;
      startPoll("", "b-" + job.id, job.id);
    }
  }, [jobs, startPoll]);

  const lastScrollChatIdRef = useRef<string | null>(null);
  useEffect(() => {
    const el = scrollRef.current;
    if (!el || !activeId) return;
    if (lastScrollChatIdRef.current !== activeId) {
      lastScrollChatIdRef.current = activeId;
      if ((activeChat?.messages.length ?? 0) > 0) {
        el.scrollTop = el.scrollHeight;
      }
      return;
    }
    el.scrollTop = el.scrollHeight;
  }, [activeChat?.messages, activeId]);

  const messages = activeChat?.messages ?? [];
  const empty =
    !loading &&
    !historyLoading &&
    messages.length === 0 &&
    !activeChat?.preview;
  const header = tabTitle(activeTab);

  return (
    <Panel id="miniapp-root-panel" className="chat-panel" mode="plain">
      <div className="chat">
      {activeTab === "chat" && (
        <header className="chat__header">
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

      <section className={"app-tab-panel" + (activeTab === "chat" ? " is-active" : "")}>
          <div className="chat__scroll" ref={scrollRef}>
            {loading && (
              <div className="splash">
                <Spinner />
              </div>
            )}
            {!loading && historyLoading && messages.length === 0 && (
              <div className="chat-inline-loader" aria-live="polite">
                <Spinner size={18} />
                <span>Загружаем диалог…</span>
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

      <section className={"app-tab-panel app-tab-panel--create" + (activeTab === "create" ? " is-active" : "")}>
        <WorkflowMode
          user={user}
          jobs={jobs}
          chats={chats}
          loading={loading}
          submitting={submitting}
          openJobRequest={workflowJobRequest}
          onOpenJobRequestHandled={clearWorkflowJobRequest}
          onCreateJob={submitJob}
        />
      </section>

      <section className={"app-tab-panel app-tab-panel--settings" + (activeTab === "settings" ? " is-active" : "")}>
        <SettingsScreen
          themeMode={themeMode}
          balance={balance}
          jobs={jobs}
          loading={loading}
          onThemeModeChange={setThemeMode}
          onRefreshBalance={refreshBalance}
          onHistoryJobClick={handleHistoryJobClick}
        />
      </section>

      <nav className="nh-tabbar" aria-label="Навигация">
        {(
          [
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
              id: "create" as AppTab,
              label: "Создать",
              icon: (
                <svg className="nh-tabbar__icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M12 2l2.2 6.8H21l-5.5 4 2.1 6.7L12 15.8 6.4 19.5l2.1-6.7L3 8.8h6.8L12 2z" stroke="currentColor" strokeWidth="1.8" />
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
