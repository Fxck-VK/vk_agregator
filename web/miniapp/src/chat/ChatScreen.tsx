// src/chat/ChatScreen.tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Panel, Tabbar, TabbarItem } from "@vkontakte/vkui";
import { Avatar, Spinner } from "../ui/ui";
import { MessageBubble } from "./MessageBubble";
import { Composer } from "./Composer";
import { ChatList } from "./ChatList";
import { ResultCard } from "../components/ResultCard";
import { WorkflowMode } from "../workflow/WorkflowMode";
import { modalityByOperation, uid, type Chat, type ChatMessage } from "./types";
import { loadAppMode, saveAppMode, type AppMode } from "../mode";
import {
  createChatMessage,
  createJob,
  createIdempotencyKey,
  estimateJob,
  getJob,
  listJobs,
  getBalance,
  isTerminal,
  statusKind,
  errorLabel,
  apiUserMessage,
  resolveBotText,
  type Job,
  type EstimateResponse,
} from "../api/client";
import { haptic, type VkUser } from "../hooks/useBridge";
import { useChats } from "../hooks/useChats";

const POLL_MS = 2000;
const POLL_MAX = 90;
const ESTIMATE_DEBOUNCE_MS = 450;
const CHAT_OPERATION = "text_generate";
const CHAT_MODEL_ID = "chatgpt";
const CHAT_MODEL_NAME = "ChatGPT";

