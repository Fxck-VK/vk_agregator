"use client";

import { useRef, useState, type ChangeEvent, type FormEvent } from "react";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { ChatScrollToBottom } from "@/components/chat/ChatScrollToBottom/ChatScrollToBottom";
import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { isSafeWebChatAcceptedResponse, parseWebChatJob } from "@/lib/web-api/contracts";

import styles from "./ConversationComposer.module.css";

type ConversationComposerProps = {
  conversationId: string;
  contentVersion: string;
  disabled?: boolean;
  forceScrollRequest: number;
  initialDraft?: string;
  onAccepted: (prompt: string) => void;
  scrollContainer: HTMLElement | null;
};

type RetryIntent = {
  prompt: string;
  idempotencyKey: string;
};

type ComposerFeedback = "error" | null;

export function ConversationComposer({
  conversationId,
  contentVersion,
  disabled = false,
  forceScrollRequest,
  initialDraft = "",
  onAccepted,
  scrollContainer,
}: ConversationComposerProps) {
  const [draft, setDraft] = useState(initialDraft);
  const [isPending, setIsPending] = useState(false);
  const [feedback, setFeedback] = useState<ComposerFeedback>(null);
  const retryIntentRef = useRef<RetryIntent | null>(null);
  const isSubmittingRef = useRef(false);
  const normalizedDraft = draft.trim();
  const canSubmit = normalizedDraft !== "" && !isPending && !disabled;

  const changeDraft = (event: ChangeEvent<HTMLTextAreaElement>) => {
    const nextDraft = event.target.value;
    const retryIntent = retryIntentRef.current;
    if (retryIntent !== null && retryIntent.prompt !== nextDraft.trim()) {
      retryIntentRef.current = null;
    }
    setDraft(nextDraft);
    setFeedback(null);
  };

  const submit = async () => {
    if (disabled || isSubmittingRef.current || normalizedDraft === "") {
      return;
    }

    const retryIntent = retryIntentRef.current;
    const intent = retryIntent?.prompt === normalizedDraft
      ? retryIntent
      : { prompt: normalizedDraft, idempotencyKey: crypto.randomUUID() };
    retryIntentRef.current = intent;
    isSubmittingRef.current = true;
    setIsPending(true);
    setFeedback(null);

    try {
      const response = await webBrowserMutation(`/web/v1/conversations/${conversationId}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": intent.idempotencyKey,
        },
        body: JSON.stringify({ prompt: intent.prompt }),
      });
      if (response.status !== 200 && response.status !== 201) {
        throw new Error("Unable to complete the request.");
      }
      const job = parseWebChatJob(await response.json());
      if (!isSafeWebChatAcceptedResponse(response.status, job)) {
        throw new Error("Unable to complete the request.");
      }

      retryIntentRef.current = null;
      setDraft("");
      onAccepted(intent.prompt);
    } catch {
      setFeedback("error");
    } finally {
      isSubmittingRef.current = false;
      setIsPending(false);
    }
  };

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submit();
  };

  return (
    <form className={styles.dock} onSubmit={submitForm}>
      <ChatScrollToBottom
        contentVersion={contentVersion}
        forceScrollRequest={forceScrollRequest}
        scrollContainer={scrollContainer}
      />
      <div className={styles.composer}>
        <label className={styles.field}>
          <span>{ru.conversations.composerLabel}</span>
          <ChatTextInput
            appearance="inset"
            disabled={isPending || disabled}
            onChange={changeDraft}
            onSend={() => void submit()}
            placeholder={ru.conversations.composerPlaceholder}
            rows={3}
            size="compact"
            value={draft}
          />
        </label>
        <div className={styles.footer}>
          <div className={styles.feedback}>
            {feedback === "error" ? <p role="alert">{ru.conversations.composerFailure}</p> : null}
          </div>
          <Button disabled={!canSubmit} type="submit">
            {isPending ? ru.conversations.composerPending : ru.conversations.composerSubmit}
          </Button>
        </div>
      </div>
    </form>
  );
}
