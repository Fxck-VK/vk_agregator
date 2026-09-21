"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { useCallback, useEffect, useState, useSyncExternalStore } from "react";

import { ModelSelector } from "@/features/models/WorkspaceModelSelector/ModelSelector";
import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";
import { useWorkspaceModelSelection } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";

import styles from "./ConversationModelSelector.module.css";

const subscribePreference = () => () => {};
const serverPreference = () => "";

export function useConversationModelSelection(conversationId: string) {
  const { catalog, status } = useGenerationCatalog();
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

export function ConversationModelSelector({ disabled, selection }: Readonly<ConversationModelSelectorProps>) {
  const msg = useMessages();
  return (
    <ModelSelector
      className={styles.selector}
      descriptionMode="tooltip"
      dialogLabel={msg("conversationModelSelector.chooseAModelForThisChat")}
      disabled={disabled}
      categoryErrors={selection.catalog?.categoryErrors}
      key={disabled ? "busy" : "ready"}
      models={selection.catalog?.items ?? []}
      onSelect={(model) => selection.selectModel(model.id)}
      renderInPortal
      selectedModelId={selection.selectedModelId}
      status={selection.status}
      variant="composer"
    />
  );
}
