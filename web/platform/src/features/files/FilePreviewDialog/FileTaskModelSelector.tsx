"use client";

import {
  ModelSelector,
  type ModelSelectorModel,
} from "@/features/models/WorkspaceModelSelector/ModelSelector";

import {
  fileModelTaskConfiguration,
  type FileModelTask,
} from "./file-action-models";

type FileTaskModelSelectorProps = {
  onSelect: (modelId: string) => void;
  selectedModelId: string;
  task: FileModelTask;
};

export function FileTaskModelSelector({
  onSelect,
  selectedModelId,
  task,
}: Readonly<FileTaskModelSelectorProps>) {
  const configuration = fileModelTaskConfiguration[task];

  const selectModel = (model: ModelSelectorModel) => {
    onSelect(model.id);
  };

  return (
    <ModelSelector
      dialogLabel={`Выбор нейросети для «${configuration.label}»`}
      models={configuration.models}
      onSelect={selectModel}
      renderInPortal
      selectedModelId={selectedModelId}
      triggerAriaLabel={(name, isOpen) => (
        `Выбрана модель ${name}. ${isOpen ? "Закрыть" : "Открыть"} список`
      )}
      variant="panel"
    />
  );
}
