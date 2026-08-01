"use client";

import { useEffect, useState } from "react";

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

  useEffect(() => {
    if (!scrollContainer) {
      setAtBottom(true);
      return;
    }

    const updatePosition = () => {
      setAtBottom(isAtBottom(scrollContainer));
    };

    updatePosition();
    scrollContainer.addEventListener("scroll", updatePosition);

    return () => {
      scrollContainer.removeEventListener("scroll", updatePosition);
    };
  }, [contentVersion, forceScrollRequest, scrollContainer]);

  if (atBottom || !scrollContainer) {
    return null;
  }

  const scrollToLatest = () => {
    scrollContainer.scrollTo({ behavior: "smooth", top: scrollContainer.scrollHeight - scrollContainer.clientHeight });
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
