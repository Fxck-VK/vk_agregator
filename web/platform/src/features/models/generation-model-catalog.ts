import type { ChatModel, ImageModel } from "@/lib/web-api/contracts";

import type { VideoModel } from "./video-model-catalog";
import { chatModelForSelector } from "./chat-model-selector";
import type { ModelSelectorModel } from "./WorkspaceModelSelector/ModelSelector";
import type { ModelSelectorCategoryId } from "./WorkspaceModelSelector/model-selector-sections";
import { loadModelCatalog } from "./model-catalog-cache";
import {
 projectChatModelCatalog,
 projectImageModelCatalog,
 projectVideoModelCatalog,
 type PublicCatalog,
 type PublicOperation,
} from "./model-catalog-contract";

type GenerationModelMetadata = {
 operations?: PublicOperation[] | unknown[];
};

export type GenerationModel = ModelSelectorModel & GenerationModelMetadata & (
 (ChatModel & { category: "text" }) | (ImageModel & { category: "images" }) | (VideoModel & { category: "video" })
);

export type GenerationModelCatalog = {
 items: GenerationModel[];
 default_model_id: string;
 categoryErrors: Partial<Record<ModelSelectorCategoryId, string>>;
};

function projectOrError<T>(project: () => T): { value: T; error?: never } | { value?: never; error: unknown } {
 try {
  return { value: project() };
 } catch (error) {
  return { error };
 }
}

export function projectGenerationModelCatalog(catalog: PublicCatalog) {
 const images = projectOrError(() => projectImageModelCatalog(catalog));
 const text = projectOrError(() => projectChatModelCatalog(catalog));
 const video = projectOrError(() => projectVideoModelCatalog(catalog));
 const imageByID = new Map((images.value?.items ?? []).map((model) => [model.id, model]));
 const textByID = new Map((text.value?.items ?? []).map((model) => [model.id, model]));
 const videoByID = new Map((video.value?.items ?? []).map((model) => [model.id, model]));
 const items = catalog.items.flatMap((catalogModel): GenerationModel[] => {
  if (catalogModel.kind === "text") {
   const chat = textByID.get(catalogModel.id);
   return chat ? [{ ...chat, ...chatModelForSelector(chat), category: "text" as const, operations: catalogModel.operations }] : [];
  }
  if (catalogModel.kind === "image") {
   const image = imageByID.get(catalogModel.id);
   return image ? [{ ...image, category: "images" as const, operations: catalogModel.operations }] : [];
  }
  if (catalogModel.kind === "video") {
   const videoModel = videoByID.get(catalogModel.id);
   return videoModel ? [{ ...videoModel, category: "video" as const, operations: catalogModel.operations }] : [];
  }
  return [];
 });
 return {
  items,
  default_model_id: items.some((model) => model.id === catalog.default_model_id) ? catalog.default_model_id : items[0]?.id ?? "",
  categoryErrors: {
   ...(images.error ? { images: "models_load_failed" } : {}),
   ...(text.error ? { text: "models_load_failed", free: "models_load_failed", "study-work": "models_load_failed" } : {}),
   ...(video.error ? { video: "models_load_failed" } : {}),
  },
 };
}

function allCategoryErrors() {
 const message = "models_load_failed";
 return {
  popular: message,
  images: message,
  text: message,
  video: message,
  audio: message,
  free: message,
  "study-work": message,
 };
}

// Both pickers and the new-chat form consume this exact adapter. Ordering,
// categories and partial failures must not vary with picker placement.
export async function loadGenerationModelCatalog(): Promise<GenerationModelCatalog> {
 try {
  return projectGenerationModelCatalog(await loadModelCatalog());
 } catch {
  return { items: [], default_model_id: "", categoryErrors: allCategoryErrors() };
 }
}
