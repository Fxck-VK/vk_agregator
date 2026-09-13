import { ModelsCatalog } from "@/features/models/ModelsCatalog/ModelsCatalog";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";

export default function ModelsPage() {
  return <ModelsCatalog includePlaceholders={isLocalWorkspacePreviewEnabled()} />;
}
