"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import {
  ModelSelector,
  type ModelSelectorModel,
  type ModelSelectorStatus,
} from "@/features/models/WorkspaceModelSelector/ModelSelector";

import {
  getFileModelTaskConfiguration,
  type FileActionModel,
  type FileModelTask,
} from "./file-action-models";

type FileTaskModelSelectorProps = {
  models: readonly FileActionModel[];
  onSelect: (modelId: string) => void;
  selectedModelId: string;
  status: ModelSelectorStatus;
  task: FileModelTask;
};

export function FileTaskModelSelector({
  models,
  onSelect,
  selectedModelId,
  status,
  task,
}: Readonly<FileTaskModelSelectorProps>) {
  const msg = useMessages();
  const configuration = getFileModelTaskConfiguration(msg)[task];

  const selectModel = (model: ModelSelectorModel) => {
    onSelect(model.id);
  };

  return (
    <ModelSelector
      dialogLabel={msg("fileTaskModelSelector.chooseAnAiModelForValue", { value1: configuration.label })}
      models={models}
      onSelect={selectModel}
      renderInPortal
      selectedModelId={selectedModelId}
      status={status}
      triggerAriaLabel={(name, isOpen) => (
        msg("fileTaskModelSelector.selectedModelValueValueList", { value1: name, value2: isOpen ? msg("fileTaskModelSelector.close") : msg("fileTaskModelSelector.open") })
      )}
      variant="panel"
    />
  );
}
