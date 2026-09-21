import { formatMessage, type MessageParameters } from "./format";
import type { Locale } from "./locales";
import { messagesRu } from "./messages/ru";
import { messagesEn } from "./messages/en";

export type MessageKey = keyof typeof messagesRu;
export type MessageCatalog = Record<MessageKey, string>;
export type Translator = ((key: MessageKey, parameters?: MessageParameters) => string) & { readonly locale: Locale };

export const messageCatalogs: Record<Locale, MessageCatalog> = { ru: messagesRu, en: messagesEn };
function createTranslator(locale: Locale): Translator {
  return Object.assign(
    (key: MessageKey, parameters?: MessageParameters) => formatMessage(messageCatalogs[locale][key], parameters),
    { locale },
  );
}
const translators = { ru: createTranslator("ru"), en: createTranslator("en") };

export function getTranslator(locale: Locale): Translator {
  return translators[locale];
}
