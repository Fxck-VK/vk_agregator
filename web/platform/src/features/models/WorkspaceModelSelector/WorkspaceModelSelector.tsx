"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import { ru } from "@/i18n/ru";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "../generation-model-catalog";





import { useWorkspaceModelSelection } from "../WorkspaceModelSelection/WorkspaceModelSelection";
import {
  ModelSelector,
  type ModelSelectorModel,
  type ModelSelectorStatus,
} from "./ModelSelector";

export function WorkspaceModelSelector() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedModelId = searchParams.get("model");
  const workspaceSelection = useWorkspaceModelSelection();
  const workspaceSelectedModelId = workspaceSelection?.selectedModelId ?? null;
  const setWorkspaceModelId = workspaceSelection?.setSelectedModelId;
  const [status, setStatus] = useState<ModelSelectorStatus>("loading");
  const [models, setModels] = useState<ModelSelectorModel[]>([]);
  const [selectedModelId, setSelectedModelId] = useState("");
  const [catalogFailures, setCatalogFailures] = useState<GenerationModelCatalog["categoryErrors"]>({});

  useEffect(() => {
    let active = true;

    void loadGenerationModelCatalog().then((catalog) => {
      if (!active) return;
      setModels(catalog.items);
      setCatalogFailures(catalog.categoryErrors);
      setStatus(catalog.items.length > 0 ? "ready" : "failure");
    });

    return () => {
      active = false;
    };
  }, []);

  const activeSelectedModelId = [requestedModelId, workspaceSelectedModelId, selectedModelId]
    .find((id) => models.some((model) => model.id === id)) ?? models[0]?.id ?? "";

  const selectModel = (model: ModelSelectorModel) => {
    setSelectedModelId(model.id);
    if (model.category === "images") setWorkspaceModelId?.(model.id);
    router.push(`/app/chats?model=${encodeURIComponent(model.id)}`);
  };

  return (
    <ModelSelector
      dialogId="workspace-model-selector-dialog"
      categoryErrors={catalogFailures}
      models={models}
      onSelect={selectModel}
      selectedModelId={activeSelectedModelId}
      status={status}
      triggerAriaLabel={(name) => ru.modelSelector.triggerLabel(name)}
    />
  );
}
