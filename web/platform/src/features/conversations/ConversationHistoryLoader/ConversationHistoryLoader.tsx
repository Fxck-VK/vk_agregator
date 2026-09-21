"use client";

import { useDictionary } from "@/i18n/LocaleProvider";


import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { z } from "zod";

import { ConversationHistory } from "@/features/conversations/ConversationHistory/ConversationHistory";
import {
  conversationHistoryPageLimit,
  type ConversationHistoryData,
} from "@/features/conversations/conversation-history-contract";
import { SessionProgressBar } from "@/features/session/SessionProgressBar/SessionProgressBar";
import { useWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { recordWorkspaceDataLoad } from "@/features/workspace/WorkspaceNavigationMetrics/workspace-navigation-metrics";
import { parseConversationMessageList } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

import { LoadFeedback } from "@/components/ui/AsyncState/LoadFeedback";

const conversationIDSchema = z.string().uuid();
const progressDelayMs = 150;

type LoaderState = {
  history: ConversationHistoryData;
  readyRevision: number;
};

export function ConversationHistoryLoader({
  conversationId,
  initialRefresh = false,
}: {
  conversationId: string;
  initialRefresh?: boolean;
}): ReactNode {
  return (
    <ConversationHistoryLoaderContent
      key={conversationId}
      conversationId={conversationId}
      initialRefresh={initialRefresh}
    />
  );
}

function ConversationHistoryLoaderContent({
  conversationId,
  initialRefresh,
}: {
  conversationId: string;
  initialRefresh: boolean;
}): ReactNode {
  const t = useDictionary();
  const cache = useWorkspaceDataCache();
  const [initialHistory] = useState<ConversationHistoryData>(
    () => cache.getConversationHistory(conversationId) ?? { kind: "loading" },
  );
  const [state, setState] = useState<LoaderState>(() => ({ history: initialHistory, readyRevision: 0 }));
  const [pending, setPending] = useState(true);
  const [failed, setFailed] = useState(false);
  const [requestRevision, setRequestRevision] = useState(0);
  const [isProgressVisible, setIsProgressVisible] = useState(false);
  const hasRecordedCacheLoad = useRef(false);
  const hasCachedReadyHistory = initialHistory.kind === "ready";
  const isColdLoading = !hasCachedReadyHistory && state.history.kind === "loading";

  useEffect(() => {
    if (!isColdLoading) {
      return;
    }

    const progressTimer = window.setTimeout(() => setIsProgressVisible(true), progressDelayMs);

    return () => {
      window.clearTimeout(progressTimer);
      setIsProgressVisible(false);
    };
  }, [isColdLoading, requestRevision]);

  useEffect(() => {
    if (initialHistory.kind !== "ready" || hasRecordedCacheLoad.current) {
      return;
    }

    hasRecordedCacheLoad.current = true;
    recordWorkspaceDataLoad({ type: "data", target: "conversation", source: "cache", durationMs: 0 });
  }, [initialHistory]);

  useEffect(() => {
    const parsedConversationID = conversationIDSchema.safeParse(conversationId);
    if (!parsedConversationID.success) {
      let active = true;
      queueMicrotask(() => {
        if (active) {
          setState((current) => ({ ...current, history: { kind: "not_found" } }));
          setPending(false);
        }
      });
      return () => {
        active = false;
      };
    }

    const request = new AbortController();
    const requestedAt = performance.now();

    const load = async () => {
      try {
        const response = await webBrowserFetch(
          `/web/v1/conversations/${parsedConversationID.data}/messages?limit=${conversationHistoryPageLimit}` as `/web/v1/${string}`,
          { signal: request.signal },
        );
        if (request.signal.aborted) {
          return;
        }
        if ([401, 403, 404].includes(response.status)) {
          cache.deleteConversationHistory(parsedConversationID.data);
          setState((current) => ({ ...current, history: { kind: "not_found" } }));
          return;
        }
        if (response.status !== 200) throw new Error("History unavailable");

        const page = parseConversationMessageList(await response.json());
        if (request.signal.aborted) {
          return;
        }

        const history: ConversationHistoryData = {
          kind: "ready",
          conversationId: parsedConversationID.data,
          messages: page.items,
          hasMoreBefore: page.has_more_before,
        };
        cache.setConversationHistory(history);
        setState((current) => ({ history, readyRevision: current.readyRevision + 1 }));
      } catch {
        if (!request.signal.aborted) {
          setFailed(true);
          setState(current => current.history.kind === "ready" ? current : { ...current, history: { kind: "unavailable" } });
        }
      } finally {
        if (!request.signal.aborted) {
          setPending(false);
          recordWorkspaceDataLoad({
            type: "data",
            target: "conversation",
            source: "network",
            durationMs: performance.now() - requestedAt,
          });
        }
      }
    };

    void load();

    return () => request.abort();
  }, [cache, conversationId, hasCachedReadyHistory, requestRevision]);

  const retryHistoryLoad = () => {
    setFailed(false);
    setPending(true);
    setState((current) => current.history.kind === "ready" ? current : ({ ...current, history: { kind: "loading" } }));
    setRequestRevision((current) => current + 1);
  };

  return (
    <>
      <SessionProgressBar
        label={t.conversations.historyProgressLabel}
        visible={isColdLoading && isProgressVisible}
      />
      <ConversationHistory
        notice={<LoadFeedback pending={pending} failed={failed && state.history.kind === "ready"} hasData={state.history.kind === "ready"} onRetry={retryHistoryLoad} />}
        key={conversationId}
        conversationId={conversationId}
        history={state.history}
        initialRefresh={initialRefresh}
        onRetry={retryHistoryLoad}
      />
    </>
  );
}
