// src/chat/Composer.tsx
import { useRef, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { Button, NativeSelect, Textarea } from "@vkontakte/vkui";
import { MODALITIES, modalityById, type ModalityId } from "./types";

export function Composer({
  modalityId,
  onModality,
  modelId,
  onModel,
  onDraftChange,
  onSend,
  disabled,
  estimateCost,
  estimateEnough,
  estimateLoading,
  estimateError,
}: {
  modalityId: ModalityId;
  onModality: (id: ModalityId) => void;
  modelId: string;
  onModel: (id: string) => void;
  onDraftChange: (text: string) => void;
  onSend: (text: string) => boolean;
  disabled?: boolean;
  estimateCost?: number | null;
  estimateEnough?: boolean | null;
  estimateLoading?: boolean;
  estimateError?: string | null;
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
    const value = e.target.value;
    setText(value);
    onDraftChange(value);
    grow();
  }

  function submit() {
    const value = text.trim();
    if (!value || disabled) return;
    if (!onSend(value)) return;
    setText("");
    onDraftChange("");
    if (ref.current) ref.current.style.height = "auto";
  }

  function onKey(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  }

  function onModelChange(_: ChangeEvent<HTMLSelectElement>, value: unknown) {
    if (typeof value === "string") {
      onModel(value);
    }
  }

  return (
    <div className="composer">
      <div className="composer__controls">
        <div className="segment" role="tablist">
          {MODALITIES.map((m) => (
            <Button
              key={m.id}
              type="button"
              role="tab"
              aria-selected={m.id === modalityId}
              className={"segment__btn" + (m.id === modalityId ? " is-active" : "")}
              mode={m.id === modalityId ? "primary" : "tertiary"}
              appearance={m.id === modalityId ? "accent" : "neutral"}
              size="m"
              onClick={() => onModality(m.id)}
            >
              {m.label}
            </Button>
          ))}
        </div>
        <div className="model-select">
          <NativeSelect
            value={modelId}
            onChange={onModelChange}
            aria-label="Модель"
          >
            {modality.models.map((md) => (
              <option key={md.id} value={md.id}>
                {md.label}
              </option>
            ))}
          </NativeSelect>
        </div>
      </div>
      <div className="estimate-line" aria-live="polite">
        {estimateLoading ? (
          <span>Оценка...</span>
        ) : estimateCost !== null && estimateCost !== undefined ? (
          <>
            <span>Стоимость: {estimateCost.toLocaleString("ru-RU")} кр.</span>
            {estimateEnough === false && (
              <span className="estimate-line__warn">Недостаточно кредитов</span>
            )}
          </>
        ) : estimateError ? (
          <span className="estimate-line__muted">{estimateError}</span>
        ) : (
          <span className="estimate-line__muted">Стоимость появится до запуска</span>
        )}
      </div>
      <div className="composer__row">
        <Textarea
          getRef={ref}
          className="composer__input"
          rows={1}
          grow
          maxHeight={140}
          placeholder="Напишите сообщение…"
          value={text}
          onChange={onInput}
          onKeyDown={onKey}
        />
        <Button
          type="button"
          className="composer__send"
          mode="primary"
          appearance="accent"
          size="l"
          rounded
          onClick={submit}
          disabled={disabled || !text.trim()}
          aria-label="Отправить"
        >
          ↑
        </Button>
      </div>
    </div>
  );
}
