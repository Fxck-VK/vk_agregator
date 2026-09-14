import { loadModelCatalog } from "./model-catalog-cache";
import { projectVideoModelCatalog, type VideoModel, type VideoModelList } from "./model-catalog-contract";

export type { VideoModel, VideoModelList };

export async function loadVideoModelCatalog() {
 return projectVideoModelCatalog(await loadModelCatalog());
}
