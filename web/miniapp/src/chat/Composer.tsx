// src/chat/Composer.tsx
import { useRef, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { Button, Textarea } from "@vkontakte/vkui";

export function Composer({
  modelName = "ChatGPT",
  onDraftChange,
  onSend,
  disabled,
  estimateCost,
  estimateEnough,
  estimateLoading,
  estimateError,
}: {
  modelName?: string;
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

  return (
    <div className="composer">
      <div className="composer__controls">
        <span className="model-pill" aria-label={`Модель ${modelName}`}>{modelName}</span>
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
