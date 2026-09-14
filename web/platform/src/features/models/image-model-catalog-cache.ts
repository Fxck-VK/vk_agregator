import type { ImageModelList } from "@/lib/web-api/contracts";

import { loadModelCatalog, resetModelCatalogCacheForTests, type ModelCatalogLoadOptions } from "./model-catalog-cache";
import { projectImageModelCatalog } from "./model-catalog-contract";

export function loadImageModelCatalog(options: ModelCatalogLoadOptions = {}): Promise<ImageModelList> {
  return loadModelCatalog(options).then(projectImageModelCatalog);
}

export function resetImageModelCatalogCacheForTests(): void {
  resetModelCatalogCacheForTests();
}
