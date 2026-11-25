"use client";

import { useMessages } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";

import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from "react";

import { ModelSelector } from "@/features/models/WorkspaceModelSelector/ModelSelector";
import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import { useWorkspaceModelSelection } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";
import { presentationChatOption } from "@/features/presentations/presentation-chat-option";
import { presentationChatModelId, presentationChatPath } from "@/features/presentations/route";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";

import styles from "./ConversationModelSelector.module.css";

const subscribePreference = () => () => {};
const serverPreference = () => "";

export function useConversationModelSelection(conversationId: string) {
  const { catalog: workspaceCatalog, status } = useGenerationCatalog();
  const catalog = useMemo(() => {
    if (!workspaceCatalog) return null;
    const items = workspaceCatalog.items.filter((model): model is GenerationModel => model.category !== "audio");
    const default_model_id = items.some(model => model.id === workspaceCatalog.default_model_id)
      ? workspaceCatalog.default_model_id : items[0]?.id ?? "";
    return { ...workspaceCatalog, items, default_model_id };
  }, [workspaceCatalog]);
  const workspaceSelection = useWorkspaceModelSelection();
  const setConversationModel = workspaceSelection?.setConversationModel;
  const [preference, setPreference] = useState<{ conversationId: string; modelId: string } | null>(null);
  const storageKey = `neirohub:conversation-model:${conversationId}`;
  const readPreference = useCallback(() => {
    try { return window.sessionStorage.getItem(storageKey) ?? ""; } catch { return ""; }
  }, [storageKey]);
  const savedPreference = useSyncExternalStore(subscribePreference, readPreference, serverPreference);
  const preferredId = preference?.conversationId === conversationId ? preference.modelId : savedPreference;
  const selectedModelId = catalog?.items.find(model => model.id === preferredId)?.id
    ?? catalog?.default_model_id ?? "";
  useEffect(() => {
    setConversationModel?.(conversationId, selectedModelId || null);
    return () => { setConversationModel?.(conversationId, null); };
  }, [conversationId, selectedModelId, setConversationModel]);
  const selectModel = (modelId: string) => {
    if (!catalog?.items.some(model => model.id === modelId)) return;
    setPreference({ conversationId, modelId });
    try { window.sessionStorage.setItem(storageKey, modelId); } catch { /* No private contents in storage. */ }
  };
  return { catalog, status, selectedModelId, selectModel };
}

type ConversationModelSelectorProps = {
  disabled: boolean;
  selection: ReturnType<typeof useConversationModelSelection>;
};

export function ConversationModelSelector(props: Readonly<ConversationModelSelectorProps>) {
  const release = useReleaseView();
  if (release.view === "dev") return <PresentationConversationSelector {...props} />;
  return <ConversationSelector {...props} />;
}

function PresentationConversationSelector(props: Readonly<ConversationModelSelectorProps>) {
  const router = useRouter();
  return <ConversationSelector {...props} onPresentations={() => router.push(presentationChatPath)} />;
}

function ConversationSelector({ disabled, selection, onPresentations }: Readonly<ConversationModelSelectorProps & {
  onPresentations?: () => void;
}>) {
  const msg = useMessages();
  return (
    <ModelSelector
      additionalOption={onPresentations ? presentationChatOption : undefined}
      className={styles.selector}
      descriptionMode="tooltip"
      dialogLabel={msg("conversationModelSelector.chooseAModelForThisChat")}
      disabled={disabled}
      categoryErrors={selection.catalog?.categoryErrors}
      key={disabled ? "busy" : "ready"}
      models={selection.catalog?.items ?? []}
      onSelect={(model) => {
        if (model.id === presentationChatModelId) onPresentations?.();
        else selection.selectModel(model.id);
      }}
      renderInPortal
      selectedModelId={selection.selectedModelId}
      status={selection.status}
      variant="composer"
    />
  );
}
