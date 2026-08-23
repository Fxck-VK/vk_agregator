"use client";

import { useCallback, useState } from "react";

import { ImageGenerationGuide } from "@/features/image-generation/ImageGenerationGuide/ImageGenerationGuide";
import { ImageGenerationPanel } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationPanel";
import { ImageJobHistory } from "@/features/image-generation/ImageJobHistory/ImageJobHistory";
import { ru } from "@/i18n/ru";
import type { ImageJob } from "@/lib/web-api/contracts";

import styles from "./ImageWorkspace.module.css";

export function ImageWorkspace() {
  const [latestJob, setLatestJob] = useState<ImageJob | null>(null);
  const handleJobChange = useCallback((job: ImageJob) => {
    setLatestJob(job);
  }, []);

  return (
    <section aria-label={ru.imageGeneration.title} className={styles.workspace}>
      <ImageGenerationPanel onJobChange={handleJobChange} />
      <ImageGenerationGuide />
      <ImageJobHistory latestJob={latestJob} />
    </section>
  );
}
