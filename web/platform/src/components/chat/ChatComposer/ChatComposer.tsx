import type { ChangeEventHandler } from "react";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";

import styles from "./ChatComposer.module.css";

export type ChatComposerVariant = "conversation" | "hero" | "newChat" | "workspace";

type ChatComposerProps = {
  canSubmit: boolean;
  disabled: boolean;
  label: string;
  mediaLabel: string;
  mediaUnavailableLabel: string;
  note?: string;
  onChange: ChangeEventHandler<HTMLTextAreaElement>;
  onSend: () => void;
  placeholder: string;
  submitLabel: string;
  value: string;
  variant: ChatComposerVariant;
};

const expandedVariants = new Set<ChatComposerVariant>(["hero", "workspace"]);

export function ChatComposer({
  canSubmit,
  disabled,
  label,
  mediaLabel,
  mediaUnavailableLabel,
  note,
  onChange,
  onSend,
  placeholder,
  submitLabel,
  value,
  variant,
}: ChatComposerProps) {
  const isExpanded = expandedVariants.has(variant);

  return (
    <>
      <div className={`${styles.surface} ${styles[variant]}`}>
        <label className={styles.field}>
          <span>{label}</span>
          <ChatTextInput
            appearance="composer"
            disabled={disabled}
            onChange={onChange}
            onSend={onSend}
            placeholder={placeholder}
            rows={isExpanded ? 4 : 2}
            size={isExpanded ? "expanded" : "compact"}
            value={value}
          />
        </label>
        <div className={styles.controls}>
          <button
            aria-label={mediaLabel}
            className={styles.media}
            disabled
            title={mediaUnavailableLabel}
            type="button"
          >
            <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
              <path d="M4 7.5A2.5 2.5 0 0 1 6.5 5H10l1.5-2h5L18 5h.5A2.5 2.5 0 0 1 21 7.5v10a2.5 2.5 0 0 1-2.5 2.5h-12A2.5 2.5 0 0 1 4 17.5z" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.7" />
              <path d="m8 15 2.5-2.5 2 2L15 12l3 3" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.7" />
              <path d="M8 3v4M6 5h4" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
            </svg>
            <span>{mediaLabel}</span>
          </button>
          <button
            aria-label={submitLabel}
            className={styles.submit}
            disabled={!canSubmit || disabled}
            title={submitLabel}
            type="submit"
          >
            <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
              <path d="M12 19V5m0 0-6 6m6-6 6 6" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
            </svg>
          </button>
        </div>
      </div>
      {note === undefined ? null : <p className={styles.note}>{note}</p>}
    </>
  );
}
