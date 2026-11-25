"use client";

import { useDictionary } from "@/i18n/LocaleProvider";
import { usePathname, useRouter, useSearchParams } from "@/i18n/navigation";
import { useState } from "react";

import { useGenerationCatalog } from "../GenerationCatalogProvider";
import { modelWorkspaceHref } from "../ModelCard/model-card-content";
import { RetryAction } from "@/components/ui/AsyncState/RetryAction";
import { presentationChatOption } from "@/features/presentations/presentation-chat-option";
import { presentationChatModelId, presentationChatPath, presentationWorkspacePath } from "@/features/presentations/route";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";

import { useWorkspaceModelSelection } from "../WorkspaceModelSelection/WorkspaceModelSelection";
import {
  ModelSelector,
  type ModelSelectorModel,
} from "./ModelSelector";

export function WorkspaceModelSelector() {
  const t = useDictionary();
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const release = useReleaseView();
  const requestedModelId = searchParams.get("model");
  const workspaceSelection = useWorkspaceModelSelection();
  const conversationModel = workspaceSelection?.conversationModel;
  const conversationModelId = conversationModel && pathname === `/app/chat/${conversationModel.conversationId}`
    ? conversationModel.modelId
    : null;
  const workspaceSelectedModelId = workspaceSelection?.selectedModelId ?? null;
  const setWorkspaceModelId = workspaceSelection?.setSelectedModelId;
  const { catalog, status, failed, retry } = useGenerationCatalog();
  const models = catalog?.items ?? [];
  const [selectedModelId, setSelectedModelId] = useState("");

  const presentationAvailable = release.allowsPath(presentationWorkspacePath);
  const presentationSelected = presentationAvailable && (conversationModelId === presentationChatModelId
    || (pathname === "/app/chats" && requestedModelId === presentationChatModelId));
  const activeSelectedModelId = presentationSelected ? presentationChatModelId
    : [conversationModelId, requestedModelId, workspaceSelectedModelId, selectedModelId]
      .find((id) => models.some((model) => model.id === id)) ?? models[0]?.id ?? "";

  const selectModel = (model: ModelSelectorModel) => {
    if (model.id === presentationChatModelId) {
      router.push(presentationChatPath);
      return;
    }
    setSelectedModelId(model.id);
    if (model.category === "images") setWorkspaceModelId?.(model.id);
    router.push(modelWorkspaceHref(model));
  };

  return (
    <div style={{ display: "flex", alignItems: "center", gap: "var(--space-2)", minWidth: 0 }}>
      <ModelSelector
        additionalOption={presentationAvailable ? presentationChatOption : undefined}
        dialogId="workspace-model-selector-dialog"
        categoryErrors={catalog?.categoryErrors}
        models={models}
        onSelect={selectModel}
        selectedModelId={activeSelectedModelId}
        status={status}
        triggerAriaLabel={(name) => t.modelSelector.triggerLabel(name)}
      />
      {failed ? <RetryAction iconOnly label={`${t.modelsCatalog.loadFailure} ${t.files.retry}`} onClick={retry} /> : null}
    </div>
  );
}
