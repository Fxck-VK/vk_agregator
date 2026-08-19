"use client";

import { useState, type ChangeEvent, type FormEvent } from "react";

import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
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
      <div className={styles.composer}>
        <label className={styles.field}>
          <span>{ru.conversations.composerLabel}</span>
          <ChatTextInput
            appearance="composer"
            disabled={disabled}
            onChange={changeDraft}
            onSend={submit}
            placeholder={ru.conversations.composerPlaceholder}
            rows={2}
            size="compact"
            value={draft}
          />
        </label>
        <div className={styles.controls}>
          <button
            aria-label={ru.conversations.composerMediaUpload}
            className={styles.media}
            disabled
            title={ru.conversations.composerMediaUploadUnavailable}
            type="button"
          >
            <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
              <path d="M4 7.5A2.5 2.5 0 0 1 6.5 5H10l1.5-2h5L18 5h.5A2.5 2.5 0 0 1 21 7.5v10a2.5 2.5 0 0 1-2.5 2.5h-12A2.5 2.5 0 0 1 4 17.5z" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.7" />
              <path d="m8 15 2.5-2.5 2 2L15 12l3 3" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.7" />
              <path d="M8 3v4M6 5h4" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
            </svg>
            <span>{ru.conversations.composerMediaUpload}</span>
          </button>
          <button
            aria-label={ru.conversations.composerSubmit}
            className={styles.submit}
            disabled={!canSubmit}
            title={ru.conversations.composerSubmit}
            type="submit"
          >
            <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
              <path d="M12 19V5m0 0-6 6m6-6 6 6" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
            </svg>
          </button>
        </div>
      </div>
      <p className={styles.disclaimer}>{ru.conversations.composerDisclaimer}</p>
    </form>
  );
}
