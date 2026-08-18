"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

import { AssistantTypingIndicator } from "@/components/chat/AssistantTypingIndicator/AssistantTypingIndicator";
import { Button } from "@/components/ui/Button/Button";
import {
  conversationHistoryPageLimit,
  type ConversationHistoryData,
} from "@/features/conversations/conversation-history-contract";
import { ConversationComposer } from "@/features/conversations/ConversationComposer/ConversationComposer";
import { ConversationMessageActions } from "@/features/conversations/ConversationMessageActions/ConversationMessageActions";
import { ConversationTitleSync } from "@/features/conversations/ConversationTitleSync/ConversationTitleSync";
import {
  clearPendingConversationPrompt,
  readPendingConversationPrompt,
} from "@/features/conversations/pending-conversation-prompt";
import {
  clearPendingConversationTitleSync,
  fallbackConversationTitle,
  readPendingConversationTitleSync,
  savePendingConversationTitleSync,
} from "@/features/conversations/pending-conversation-title-sync";
import { useOptionalWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";
import {
  isSafeWebChatAcceptedResponse,
  parseConversationMessageList,
  parseWebChatJob,
  type ConversationItem,
  type ConversationMessage,
} from "@/lib/web-api/contracts";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

import styles from "./ConversationHistory.module.css";

type ConversationHistoryProps = {
  history: ConversationHistoryData;
  initialRefresh?: boolean;
};

type PollRequest = {
  id: number;
  baselineSeq: number;
};

type PendingTurn = PollRequest & {
  idempotencyKey: string | null;
  prompt: string;
  status: "sending" | "accepted" | "failed";
};

type ComposerDraftRequest = {
  id: number;
  text: string;
};

const conversationRefreshIntervalMs = 2_000;
const conversationRefreshDeadlineMs = 30_000;
const conversationRefreshMaxAttempts = 15;

export function ConversationHistory({ history, initialRefresh = false }: ConversationHistoryProps) {
  if (history.kind === "loading") {
    return <ConversationHistoryState message={ru.conversations.historyLoadEarlierPending} />;
  }

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
  const workspaceConversationList = useOptionalWorkspaceConversationList();
  const replaceConversation = workspaceConversationList?.replaceConversation;
  const updateConversationTitle = workspaceConversationList?.updateConversationTitle;
  const initialRefreshBaselineSeq = history.messages.at(-1)?.seq ?? 0;
  const shouldStartInitialRefresh = initialRefresh && !history.messages.some((message) => message.role === "assistant");
  const initialRefreshRequest = shouldStartInitialRefresh
    ? { id: 1, baselineSeq: initialRefreshBaselineSeq }
    : null;
  const [messages, setMessages] = useState(history.messages);
  const [hasMoreBefore, setHasMoreBefore] = useState(history.hasMoreBefore);
  const [isLoadingEarlier, setIsLoadingEarlier] = useState(false);
  const [loadEarlierFailed, setLoadEarlierFailed] = useState(false);
  const [pollRequest, setPollRequest] = useState<PollRequest | null>(initialRefreshRequest);
  const [activeRefreshID, setActiveRefreshID] = useState<number | null>(initialRefreshRequest?.id ?? null);
  const [refreshDelayed, setRefreshDelayed] = useState(false);
  const [pendingTurn, setPendingTurn] = useState<PendingTurn | null>(null);
  const [forceScrollRequest, setForceScrollRequest] = useState(0);
  const [workspaceScrollRegion, setWorkspaceScrollRegion] = useState<HTMLElement | null>(null);
  const [composerDraftRequest, setComposerDraftRequest] = useState<ComposerDraftRequest | null>(null);
  const [titleSyncFallback, setTitleSyncFallback] = useState(() => readPendingConversationTitleSync(history.conversationId));
  const refreshSequenceRef = useRef(initialRefreshRequest?.id ?? 0);
  const composerDraftRequestSequenceRef = useRef(0);

  const recreateMessage = useCallback((messageText: string) => {
    composerDraftRequestSequenceRef.current += 1;
    setComposerDraftRequest({
      id: composerDraftRequestSequenceRef.current,
      text: messageText,
    });
  }, []);

  const completeTitleSync = useCallback(() => {
    clearPendingConversationTitleSync(history.conversationId);
    setTitleSyncFallback(null);
  }, [history.conversationId]);

  const acceptSyncedConversation = useCallback((conversation: ConversationItem) => {
    replaceConversation?.(conversation);
  }, [replaceConversation]);

  useLayoutEffect(() => {
    if (titleSyncFallback !== null) {
      updateConversationTitle?.(history.conversationId, titleSyncFallback);
    }
  }, [history.conversationId, titleSyncFallback, updateConversationTitle]);

  useEffect(() => {
    const scrollRegion = document.querySelector<HTMLElement>('main[data-testid="workspace-scroll-region"]');
    let active = true;
    queueMicrotask(() => {
      if (active) {
        setWorkspaceScrollRegion(scrollRegion);
      }
    });

    return () => {
      active = false;
    };
  }, []);

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

  const submitPendingTurn = async (turn: PendingTurn) => {
    if (turn.idempotencyKey === null) return;

    setRefreshDelayed(false);
    setPendingTurn({ ...turn, status: "sending" });
    setActiveRefreshID(turn.id);
    setForceScrollRequest((currentRequest) => currentRequest + 1);

    try {
      const response = await webBrowserMutation(`/web/v1/conversations/${history.conversationId}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": turn.idempotencyKey,
        },
        body: JSON.stringify({ prompt: turn.prompt }),
      });
      if (response.status !== 200 && response.status !== 201) {
        throw new Error("Unable to complete the request.");
      }
      const job = parseWebChatJob(await response.json());
      if (!isSafeWebChatAcceptedResponse(response.status, job)) {
        throw new Error("Unable to complete the request.");
      }

      setPendingTurn((currentTurn) => currentTurn?.id === turn.id
        ? { ...currentTurn, status: "accepted" }
        : currentTurn);
      setPollRequest({ id: turn.id, baselineSeq: turn.baselineSeq });
    } catch {
      setActiveRefreshID((currentID) => currentID === turn.id ? null : currentID);
      setPendingTurn((currentTurn) => currentTurn?.id === turn.id
        ? { ...currentTurn, status: "failed" }
        : currentTurn);
    }
  };

  const beginMessageSubmission = (prompt: string) => {
    const baselineSeq = messages.at(-1)?.seq ?? 0;
    if (baselineSeq === 0 && titleSyncFallback === null) {
      const fallbackTitle = fallbackConversationTitle(prompt);
      if (fallbackTitle !== "") {
        updateConversationTitle?.(history.conversationId, fallbackTitle);
        savePendingConversationTitleSync(history.conversationId, fallbackTitle);
        setTitleSyncFallback(fallbackTitle);
      }
    }
    refreshSequenceRef.current += 1;
    const request = {
      id: refreshSequenceRef.current,
      baselineSeq,
    };
    void submitPendingTurn({
      ...request,
      idempotencyKey: crypto.randomUUID(),
      prompt,
      status: "sending",
    });
  };

  const retryPendingTurn = () => {
    if (pendingTurn?.status !== "failed" || pendingTurn.idempotencyKey === null) return;
    void submitPendingTurn(pendingTurn);
  };

  useEffect(() => {
    if (!initialRefresh) {
      return;
    }

    const prompt = readPendingConversationPrompt(history.conversationId);
    if (prompt === null) {
      return;
    }

    const persistedPrompt = messages.some(
      (message) => message.role === "user" && message.text.trim() === prompt,
    );
    if (persistedPrompt || pendingTurn !== null) {
      clearPendingConversationPrompt(history.conversationId);
      return;
    }

    if (activeRefreshID === null) {
      return;
    }

    let active = true;
    queueMicrotask(() => {
      if (active) {
        setPendingTurn({
          id: activeRefreshID,
          baselineSeq: initialRefreshBaselineSeq,
          idempotencyKey: null,
          prompt,
          status: "accepted",
        });
      }
    });

    return () => {
      active = false;
    };
  }, [activeRefreshID, history.conversationId, initialRefresh, initialRefreshBaselineSeq, messages, pendingTurn]);

  useEffect(() => {
    if (pollRequest === null) {
      return;
    }

    let active = true;
    let attempts = 0;
    let afterSeq = pollRequest.baselineSeq;
    let nextPollTimer: ReturnType<typeof setTimeout> | undefined;
    let activeRequest: AbortController | undefined;

    const stop = (clearActiveRefresh = true) => {
      if (!active) {
        return;
      }
      active = false;
      if (nextPollTimer !== undefined) {
        clearTimeout(nextPollTimer);
      }
      clearTimeout(deadlineTimer);
      activeRequest?.abort();
      if (clearActiveRefresh) {
        setActiveRefreshID((currentID) => currentID === pollRequest.id ? null : currentID);
      }
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
          setPendingTurn((currentTurn) => {
            if (currentTurn?.id !== pollRequest.id) {
              return currentTurn;
            }

            const persistedPrompt = newerMessages.some(
              (message) => (
                message.role === "user"
                && message.seq > pollRequest.baselineSeq
                && message.text.trim() === currentTurn.prompt
              ),
            );
            return persistedPrompt || assistantObserved ? null : currentTurn;
          });
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

    return () => stop(false);
  }, [history.conversationId, pollRequest]);

  const pendingTurnIsActive = pendingTurn?.id === activeRefreshID && pendingTurn.status !== "failed";
  const hasVisibleMessages = messages.length > 0 || pendingTurn !== null || activeRefreshID !== null;
  const contentVersion = `${messages.at(-1)?.id ?? ""}:${pendingTurn?.id ?? ""}:${pendingTurn?.status ?? ""}:${activeRefreshID ?? ""}`;

  return (
    <section aria-labelledby="conversation-history-title" className={styles.content}>
      {titleSyncFallback !== null ? (
        <ConversationTitleSync
          conversationId={history.conversationId}
          fallbackTitle={titleSyncFallback}
          onComplete={completeTitleSync}
          onConversation={acceptSyncedConversation}
        />
      ) : null}
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
        {!hasVisibleMessages ? (
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
                  {message.role === "user" ? (
                    <ConversationMessageActions kind="user" messageText={message.text} onRecreate={recreateMessage} />
                  ) : (
                    <ConversationMessageActions kind="assistant" messageText={message.text} />
                  )}
                </li>
              ))}
              {pendingTurn !== null ? (
                <PendingTurnItems
                  onRecreate={recreateMessage}
                  onRetry={retryPendingTurn}
                  pendingTurn={pendingTurn}
                  showIndicator={pendingTurnIsActive}
                />
              ) : null}
              {pendingTurn === null && activeRefreshID !== null ? (
                <li className={styles.assistantMessage} data-chat-pending="assistant">
                  <AssistantTypingIndicator label={ru.conversations.composerAwaitingResponse} />
                </li>
              ) : null}
            </ol>
          </>
        )}
      </div>
      <ConversationComposer
        contentVersion={contentVersion}
        disabled={pendingTurn !== null || activeRefreshID !== null}
        forceScrollRequest={forceScrollRequest}
        initialDraft={composerDraftRequest?.text}
        key={`composer:${composerDraftRequest?.id ?? 0}`}
        onSubmit={beginMessageSubmission}
        scrollContainer={workspaceScrollRegion}
      />
    </section>
  );
}

function PendingTurnItems({
  onRecreate,
  onRetry,
  pendingTurn,
  showIndicator,
}: Readonly<{
  onRecreate: (messageText: string) => void;
  onRetry: () => void;
  pendingTurn: PendingTurn;
  showIndicator: boolean;
}>) {
  return (
    <>
      <li className={styles.userMessage} data-chat-pending="user">
        <span className={styles.role}>{ru.conversations.userRole}</span>
        <p>{pendingTurn.prompt}</p>
        <ConversationMessageActions kind="user" messageText={pendingTurn.prompt} onRecreate={onRecreate} />
        {pendingTurn.status === "failed" ? (
          <div className={styles.pendingTurnFailure}>
            <span role="alert">{ru.conversations.messageNotSent}</span>
            <Button onClick={onRetry}>{ru.conversations.messageRetryLabel}</Button>
          </div>
        ) : null}
      </li>
      {showIndicator ? (
        <li className={styles.assistantMessage} data-chat-pending="assistant">
          <AssistantTypingIndicator label={ru.conversations.composerAwaitingResponse} />
        </li>
      ) : null}
    </>
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
