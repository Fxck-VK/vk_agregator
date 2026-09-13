"use client";

import { useCallback, useEffect, useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from "react";
import { useRouter } from "next/navigation";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { savePendingConversationBootstrap } from "@/features/conversations/pending-conversation-bootstrap";
import { fallbackConversationTitle } from "@/features/conversations/pending-conversation-title-sync";
import { useOptionalWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";

import styles from "./WorkspacePrompt.module.css";

type WorkspacePromptProps = {
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

const heroPlaceholderPrefix = "Спросите NeiroHub или ";
const heroPlaceholderSuggestions = [
  "составьте план проекта",
  "придумайте идею для поста",
  "объясните сложную тему",
  "помогите написать текст",
] as const;

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

function advanceTypewriter(state: TypewriterState): TypewriterState {
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
      () => setTypewriter((current) => advanceTypewriter(current)),
      typewriterDelays[typewriter.phase],
    );
    return () => window.clearTimeout(timeout);
  }, [enabled, prefersReducedMotion, typewriter]);

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

export function WorkspacePrompt({ access = "authenticated", variant = "workspace", promptValue, onPromptChange, leadingControls, submitAction }: WorkspacePromptProps) {
  const router = useRouter();
  const conversationList = useOptionalWorkspaceConversationList();
  const submissionStartedRef = useRef(false);
  const [localPrompt, setLocalPrompt] = useState("");
  const prompt = promptValue ?? localPrompt;
  const setPrompt = (value: string) => {
    setLocalPrompt(value);
    onPromptChange?.(value);
  };
  const canSubmit = prompt.trim() !== "" && (submitAction?.canSubmit ?? true);
  const disabled = submitAction?.disabled ?? false;
  const isNewChat = variant === "newChat";
  const isHero = variant === "hero";
  const { placeholder: heroPlaceholder, reset: resetHeroPlaceholder } = useHeroPlaceholder(
    isHero && prompt === "",
  );
  const promptLabel = isHero
    ? "Задайте вопрос NeiroHub"
    : isNewChat
      ? ru.conversations.composerPlaceholder
      : ru.workspace.promptLabel;
  const promptPlaceholder = isHero
    ? heroPlaceholder
    : isNewChat
      ? ru.conversations.composerPlaceholder
      : ru.workspace.promptPlaceholder;

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
    <form className={styles.form} onSubmit={submitForm}>
      <ChatComposer
        canSubmit={canSubmit}
        disabled={disabled}
        leadingControls={leadingControls}
        label={promptLabel}
        mediaLabel={ru.conversations.composerMediaUpload}
        mediaLibraryEnabled={access === "authenticated"}
        mediaMenuLabels={{
          chooseGenerated: ru.conversations.composerMediaChooseGenerated,
          chooseUploaded: ru.conversations.composerMediaChooseUploaded,
          menu: ru.conversations.composerMediaMenu,
          uploadFile: ru.conversations.composerMediaUploadFile,
        }}
        note={isNewChat || isHero ? undefined : ru.workspace.promptSupport}
        onChange={changePrompt}
        onSend={submit}
        placeholder={promptPlaceholder}
        submitLabel={submitAction?.label ?? ru.workspace.promptSubmit}
        value={prompt}
        variant={variant}
        wrapLeadingControls={isHero}
        generatedMediaHref={access === "guest" ? "/login" : "/app/files?category=images"}
        uploadedMediaHref={access === "guest" ? "/login" : "/app/files?category=uploads"}
      />
    </form>
  );
}
