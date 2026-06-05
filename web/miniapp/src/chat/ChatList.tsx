// src/chat/ChatList.tsx
import type { Chat } from "./types";

export function ChatList({
  chats,
  activeId,
  open,
  onClose,
  onSelect,
  onNew,
  onDelete,
}: {
  chats: Chat[];
  activeId: string | null;
  open: boolean;
  onClose: () => void;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
}) {
  return (
    <>
      <div
        className={"drawer-overlay" + (open ? " is-open" : "")}
        onClick={onClose}
      />
      <aside className={"drawer" + (open ? " is-open" : "")}>
        <button type="button" className="drawer__new" onClick={onNew}>
          + Новый чат
        </button>
        <div className="drawer__list">
          {chats.length === 0 && <div className="chat-empty">Пока нет чатов</div>}
          {chats.map((c) => (
            <div
              key={c.id}
              className={"chat-item" + (c.id === activeId ? " is-active" : "")}
              onClick={() => onSelect(c.id)}
            >
              <span className="chat-item__title">{c.title}</span>
              <button
                type="button"
                className="chat-item__del"
                aria-label="Удалить чат"
                onClick={(e) => {
                  e.stopPropagation();
                  onDelete(c.id);
                }}
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      </aside>
    </>
  );
}