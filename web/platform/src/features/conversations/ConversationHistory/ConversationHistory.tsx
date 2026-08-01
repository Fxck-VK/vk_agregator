"use client";

import { useEffect, useRef, useState } from "react";

import {
  conversationHistoryPageLimit,
  type ConversationHistoryData,
} from "@/features/conversations/conversation-history-contract";
import { ConversationComposer } from "@/features/conversations/ConversationComposer/ConversationComposer";
import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { parseConversationMessageList, type ConversationMessage } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

import styles from "./ConversationHistory.module.css";

type ConversationHistoryProps = {
  history: ConversationHistoryData;
  initialRefresh?: boolean;
};

type PollRequest = {
  id: number;
  baselineSeq: number;
};

const conversationRefreshIntervalMs = 2_000;
const conversationRefreshDeadlineMs = 30_000;
const conversationRefreshMaxAttempts = 15;

export function ConversationHistory({ history, initialRefresh = false }: ConversationHistoryProps) {
  if (history.kind === "not_found") {
    return <ConversationHistoryState message={ru.conversations.historyUnavailable} />;
  }

  if (history.kind === "unavailable") {
    return <ConversationHistoryState message={ru.conversations.historyLoadFailure} />;
  }

  return <ConversationHistoryReady key={history.conversationId} history={history} initialRefresh={initialRefresh} />;
}

function ConversationHistoryReady({
  history,
  initialRefresh,
}: Readonly<{
  history: Extract<ConversationHistoryData, { kind: "ready" }>;
  initialRefresh: boolean;
}>) {
  const shouldStartInitialRefresh = initialRefresh && !history.messages.some((message) => message.role === "assistant");
  const initialRefreshRequest = shouldStartInitialRefresh
    ? { id: 1, baselineSeq: history.messages.at(-1)?.seq ?? 0 }
    : null;
  const [messages, setMessages] = useState(history.messages);
  const [hasMoreBefore, setHasMoreBefore] = useState(history.hasMoreBefore);
  const [isLoadingEarlier, setIsLoadingEarlier] = useState(false);
  const [loadEarlierFailed, setLoadEarlierFailed] = useState(false);
  const [pollRequest, setPollRequest] = useState<PollRequest | null>(initialRefreshRequest);
  const [activeRefreshID, setActiveRefreshID] = useState<number | null>(initialRefreshRequest?.id ?? null);
  const [refreshDelayed, setRefreshDelayed] = useState(false);
  const refreshSequenceRef = useRef(initialRefreshRequest?.id ?? 0);

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

  const beginRefresh = () => {
    const baselineSeq = messages.at(-1)?.seq ?? 0;
    refreshSequenceRef.current += 1;
    const request = {
      id: refreshSequenceRef.current,
      baselineSeq,
    };
    setRefreshDelayed(false);
    setActiveRefreshID(request.id);
    setPollRequest(request);
  };

  useEffect(() => {
    if (pollRequest === null) {
      return;
    }

    let active = true;
    let attempts = 0;
    let afterSeq = pollRequest.baselineSeq;
    let nextPollTimer: ReturnType<typeof setTimeout> | undefined;
    let activeRequest: AbortController | undefined;

    const stop = () => {
      if (!active) {
        return;
      }
      active = false;
      if (nextPollTimer !== undefined) {
        clearTimeout(nextPollTimer);
      }
      clearTimeout(deadlineTimer);
      activeRequest?.abort();
      setActiveRefreshID((currentID) => currentID === pollRequest.id ? null : currentID);
    };

    const poll = async () => {
      if (!active || attempts >= conversationRefreshMaxAttempts) {
        stop();
        return;
      }

      attempts += 1;
      const requestCursor = afterSeq;
      const request = new AbortController();
      activeRequest = request;
      let assistantObserved = false;
      try {
        const response = await webBrowserFetch(
          `/web/v1/conversations/${history.conversationId}/messages?after_seq=${requestCursor}&limit=${conversationHistoryPageLimit}` as `/web/v1/${string}`,
          { signal: request.signal },
        );
        if (response.status !== 200) {
          throw new Error("Unable to refresh conversation history.");
        }
        const page = parseConversationMessageList(await response.json());
        if (!active) {
          return;
        }

        const newerMessages = page.items
          .filter((message) => message.seq > requestCursor)
          .slice()
          .sort((left, right) => left.seq - right.seq);
        if (newerMessages.length > 0) {
          afterSeq = newerMessages.at(-1)?.seq ?? afterSeq;
          setMessages((currentMessages) => appendNewerMessages(currentMessages, newerMessages, requestCursor));
          assistantObserved = newerMessages.some(
            (message) => message.role === "assistant" && message.seq > pollRequest.baselineSeq,
          );
        }
        if (assistantObserved) {
          setRefreshDelayed(false);
          stop();
        }
      } catch {
        if (active) {
          setRefreshDelayed(true);
        }
      } finally {
        if (activeRequest === request) {
          activeRequest = undefined;
        }
        if (!active) {
          return;
        }
        if (attempts >= conversationRefreshMaxAttempts) {
          stop();
          return;
        }
        nextPollTimer = setTimeout(() => void poll(), conversationRefreshIntervalMs);
      }
    };

    const deadlineTimer = setTimeout(stop, conversationRefreshDeadlineMs);
    void poll();

    return stop;
  }, [history.conversationId, pollRequest]);

  return (
    <section aria-labelledby="conversation-history-title" className={styles.content}>
      <div className={styles.history}>
        <header className={styles.header}>
          <p className={styles.eyebrow}>{ru.conversations.historyEyebrow}</p>
          <h1 id="conversation-history-title">{ru.conversations.historyTitle}</h1>
        </header>
        {refreshDelayed ? (
          <p className={styles.refreshStatus} role="status">
            {ru.conversations.refreshDelayed}
          </p>
        ) : null}
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
      </div>
      <ConversationComposer
        conversationId={history.conversationId}
        disabled={activeRefreshID !== null}
        onAccepted={beginRefresh}
      />
    </section>
  );
}

function prependOlderMessages(currentMessages: ConversationMessage[], olderMessages: ConversationMessage[]): ConversationMessage[] {
  const knownMessageIDs = new Set(currentMessages.map((message) => message.id));
  return [...olderMessages.filter((message) => !knownMessageIDs.has(message.id)), ...currentMessages];
}

function appendNewerMessages(
  currentMessages: ConversationMessage[],
  newerMessages: ConversationMessage[],
  minimumSeq: number,
): ConversationMessage[] {
  const knownMessageIDs = new Set(currentMessages.map((message) => message.id));
  const appendedMessages = newerMessages.filter((message) => {
    if (message.seq <= minimumSeq || knownMessageIDs.has(message.id)) {
      return false;
    }
    knownMessageIDs.add(message.id);
    return true;
  });
  return [...currentMessages, ...appendedMessages];
}

function ConversationHistoryState({ message }: Readonly<{ message: string }>) {
  return (
    <section aria-label={ru.conversations.historyTitle} className={`${styles.content} ${styles.state}`}>
      <p className={styles.empty} role="status">
        {message}
      </p>
    </section>
  );
}
