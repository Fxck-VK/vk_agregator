"use client";

import {
  ModelSelector,
  type ModelSelectorModel,
  type ModelSelectorStatus,
} from "@/features/models/WorkspaceModelSelector/ModelSelector";

import {
  fileModelTaskConfiguration,
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
  const configuration = fileModelTaskConfiguration[task];

  const selectModel = (model: ModelSelectorModel) => {
    onSelect(model.id);
  };

  return (
    <ModelSelector
      dialogLabel={`Выбор нейросети для «${configuration.label}»`}
      models={models}
      onSelect={selectModel}
      renderInPortal
      selectedModelId={selectedModelId}
      status={status}
      triggerAriaLabel={(name, isOpen) => (
        `Выбрана модель ${name}. ${isOpen ? "Закрыть" : "Открыть"} список`
      )}
      variant="panel"
    />
  );
}
