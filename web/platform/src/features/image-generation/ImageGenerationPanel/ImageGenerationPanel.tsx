"use client";

import { useSearchParams } from "next/navigation";

import { ImageGenerationComposer } from "@/features/image-generation/ImageGenerationComposer/ImageGenerationComposer";
import { ru } from "@/i18n/ru";

import { ImageGenerationFeedback } from "./ImageGenerationFeedback";
import { useImageGeneration, type ImageGenerationOptions } from "./useImageGeneration";
import styles from "./ImageGenerationPanel.module.css";

type ImageGenerationPanelProps = Omit<ImageGenerationOptions, "initialValues">;

export function ImageGenerationPanel(props: Readonly<ImageGenerationPanelProps>) {
  const searchParams = useSearchParams();
  const generation = useImageGeneration({
    ...props,
    initialValues: {
      modelID: searchParams.get("model"),
      imageQuality: searchParams.get("quality"),
      prompt: searchParams.get("prompt") ?? "",
    },
  });

  return (
    <section aria-label={ru.imageGeneration.title} className={styles.panel}>
      {(generation.stage === "editor" || generation.stage === "preparing") && generation.selectedModel !== null ? (
        <ImageGenerationComposer {...generation.composerProps} />
      ) : null}
      <ImageGenerationFeedback generation={generation} />
    </section>
  );
}
