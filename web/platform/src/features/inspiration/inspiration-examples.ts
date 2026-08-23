import { assetPaths } from "@/assets/asset-paths";
import { ru } from "@/i18n/ru";

export type InspirationExample = {
  id: string;
  modelId: string;
  modelName: string;
  quality: string;
  imagePath: string;
  imageAlt: string;
  title: string;
  prompt: string;
  downloadName: string;
  openLabel: string;
};

export const inspirationExamples: readonly InspirationExample[] = [
  {
    id: "paper-crane-cloud",
    modelId: "gpt-image-2",
    modelName: ru.inspiration.modelName,
    quality: "1K",
    imagePath: assetPaths.images.inspiration.paperCraneCloud,
    imageAlt: ru.inspiration.exampleAlt,
    title: ru.inspiration.exampleTitle,
    prompt: ru.inspiration.prompt,
    downloadName: "neirohub-paper-crane-cloud.png",
    openLabel: ru.inspiration.openExample,
  },
];

export function selectInspirationExamples(
  modelId: string | null | undefined,
  limit = 3,
): readonly InspirationExample[] {
  const modelExamples = modelId
    ? inspirationExamples.filter((example) => example.modelId === modelId)
    : [];
  const source = modelExamples.length > 0 ? modelExamples : inspirationExamples;

  return source.slice(0, limit);
}
