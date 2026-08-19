"use client";

import { useEffect, useRef, useState } from "react";

import { CheckIcon } from "@/components/icons/CheckIcon";
import { CopyIcon } from "@/components/icons/CopyIcon";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import {
  parseConversationMessageRatingResponse,
  type ConversationMessageRating,
} from "@/lib/web-api/contracts";

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
      conversationId: string;
      initialRating: ConversationMessageRating;
      messageId: string;
    }
);

type RatingMutation = {
  rating: ConversationMessageRating;
  sequence: number;
};

export function ConversationMessageActions(props: Readonly<ConversationMessageActionsProps>) {
  const { kind, messageText } = props;
  const [copied, setCopied] = useState(false);
  const initialRating = kind === "assistant" ? props.initialRating : null;
  const [rating, setRating] = useState<ConversationMessageRating>(initialRating);
  const copyResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const confirmedRatingRef = useRef<ConversationMessageRating>(initialRating);
  const desiredRatingRef = useRef<ConversationMessageRating>(initialRating);
  const ratingQueueRef = useRef<RatingMutation[]>([]);
  const ratingSequenceRef = useRef(0);
  const ratingPendingCountRef = useRef(0);
  const ratingProcessingRef = useRef(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (copyResetTimerRef.current !== null) {
        clearTimeout(copyResetTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (kind !== "assistant" || ratingPendingCountRef.current > 0) {
      return;
    }
    confirmedRatingRef.current = initialRating;
    desiredRatingRef.current = initialRating;
    setRating(initialRating);
  }, [initialRating, kind]);

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

  const processRatingQueue = async () => {
    if (kind !== "assistant" || ratingProcessingRef.current) {
      return;
    }
    const mutation = ratingQueueRef.current.shift();
    if (mutation === undefined) {
      return;
    }

    ratingProcessingRef.current = true;
    try {
      const response = await webBrowserMutation(
        `/web/v1/conversations/${props.conversationId}/messages/${props.messageId}/rating`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ rating: mutation.rating }),
        },
      );
      if (response.status !== 200) {
        throw new Error("Unable to save message rating.");
      }
      const saved = parseConversationMessageRatingResponse(await response.json()).rating;
      confirmedRatingRef.current = saved;
      if (mountedRef.current && mutation.sequence === ratingSequenceRef.current) {
        desiredRatingRef.current = saved;
        setRating(saved);
      }
    } catch {
      if (mountedRef.current && mutation.sequence === ratingSequenceRef.current) {
        desiredRatingRef.current = confirmedRatingRef.current;
        setRating(confirmedRatingRef.current);
      }
    } finally {
      ratingPendingCountRef.current = Math.max(0, ratingPendingCountRef.current - 1);
      ratingProcessingRef.current = false;
      void processRatingQueue();
    }
  };

  const changeRating = (selectedRating: Exclude<ConversationMessageRating, null>) => {
    if (kind !== "assistant") {
      return;
    }
    const nextRating = desiredRatingRef.current === selectedRating ? null : selectedRating;
    desiredRatingRef.current = nextRating;
    setRating(nextRating);
    const sequence = ratingSequenceRef.current + 1;
    ratingSequenceRef.current = sequence;
    ratingPendingCountRef.current += 1;
    ratingQueueRef.current.push({ rating: nextRating, sequence });
    void processRatingQueue();
  };

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
            onClick={() => changeRating("like")}
            type="button"
          >
            <LikeIcon />
          </button>
          <button
            aria-label={ru.conversations.dislikeMessage}
            aria-pressed={rating === "dislike"}
            className={styles.action}
            data-tooltip={ru.conversations.dislikeMessage}
            onClick={() => changeRating("dislike")}
            type="button"
          >
            <DislikeIcon />
          </button>
        </>
      )}
    </div>
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
