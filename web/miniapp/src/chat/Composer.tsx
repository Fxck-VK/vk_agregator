// src/chat/Composer.tsx
import { useRef, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { MODELS } from "./types";

export function Composer({
  modelId,
  onModel,
  onSend,
  disabled,
}: {
  modelId: string;
  onModel: (id: string) => void;
  onSend: (text: string) => void;
  disabled?: boolean;
}) {
  const [text, setText] = useState("");
  const ref = useRef<HTMLTextAreaElement>(null);
  const active = MODELS.find((m) => m.id === modelId) ?? MODELS[0];

  function grow() {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 140) + "px";
  }

  function onInput(e: ChangeEvent<HTMLTextAreaElement>) {
    setText(e.target.value);
    grow();
  }

  function submit() {
    const value = text.trim();
    if (!value || disabled) return;
    onSend(value);
    setText("");
    if (ref.current) ref.current.style.height = "auto";
  }

  function onKey(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  }

  return (
    <div className="composer">
      <div className="models">
        {MODELS.map((m) => (
          <button
            key={m.id}
            type="button"
            className={"model" + (m.id === modelId ? " is-active" : "")}
            onClick={() => onModel(m.id)}
          >
            {m.label}
          </button>
        ))}
        <span className="model__hint">{active.label}</span>
      </div>
      <div className="composer__row">
        <textarea
          ref={ref}
          className="composer__input"
          rows={1}
          placeholder="Напишите сообщение…"
          value={text}
          onChange={onInput}
          onKeyDown={onKey}
        />
        <button
          type="button"
          className="composer__send"
          onClick={submit}
          disabled={disabled || !text.trim()}
          aria-label="Отправить"
        >
          ↑
        </button>
      </div>
    </div>
  );
}
