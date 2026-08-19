"use client";

import { useState, type ChangeEvent, type FormEvent } from "react";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { ChatScrollToBottom } from "@/components/chat/ChatScrollToBottom/ChatScrollToBottom";
import { ru } from "@/i18n/ru";

import styles from "./ConversationComposer.module.css";

type ConversationComposerProps = {
  contentVersion: string;
  disabled?: boolean;
  forceScrollRequest: number;
  initialDraft?: string;
  isAwaitingResponse?: boolean;
  onSubmit: (prompt: string) => void;
  scrollContainer: HTMLElement | null;
};

export function ConversationComposer({
  contentVersion,
  disabled = false,
  forceScrollRequest,
  initialDraft = "",
  isAwaitingResponse = false,
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
        isAwaitingResponse={isAwaitingResponse}
        scrollContainer={scrollContainer}
      />
      <ChatComposer
        canSubmit={canSubmit}
        disabled={disabled}
        label={ru.conversations.composerLabel}
        mediaLabel={ru.conversations.composerMediaUpload}
        mediaMenuLabels={{
          chooseGenerated: ru.conversations.composerMediaChooseGenerated,
          chooseUploaded: ru.conversations.composerMediaChooseUploaded,
          menu: ru.conversations.composerMediaMenu,
          uploadFile: ru.conversations.composerMediaUploadFile,
        }}
        note={ru.conversations.composerDisclaimer}
        onChange={changeDraft}
        onSend={submit}
        placeholder={ru.conversations.composerPlaceholder}
        submitLabel={ru.conversations.composerSubmit}
        value={draft}
        variant="conversation"
      />
    </form>
  );
}
