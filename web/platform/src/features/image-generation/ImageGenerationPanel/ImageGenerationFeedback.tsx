"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";


import { Button } from "@/components/ui/Button/Button";
import { ImageGenerationConfirmation } from "@/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation";
import { ImageGenerationResult } from "@/features/image-generation/ImageGenerationResult/ImageGenerationResult";
import { ImageJobTracker } from "@/features/image-generation/ImageJobTracker/ImageJobTracker";

import type { ImageGenerationController } from "./useImageGeneration";


type ImageGenerationFeedbackProps = {
  generation: ImageGenerationController;
  showEditorError?: boolean;
};

export function ImageGenerationFeedback({ generation, showEditorError = false }: ImageGenerationFeedbackProps) {
  const t = useDictionary();
  const { stage, preparation, activeJob, result } = generation;

  return (
    <>
      {stage === "loading" ? <StateNotice inline kind="loading">{t.imageGeneration.loadingModels}</StateNotice> : null}
      {stage === "loadFailure" ? (
        <div>
          <StateNotice inline kind="error">{generation.loadFailure}</StateNotice>
          <Button variant="outline" onClick={generation.retryModelCatalog}>{t.imageGeneration.retryModels}</Button>
        </div>
      ) : null}
      {showEditorError && stage === "editor" && generation.editorError !== null ? (
        <StateNotice inline kind="error">{generation.editorError}</StateNotice>
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
