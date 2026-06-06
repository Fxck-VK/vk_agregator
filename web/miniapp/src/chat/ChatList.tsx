// src/chat/ChatList.tsx
import { Button } from "@vkontakte/vkui";
import type { Chat } from "./types";

function lastPreview(chat: Chat): string {
  const messages = chat.messages;
  const last = [...messages].reverse().find((msg) => msg.text || msg.error || msg.pending || msg.status);
  if (!last && chat.preview) return chat.preview;
  if (!last) return "Пока нет сообщений";
  if (last.pending) return "НейроХаб печатает...";
  if (last.error) return last.error;
  if (last.text) return last.text.length > 80 ? last.text.slice(0, 80) + "..." : last.text;
  if (last.status) return last.status;
  return "Диалог";
}

function timeLabel(value: number): string {
  try {
    return new Intl.DateTimeFormat("ru-RU", {
      day: "2-digit",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(value));
  } catch {
    return "";
  }
}

export function ChatList({
  chats,
  activeId,
  open,
  onClose,
  onSelect,
  onNew,
  onDelete,
  onClearHistory,
}: {
  chats: Chat[];
  activeId: string | null;
  open: boolean;
  onClose: () => void;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
  onClearHistory: () => void;
}) {
  return (
    <>
      <div
        className={"drawer-overlay" + (open ? " is-open" : "")}
        onClick={onClose}
      />
      <aside className={"drawer" + (open ? " is-open" : "")} aria-label="Панель диалогов">
        <div className="drawer__head">
          <div>
            <strong>Диалоги</strong>
            <span>Локально хранится только список диалогов без текста сообщений.</span>
          </div>
          <Button type="button" mode="tertiary" appearance="neutral" size="m" aria-label="Закрыть" onClick={onClose}>
            ×
          </Button>
        </div>
        <Button type="button" className="drawer__new" mode="primary" size="l" stretched onClick={onNew}>
          Новый диалог
        </Button>
        <div className="drawer__list">
          {chats.length === 0 && <div className="chat-empty">Пока нет диалогов</div>}
          {chats.map((c) => (
            <div
              key={c.id}
              className={"chat-item" + (c.id === activeId ? " is-active" : "")}
              role="button"
              tabIndex={0}
              aria-current={c.id === activeId ? "true" : undefined}
              onClick={() => onSelect(c.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  onSelect(c.id);
                }
              }}
            >
              <div className="chat-item__body">
                <span className="chat-item__title">{c.title}</span>
                <small>{lastPreview(c)}</small>
              </div>
              <time className="chat-item__time">{timeLabel(c.updatedAt)}</time>
              <Button
                type="button"
                className="chat-item__del"
                mode="tertiary"
                appearance="neutral"
                size="s"
                aria-label="Удалить диалог"
                onClick={(e) => {
                  e.stopPropagation();
                  onDelete(c.id);
                }}
              >
                ×
              </Button>
            </div>
          ))}
        </div>
        <Button
          type="button"
          className="drawer__clear"
          mode="secondary"
          appearance="neutral"
          size="m"
          stretched
          onClick={onClearHistory}
        >
          Очистить локальные диалоги
        </Button>
      </aside>
    </>
  );
}
