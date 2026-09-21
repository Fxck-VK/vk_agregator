import { getDictionary, type Dictionary } from "@/i18n/dictionary";
import { matchesModelCatalogCategory } from "../ModelsCatalog/model-filters";

import type { ModelSelectorModel } from "./ModelSelector";

export type ModelSelectorCategoryId = Dictionary["modelsCatalog"]["categories"][number]["id"];
export type ModelSelectorCategoryModelIds = Partial<Record<ModelSelectorCategoryId, readonly string[]>>;

// Shared by the header and composer. Set ordered model IDs to curate a category;
// [] hides it. Omitted categories use the first five matching available models.
export const modelSelectorCategoryModelIds: ModelSelectorCategoryModelIds = {};

const categoryModelLimit = 5;

export function getModelSelectorSections(
  models: readonly ModelSelectorModel[],
  selections: ModelSelectorCategoryModelIds,
  errors?: Partial<Record<ModelSelectorCategoryId, string>>,
  dictionary: Dictionary = getDictionary("ru"),
) {
  return dictionary.modelsCatalog.categories.map((category) => {
    const candidates = models.filter((model) => matchesModelCatalogCategory(model, category.id));
    const ids = selections[category.id];
    const chosenModels = ids === undefined ? candidates : [...new Set(ids)].flatMap((id) => {
      const model = candidates.find((candidate) => candidate.id === id);
      return model ? [model] : [];
    });
    return {
      ...category,
      models: chosenModels.slice(0, categoryModelLimit),
      error: ids?.length === 0 ? undefined : errors?.[category.id] === "models_load_failed" ? dictionary.modelsCatalog.loadFailure : errors?.[category.id],
    };
  }).filter((section) => section.models.length > 0 || section.error);
}
