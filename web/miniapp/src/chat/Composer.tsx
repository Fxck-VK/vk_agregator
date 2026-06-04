// src/chat/Composer.tsx
import { useRef, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { MODALITIES, modalityById, type ModalityId } from "./types";

export function Composer({
  modalityId,
  onModality,
  modelId,
  onModel,
  onSend,
  disabled,
}: {
  modalityId: ModalityId;
  onModality: (id: ModalityId) => void;
  modelId: string;
  onModel: (id: string) => void;
  onSend: (text: string) => void;
  disabled?: boolean;
}) {
  const [text, setText] = useState("");
  const ref = useRef<HTMLTextAreaElement>(null);
  const modality = modalityById(modalityId);

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
      <div className="composer__controls">
        <div className="segment" role="tablist">
          {MODALITIES.map((m) => (
            <button
              key={m.id}
              type="button"
              role="tab"
              aria-selected={m.id === modalityId}
              className={"segment__btn" + (m.id === modalityId ? " is-active" : "")}
              onClick={() => onModality(m.id)}
            >
              {m.label}
            </button>
          ))}
        </div>
        <div className="model-select">
          <select
            value={modelId}
            onChange={(e) => onModel(e.target.value)}
            aria-label="Модель"
          >
            {modality.models.map((md) => (
              <option key={md.id} value={md.id}>
                {md.label}
              </option>
            ))}
          </select>
        </div>
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