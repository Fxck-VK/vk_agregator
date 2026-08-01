"use client";

import { useEffect, useRef, useState } from "react";

import { ru } from "@/i18n/ru";

import styles from "./ChatScrollToBottom.module.css";

type ChatScrollToBottomProps = {
  contentVersion: string;
  forceScrollRequest: number;
  scrollContainer: HTMLElement | null;
};

const isAtBottom = (container: HTMLElement) =>
  container.scrollHeight - container.scrollTop - container.clientHeight <= 1;

export function ChatScrollToBottom({
  contentVersion,
  forceScrollRequest,
  scrollContainer,
}: ChatScrollToBottomProps) {
  const [atBottom, setAtBottom] = useState(() => !scrollContainer || isAtBottom(scrollContainer));
  const followLatestRef = useRef(atBottom);
  const previousContentVersion = useRef(contentVersion);
  const previousForceScrollRequest = useRef(forceScrollRequest);

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
    setAtBottom(true);
    scrollContainer.scrollTo({
      behavior: window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ? "auto" : "smooth",
      top: scrollContainer.scrollHeight,
    });
  }, [contentVersion, forceScrollRequest, scrollContainer]);

  useEffect(() => {
    if (!scrollContainer) {
      return;
    }

    const updatePosition = () => {
      const nextAtBottom = isAtBottom(scrollContainer);
      followLatestRef.current = nextAtBottom;
      setAtBottom(nextAtBottom);
    };

    updatePosition();
    scrollContainer.addEventListener("scroll", updatePosition);

    return () => {
      scrollContainer.removeEventListener("scroll", updatePosition);
    };
  }, [scrollContainer]);

  if (atBottom || !scrollContainer) {
    return null;
  }

  const scrollToLatest = () => {
    followLatestRef.current = true;
    setAtBottom(true);
    scrollContainer.scrollTo({
      behavior: window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ? "auto" : "smooth",
      top: scrollContainer.scrollHeight - scrollContainer.clientHeight,
    });
  };

  return (
    <button
      aria-label={ru.conversations.scrollToLatest}
      className={styles.button}
      onClick={scrollToLatest}
      type="button"
    >
      <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
        <path d="m6 9 6 6 6-6" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
      </svg>
    </button>
  );
}
