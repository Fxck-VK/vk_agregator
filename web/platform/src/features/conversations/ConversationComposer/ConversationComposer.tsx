"use client";

import { useState, type ChangeEvent, type FormEvent, type ReactNode } from "react";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { ChatScrollToBottom } from "@/components/chat/ChatScrollToBottom/ChatScrollToBottom";
import { ru } from "@/i18n/ru";
import { useGenerationControls, type GenerationOptions } from "@/features/models/generation-options";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import type { ChatModel } from "@/lib/web-api/contracts";

import styles from "./ConversationComposer.module.css";

type ConversationComposerProps = {
  contentVersion: string;
  disabled?: boolean;
  forceScrollRequest: number;
  initialDraft?: string;
  isAwaitingResponse?: boolean;
  modelSelector?: ReactNode;
  selectedModel?: ChatModel;
  generationModel?: GenerationModel;
  onSubmit: (prompt: string, options?: GenerationOptions) => void;
  scrollContainer: HTMLElement | null;
};

export function ConversationComposer({
  contentVersion,
  disabled = false,
  forceScrollRequest,
  initialDraft = "",
  isAwaitingResponse = false,
  modelSelector,
  selectedModel,
  generationModel,
  onSubmit,
  scrollContainer,
}: ConversationComposerProps) {
  const [draft, setDraft] = useState(initialDraft);
  const generation = useGenerationControls(generationModel, disabled, setDraft);
  const normalizedDraft = draft.trim();
  const tooLong = selectedModel?.max_prompt_bytes !== undefined
    && new TextEncoder().encode(normalizedDraft).length > selectedModel.max_prompt_bytes;
  const canSubmit = normalizedDraft !== "" && !disabled && !tooLong && generation.canSubmit;

  const changeDraft = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setDraft(event.target.value);
  };

  const submit = () => {
    if (!canSubmit) {
      return;
    }

    const prompt = normalizedDraft;
    setDraft("");
    if (generationModel && generationModel.category !== "text") onSubmit(prompt, generation.options);
    else onSubmit(prompt);
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
        additionalControls={modelSelector}
        leadingControls={generation.controls}
        wrapLeadingControls
        attachmentsEnabled={false}
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
        note={generationModel && generationModel.category !== "text"
          ? `Стоимость: ${generation.cost ?? "—"} токенов`
          : (selectedModel?.estimate_credits ?? 0) > 0
          ? `${selectedModel!.estimate_credits} токенов за ответ · до ${selectedModel!.max_output_tokens} токенов ответа. При длинном диалоге может потребоваться новый чат.`
          : ru.conversations.composerDisclaimer}
        onChange={changeDraft}
        onSend={submit}
        placeholder={ru.conversations.composerPlaceholder}
        submitLabel={ru.conversations.composerSubmit}
        value={draft}
        variant="conversation"
      />
      {tooLong ? <p role="alert">Сообщение слишком длинное для выбранной модели.</p> : null}
    </form>
  );
}
