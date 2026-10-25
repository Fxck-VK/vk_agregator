"use client";

import { useEffect, useState } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import { clearImagePrompt, readImagePrompt } from "../image-prompt-draft";


import { useSearchParams } from "@/i18n/navigation";

import { ImageGenerationComposer } from "@/features/image-generation/ImageGenerationComposer/ImageGenerationComposer";

import { ImageGenerationFeedback } from "./ImageGenerationFeedback";
import { useImageGeneration, type ImageGenerationOptions } from "./useImageGeneration";
import styles from "./ImageGenerationPanel.module.css";

type ImageGenerationPanelProps = Omit<ImageGenerationOptions, "initialValues">;

export function ImageGenerationPanel(props: Readonly<ImageGenerationPanelProps>) {
  const t = useDictionary();
  const searchParams = useSearchParams();
  const [landingPrompt] = useState(readImagePrompt);
  useEffect(() => { clearImagePrompt(landingPrompt); }, [landingPrompt]);
  const generation = useImageGeneration({
    ...props,
    initialValues: {
      modelID: searchParams.get("model"),
      imageQuality: searchParams.get("quality"),
      prompt: landingPrompt || searchParams.get("prompt") || "",
    },
  });

  return (
    <section aria-label={t.imageGeneration.title} className={styles.panel}>
      {(generation.stage === "editor" || generation.stage === "preparing") && generation.selectedModel !== null ? (
        <ImageGenerationComposer {...generation.composerProps} />
      ) : null}
      <ImageGenerationFeedback generation={generation} />
    </section>
  );
}
