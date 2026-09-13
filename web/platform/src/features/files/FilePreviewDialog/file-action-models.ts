import type {
  ModelSelectorCategory,
  ModelSelectorModel,
} from "@/features/models/WorkspaceModelSelector/ModelSelector";

export type FileModelTask = "animate" | "enhance" | "remove-background" | "edit";

export type FileActionModel = ModelSelectorModel & {
  cost: number;
  description: string;
};

type FileModelTaskConfiguration = {
  label: string;
  models: readonly FileActionModel[];
};

const imageCategory = "images" satisfies Exclude<ModelSelectorCategory, "popular">;
const videoCategory = "video" satisfies Exclude<ModelSelectorCategory, "popular">;

const animateModels = [
  {
    category: videoCategory,
    cost: 17,
    description: "Генерация реалистичного видео по тексту и картинке",
    id: "video-generator",
    name: "Генератор видео",
  },
  {
    category: videoCategory,
    cost: 264,
    description: "Лучшая модель для генерации видео от Google",
    id: "google-veo-3-1",
    name: "Google Veo 3.1",
  },
  {
    category: videoCategory,
    cost: 17,
    description: "Создай анимацию своей фотографии",
    id: "image-animation",
    name: "Оживление картинок",
  },
  {
    category: videoCategory,
    cost: 500,
    description: "Мощная модель для плавной анимации изображения",
    id: "kling-3",
    name: "Kling 3.0",
  },
] as const satisfies readonly FileActionModel[];

const enhanceModels = [
  {
    category: imageCategory,
    cost: 55,
    description: "Лучшая нейросеть для генерации изображения",
    id: "nano-banana-pro",
    name: "Nano Banana Pro",
  },
  {
    category: imageCategory,
    cost: 51,
    description: "Лучшая модель для генерации изображений от OpenAI",
    id: "gpt-image-2",
    name: "GPT Image 2",
  },
  {
    category: imageCategory,
    cost: 55,
    description: "Новейшая модель Nano Banana 2 от Google",
    id: "nano-banana-2",
    name: "Nano Banana 2",
  },
  {
    category: imageCategory,
    cost: 52,
    description: "Модель для генерации изображений по текстовому описанию",
    id: "image-generator",
    name: "Генератор изображений",
  },
] as const satisfies readonly FileActionModel[];

const backgroundRemovalModels = [
  {
    category: imageCategory,
    cost: 5,
    description: "Удаляет фон изображения и сохраняет объект",
    id: "recraft-ai",
    name: "Recraft AI",
  },
] as const satisfies readonly FileActionModel[];

const editModels = [
  {
    category: imageCategory,
    cost: 55,
    description: "Точечно изменяет выделенную область изображения по промпту",
    id: "nano-banana-pro",
    name: "Nano Banana Pro",
  },
] as const satisfies readonly FileActionModel[];

export const fileModelTaskConfiguration = {
  animate: { label: "Оживить", models: animateModels },
  enhance: { label: "Улучшить", models: enhanceModels },
  "remove-background": { label: "Удалить фон", models: backgroundRemovalModels },
  edit: { label: "Редактировать", models: editModels },
} as const satisfies Record<FileModelTask, FileModelTaskConfiguration>;

export function getFileActionModels(task: FileModelTask): readonly FileActionModel[] {
  return fileModelTaskConfiguration[task].models;
}
