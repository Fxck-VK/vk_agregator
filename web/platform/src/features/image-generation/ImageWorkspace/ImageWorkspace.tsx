"use client";

import { ImageGenerationGuide } from "@/features/image-generation/ImageGenerationGuide/ImageGenerationGuide";
import { ImageGenerationPanel } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel";
import { ru } from "@/i18n/ru";

import styles from "./ImageWorkspace.module.css";

export function ImageWorkspace() {
  return (
    <section aria-label={ru.imageGeneration.title} className={styles.workspace}>
      <ImageGenerationPanel />
      <ImageGenerationGuide />
    </section>
  );
}
