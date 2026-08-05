"use client";

import { useEffect, useRef, useState } from "react";

import { ru } from "@/i18n/ru";

import styles from "./ConversationMessageActions.module.css";

type SharedActionsProps = {
  messageText: string;
};

type ConversationMessageActionsProps = SharedActionsProps & (
  | {
      kind: "user";
      onRecreate: (messageText: string) => void;
    }
  | {
      kind: "assistant";
    }
);

type MessageRating = "like" | "dislike" | null;

export function ConversationMessageActions(props: Readonly<ConversationMessageActionsProps>) {
  const { kind, messageText } = props;
  const [copied, setCopied] = useState(false);
  const [rating, setRating] = useState<MessageRating>(null);
  const copyResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (copyResetTimerRef.current !== null) {
      clearTimeout(copyResetTimerRef.current);
    }
  }, []);

  const copyMessage = async () => {
    try {
      await navigator.clipboard.writeText(messageText);
      setCopied(true);
      if (copyResetTimerRef.current !== null) {
        clearTimeout(copyResetTimerRef.current);
      }
      copyResetTimerRef.current = setTimeout(() => {
        setCopied(false);
        copyResetTimerRef.current = null;
      }, 2_000);
    } catch {
      // The message remains visible so it can still be selected manually.
    }
  };

  const copyLabel = copied ? ru.conversations.copiedMessage : ru.conversations.copyMessage;

  return (
    <div className={styles.actions}>
      <button
        aria-label={copyLabel}
        className={styles.action}
        data-tooltip={copyLabel}
        onClick={() => void copyMessage()}
        type="button"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </button>
      {kind === "user" ? (
        <button
          aria-label={ru.conversations.recreateMessage}
          className={styles.action}
          data-tooltip={ru.conversations.recreateMessage}
          onClick={() => props.onRecreate(messageText)}
          type="button"
        >
          <RecreateIcon />
        </button>
      ) : (
        <>
          <button
            aria-label={ru.conversations.likeMessage}
            aria-pressed={rating === "like"}
            className={styles.action}
            data-tooltip={ru.conversations.likeMessage}
            onClick={() => setRating((current) => current === "like" ? null : "like")}
            type="button"
          >
            <LikeIcon />
          </button>
          <button
            aria-label={ru.conversations.dislikeMessage}
            aria-pressed={rating === "dislike"}
            className={styles.action}
            data-tooltip={ru.conversations.dislikeMessage}
            onClick={() => setRating((current) => current === "dislike" ? null : "dislike")}
            type="button"
          >
            <DislikeIcon />
          </button>
        </>
      )}
    </div>
  );
}

function CopyIcon() {
  return (
    <svg aria-hidden="true" data-icon="copy" fill="none" viewBox="0 0 24 24">
      <rect height="13" rx="2.5" stroke="currentColor" strokeWidth="1.8" width="11" x="8" y="8" />
      <path d="M16 8V6.5A2.5 2.5 0 0 0 13.5 4h-7A2.5 2.5 0 0 0 4 6.5v7A2.5 2.5 0 0 0 6.5 16H8" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg aria-hidden="true" data-icon="check" fill="none" viewBox="0 0 24 24">
      <path d="m5 12.5 4.25 4.25L19 7" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" />
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

function LikeIcon() {
  return (
    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24">
      <path d="M7.5 10.25 11 4.5c.38-.64 1.35-.37 1.35.38v3.87h4.57a2 2 0 0 1 1.94 2.5l-1.75 6.75a2 2 0 0 1-1.94 1.5H7.5m0-9.25v9.25m0-9.25H4.25v9.25H7.5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
    </svg>
  );
}

function DislikeIcon() {
  return (
    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24">
      <path d="m7.5 13.75 3.5 5.75c.38.64 1.35.37 1.35-.38v-3.87h4.57a2 2 0 0 0 1.94-2.5L17.11 6a2 2 0 0 0-1.94-1.5H7.5m0 9.25V4.5m0 9.25H4.25V4.5H7.5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
    </svg>
  );
}
