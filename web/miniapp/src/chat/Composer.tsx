// src/chat/Composer.tsx
import { useRef, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { Button, Textarea } from "@vkontakte/vkui";

export function Composer({
  onDraftChange,
  onSend,
  disabled,
}: {
  onDraftChange: (text: string) => void;
  onSend: (text: string) => boolean;
  disabled?: boolean;
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
      <div className="composer__row">
        <Textarea
          slotProps={{ textArea: { getRootRef: ref } }}
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
          <span className="composer__send-icon" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}
