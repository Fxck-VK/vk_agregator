"use client";

import { Fragment, type ReactNode } from "react";
import { useLocale } from "./LocaleProvider";
import { messageCatalogs, type MessageKey } from "./messages";

// Named slots let translators move emphasis/links without concatenating phrases.
// React escapes text and values; translations never contain executable HTML.
export function RichMessage({ id, values }: { id: MessageKey; values: Record<string, ReactNode> }) {
  const template = messageCatalogs[useLocale()][id];
  return template.split(/(\{[a-zA-Z][a-zA-Z0-9_]*\})/g).map((part, index) => {
    const key = /^\{(\w+)\}$/.exec(part)?.[1];
    if (key && !Object.hasOwn(values, key)) throw new Error(`Missing rich message parameter: ${key}`);
    return <Fragment key={index}>{key ? values[key] : part}</Fragment>;
  });
}