type SubmitRequest = {
  operation: string;
  modelId: string;
  chat?: boolean;
};

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
  const [submitting, setSubmitting] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const [estimate, setEstimate] = useState<EstimateResponse | null>(null);
  const [estimateLoading, setEstimateLoading] = useState(false);
  const [estimateError, setEstimateError] = useState<string | null>(null);
  const [mode, setMode] = useState<AppMode>(() => loadAppMode());
  const [jobs, setJobs] = useState<Job[]>([]);

  const scrollRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const pollingRef = useRef(new Set<string>());
  const pollTimersRef = useRef(new Map<string, number>());
  const seededRef = useRef(false);
  const submittingRef = useRef(false);

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

  function changeMode(nextMode: AppMode) {
    setMode(nextMode);
    saveAppMode(nextMode);
    setDrawerOpen(false);
  }

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      pollTimersRef.current.forEach((timer) => window.clearTimeout(timer));
      pollTimersRef.current.clear();
      pollingRef.current.clear();
    };
  }, []);

  useEffect(() => {
    const prompt = draft.trim();
    setEstimate(null);
    setEstimateError(null);
    if (!prompt) {
      setEstimateLoading(false);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(() => {
      setEstimateLoading(true);
      estimateJob({ operation: CHAT_OPERATION, prompt, model_id: CHAT_MODEL_ID })
        .then((data) => {
          if (cancelled) return;
          setEstimate(data);
          setBalance(data.balance_credits);
        })
        .catch((e) => {
          if (cancelled) return;
          const message = apiUserMessage(e);
          setEstimateError(
            message === "Выбранная модель недоступна. Выберите другую модель"
              ? message
              : "Оценка временно недоступна. Запуск можно продолжить",
          );
        })
        .finally(() => {
          if (!cancelled) setEstimateLoading(false);
        });
    }, ESTIMATE_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [draft]);

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
    [patchInChat, refreshBalance],
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
    const chatId = ensureActive();
    const botId = "b-" + uid();
    const idempotencyKey = createIdempotencyKey();
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
    haptic("light");
    try {
      const job = isChat
        ? await createChatMessage(
            { prompt: text },
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
    if (seededRef.current) {
      setLoading(false);
      return;
    }
    seededRef.current = true;
    listJobs()
      .then((jobs) => {
        if (!mountedRef.current) return;
        const sorted = [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at));
        setJobs(sorted);
        const localJobIds = jobIdsFromChats(chats);
        const restored = sorted.filter((job) => !isTerminal(job.status) || localJobIds.has(job.id));
        if (restored.length > 0) {
          setChats((prev) =>
            restored.reduceRight((next, job) => upsertJobChat(next, job), prev),
          );
          setActiveId((cur) => cur ?? chatIdForJob(restored[0].id));
        }
        for (const job of restored) {
          const chatId = chatIdForJob(job.id);
          if (!isTerminal(job.status)) {
            startPoll(chatId, "b-" + job.id, job.id);
          } else if (statusKind(job.status) === "done" && job.operation === "text_generate") {
            void resolveBotText(job).then((text) => {
              if (text && mountedRef.current) {
                patchInChat(chatId, "b-" + job.id, { text });
              }
            });
          }
        }
      })
      .catch(() => undefined)
      .finally(() => {
        if (mountedRef.current) setLoading(false);
      });
  }, [chats.length, refreshBalance, setChats, setActiveId, startPoll, patchInChat]);

  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [activeChat?.messages]);

  const messages = activeChat?.messages ?? [];
  const empty = !loading && messages.length === 0;

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

      <header className="chat__header">
        <Button
          type="button"
          className={"icon-btn" + (mode === "workflow" ? " icon-btn--ghost" : "")}
          mode="tertiary"
          appearance="neutral"
          size="l"
          aria-label="Чаты"
          onClick={() => setDrawerOpen(true)}
          disabled={mode === "workflow"}
        >
          ☰
        </Button>
        <Avatar src={null} fallback="AI" />
        <div className="chat__title">
          <span className="chat__name">{mode === "workflow" ? "Workflow" : "Ассистент"}</span>
          <span className="chat__sub">
            {mode === "workflow" ? "создание VK-контента" : activeChat?.title ?? "ChatGPT диалог"}
          </span>
        </div>
        <span className="chat__spacer" />
        {balance !== null && (
          <span className="balance-pill">{balance.toLocaleString("ru-RU")} кр.</span>
        )}
      </header>

      {mode === "chat" ? (
        <>
          <div className="chat__scroll" ref={scrollRef}>
            {loading && (
              <div className="splash">
                <Spinner />
              </div>
            )}
            {empty && (
              <div className="greeting">
                <span className="greeting__avatar">AI</span>
                <h1 className="greeting__title">Привет, {user.firstName}!</h1>
                <p className="greeting__text">
                  Напишите сообщение — ChatGPT ответит в этом диалоге с учетом последних реплик.
                </p>
              </div>
            )}
            {messages.map((m, index) =>
              m.role === "bot" ? (
                <ResultCard
                  key={m.id}
                  msg={m}
                  prompt={promptForBot(messages, index)}
                  onRetry={() => handleRetry(m, promptForBot(messages, index))}
                />
              ) : (
                <MessageBubble key={m.id} msg={m} userName={user.name} userAvatar={user.avatar} />
              ),
            )}
          </div>

          <Composer
            modelName={CHAT_MODEL_NAME}
            onDraftChange={setDraft}
            onSend={handleSend}
            disabled={loading || submitting}
            estimateCost={estimate?.cost_estimate ?? null}
            estimateEnough={estimate?.enough_credits ?? null}
            estimateLoading={estimateLoading}
            estimateError={estimateError}
          />
        </>
      ) : (
        <WorkflowMode
          user={user}
          balance={balance}
          jobs={jobs}
          chats={chats}
          loading={loading}
          submitting={submitting}
          onCreateJob={submitJob}
          onClearLocalHistory={clearChats}
        />
      )}

      <Tabbar className="mode-tabbar" mode="horizontal" plain>
        <TabbarItem
          selected={mode === "chat"}
          label="Chat"
          aria-label="Chat"
          onClick={() => changeMode("chat")}
        >
          <span className="mode-tabbar__icon" aria-hidden="true">C</span>
        </TabbarItem>
        <TabbarItem
          selected={mode === "workflow"}
          label="Workflow"
          aria-label="Workflow"
          onClick={() => changeMode("workflow")}
        >
          <span className="mode-tabbar__icon" aria-hidden="true">W</span>
        </TabbarItem>
      </Tabbar>
      </div>
    </Panel>
  );
}
