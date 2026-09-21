import type { MessageParameters } from "./format";
import type { MessageKey, Translator } from "./messages";

// Persist codes and parameters, never a rendered translation or a raw provider error.
export type MessageReference = { key: MessageKey; parameters?: MessageParameters };

export class LocalizedError extends Error implements MessageReference {
  constructor(public readonly key: MessageKey, public readonly parameters?: MessageParameters) {
    super(key);
  }
}

export function renderMessage(msg: Translator, reference: MessageReference | null | undefined): string | null {
  return reference ? msg(reference.key, reference.parameters) : null;
}
