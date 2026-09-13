"use client";

import { Button } from "@/components/ui/Button/Button";
import { ImageGenerationConfirmation } from "@/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation";
import { ImageGenerationResult } from "@/features/image-generation/ImageGenerationResult/ImageGenerationResult";
import { ImageJobTracker } from "@/features/image-generation/ImageJobTracker/ImageJobTracker";
import { ru } from "@/i18n/ru";

import type { ImageGenerationController } from "./useImageGeneration";
import styles from "./ImageGenerationPanel.module.css";

type ImageGenerationFeedbackProps = {
  generation: ImageGenerationController;
  showEditorError?: boolean;
};

export function ImageGenerationFeedback({ generation, showEditorError = false }: ImageGenerationFeedbackProps) {
  const { stage, preparation, activeJob, result } = generation;

  return (
    <>
      {stage === "loading" ? <p role="status">{ru.imageGeneration.loadingModels}</p> : null}
      {stage === "loadFailure" ? (
        <div className={styles.loadFailure}>
          <p className={styles.error} role="alert">{generation.loadFailure}</p>
          <Button onClick={generation.retryModelCatalog}>{ru.imageGeneration.retryModels}</Button>
        </div>
      ) : null}
      {showEditorError && stage === "editor" && generation.editorError !== null ? (
        <p className={styles.error} role="alert">{generation.editorError}</p>
      ) : null}
      {(stage === "confirmation" || stage === "activating") && preparation !== null ? (
        <ImageGenerationConfirmation
          errorMessage={generation.confirmationError}
          isActivating={stage === "activating"}
          onConfirm={() => void generation.activateImage()}
          preparation={preparation}
        />
      ) : null}
      {stage === "tracking" && activeJob !== null ? (
        <ImageJobTracker job={activeJob} onJobUpdate={generation.handleJobUpdate} onResult={generation.showResult} />
      ) : null}
      {stage === "result" && result !== null ? (
        <ImageGenerationResult onCreateAnother={generation.createAnother} prompt={generation.prompt} result={result} />
      ) : null}
    </>
  );
}
