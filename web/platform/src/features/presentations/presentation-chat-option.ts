import { brandLogoPath } from "@/config/brand";
import type { ModelSelectorModel } from "@/features/models/WorkspaceModelSelector/ModelSelector";
import { presentationChatCopy } from "@/i18n/presentation-chat";
import { presentationChatModelId } from "./route";

export const presentationChatOption: ModelSelectorModel = {
  id: presentationChatModelId,
  category: "text",
  categories: ["popular", "text"],
  name: presentationChatCopy.title,
  description: presentationChatCopy.description,
  artworkSrc: brandLogoPath,
};
