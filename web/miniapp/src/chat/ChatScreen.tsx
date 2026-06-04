// src/chat/ChatScreen.tsx
import { useCallback, useEffect, useRef, useState } from "react";
import { Avatar, Spinner } from "../ui/ui";
import { MessageBubble } from "./MessageBubble";
import { Composer } from "./Composer";
import { MODELS, uid, type ChatMessage } from "./types";
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

const POLL_MS = 2000;
const POLL_MAX = 90;

function jobToMessages(job: Job): ChatMessage[] {
  const terminal = isTerminal(job.status);
  const failed = statusKind(job.status) === "failed";
  const user: ChatMessage = {
    id: "u-" + job.id,
    role: "user",
    text: job.prompt ?? "(запрос)",
  };
  const bot: ChatMessage = {
    id: "b-" + job.id,
    role: "bot",
    jobId: job.id,
    operation: job.operation,
    status: job.status,
    pending: !terminal,
    error: failed ? errorLabel(job) : undefined,
    artifactIds: job.output_artifact_ids,
  };
  return [user, bot];
}

export function ChatScreen({ user }: { user: VkUser }) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [modelId, setModelId] = useState(MODELS[0].id);
  const [balance, setBalance] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const pollingRef = useRef(new Set<string>());

  function patchMessage(id: string, patch: Partial<ChatMessage>) {
    setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, ...patch } : m)));
  }

  const refreshBalance = useCallback(() => {
    getBalance().then(setBalance).catch(() => undefined);
  }, []);

  const poll = useCallback(
    async (botMsgId: string, jobId: string) => {
      for (let i = 0; i < POLL_MAX; i++) {
        if (!mountedRef.current) return;
        await new Promise((r) => setTimeout(r, POLL_MS));
        if (!mountedRef.current) return;
        let job: Job;
        try {
          job = await getJob(jobId);
        } catch {
          continue;
        }
        if (isTerminal(job.status)) {
          if (statusKind(job.status) === "failed") {
            patchMessage(botMsgId, {
              pending: false,
              status: job.status,
              error: errorLabel(job),
            });
            haptic("error");
          } else {
            const text = await resolveBotText(job);
            patchMessage(botMsgId, {
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
        patchMessage(botMsgId, { status: job.status });
      }
      if (mountedRef.current) {
        patchMessage(botMsgId, { pending: false, error: "Превышено время ожидания" });
      }
    },
    [refreshBalance],
  );

  const startPoll = useCallback(
    (botMsgId: string, jobId: string) => {
      if (pollingRef.current.has(jobId)) return;
      pollingRef.current.add(jobId);
      void poll(botMsgId, jobId).finally(() => {
        pollingRef.current.delete(jobId);
      });
    },
    [poll],
  );

  async function handleSend(text: string) {
    const model = MODELS.find((m) => m.id === modelId) ?? MODELS[0];
    const botId = "b-" + uid();
    const next: ChatMessage[] = [
      { id: "u-" + uid(), role: "user", text },
      { id: botId, role: "bot", operation: model.operation, pending: true, status: "received" },
    ];
    setMessages((prev) => [...prev, ...next]);
    haptic("light");
    try {
      const job = await createJob({ operation: model.operation, prompt: text });
      patchMessage(botId, { jobId: job.id, status: job.status });
      startPoll(botId, job.id);
    } catch (e) {
      patchMessage(botId, {
        pending: false,
        error: e instanceof Error ? e.message : "Не удалось отправить",
      });
      haptic("error");
    }
  }

  useEffect(() => {
    mountedRef.current = true;
    refreshBalance();
    listJobs()
      .then((jobs) => {
        if (!mountedRef.current) return;
        const sorted = [...jobs].sort((a, b) => a.created_at.localeCompare(b.created_at));
        setMessages(sorted.flatMap(jobToMessages));
        for (const j of sorted) {
          if (!isTerminal(j.status)) startPoll("b-" + j.id, j.id);
        }
        for (const j of sorted) {
          if (isTerminal(j.status) && statusKind(j.status) === "done" && j.operation === "text_generate") {
            void resolveBotText(j).then((text) => {
              if (text && mountedRef.current) {
                patchMessage("b-" + j.id, { text });
              }
            });
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
  }, [refreshBalance, startPoll]);

  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages]);

  const empty = !loading && messages.length === 0;

  return (
    <div className="chat">
      <header className="chat__header">
        <Avatar src={null} fallback="AI" />
        <div className="chat__title">
          <span className="chat__name">Ассистент</span>
          <span className="chat__sub">генеративный помощник</span>
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
              Выберите модель и напишите запрос — я сгенерирую текст, изображение или видео.
            </p>
          </div>
        )}
        {messages.map((m) => (
          <MessageBubble key={m.id} msg={m} userName={user.name} userAvatar={user.avatar} />
        ))}
      </div>

      <Composer modelId={modelId} onModel={setModelId} onSend={handleSend} disabled={loading} />
    </div>
  );
}
