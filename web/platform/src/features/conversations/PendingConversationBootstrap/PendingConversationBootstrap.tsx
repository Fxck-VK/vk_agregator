"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";

import { pendingImagePreview, savePendingMediaJob } from "../pending-media-job";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "@/i18n/navigation";

import { PendingGenerationIndicator } from "../PendingGenerationIndicator";
import { ConversationInputImages } from "@/features/conversations/ConversationInputImages/ConversationInputImages";
import {
  clearPendingConversationBootstrap,
  readPendingConversationBootstrap,
  updatePendingConversationBootstrap,
  type PendingConversationBootstrapIntent,
} from "@/features/conversations/pending-conversation-bootstrap";
import { savePendingConversationPrompt } from "@/features/conversations/pending-conversation-prompt";
import { fallbackConversationTitle, savePendingConversationTitleSync } from "@/features/conversations/pending-conversation-title-sync";
import { useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { isSafeWebChatAcceptedResponse, parseConversationItem, parseWebChatJob } from "@/lib/web-api/contracts";

import styles from "@/features/conversations/ConversationHistory/ConversationHistory.module.css";

type PendingConversationBootstrapProps = {
  conversationKey: string;
};

type BootstrapStatus = "active" | "failed" | "missing";

export function PendingConversationBootstrap({ conversationKey }: PendingConversationBootstrapProps) {
  const t = useDictionary();
  const router = useRouter();
  const { resolvePendingConversation, upsertConversation } = useWorkspaceConversationList();
  const [intent] = useState<PendingConversationBootstrapIntent | null>(() => readPendingConversationBootstrap(conversationKey));
  const [status, setStatus] = useState<BootstrapStatus>(intent === null ? "missing" : "active");
  const [attempt, setAttempt] = useState(0);
  const startedAttemptRef = useRef<number | null>(null);

  const run = useCallback(async () => {
    const currentIntent = readPendingConversationBootstrap(conversationKey);
    if (currentIntent === null) {
      setStatus("missing");
      return;
    }

    try {
      let conversationID = currentIntent.conversationId;
      if (conversationID === undefined) {
        const conversationResponse = await webBrowserMutation("/web/v1/conversations", {
          method: "POST",
          headers: { "X-Idempotency-Key": currentIntent.conversationKey },
        });
        if (conversationResponse.status !== 200 && conversationResponse.status !== 201) {
          throw new Error("Unable to create conversation.");
        }

        const conversation = parseConversationItem(await conversationResponse.json());
        conversationID = conversation.id;
        updatePendingConversationBootstrap(conversationKey, { conversationId: conversationID });

        const fallbackTitle = fallbackConversationTitle(currentIntent.prompt);
        const hasServerTitle = conversation.title.trim() !== "";
        const visibleConversation = hasServerTitle || fallbackTitle === ""
          ? conversation
          : { ...conversation, title: fallbackTitle };
        resolvePendingConversation(conversationKey, visibleConversation);
        if (!hasServerTitle && fallbackTitle !== "") {
          savePendingConversationTitleSync(conversationID, fallbackTitle);
        }
      }

      const messageResponse = await webBrowserMutation(`/web/v1/conversations/${conversationID}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": currentIntent.messageKey,
        },
        body: JSON.stringify({ prompt: currentIntent.prompt, ...currentIntent.generationOptions, ...(currentIntent.modelId ? { model_id: currentIntent.modelId } : {}) }),
      });
      if (messageResponse.status !== 200 && messageResponse.status !== 201) {
        throw new Error("Unable to send first message.");
      }
      const job = parseWebChatJob(await messageResponse.json());
      if (!isSafeWebChatAcceptedResponse(messageResponse.status, job)) {
        throw new Error("Unsafe first message response.");
      }

      if (currentIntent.generationOptions && Object.keys(currentIntent.generationOptions).length > 0) savePendingMediaJob(conversationID, job.job_id, 0, currentIntent.generationOptions);
      savePendingConversationPrompt(conversationID, currentIntent.prompt);
      if (currentIntent.modelId) {
        try {
          window.sessionStorage.setItem(`neirohub:conversation-model:${conversationID}`, currentIntent.modelId);
        } catch {
          // The accepted message must remain successful when storage is unavailable.
        }
      }
      clearPendingConversationBootstrap(conversationKey);
      router.replace(`/app/chat/${conversationID}?refresh=1`);
    } catch {
      setStatus("failed");
    }
  }, [conversationKey, resolvePendingConversation, router]);

  useEffect(() => {
    if (intent === null) {
      return;
    }

    const createdAt = new Date().toISOString();
    upsertConversation({
      id: conversationKey,
      title: fallbackConversationTitle(intent.prompt),
      created_at: createdAt,
      updated_at: createdAt,
      isPending: true,
    });
  }, [conversationKey, intent, upsertConversation]);

  useEffect(() => {
    if (intent === null || status !== "active" || startedAttemptRef.current === attempt) {
      return;
    }
    startedAttemptRef.current = attempt;
    void run();
  }, [attempt, intent, run, status]);

  const retry = () => {
    setStatus("active");
    setAttempt((currentAttempt) => currentAttempt + 1);
  };

  if (intent === null || status === "missing") {
    return (
      <section aria-label={t.conversations.historyTitle} className={`${styles.content} ${styles.state}`}>
        <StateNotice inline kind="empty">{t.conversations.historyUnavailable}</StateNotice>
      </section>
    );
  }

  return (
    <section aria-label={t.conversations.historyTitle} className={styles.content}>
      <div className={styles.history}>
        <ol className={styles.messages}>
          <li className={styles.userMessage} data-chat-pending="user">
            <ConversationInputImages ids={intent.generationOptions?.reference_artifact_ids} />
            <p>{intent.prompt}</p>
            {status === "failed" ? (
              <StateNotice inline kind="error" action={{ label: t.conversations.messageRetryLabel, onClick: retry }}>{t.conversations.messageNotSent}</StateNotice>
            ) : null}
          </li>
          {status === "active" ? (
            <li className={styles.assistantMessage} data-chat-pending="assistant">
              <PendingGenerationIndicator image={pendingImagePreview(intent.generationOptions)} />
            </li>
          ) : null}
        </ol>
      </div>
    </section>
  );
}
