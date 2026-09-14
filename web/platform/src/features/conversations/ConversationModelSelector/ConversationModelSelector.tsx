"use client";

import { useEffect, useState } from "react";

import { ModelSelector, type ModelSelectorStatus } from "@/features/models/WorkspaceModelSelector/ModelSelector";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "@/features/models/generation-model-catalog";



import styles from "./ConversationModelSelector.module.css";

type ModelSelection = {
  catalog: GenerationModelCatalog | null;
  selectedModelId: string;
  status: ModelSelectorStatus;
};

export function useConversationModelSelection(conversationId: string) {
  const [selection, setSelection] = useState<ModelSelection>({
    catalog: null,
    selectedModelId: "",
    status: "loading",
  });
  const storageKey = `neirohub:conversation-model:${conversationId}`;

  useEffect(() => {
    let active = true;
    void loadGenerationModelCatalog().then((catalog) => {
      if (!active) return;
      let savedModel: string | null = null;
      try {
        savedModel = window.sessionStorage.getItem(storageKey);
      } catch {
        // Storage is optional; the current dialogue still works without it.
      }
      const selectedModelId = catalog.items.find((model) => model.id === savedModel)?.id
        ?? catalog.default_model_id;
      setSelection({ catalog, selectedModelId, status: catalog.items.length > 0 ? "ready" : "failure" });
    }).catch(() => {
      if (active) setSelection((current) => ({ ...current, status: "failure" }));
    });
    return () => { active = false; };
  }, [storageKey]);

  const selectModel = (modelId: string) => {
    if (!selection.catalog?.items.some((model) => model.id === modelId)) return;
    setSelection((current) => ({ ...current, selectedModelId: modelId }));
    try {
      window.sessionStorage.setItem(storageKey, modelId);
    } catch {
      // Persist only the public model preference, never message contents.
    }
  };

  return { ...selection, selectModel };
}

type ConversationModelSelectorProps = {
  disabled: boolean;
  selection: ReturnType<typeof useConversationModelSelection>;
};

export function ConversationModelSelector({ disabled, selection }: Readonly<ConversationModelSelectorProps>) {
  return (
    <ModelSelector
      className={styles.selector}
      descriptionMode="tooltip"
      dialogLabel="Выбор модели для диалога"
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
