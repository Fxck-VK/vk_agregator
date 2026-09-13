import type { ImageModel } from "@/lib/web-api/contracts";

type ModelPresentationEntry = {
  artworkSrc?: string;
  description: string;
};

export type ModelPresentation = ModelPresentationEntry & {
  href: string;
};

const modelPresentationById: Readonly<Record<string, ModelPresentationEntry>> = {
  "nano-banana-2": {
    description: "Быстрая генерация и редактирование изображений для повседневных задач",
  },
  "nano-banana-pro": {
    description: "Детализированные изображения для сложных творческих и рабочих задач",
  },
  "gpt-image-2": {
    description: "Точное создание изображений по описанию с хорошей передачей текста",
  },
  "seedream-4-5": {
    description: "Фотореалистичные изображения с высокой детализацией и выразительным стилем",
  },
  midjourney: {
    description: "Выразительные изображения и концепты в узнаваемом художественном стиле",
  },
  "flux-2-pro": {
    description: "Точная генерация изображений с детальной композицией и естественным светом",
  },
  "catalog-preview-recraft-v3": {
    description: "Векторные иллюстрации и фирменная графика с точным контролем стиля",
  },
  "catalog-preview-ideogram-3": {
    description: "Постеры и рекламные изображения с аккуратной передачей текста",
  },
  "catalog-preview-stable-diffusion-3-5": {
    description: "Гибкая генерация изображений для экспериментов с разными стилями",
  },
  "catalog-preview-leonardo-phoenix": {
    description: "Яркие концепты и персонажи с выразительными деталями",
  },
};

type ModelPresentationSource = Pick<ImageModel, "id" | "name"> & {
  artworkSrc?: string;
  description?: string;
};

export function getModelPresentation(model: ModelPresentationSource): ModelPresentation {
  const presentationId = model.id.toLowerCase().replaceAll(".", "-");
  const entry = modelPresentationById[presentationId];

  return {
    artworkSrc: model.artworkSrc ?? entry?.artworkSrc,
    description: model.description
      ?? entry?.description
      ?? `${model.name} для создания изображений по вашему описанию`,
    href: `/app/image?model=${encodeURIComponent(model.id)}`,
  };
}
