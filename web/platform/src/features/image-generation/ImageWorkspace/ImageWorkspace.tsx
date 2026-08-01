import { ImageGenerationPanel } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel";
import { ImageJobHistory } from "@/features/image-generation/ImageJobHistory/ImageJobHistory";
import { ru } from "@/i18n/ru";

import styles from "./ImageWorkspace.module.css";

export function ImageWorkspace() {
  return (
    <section aria-label={ru.imageGeneration.title} className={styles.workspace}>
      <ImageGenerationPanel />
      <ImageJobHistory />
    </section>
  );
}
