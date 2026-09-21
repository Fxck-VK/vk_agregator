"use client";

import { useDictionary } from "@/i18n/LocaleProvider";


import { ImageGenerationGuide } from "@/features/image-generation/ImageGenerationGuide/ImageGenerationGuide";
import { ImageGenerationPanel } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel";

import styles from "./ImageWorkspace.module.css";

export function ImageWorkspace() {
  const t = useDictionary();
  return (
    <section aria-label={t.imageGeneration.title} className={styles.workspace}>
      <ImageGenerationPanel />
      <ImageGenerationGuide />
    </section>
  );
}
