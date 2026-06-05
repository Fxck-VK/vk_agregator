// src/chat/ChatScreen.tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Avatar, Spinner } from "../ui/ui";
import { MessageBubble } from "./MessageBubble";
import { Composer } from "./Composer";
import { ChatList } from "./ChatList";
import { ResultCard } from "../components/ResultCard";
import { modalityById, uid, type ChatMessage, type ModalityId } from "./types";
import {
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

function jobToMessages(job: Job): ChatMessage[] {
  const terminal = isTerminal(job.status);
  const failed = statusKind(job.status) === "failed";
  return [
    { id: "u-" + job.id, role: "user", text: job.prompt ?? "(запрос)" },
    {
      id: "b-" + job.id,
      role: "bot",
      jobId: job.id,
      operation: job.operation,
      status: job.status,
      pending: !terminal,
      error: failed ? errorLabel(job) : undefined,
      artifactIds: job.output_artifact_ids,
    },
  ];
}

function promptForBot(messages: ChatMessage[], index: number): string {
  for (let i = index - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg.role === "user" && msg.text) return msg.text;
  }
  return "";
}

export function ChatScreen({ user }: { user: VkUser }) {
  const {
    chats,
    activeChat,
    activeId,
    newChat,
    selectChat,
    deleteChat,
    ensureActive,
    setMessages,
    setChats,
    setActiveId,
  } = useChats();

  const [modalityId, setModalityId] = useState<ModalityId>("text");
  const [modelId, setModelId] = useState(modalityById("text").models[0].id);
  const [balance, setBalance] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const [estimate, setEstimate] = useState<EstimateResponse | null>(null);
  const [estimateLoading, setEstimateLoading] = useState(false);
  const [estimateError, setEstimateError] = useState<string | null>(null);

  const scrollRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const pollingRef = useRef(new Set<string>());
  const seededRef = useRef(false);
  const submittingRef = useRef(false);

  function changeModality(id: ModalityId) {
    setModalityId(id);
    setModelId(modalityById(id).models[0].id);
  }

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

  useEffect(() => {
    const prompt = draft.trim();
    setEstimate(null);
    setEstimateError(null);
    if (!prompt) {
      setEstimateLoading(false);
      return;
    }

    let cancelled = false;
    const modality = modalityById(modalityId);
    const timer = window.setTimeout(() => {
      setEstimateLoading(true);
      estimateJob({ operation: modality.operation, prompt, model_id: modelId })
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
  }, [draft, modalityId, modelId]);

  const poll = useCallback(
    async (chatId: string, botMsgId: string, jobId: string) => {
      for (let i = 0; i < POLL_MAX; i++) {
        if (!mountedRef.current) return;
        let job: Job;
        try {
          job = await getJob(jobId);
        } catch {
          if (i < POLL_MAX - 1) {
            await new Promise((r) => setTimeout(r, POLL_MS));
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
          await new Promise((r) => setTimeout(r, POLL_MS));
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

  function runSubmit(text: string, request?: { operation: string; modelId: string }): boolean {
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
    return runSubmit(text);
  }

  function handleRetry(msg: ChatMessage, prompt: string): void {
    if (!prompt) return;
    const modality = modalityById(modalityId);
    runSubmit(prompt, {
      operation: msg.operation ?? modality.operation,
      modelId: msg.model ?? modelId,
    });
  }

  async function submitJob(text: string, request?: { operation: string; modelId: string }) {
    const modality = modalityById(modalityId);
    const operation = request?.operation ?? modality.operation;
    const selectedModel = request?.modelId ?? modelId;
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
      const job = await createJob(
        { operation, prompt: text, model_id: selectedModel },
        { idempotencyKey },
      );
      patchInChat(chatId, botId, { jobId: job.id, status: job.status });
      startPoll(chatId, botId, job.id);
    } catch (e) {
      patchInChat(chatId, botId, {
        pending: false,
        error: apiUserMessage(e),
      });
      haptic("error");
    }
  }

  // Первый запуск: баланс + (если локальных чатов нет) затравка из истории задач.
  useEffect(() => {
    mountedRef.current = true;
    refreshBalance();
    if (seededRef.current) {
      setLoading(false);
      return;
    }
    seededRef.current = true;
    listJobs()
      .then((jobs) => {
        if (!mountedRef.current) return;
        if (chats.length === 0 && jobs.length > 0) {
          const sorted = [...jobs].sort((a, b) =>
            a.created_at.localeCompare(b.created_at),
          );
          const messages = sorted.flatMap(jobToMessages);
          const seededId = uid();
          setChats((prev) =>
            prev.length > 0
              ? prev
              : [
                  {
                    id: seededId,
                    title: "История",
                    createdAt: Date.now(),
                    updatedAt: Date.now(),
                    messages,
                  },
                ],
          );
          setActiveId((cur) => cur ?? seededId);
          for (const j of sorted) {
            if (!isTerminal(j.status)) {
              startPoll(seededId, "b-" + j.id, j.id);
            } else if (
              statusKind(j.status) === "done" &&
              j.operation === "text_generate"
            ) {
              void resolveBotText(j).then((t) => {
                if (t && mountedRef.current) {
                  patchInChat(seededId, "b-" + j.id, { text: t });
                }
              });
            }
          }
        }
      })
      .catch(() => undefined)
      .finally(() => {
        if (mountedRef.current) setLoading(false);
      });
    return () => {
      mountedRef.current = false;
    };
  }, [chats.length, refreshBalance, setChats, setActiveId, startPoll, patchInChat]);

  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [activeChat?.messages]);

  const messages = activeChat?.messages ?? [];
  const empty = !loading && messages.length === 0;

  return (
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
      />

      <header className="chat__header">
        <button
          type="button"
          className="icon-btn"
          aria-label="Чаты"
          onClick={() => setDrawerOpen(true)}
        >
          ☰
        </button>
        <Avatar src={null} fallback="AI" />
        <div className="chat__title">
          <span className="chat__name">Ассистент</span>
          <span className="chat__sub">{activeChat?.title ?? "генеративный помощник"}</span>
        </div>
        <span className="chat__spacer" />
        {balance !== null && (
          <span className="balance-pill">{balance.toLocaleString("ru-RU")} кр.</span>
        )}
      </header>

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
              Выберите тип и модель, напишите запрос — я сгенерирую текст, изображение или видео.
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
        modalityId={modalityId}
        onModality={changeModality}
        modelId={modelId}
        onModel={setModelId}
        onDraftChange={setDraft}
        onSend={handleSend}
        disabled={loading || submitting}
        estimateCost={estimate?.cost_estimate ?? null}
        estimateEnough={estimate?.enough_credits ?? null}
        estimateLoading={estimateLoading}
        estimateError={estimateError}
      />
    </div>
  );
}
