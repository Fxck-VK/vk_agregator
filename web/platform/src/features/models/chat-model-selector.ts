import type { ChatModel } from "@/lib/web-api/contracts";

import type { ModelSelectorModel } from "./WorkspaceModelSelector/ModelSelector";

export function chatModelForSelector(model: ChatModel): ModelSelectorModel {
  return {
    ...model,
    category: "text",
    isFree: model.categories?.includes("free") === true || model.estimate_credits === 0,
    description: model.description ?? ((model.estimate_credits ?? 0) > 0
      ? `${model.estimate_credits} токенов за ответ${model.max_output_tokens ? ` · до ${model.max_output_tokens} токенов ответа` : ""}`
      : "Ответы на вопросы и работа с текстом в текущем диалоге"),
  };
}
