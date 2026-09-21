import { getTranslator, type Translator } from "@/i18n/messages";

import type { ChatModel } from "@/lib/web-api/contracts";

import type { ModelSelectorModel } from "./WorkspaceModelSelector/ModelSelector";

export function chatModelForSelector(model: ChatModel, msg: Translator = getTranslator("ru")): ModelSelectorModel {
  const hasDescription = model.description !== undefined || Object.values(model.description_translations ?? {}).some(Boolean);
  return {
    ...model,
    category: "text",
    isFree: model.categories?.includes("free") === true || model.estimate_credits === 0,
    description: model.description ?? msg("chatModelSelector.answersAndTextAssistanceInTheCurrent"),
    responsePrice: !hasDescription && (model.estimate_credits ?? 0) > 0
      ? { credits: model.estimate_credits!, maxOutputTokens: model.max_output_tokens }
      : undefined,
  };
}
