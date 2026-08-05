"use client";

import { ru } from "@/i18n/ru";

import styles from "./ConversationMessageActions.module.css";

type ConversationMessageActionsProps = {
  messageText: string;
  onRecreate: (messageText: string) => void;
};

export function ConversationMessageActions({
  messageText,
  onRecreate,
}: Readonly<ConversationMessageActionsProps>) {
  const copyMessage = async () => {
    try {
      await navigator.clipboard.writeText(messageText);
    } catch {
      // The message remains visible so it can still be selected manually.
    }
  };

  return (
    <div className={styles.actions}>
      <button
        aria-label={ru.conversations.copyMessage}
        className={styles.action}
        onClick={() => void copyMessage()}
        title={ru.conversations.copyMessage}
        type="button"
      >
        <CopyIcon />
      </button>
      <button
        aria-label={ru.conversations.recreateMessage}
        className={styles.action}
        onClick={() => onRecreate(messageText)}
        title={ru.conversations.recreateMessage}
        type="button"
      >
        <RecreateIcon />
      </button>
    </div>
  );
}

function CopyIcon() {
  return (
    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24">
      <rect height="13" rx="2.5" stroke="currentColor" strokeWidth="1.8" width="11" x="8" y="8" />
      <path d="M16 8V6.5A2.5 2.5 0 0 0 13.5 4h-7A2.5 2.5 0 0 0 4 6.5v7A2.5 2.5 0 0 0 6.5 16H8" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  );
}

function RecreateIcon() {
  return (
    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24">
      <path d="m12 3 1.35 3.65L17 8l-3.65 1.35L12 13l-1.35-3.65L7 8l3.65-1.35L12 3Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.6" />
      <path d="m18.5 13 .82 2.18L21.5 16l-2.18.82L18.5 19l-.82-2.18L15.5 16l2.18-.82L18.5 13Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.6" />
    </svg>
  );
}
