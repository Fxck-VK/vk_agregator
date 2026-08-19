"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { AssistantTypingIndicator } from "@/components/chat/AssistantTypingIndicator/AssistantTypingIndicator";
import { ru } from "@/i18n/ru";

import styles from "./ChatScrollToBottom.module.css";

type ChatScrollToBottomProps = {
  contentVersion: string;
  forceScrollRequest: number;
  isAwaitingResponse?: boolean;
  scrollContainer: HTMLElement | null;
};

const isAtBottom = (container: HTMLElement) =>
  container.scrollHeight - container.scrollTop - container.clientHeight <= 1;

const scrollBehavior = () => (window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ? "auto" : "smooth");

const SCROLL_SETTLE_DELAY_MS = 150;

export function ChatScrollToBottom({
  contentVersion,
  forceScrollRequest,
  isAwaitingResponse = false,
  scrollContainer,
}: ChatScrollToBottomProps) {
  const [atBottom, setAtBottom] = useState(() => !scrollContainer || isAtBottom(scrollContainer));
  const followLatestRef = useRef(atBottom);
  const programmaticScrollRef = useRef(false);
  const settleTimerRef = useRef<number | null>(null);
  const previousContentVersion = useRef(contentVersion);
  const previousForceScrollRequest = useRef(forceScrollRequest);

  const scheduleSettle = useCallback(() => {
    if (settleTimerRef.current !== null) {
      window.clearTimeout(settleTimerRef.current);
    }

    settleTimerRef.current = window.setTimeout(() => {
      settleTimerRef.current = null;
      if (!scrollContainer || !programmaticScrollRef.current) {
        return;
      }

      const settledAtBottom = isAtBottom(scrollContainer);
      programmaticScrollRef.current = false;
      followLatestRef.current = settledAtBottom;
      setAtBottom(settledAtBottom);
    }, SCROLL_SETTLE_DELAY_MS);
  }, [scrollContainer]);

  const scrollToLatest = useCallback(
    (top: number) => {
      if (!scrollContainer) {
        return;
      }

      programmaticScrollRef.current = true;
      scrollContainer.scrollTo({ behavior: scrollBehavior(), top });

      const nextAtBottom = isAtBottom(scrollContainer);
      setAtBottom(nextAtBottom);
      if (nextAtBottom) {
        programmaticScrollRef.current = false;
        followLatestRef.current = true;
        return;
      }

      scheduleSettle();
    },
    [scheduleSettle, scrollContainer],
  );

  useEffect(() => {
    if (scrollContainer === null) {
      return;
    }

    const contentChanged = contentVersion !== previousContentVersion.current;
    const forceScrollRequested = forceScrollRequest !== previousForceScrollRequest.current;
    previousContentVersion.current = contentVersion;
    previousForceScrollRequest.current = forceScrollRequest;

    if (!forceScrollRequested && (!contentChanged || !followLatestRef.current)) {
      return;
    }

    followLatestRef.current = true;
    scrollToLatest(scrollContainer.scrollHeight);
  }, [contentVersion, forceScrollRequest, scrollContainer, scrollToLatest]);

  useEffect(() => {
    if (!scrollContainer) {
      return;
    }

    const updatePosition = () => {
      const nextAtBottom = isAtBottom(scrollContainer);
      setAtBottom(nextAtBottom);
      if (programmaticScrollRef.current) {
        if (nextAtBottom) {
          if (settleTimerRef.current !== null) {
            window.clearTimeout(settleTimerRef.current);
            settleTimerRef.current = null;
          }
          programmaticScrollRef.current = false;
          followLatestRef.current = true;
          return;
        }

        scheduleSettle();
        return;
      }

      followLatestRef.current = nextAtBottom;
    };

    updatePosition();
    scrollContainer.addEventListener("scroll", updatePosition);

    return () => {
      scrollContainer.removeEventListener("scroll", updatePosition);
    };
  }, [scheduleSettle, scrollContainer]);

  useEffect(
    () => () => {
      if (settleTimerRef.current !== null) {
        window.clearTimeout(settleTimerRef.current);
      }
    },
    [],
  );

  if (!scrollContainer) {
    return null;
  }

  const handleScrollToLatest = () => {
    followLatestRef.current = true;
    scrollToLatest(scrollContainer.scrollHeight - scrollContainer.clientHeight);
  };

  if (isAwaitingResponse) {
    return (
      <button
        aria-label={ru.conversations.scrollToLatest}
        className={styles.button}
        onClick={handleScrollToLatest}
        type="button"
      >
        <AssistantTypingIndicator label={ru.conversations.composerAwaitingResponse} />
      </button>
    );
  }

  if (atBottom) {
    return null;
  }

  return (
    <button
      aria-label={ru.conversations.scrollToLatest}
      className={styles.button}
      onClick={handleScrollToLatest}
      type="button"
    >
      <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
        <path d="m6 9 6 6 6-6" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
      </svg>
    </button>
  );
}
