"use client";

import { useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { Button } from "@/components/ui/Button/Button";
import { savePendingConversationPrompt } from "@/features/conversations/pending-conversation-prompt";
import { fallbackConversationTitle, savePendingConversationTitleSync } from "@/features/conversations/pending-conversation-title-sync";
import { useOptionalWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { isSafeWebChatAcceptedResponse, parseConversationItem, parseWebChatJob } from "@/lib/web-api/contracts";

import styles from "./WorkspacePrompt.module.css";

type RetryIntent = {
  prompt: string;
  conversationKey: string;
  messageKey: string;
  conversationId?: string;
};

type WorkspacePromptProps = {
  access?: "authenticated" | "guest";
  variant?: "workspace" | "newChat" | "hero";
};

export function WorkspacePrompt({ access = "authenticated", variant = "workspace" }: WorkspacePromptProps) {
  const router = useRouter();
  const conversationList = useOptionalWorkspaceConversationList();
  const [prompt, setPrompt] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);
  const retryIntentRef = useRef<RetryIntent | null>(null);
  const canSubmit = prompt.trim() !== "" && !isPending;
  const isNewChat = variant === "newChat";
  const isHero = variant === "hero";
  const promptLabel = isHero
    ? "Задайте вопрос NeiroHub"
    : isNewChat
      ? ru.conversations.composerPlaceholder
      : ru.workspace.promptLabel;
  const promptPlaceholder = isHero
    ? "Напиши свой вопрос, и я помогу тебе"
    : isNewChat
      ? ru.conversations.composerPlaceholder
      : ru.workspace.promptPlaceholder;

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

    if (access === "guest") {
      router.push("/login");
      return;
    }

    if (conversationList === undefined) {
      throw new Error("Authenticated WorkspacePrompt requires WorkspaceConversationListProvider.");
    }

    setHasError(false);
    setIsPending(true);
    let intent: RetryIntent | null = null;
    try {
      const retryIntent = retryIntentRef.current;
      intent = retryIntent?.prompt === normalizedPrompt
        ? retryIntent
        : {
            prompt: normalizedPrompt,
            conversationKey: crypto.randomUUID(),
            messageKey: crypto.randomUUID(),
          };
      retryIntentRef.current = intent;

      if (intent.conversationId === undefined) {
        const fallbackTitle = fallbackConversationTitle(intent.prompt);
        const createdAt = new Date().toISOString();
        conversationList.upsertConversation({
          id: intent.conversationKey,
          title: fallbackTitle,
          created_at: createdAt,
          updated_at: createdAt,
          isPending: true,
        });
        const conversationResponse = await webBrowserMutation("/web/v1/conversations", {
          method: "POST",
          headers: { "X-Idempotency-Key": intent.conversationKey },
        });
        if (conversationResponse.status !== 200 && conversationResponse.status !== 201) {
          throw new Error("Unable to complete the request.");
        }
        const conversation = parseConversationItem(await conversationResponse.json());
        intent.conversationId = conversation.id;
        const hasServerTitle = conversation.title.trim() !== "";
        const visibleConversation = hasServerTitle || fallbackTitle === "" ? conversation : { ...conversation, title: fallbackTitle };
        conversationList.resolvePendingConversation(intent.conversationKey, visibleConversation);
        if (!hasServerTitle && fallbackTitle !== "") {
          savePendingConversationTitleSync(conversation.id, fallbackTitle);
        }
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
      if (intent !== null && intent.conversationId === undefined) {
        conversationList.discardPendingConversation(intent.conversationKey);
      }
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
    <form
      className={`${styles.form} ${isNewChat ? styles.newChatForm : ""} ${isHero ? styles.heroForm : ""}`}
      onSubmit={submitForm}
    >
      <label className={styles.promptField}>
        <span>{promptLabel}</span>
        <ChatTextInput
          appearance="plain"
          disabled={isPending}
          onChange={changePrompt}
          onSend={() => void submit()}
          placeholder={promptPlaceholder}
          rows={isNewChat || isHero ? 4 : 5}
          size="expanded"
          value={prompt}
        />
      </label>
      <div className={styles.actions}>
        <Button disabled={!canSubmit} type="submit">
          {isPending ? ru.workspace.promptPending : ru.workspace.promptSubmit}
        </Button>
        {isNewChat || isHero ? null : <p>{ru.workspace.promptSupport}</p>}
      </div>
      {hasError ? (
        <p className={styles.error} role="alert">
          {ru.workspace.promptFailure}
        </p>
      ) : null}
    </form>
  );
}
