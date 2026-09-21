import type { ModelSelectorCategoryId } from "../WorkspaceModelSelector/model-selector-sections";

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
  const query = filters.query.trim().toLowerCase();

  return models.filter((model) => {
    const matchesQuery =
      query === "" || model.name.toLowerCase().includes(query) || model.id.toLowerCase().includes(query);
    const matchesCategory = matchesModelCatalogCategory(model, filters.category);

    return matchesQuery && matchesCategory;
  });
}

export function filterAndSortCatalogModels<T extends ModelCatalogModel>(
  models: readonly T[],
  filters: ModelCatalogFilters,
  sort: ImageModelSort,
): T[] {
  const filtered = filterCatalogModels(models, filters);

  return sort === "name" ? [...filtered].sort((left, right) => left.name.localeCompare(right.name)) : filtered;
}
