"use client";

import { useState, type ChangeEvent, type FormEvent } from "react";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { ChatScrollToBottom } from "@/components/chat/ChatScrollToBottom/ChatScrollToBottom";
import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";

import styles from "./ConversationComposer.module.css";

type ConversationComposerProps = {
  contentVersion: string;
  disabled?: boolean;
  forceScrollRequest: number;
  initialDraft?: string;
  onSubmit: (prompt: string) => void;
  scrollContainer: HTMLElement | null;
};

export function ConversationComposer({
  contentVersion,
  disabled = false,
  forceScrollRequest,
  initialDraft = "",
  onSubmit,
  scrollContainer,
}: ConversationComposerProps) {
  const [draft, setDraft] = useState(initialDraft);
  const normalizedDraft = draft.trim();
  const canSubmit = normalizedDraft !== "" && !disabled;

  const changeDraft = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setDraft(event.target.value);
  };

  const submit = () => {
    if (disabled || normalizedDraft === "") {
      return;
    }

    const prompt = normalizedDraft;
    setDraft("");
    onSubmit(prompt);
  };

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    submit();
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
            disabled={disabled}
            onChange={changeDraft}
            onSend={submit}
            placeholder={ru.conversations.composerPlaceholder}
            rows={3}
            size="compact"
            value={draft}
          />
        </label>
        <div className={styles.footer}>
          <div className={styles.feedback} />
          <Button disabled={!canSubmit} type="submit">
            {ru.conversations.composerSubmit}
          </Button>
        </div>
      </div>
    </form>
  );
}
