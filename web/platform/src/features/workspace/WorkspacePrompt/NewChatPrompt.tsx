"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";


import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";


import { WorkspacePrompt } from "./WorkspacePrompt";

export function NewChatPrompt({ modelId }: { modelId: string }) {
  const t = useDictionary();
  const { catalog, failed: catalogFailed, retry } = useGenerationCatalog();
  const model = catalog?.items.find((item) => item.id === modelId) ?? null;
  const failed = catalogFailed || (catalog !== null && model === null);

  return <>
    <WorkspacePrompt chatModelUnavailable={model === null} selectedGenerationModel={model ?? undefined} variant="newChat" />
    {failed ? <StateNotice inline kind="error" action={{ label: t.files.retry, onClick: retry }}>{t.modelsCatalog.loadFailure}</StateNotice> : model === null ? <StateNotice inline kind="loading">{t.modelSelector.loading}</StateNotice> : null}
  </>;
}
