"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { loadImageModelCatalog } from "../image-model-catalog-cache";
import { getModelPresentation } from "../ModelCard/model-card-content";
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
  const [models, setModels] = useState<ImageModel[]>([]);
  const [selectedModelId, setSelectedModelId] = useState("");

  useEffect(() => {
    let active = true;

    void loadImageModelCatalog()
      .then((catalogue) => {
        if (!active) return;

        const requestedModelExists = requestedModelId !== null
          && catalogue.items.some((model) => model.id === requestedModelId);
        const workspaceModelExists = workspaceSelectedModelId !== null
          && catalogue.items.some((model) => model.id === workspaceSelectedModelId);
        const initialModelId = requestedModelExists
          ? requestedModelId
          : workspaceModelExists
            ? workspaceSelectedModelId
            : (catalogue.items[0]?.id ?? "");

        setModels(catalogue.items);
        setSelectedModelId(initialModelId);
        if (initialModelId !== "") {
          setWorkspaceModelId?.(initialModelId);
        }
        setStatus(catalogue.items.length > 0 ? "ready" : "failure");
      })
      .catch(() => {
        if (active) setStatus("failure");
      });

    return () => {
      active = false;
    };
  }, [requestedModelId, setWorkspaceModelId, workspaceSelectedModelId]);

  const selectorModels = useMemo<readonly ModelSelectorModel[]>(() => (
    models.map((model) => ({ ...model, category: "images" }))
  ), [models]);
  const activeSelectedModelId = workspaceSelectedModelId ?? selectedModelId;

  const selectModel = (model: ModelSelectorModel) => {
    setSelectedModelId(model.id);
    setWorkspaceModelId?.(model.id);
    router.push(getModelPresentation(model).href);
  };

  return (
    <ModelSelector
      dialogId="workspace-model-selector-dialog"
      models={selectorModels}
      onSelect={selectModel}
      selectedModelId={activeSelectedModelId}
      status={status}
      triggerAriaLabel={(name) => ru.modelSelector.triggerLabel(name)}
    />
  );
}
