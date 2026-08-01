"use client";

import { useRef, useState, type ChangeEvent, type FormEvent } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { isSafeWebChatAcceptedResponse, parseWebChatJob } from "@/lib/web-api/contracts";

import styles from "./ConversationComposer.module.css";

type ConversationComposerProps = {
  conversationId: string;
  disabled?: boolean;
  onAccepted: () => void;
};

type RetryIntent = {
  prompt: string;
  idempotencyKey: string;
};

type ComposerFeedback = "accepted" | "error" | null;

export function ConversationComposer({ conversationId, disabled = false, onAccepted }: ConversationComposerProps) {
  const [draft, setDraft] = useState("");
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

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
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
      setFeedback("accepted");
      onAccepted();
    } catch {
      setFeedback("error");
    } finally {
      isSubmittingRef.current = false;
      setIsPending(false);
    }
  };

  return (
    <form className={styles.dock} onSubmit={(event) => void submit(event)}>
      <div className={styles.composer}>
        <label className={styles.field}>
          <span>{ru.conversations.composerLabel}</span>
          <textarea
            disabled={isPending || disabled}
            onChange={changeDraft}
            placeholder={ru.conversations.composerPlaceholder}
            rows={3}
            value={draft}
          />
        </label>
        <div className={styles.footer}>
          <div className={styles.feedback}>
            {feedback === "accepted" ? <p role="status">{ru.conversations.composerAccepted}</p> : null}
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
