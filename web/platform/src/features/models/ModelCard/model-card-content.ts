import { getTranslator, type Translator } from "@/i18n/messages";
import { localizedText, translateCatalogText, type LocalizedText } from "@/i18n/catalog";

export type ModelPresentation = {
  artworkSrc?: string;
  description: string;
  href: string;
};

type ModelPresentationSource = {
  artworkSrc?: string;
  category?: unknown;
  description?: string;
  description_translations?: LocalizedText;
  id: string;
  name: string;
};

export function modelWorkspaceHref(model: { id: string; category?: unknown }): string {
  const path = model.category === "audio" ? "/app/music" : "/app/chats";
  return `${path}?model=${encodeURIComponent(model.id)}`;
}

export function getModelPresentation(model: ModelPresentationSource, msg: Translator = getTranslator("ru")): ModelPresentation {
  return {
    artworkSrc: model.artworkSrc,
    description: localizedText(model.description_translations, msg.locale,
      model.description ? translateCatalogText(model.description, msg) : msg("modelCardContent.valueIsAvailableInNeirohub", { value1: model.name })),
    href: modelWorkspaceHref(model),
  };
}
