"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { tokenAmount } from "@/i18n/counts";
import { RichMessage } from "@/i18n/RichMessage";


import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import { useState, type ChangeEvent, type FormEvent, type ReactNode } from "react";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { ChatScrollToBottom } from "@/components/chat/ChatScrollToBottom/ChatScrollToBottom";
import { ReferenceQuoteNotice } from "@/components/chat/ReferenceQuoteNotice/ReferenceQuoteNotice";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { useGenerationControls, type GenerationOptions } from "@/features/models/generation-options";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import type { ChatModel } from "@/lib/web-api/contracts";

import { useReferenceQuote } from "@/features/conversations/use-reference-quote";
import { useChatAttachments } from "@/features/conversations/use-chat-attachments";

import styles from "./ConversationComposer.module.css";

type ConversationComposerProps = {
  contentVersion: string;
  disabled?: boolean;
  submitDisabled?: boolean;
  forceScrollRequest: number;
  initialDraft?: string;
  isAwaitingResponse?: boolean;
  modelSelector?: ReactNode;
  selectedModel?: ChatModel;
  generationModel?: GenerationModel;
  onSubmit: (prompt: string, options?: GenerationOptions) => void | boolean | Promise<void | boolean>;
  scrollContainer: HTMLElement | null;
};

export function ConversationComposer({
  contentVersion,
  disabled = false,
  submitDisabled = false,
  forceScrollRequest,
  initialDraft = "",
  isAwaitingResponse = false,
  modelSelector,
  selectedModel,
  generationModel,
  onSubmit,
  scrollContainer,
}: ConversationComposerProps) {
  const msg = useMessages();
  const t = useDictionary();
  const [draft, setDraft] = useState(initialDraft);
  const attachments = useChatAttachments(generationModel);
  const generation = useGenerationControls(generationModel, disabled, setDraft);
  const referenceQuote = useReferenceQuote(generationModel, generation.options, {
    count: attachments.items.length, ready: !attachments.blocked, artifactIds: attachments.ids,
  });
  const cost = referenceQuote.active ? referenceQuote.cost : generation.cost;
  const normalizedDraft = draft.trim();
  const tooLong = selectedModel?.max_prompt_bytes !== undefined
    && new TextEncoder().encode(normalizedDraft).length > selectedModel.max_prompt_bytes;
  const canSubmit = normalizedDraft !== "" && !submitDisabled && !disabled && !tooLong && generation.canSubmit && !attachments.blocked && referenceQuote.ready;

  const changeDraft = (event: ChangeEvent<HTMLTextAreaElement>) => {
    setDraft(event.target.value);
  };

  const submit = async () => {
    if (!canSubmit) {
      return;
    }

    const prompt = normalizedDraft;
    const accepted = generationModel && generationModel.category !== "text"
      ? await onSubmit(prompt, { ...generation.options, ...(attachments.ids.length ? { reference_artifact_ids: attachments.ids } : {}) })
      : await onSubmit(prompt);
    if (accepted !== false) { setDraft(""); attachments.clear(); }
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
        attachmentController={attachments}
        attachmentsEnabled={attachments.enabled}
        canSubmit={canSubmit}
        disabled={disabled}
        label={t.conversations.composerLabel}
        mediaLabel={t.conversations.composerMediaUpload}
        mediaMenuLabels={{
          chooseGenerated: t.conversations.composerMediaChooseGenerated,
          chooseUploaded: t.conversations.composerMediaChooseUploaded,
          menu: t.conversations.composerMediaMenu,
          uploadFile: t.conversations.composerMediaUploadFile,
        }}
        note={generationModel && generationModel.category !== "text"
          ? cost === undefined ? undefined : <CreditAmount prefix={`${t.imageGeneration.priceLabel}:`} value={cost} />
          : (selectedModel?.estimate_credits ?? 0) > 0
          ? <RichMessage id="conversationComposer.valueTokensPerResponseUpToValue" values={{ value1: <CreditAmount value={selectedModel!.estimate_credits!} />, value2: tokenAmount(msg, selectedModel!.max_output_tokens, "outputTokens") }} />
          : t.conversations.composerDisclaimer}
        onChange={changeDraft}
        onSend={submit}
        placeholder={t.conversations.composerPlaceholder}
        submitLabel={t.conversations.composerSubmit}
        value={draft}
        variant="conversation"
      />
      <ReferenceQuoteNotice message={referenceQuote.error} onRetry={referenceQuote.retry} />
      {tooLong ? <StateNotice inline kind="error">{msg("conversationComposer.theMessageIsTooLongForThe")}</StateNotice> : null}
    </form>
  );
}
