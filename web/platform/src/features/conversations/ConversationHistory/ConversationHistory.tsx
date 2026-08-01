"use client";

import { useState } from "react";

import {
  conversationHistoryPageLimit,
  type ConversationHistoryData,
} from "@/features/conversations/conversation-history-contract";
import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { parseConversationMessageList, type ConversationMessage } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

import styles from "./ConversationHistory.module.css";

type ConversationHistoryProps = {
  history: ConversationHistoryData;
};

export function ConversationHistory({ history }: ConversationHistoryProps) {
  if (history.kind === "not_found") {
    return <ConversationHistoryState message={ru.conversations.historyUnavailable} />;
  }

  if (history.kind === "unavailable") {
    return <ConversationHistoryState message={ru.conversations.historyLoadFailure} />;
  }

  return <ConversationHistoryReady key={history.conversationId} history={history} />;
}

function ConversationHistoryReady({
  history,
}: Readonly<{
  history: Extract<ConversationHistoryData, { kind: "ready" }>;
}>) {
  const [messages, setMessages] = useState(history.messages);
  const [hasMoreBefore, setHasMoreBefore] = useState(history.hasMoreBefore);
  const [isLoadingEarlier, setIsLoadingEarlier] = useState(false);
  const [loadEarlierFailed, setLoadEarlierFailed] = useState(false);

  const loadEarlier = async () => {
    const beforeSeq = messages[0]?.seq;
    if (!hasMoreBefore || beforeSeq === undefined || isLoadingEarlier) {
      return;
    }

    setIsLoadingEarlier(true);
    setLoadEarlierFailed(false);
    try {
      const response = await webBrowserFetch(
        `/web/v1/conversations/${history.conversationId}/messages?before_seq=${beforeSeq}&limit=${conversationHistoryPageLimit}` as `/web/v1/${string}`,
      );
      if (response.status !== 200) {
        throw new Error("Unable to load conversation history.");
      }
      const page = parseConversationMessageList(await response.json());
      setMessages((currentMessages) => prependOlderMessages(currentMessages, page.items));
      setHasMoreBefore(page.has_more_before);
    } catch {
      setLoadEarlierFailed(true);
    } finally {
      setIsLoadingEarlier(false);
    }
  };

  return (
    <section aria-labelledby="conversation-history-title" className={styles.content}>
      <header className={styles.header}>
        <p className={styles.eyebrow}>{ru.conversations.historyEyebrow}</p>
        <h1 id="conversation-history-title">{ru.conversations.historyTitle}</h1>
      </header>
      {messages.length === 0 ? (
        <p className={styles.empty} role="status">
          {ru.conversations.historyEmpty}
        </p>
      ) : (
        <>
          {hasMoreBefore ? (
            <div className={styles.loadEarlier}>
              <Button disabled={isLoadingEarlier} onClick={loadEarlier}>
                {isLoadingEarlier ? ru.conversations.historyLoadEarlierPending : ru.conversations.historyLoadEarlier}
              </Button>
              {loadEarlierFailed ? (
                <p className={styles.loadEarlierFailure} role="alert">
                  {ru.conversations.historyLoadEarlierFailure}
                </p>
              ) : null}
            </div>
          ) : null}
          <ol className={styles.messages}>
            {messages.map((message) => (
              <li
                className={message.role === "user" ? styles.userMessage : styles.assistantMessage}
                key={message.id}
              >
                <span className={styles.role}>
                  {message.role === "user" ? ru.conversations.userRole : ru.conversations.assistantRole}
                </span>
                <p>{message.text}</p>
              </li>
            ))}
          </ol>
        </>
      )}
    </section>
  );
}

function prependOlderMessages(currentMessages: ConversationMessage[], olderMessages: ConversationMessage[]): ConversationMessage[] {
  const knownMessageIDs = new Set(currentMessages.map((message) => message.id));
  return [...olderMessages.filter((message) => !knownMessageIDs.has(message.id)), ...currentMessages];
}

function ConversationHistoryState({ message }: Readonly<{ message: string }>) {
  return (
    <section aria-label={ru.conversations.historyTitle} className={styles.content}>
      <p className={styles.empty} role="status">
        {message}
      </p>
    </section>
  );
}
