"use client";

import { useEffect, useState } from "react";

import { loadGenerationModelCatalog, type GenerationModel } from "@/features/models/generation-model-catalog";
import { ru } from "@/i18n/ru";


import { WorkspacePrompt } from "./WorkspacePrompt";

export function NewChatPrompt({ modelId }: { modelId: string }) {
  const [result, setResult] = useState<{ requestedId: string; model: GenerationModel | null } | null>(null);
  const currentResult = result?.requestedId === modelId ? result : null;
  const model = currentResult?.model ?? null;
  const failed = currentResult !== null && model === null;

  useEffect(() => {
    let active = true;
    void loadGenerationModelCatalog().then((catalog) => {
      if (!active) return;
      const requestedModel = catalog.items.find((item) => item.id === modelId);
      setResult({ requestedId: modelId, model: requestedModel ?? null });
    }).catch(() => { if (active) setResult({ requestedId: modelId, model: null }); });
    return () => { active = false; };
  }, [modelId]);

  return <>
    <WorkspacePrompt chatModelUnavailable={model === null} selectedGenerationModel={model ?? undefined} variant="newChat" />
    {failed ? <p role="alert">{ru.modelsCatalog.loadFailure}</p> : model === null ? <p role="status">{ru.modelSelector.loading}</p> : null}
  </>;
}
