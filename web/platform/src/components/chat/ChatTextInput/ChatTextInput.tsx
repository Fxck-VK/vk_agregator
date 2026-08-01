import type { ChangeEventHandler, KeyboardEvent } from "react";

import styles from "./ChatTextInput.module.css";

type ChatTextInputAppearance = "inset" | "plain";
type ChatTextInputSize = "compact" | "expanded";

type ChatTextInputProps = {
  appearance: ChatTextInputAppearance;
  disabled: boolean;
  onChange: ChangeEventHandler<HTMLTextAreaElement>;
  onSend: () => void;
  placeholder: string;
  rows: number;
  size: ChatTextInputSize;
  value: string;
};

export function ChatTextInput({
  appearance,
  disabled,
  onChange,
  onSend,
  placeholder,
  rows,
  size,
  value,
}: ChatTextInputProps) {
  const submitOnEnter = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (disabled || event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) {
      return;
    }

    event.preventDefault();
    onSend();
  };

  return (
    <textarea
      className={`${styles.input} ${styles[appearance]} ${styles[size]}`}
      disabled={disabled}
      onChange={onChange}
      onKeyDown={submitOnEnter}
      placeholder={placeholder}
      rows={rows}
      value={value}
    />
  );
}
