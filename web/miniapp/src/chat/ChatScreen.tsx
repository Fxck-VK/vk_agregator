// src/chat/ChatScreen.tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Avatar, Spinner } from "../ui/ui";
import { MessageBubble } from "./MessageBubble";
import { Composer } from "./Composer";
import { ChatList } from "./ChatList";
import { modalityById, uid, type ChatMessage, type ModalityId } from "./types";
import {
  createJob,
  getJob,
  listJobs,
  getBalance,
  isTerminal,
  statusKind,
  errorLabel,
  resolveBotText,
  type Job,
} from "../api/client";
import { haptic, type VkUser } from "../hooks/useBridge";
import { useChats } from "../hooks/useChats";

const POLL_MS = 2000;
const POLL_MAX = 90;

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
  const [drawerOpen, setDrawerOpen] = useState(false);

  const scrollRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const pollingRef = useRef(new Set<string>());
  const seededRef = useRef(false);

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

  async function handleSend(text: string) {
    const modality = modalityById(modalityId);
    const chatId = ensureActive();
    const botId = "b-" + uid();
    setMessages(chatId, (prev) => [
      ...prev,
      { id: "u-" + uid(), role: "user", text },
      {
        id: botId,
        role: "bot",
        operation: modality.operation,
        model: modelId,
        pending: true,
        status: "received",
      },
    ]);
    haptic("light");
    try {
      const job = await createJob({ operation: modality.operation, prompt: text });
      patchInChat(chatId, botId, { jobId: job.id, status: job.status });
      startPoll(chatId, botId, job.id);
    } catch (e) {
      patchInChat(chatId, botId, {
        pending: false,
        error: e instanceof Error ? e.message : "Не удалось отправить",
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
        {messages.map((m) => (
          <MessageBubble key={m.id} msg={m} userName={user.name} userAvatar={user.avatar} />
        ))}
      </div>

      <Composer
        modalityId={modalityId}
        onModality={changeModality}
        modelId={modelId}
        onModel={setModelId}
        onSend={handleSend}
        disabled={loading}
      />
    </div>
  );
}