// src/chat/ChatList.tsx
import { Button } from "@vkontakte/vkui";
import type { Chat } from "./types";

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
      <aside className={"drawer" + (open ? " is-open" : "")}>
        <Button type="button" className="drawer__new" mode="primary" size="l" stretched onClick={onNew}>
          + Новый чат
        </Button>
        <p className="drawer__privacy">
          Локально хранятся только job ID, тип, статус и дата за 7 дней. Тексты запросов, ссылки и ключи не сохраняются.
        </p>
        <div className="drawer__list">
          {chats.length === 0 && <div className="chat-empty">Пока нет локальной истории</div>}
          {chats.map((c) => (
            <div
              key={c.id}
              className={"chat-item" + (c.id === activeId ? " is-active" : "")}
              onClick={() => onSelect(c.id)}
            >
              <span className="chat-item__title">{c.title}</span>
              <Button
                type="button"
                className="chat-item__del"
                mode="tertiary"
                appearance="neutral"
                size="s"
                aria-label="Удалить чат"
                onClick={(e) => {
                  e.stopPropagation();
                  onDelete(c.id);
                }}
              >
                ✕
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
          Очистить локальную историю
        </Button>
      </aside>
    </>
  );
}
