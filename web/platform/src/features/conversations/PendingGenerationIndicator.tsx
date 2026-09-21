"use client";

import { AssistantTypingIndicator } from "@/components/chat/AssistantTypingIndicator/AssistantTypingIndicator";
import { ImageGenerationPlaceholders, type PendingImagePreview } from "@/features/image-generation/ImageGenerationGrid/ImageGenerationGrid";
import { useDictionary } from "@/i18n/LocaleProvider";

export function PendingGenerationIndicator({ image }: Readonly<{ image?: PendingImagePreview }>) {
  const t = useDictionary();
  return image ? <ImageGenerationPlaceholders {...image} /> : <AssistantTypingIndicator label={t.conversations.composerAwaitingResponse} />;
}
