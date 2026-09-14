import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import { projectChatModelCatalog } from "@/features/models/model-catalog-contract";

export async function loadChatModelCatalog() {
  return projectChatModelCatalog(await loadModelCatalog());
}
