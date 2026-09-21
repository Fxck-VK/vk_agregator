"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { RichMessage } from "@/i18n/RichMessage";

import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import type { Translator } from "@/i18n/messages";

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from "react";
import { useRouter } from "@/i18n/navigation";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { ReferenceQuoteNotice } from "@/components/chat/ReferenceQuoteNotice/ReferenceQuoteNotice";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { savePendingConversationBootstrap } from "@/features/conversations/pending-conversation-bootstrap";
import { fallbackConversationTitle } from "@/features/conversations/pending-conversation-title-sync";
import { useOptionalWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { useGenerationControls } from "@/features/models/generation-options";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import type { ChatModel } from "@/lib/web-api/contracts";

import { useReferenceQuote } from "@/features/conversations/use-reference-quote";
import { useChatAttachments } from "@/features/conversations/use-chat-attachments";

import styles from "./WorkspacePrompt.module.css";

type WorkspacePromptProps = {
  selectedChatModel?: ChatModel;
  selectedGenerationModel?: GenerationModel;
  chatModelUnavailable?: boolean;
  access?: "authenticated" | "guest";
  promptValue?: string;
  onPromptChange?: (prompt: string) => void;
  leadingControls?: ReactNode;
  submitAction?: {
    canSubmit: boolean;
    disabled: boolean;
    label: string;
    onSubmit: () => void;
  };
  variant?: "workspace" | "newChat" | "hero";
};

function normalizeChatModel(model: ChatModel): Extract<GenerationModel, { category: "text" }> {
  return { ...model, operations: undefined, category: "text" as const };
}

function getHeroPlaceholderCopy(msg: Translator) {
  const heroPlaceholderPrefix = msg("workspacePrompt.askNeirohubOr");
  const heroPlaceholderSuggestions = [
    msg("workspacePrompt.planAProject"),
    msg("workspacePrompt.comeUpWithAPostIdea"),
    msg("workspacePrompt.explainAComplexTopic"),
    msg("workspacePrompt.helpWriteAText"),
  ] as const;

  return { heroPlaceholderPrefix, heroPlaceholderSuggestions };
}

type TypewriterPhase = "typing" | "holding" | "deleting" | "waiting";

type TypewriterState = {
  phase: TypewriterPhase;
  suggestionIndex: number;
  visibleCharacters: number;
};

const initialTypewriterState: TypewriterState = {
  phase: "typing",
  suggestionIndex: 0,
  visibleCharacters: 0,
};

const typewriterDelays: Record<TypewriterPhase, number> = {
  typing: 80,
  holding: 1_600,
  deleting: 45,
  waiting: 350,
};

function advanceTypewriter(state: TypewriterState, heroPlaceholderSuggestions: readonly string[]): TypewriterState {
  const suggestion = heroPlaceholderSuggestions[state.suggestionIndex];

  if (state.phase === "typing") {
    const visibleCharacters = Math.min(state.visibleCharacters + 1, suggestion.length);
    return {
      ...state,
      phase: visibleCharacters === suggestion.length ? "holding" : "typing",
      visibleCharacters,
    };
  }

  if (state.phase === "holding") {
    return { ...state, phase: "deleting" };
  }

  if (state.phase === "deleting") {
    if (state.visibleCharacters > 1) {
      return { ...state, visibleCharacters: state.visibleCharacters - 1 };
    }

    return {
      phase: "waiting",
      suggestionIndex: (state.suggestionIndex + 1) % heroPlaceholderSuggestions.length,
      visibleCharacters: 0,
    };
  }

  return { ...state, phase: "typing" };
}

function useHeroPlaceholder(enabled: boolean) {
  const msg = useMessages();
  const { heroPlaceholderPrefix, heroPlaceholderSuggestions } = useMemo(() => getHeroPlaceholderCopy(msg), [msg]);
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);
  const [typewriter, setTypewriter] = useState<TypewriterState>(initialTypewriterState);
  const reset = useCallback(() => setTypewriter(initialTypewriterState), []);

  useEffect(() => {
    const mediaQuery = window.matchMedia?.("(prefers-reduced-motion: reduce)");
    if (mediaQuery === undefined) {
      return;
    }

    const updatePreference = () => setPrefersReducedMotion(mediaQuery.matches);
    updatePreference();
    mediaQuery.addEventListener?.("change", updatePreference);
    return () => mediaQuery.removeEventListener?.("change", updatePreference);
  }, []);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    if (prefersReducedMotion) {
      return;
    }

    const timeout = window.setTimeout(
      () => setTypewriter((current) => advanceTypewriter(current, heroPlaceholderSuggestions)),
      typewriterDelays[typewriter.phase],
    );
    return () => window.clearTimeout(timeout);
  }, [enabled, prefersReducedMotion, typewriter, heroPlaceholderSuggestions]);

  if (!enabled) {
    return { placeholder: "", reset };
  }
  if (prefersReducedMotion) {
    return {
      placeholder: `${heroPlaceholderPrefix}${heroPlaceholderSuggestions[0]}`,
      reset,
    };
  }

  const suggestion = heroPlaceholderSuggestions[typewriter.suggestionIndex];
  return {
    placeholder: `${heroPlaceholderPrefix}${suggestion.slice(0, typewriter.visibleCharacters)}`,
    reset,
  };
}

