import type { ModelSelectorCategoryId } from "../WorkspaceModelSelector/model-selector-sections";
import { searchModels } from "../model-search";

export type ModelCatalogModel = {
  category?: string;
  categories?: readonly string[];
  id: string;
  name: string;
  [key: string]: unknown;
};

export type ModelCatalogFilters = {
  category: ModelSelectorCategoryId;
  query: string;
};

export type ImageModelSort = "catalog" | "name";

export function matchesModelCatalogCategory(model: Pick<ModelCatalogModel, "categories" | "category">, category: ModelSelectorCategoryId) {
  if (category === "video" || category === "audio") {
    return (model.categories?.includes("video-audio") ?? false) && model.category === category;
  }
  return model.categories?.includes(category) ?? false;
}

export function filterCatalogModels<T extends ModelCatalogModel>(models: readonly T[], filters: ModelCatalogFilters): T[] {
  return searchModels(models.filter(model => matchesModelCatalogCategory(model, filters.category)), filters.query);
}

export function filterAndSortCatalogModels<T extends ModelCatalogModel>(
  models: readonly T[],
  filters: ModelCatalogFilters,
  sort: ImageModelSort,
): T[] {
  const filtered = filterCatalogModels(models, filters);

  return sort === "name" ? [...filtered].sort((left, right) => left.name.localeCompare(right.name)) : filtered;
}
