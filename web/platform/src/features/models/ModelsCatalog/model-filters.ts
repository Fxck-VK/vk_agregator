import type { ImageModel } from "@/lib/web-api/contracts";

export type ModelCatalogFilters = {
  query: string;
  referenceOnly: boolean;
  quality: string | null;
};

export type ImageModelSort = "catalog" | "name";

export function filterImageModels(models: ImageModel[], filters: ModelCatalogFilters): ImageModel[] {
  const query = filters.query.trim().toLowerCase();

  return models.filter((model) => {
    const matchesQuery =
      query === "" || model.name.toLowerCase().includes(query) || model.id.toLowerCase().includes(query);
    const matchesReference = !filters.referenceOnly || model.supports_reference_image;
    const matchesQuality = filters.quality === null || model.quality_options.includes(filters.quality);

    return matchesQuery && matchesReference && matchesQuality;
  });
}

export function filterAndSortImageModels(
  models: ImageModel[],
  filters: ModelCatalogFilters,
  sort: ImageModelSort,
): ImageModel[] {
  const filtered = filterImageModels(models, filters);

  return sort === "name" ? [...filtered].sort((left, right) => left.name.localeCompare(right.name)) : filtered;
}

export function imageModelQualities(models: ImageModel[]): string[] {
  const qualities = new Set<string>();

  for (const model of models) {
    for (const quality of model.quality_options) {
      qualities.add(quality);
    }
  }

  return [...qualities];
}