export function WorkspacePrompt({ access = "authenticated", variant = "workspace", promptValue, onPromptChange, leadingControls, submitAction, selectedChatModel: explicitChatModel, selectedGenerationModel, chatModelUnavailable = false }: WorkspacePromptProps) {
  const msg = useMessages();
  const t = useDictionary();
  const router = useRouter();
  const conversationList = useOptionalWorkspaceConversationList();
  const submissionStartedRef = useRef(false);
  const [localPrompt, setLocalPrompt] = useState("");
  const prompt = promptValue ?? localPrompt;
  const setPrompt = (value: string) => {
    setLocalPrompt(value);
    onPromptChange?.(value);
  };
  const selectedModel = selectedGenerationModel ?? (explicitChatModel === undefined ? undefined : normalizeChatModel(explicitChatModel));
  const selectedChatModel = selectedModel?.category === "text" ? selectedModel : undefined;
  const attachments = useChatAttachments(access === "authenticated" && !submitAction ? selectedModel : undefined);
  const generation = useGenerationControls(selectedModel, chatModelUnavailable, setPrompt);
  const referenceQuote = useReferenceQuote(selectedModel, generation.options, {
    count: attachments.items.length, ready: !attachments.blocked, artifactIds: attachments.ids,
  });
  const cost = referenceQuote.active ? referenceQuote.cost : generation.cost;
  const tooLong = selectedChatModel?.max_prompt_bytes !== undefined
    && new TextEncoder().encode(prompt.trim()).length > selectedChatModel.max_prompt_bytes;
  const canSubmit = !attachments.blocked && referenceQuote.ready && prompt.trim() !== "" && !tooLong && !chatModelUnavailable && (submitAction?.canSubmit ?? generation.canSubmit);
  const disabled = submitAction?.disabled ?? false;
  const isNewChat = variant === "newChat";
  const isHero = variant === "hero";
  const { placeholder: heroPlaceholder, reset: resetHeroPlaceholder } = useHeroPlaceholder(
    isHero && prompt === "",
  );
  const promptLabel = isHero
    ? msg("workspacePrompt.askNeirohubAQuestion")
    : isNewChat
      ? t.conversations.composerPlaceholder
      : t.workspace.promptLabel;
  const promptPlaceholder = isHero
    ? heroPlaceholder
    : isNewChat
      ? t.conversations.composerPlaceholder
      : t.workspace.promptPlaceholder;

  const changePrompt = (event: ChangeEvent<HTMLTextAreaElement>) => {
    const nextPrompt = event.target.value;
    if (isHero && prompt !== "" && nextPrompt === "") {
      resetHeroPlaceholder();
    }
    setPrompt(nextPrompt);
  };

  const submit = () => {
    const normalizedPrompt = prompt.trim();
    if (!canSubmit || disabled || submissionStartedRef.current) {
      return;
    }

    if (submitAction !== undefined) {
      submitAction.onSubmit();
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

    savePendingConversationBootstrap({
      conversationKey,
      messageKey,
      prompt: normalizedPrompt,
      ...(selectedModel ? {
        modelId: selectedModel.id,
        ...(selectedModel.category === "text" ? {} : { generationOptions: { ...generation.options, ...(attachments.ids.length ? { reference_artifact_ids: attachments.ids } : {}) } }),
      } : {}),
    });
    conversationList.upsertConversation({
      id: conversationKey,
      title: fallbackTitle,
      created_at: createdAt,
      updated_at: createdAt,
      isPending: true,
    });
    attachments.clear();
    setPrompt("");
    router.push(`/app/chat/${conversationKey}?pending=1`);
  };

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submit();
  };

  return (
    <form className={styles.form} onSubmit={submitForm}>
      <ChatComposer
        attachmentController={attachments}
        canSubmit={canSubmit}
        disabled={disabled}
        leadingControls={leadingControls ?? generation.controls}
        label={promptLabel}
        mediaLabel={t.conversations.composerMediaUpload}
        mediaLibraryEnabled={access === "authenticated"}
        mediaMenuLabels={{
          chooseGenerated: t.conversations.composerMediaChooseGenerated,
          chooseUploaded: t.conversations.composerMediaChooseUploaded,
          menu: t.conversations.composerMediaMenu,
          uploadFile: t.conversations.composerMediaUploadFile,
        }}
        note={selectedModel && selectedModel.category !== "text"
          ? cost === undefined ? undefined : <>{selectedModel.name} · <CreditAmount prefix={`${t.imageGeneration.priceLabel}:`} value={cost} /></>
          : selectedChatModel
          ? <>{selectedChatModel.name}{selectedChatModel.estimate_credits !== undefined ? <RichMessage id="workspacePrompt.valueTokensPerResponse" values={{ value1: <CreditAmount value={selectedChatModel.estimate_credits} /> }} /> : null}</>
          : isNewChat || isHero ? undefined : t.workspace.promptSupport}
        onChange={changePrompt}
        onSend={submit}
        placeholder={promptPlaceholder}
        submitLabel={submitAction?.label ?? t.workspace.promptSubmit}
        value={prompt}
        variant={variant}
        wrapLeadingControls={isHero || selectedModel !== undefined}
        generatedMediaHref={access === "guest" ? "/login" : "/app/files?category=images"}
        uploadedMediaHref={access === "guest" ? "/login" : "/app/files?category=uploads"}
      />
      <ReferenceQuoteNotice message={referenceQuote.error} onRetry={referenceQuote.retry} />
      {tooLong ? <StateNotice inline kind="error">{msg("workspacePrompt.theMessageIsTooLongForThe")}</StateNotice> : null}
    </form>
  );
}
