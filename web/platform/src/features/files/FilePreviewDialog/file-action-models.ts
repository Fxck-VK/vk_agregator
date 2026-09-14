import { useEffect, useState } from "react";

import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import type {
  PublicCatalog,
  PublicCatalogModel,
  PublicOperation,
} from "@/features/models/model-catalog-contract";
import type {
  ModelSelectorCategory,
  ModelSelectorModel,
  ModelSelectorStatus,
} from "@/features/models/WorkspaceModelSelector/ModelSelector";

export type FileModelTask = "animate" | "enhance" | "remove-background" | "edit";

export type WorkspaceModelCatalog = PublicCatalog;
export type WorkspaceOperation = PublicOperation;

export type FileActionModel = ModelSelectorModel & {
  cost: number;
  operation: PublicOperation;
};

type FileModelTaskConfiguration = {
  label: string;
};

type FileActionModelCatalogState = {
  error: Error | null;
  models: readonly FileActionModel[];
  status: ModelSelectorStatus;
};

type ModelCatalogLoader = () => Promise<WorkspaceModelCatalog>;

const imageCategory = "images" satisfies Exclude<ModelSelectorCategory, "popular">;
const videoCategory = "video" satisfies Exclude<ModelSelectorCategory, "popular">;

let modelCatalogLoaderForTests: ModelCatalogLoader | null = null;

export const fileModelTaskConfiguration = {
  animate: { label: "Оживить" },
  enhance: { label: "Улучшить" },
  "remove-background": { label: "Удалить фон" },
  edit: { label: "Редактировать" },
} as const satisfies Record<FileModelTask, FileModelTaskConfiguration>;

function getModelCatalogLoader(): ModelCatalogLoader {
  return modelCatalogLoaderForTests ?? loadModelCatalog;
}

export function setFileActionModelCatalogLoaderForTests(loader: ModelCatalogLoader): void {
  modelCatalogLoaderForTests = loader;
}

export function resetFileActionModelCatalogLoaderForTests(): void {
  modelCatalogLoaderForTests = null;
}

function isPositivePrice(value: number | undefined): value is number {
  return Number.isFinite(value) && value !== undefined && value > 0;
}

function minimumPositivePrice(values: Iterable<number | undefined>): number | null {
  const prices = [...values].filter(isPositivePrice);
  return prices.length > 0 ? Math.min(...prices) : null;
}

function imageOperationCost(image: PublicOperation["image"]): number | null {
  if (!image) return null;
  const quality = image.default_quality;
  const ratio = image.default_aspect_ratio;
  const defaultVariantCost = quality && ratio
    ? image.price_by_variant?.[`${quality}:${ratio}`]
    : undefined;
  if (isPositivePrice(defaultVariantCost)) return defaultVariantCost;

  const defaultQualityCost = quality ? image.price_by_quality?.[quality] : undefined;
  if (isPositivePrice(defaultQualityCost)) return defaultQualityCost;

  return minimumPositivePrice([
    ...Object.values(image.price_by_variant ?? {}),
    ...Object.values(image.price_by_quality ?? {}),
  ]);
}

function videoOperationCost(video: PublicOperation["video"]): number | null {
  if (!video) return null;
  const resolution = video.default_resolution;
  const duration = video.default_duration_sec;
  const defaultCost = resolution && duration
    ? video.price_by_option?.[`${resolution}:${duration}`]
    : undefined;
  if (isPositivePrice(defaultCost)) return defaultCost;
  return minimumPositivePrice(Object.values(video.price_by_option ?? {}));
}

function operationCategory(operation: WorkspaceOperation): FileActionModel["category"] | null {
  if (operation.kind === "image") return imageCategory;
  if (operation.kind === "video") return videoCategory;
  return null;
}

function hasEnabledImageInput(operation: WorkspaceOperation): boolean {
  return operation.inputs?.images?.enabled === true;
}

function fileTaskOperationCost(task: FileModelTask, operation: WorkspaceOperation): number | null {
  if (task === "animate") return videoOperationCost(operation.video);
  return imageOperationCost(operation.image);
}

function matchesFileTask(task: FileModelTask, operation: WorkspaceOperation): boolean {
  if (operation.enabled !== true || !hasEnabledImageInput(operation)) {
    return false;
  }

  if (task === "animate") {
    return operation.id === "generate" && operation.kind === "video";
  }

  return operation.id === task && operation.kind === "image";
}

function actionModelFromOperation(
  task: FileModelTask,
  model: PublicCatalogModel,
  operation: PublicOperation,
): FileActionModel | null {
  const category = operationCategory(operation);
  const cost = fileTaskOperationCost(task, operation);
  if (
    category === null
    || cost === null
  ) {
    return null;
  }

  return {
    category,
    cost,
    description: model.description ?? "",
    id: model.id,
    isFree: cost === 0,
    name: model.name,
    categories: model.categories,
    operation,
  };
}

export function getFileActionModels(
  task: FileModelTask,
  catalog: WorkspaceModelCatalog,
): readonly FileActionModel[] {
  return catalog.items.flatMap((model) => (
    model.operations
  ).flatMap((operation) => {
    if (!matchesFileTask(task, operation)) return [];
    const actionModel = actionModelFromOperation(task, model, operation);
    return actionModel ? [actionModel] : [];
  }));
}

export async function loadFileActionModels(task: FileModelTask): Promise<readonly FileActionModel[]> {
  return getFileActionModels(task, await getModelCatalogLoader()());
}

export function useFileActionModels(task: FileModelTask): FileActionModelCatalogState {
  const [state, setState] = useState<FileActionModelCatalogState & { task: FileModelTask }>({
    task,
    error: null,
    models: [],
    status: "loading",
  });

  useEffect(() => {
    let isCurrent = true;

    loadFileActionModels(task)
      .then((models) => {
        if (!isCurrent) return;
        setState({ task, error: null, models, status: "ready" });
      })
      .catch((error: unknown) => {
        if (!isCurrent) return;
        setState({
          task,
          error: error instanceof Error ? error : new Error("Unable to load file action models."),
          models: [],
          status: "failure",
        });
      });

    return () => {
      isCurrent = false;
    };
  }, [task]);

  return state.task === task ? state : { error: null, models: [], status: "loading" };
}
