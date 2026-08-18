"use client";

import { useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { Button } from "@/components/ui/Button/Button";
import { savePendingConversationBootstrap } from "@/features/conversations/pending-conversation-bootstrap";
import { fallbackConversationTitle } from "@/features/conversations/pending-conversation-title-sync";
import { useOptionalWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";

import styles from "./WorkspacePrompt.module.css";

type WorkspacePromptProps = {
  access?: "authenticated" | "guest";
  variant?: "workspace" | "newChat" | "hero";
};

export function WorkspacePrompt({ access = "authenticated", variant = "workspace" }: WorkspacePromptProps) {
  const router = useRouter();
  const conversationList = useOptionalWorkspaceConversationList();
  const submissionStartedRef = useRef(false);
  const [prompt, setPrompt] = useState("");
  const canSubmit = prompt.trim() !== "";
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
    setPrompt(event.target.value);
  };

  const submit = () => {
    const normalizedPrompt = prompt.trim();
    if (normalizedPrompt === "" || submissionStartedRef.current) {
      return;
    }

    if (access === "guest") {
      router.push("/login");
      return;
    }

    if (conversationList === undefined) {
      throw new Error("Authenticated WorkspacePrompt requires WorkspaceConversationListProvider.");
    }

    submissionStartedRef.current = true;

    const conversationKey = crypto.randomUUID();
    const messageKey = crypto.randomUUID();
    const createdAt = new Date().toISOString();
    const fallbackTitle = fallbackConversationTitle(normalizedPrompt);

    savePendingConversationBootstrap({ conversationKey, messageKey, prompt: normalizedPrompt });
    conversationList.upsertConversation({
      id: conversationKey,
      title: fallbackTitle,
      created_at: createdAt,
      updated_at: createdAt,
      isPending: true,
    });
    setPrompt("");
    router.push(`/app/chat/${conversationKey}?pending=1`);
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
          disabled={false}
          onChange={changePrompt}
          onSend={submit}
          placeholder={promptPlaceholder}
          rows={isNewChat || isHero ? 4 : 5}
          size="expanded"
          value={prompt}
        />
      </label>
      <div className={styles.actions}>
        <Button disabled={!canSubmit} type="submit">
          {ru.workspace.promptSubmit}
        </Button>
        {isNewChat || isHero ? null : <p>{ru.workspace.promptSupport}</p>}
      </div>
    </form>
  );
}
