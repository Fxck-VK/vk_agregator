// src/chat/ChatList.tsx
import { displayChatTitle } from "./display";
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
      <aside className={"drawer" + (open ? " is-open" : "")} aria-label="История чатов">
        <div className="drawer__head">
          <div>
            <strong>История чатов</strong>
            <span>{chats.length} диалогов</span>
          </div>
          <button type="button" className="chat__history-btn" aria-label="Закрыть" onClick={onClose}>
            ×
          </button>
        </div>
        <button type="button" className="drawer__new" onClick={onNew}>
          <span aria-hidden="true">+</span>
          Новый диалог
        </button>
        <div className="drawer__list nh-scroll">
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
              <span className="chat-item__icon" aria-hidden="true">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M4 5h16a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H9l-5 4V7a2 2 0 0 1 2-2z" stroke="currentColor" strokeWidth="1.8" />
                </svg>
              </span>
              <div className="chat-item__body">
                <span className="chat-item__title">{displayChatTitle(c)}</span>
                <small>{lastPreview(c)}</small>
              </div>
              <time className="chat-item__time">{timeLabel(c.updatedAt)}</time>
              <button
                type="button"
                className="chat-item__del"
                aria-label="Удалить диалог"
                onClick={(e) => {
                  e.stopPropagation();
                  onDelete(c.id);
                }}
              >
                ×
              </button>
            </div>
          ))}
        </div>
        <button type="button" className="drawer__clear" onClick={onClearHistory}>
          Очистить локальные диалоги
        </button>
      </aside>
    </>
  );
}
