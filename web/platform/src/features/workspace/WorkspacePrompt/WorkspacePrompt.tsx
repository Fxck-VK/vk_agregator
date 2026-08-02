"use client";

import { useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { Button } from "@/components/ui/Button/Button";
import { savePendingConversationPrompt } from "@/features/conversations/pending-conversation-prompt";
import { useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { isSafeWebChatAcceptedResponse, parseConversationList, parseWebChatJob } from "@/lib/web-api/contracts";

import styles from "./WorkspacePrompt.module.css";

type RetryIntent = {
  prompt: string;
  conversationKey: string;
  messageKey: string;
  conversationId?: string;
};

export function WorkspacePrompt() {
  const router = useRouter();
  const { upsertConversation } = useWorkspaceConversationList();
  const [prompt, setPrompt] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);
  const retryIntentRef = useRef<RetryIntent | null>(null);
  const canSubmit = prompt.trim() !== "" && !isPending;

  const changePrompt = (event: ChangeEvent<HTMLTextAreaElement>) => {
    const nextPrompt = event.target.value;
    const retryIntent = retryIntentRef.current;
    if (retryIntent !== null && retryIntent.prompt !== nextPrompt.trim()) {
      retryIntentRef.current = null;
    }
    setPrompt(nextPrompt);
    setHasError(false);
  };

  const submit = async () => {
    const normalizedPrompt = prompt.trim();
    if (isPending || normalizedPrompt === "") {
      return;
    }

    setHasError(false);
    setIsPending(true);
    try {
      const retryIntent = retryIntentRef.current;
      const intent = retryIntent?.prompt === normalizedPrompt
        ? retryIntent
        : {
            prompt: normalizedPrompt,
            conversationKey: crypto.randomUUID(),
            messageKey: crypto.randomUUID(),
          };
      retryIntentRef.current = intent;

      if (intent.conversationId === undefined) {
        const conversationResponse = await webBrowserMutation("/web/v1/conversations", {
          method: "POST",
          headers: { "X-Idempotency-Key": intent.conversationKey },
        });
        if (conversationResponse.status !== 200 && conversationResponse.status !== 201) {
          throw new Error("Unable to complete the request.");
        }
        const conversation = parseConversationList({ items: [await conversationResponse.json()] }).items[0];
        if (conversation === undefined) {
          throw new Error("Unable to complete the request.");
        }
        intent.conversationId = conversation.id;
        upsertConversation(conversation);
      }

      const conversationID = intent.conversationId;
      if (conversationID === undefined) {
        throw new Error("Unable to complete the request.");
      }

      const messageResponse = await webBrowserMutation(`/web/v1/conversations/${conversationID}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": intent.messageKey,
        },
        body: JSON.stringify({ prompt: intent.prompt }),
      });
      if (messageResponse.status !== 200 && messageResponse.status !== 201) {
        throw new Error("Unable to complete the request.");
      }
      const job = parseWebChatJob(await messageResponse.json());
      if (!isSafeWebChatAcceptedResponse(messageResponse.status, job)) {
        throw new Error("Unable to complete the request.");
      }
      retryIntentRef.current = null;
      savePendingConversationPrompt(conversationID, intent.prompt);
      router.push(`/app/chat/${conversationID}?refresh=1`);
    } catch {
      setHasError(true);
    } finally {
      setIsPending(false);
    }
  };

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submit();
  };

  return (
    <form className={styles.form} onSubmit={submitForm}>
      <label className={styles.promptField}>
        <span>{ru.workspace.promptLabel}</span>
        <ChatTextInput
          appearance="plain"
          disabled={isPending}
          onChange={changePrompt}
          onSend={() => void submit()}
          placeholder={ru.workspace.promptPlaceholder}
          rows={5}
          size="expanded"
          value={prompt}
        />
      </label>
      <div className={styles.actions}>
        <Button disabled={!canSubmit} type="submit">
          {isPending ? ru.workspace.promptPending : ru.workspace.promptSubmit}
        </Button>
        <p>{ru.workspace.promptSupport}</p>
      </div>
      {hasError ? (
        <p className={styles.error} role="alert">
          {ru.workspace.promptFailure}
        </p>
      ) : null}
    </form>
  );
}
