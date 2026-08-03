"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";

import { Button } from "@/components/ui/Button/Button";
import { ImageGenerationConfirmation } from "@/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation";
import { ImageGenerationEditor } from "@/features/image-generation/ImageGenerationEditor/ImageGenerationEditor";
import { ImageGenerationResult } from "@/features/image-generation/ImageGenerationResult/ImageGenerationResult";
import { ImageJobTracker } from "@/features/image-generation/ImageJobTracker/ImageJobTracker";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { ru } from "@/i18n/ru";
import {
  parseImageJobActivation,
  parseImageJobPreparation,
  type ImageJob,
  type ImageJobPreparation,
  type ImageJobResult,
  type ImageModel,
} from "@/lib/web-api/contracts";
import { webBrowserMutation } from "@/lib/web-api/browser";

import styles from "./ImageGenerationPanel.module.css";

type PanelStage = "loading" | "loadFailure" | "editor" | "preparing" | "confirmation" | "activating" | "tracking" | "result";

type PrepareIntent = {
  prompt: string;
  modelID: string;
  imageQuality: string;
  idempotencyKey: string;
};

type ImageGenerationPanelProps = {
  onJobChange?: (job: ImageJob) => void;
};

export function ImageGenerationPanel({ onJobChange }: Readonly<ImageGenerationPanelProps>) {
  const requestedModelID = useSearchParams().get("model");
  const [stage, setStage] = useState<PanelStage>("loading");
  const [catalogLoadAttempt, setCatalogLoadAttempt] = useState(0);
  const [models, setModels] = useState<ImageModel[]>([]);
  const [modelID, setModelID] = useState("");
  const [imageQuality, setImageQuality] = useState("");
  const [prompt, setPrompt] = useState("");
  const [prepareIntent, setPrepareIntent] = useState<PrepareIntent | null>(null);
  const [preparation, setPreparation] = useState<ImageJobPreparation | null>(null);
  const [activeJob, setActiveJob] = useState<ImageJob | null>(null);
  const [result, setResult] = useState<ImageJobResult | null>(null);
  const [error, setError] = useState<"load" | "noModels" | "prepare" | "activation" | "insufficient" | null>(null);

  const selectedModel = useMemo(() => models.find((model) => model.id === modelID) ?? null, [modelID, models]);
  const canPrepare = stage === "editor" && prompt.trim() !== "" && selectedModel !== null && imageQuality !== "";

  useEffect(() => {
    let active = true;

    const loadModels = async () => {
      try {
        const catalog = await loadImageModelCatalog();
        if (!active) {
          return;
        }
        const requestedModel = requestedModelID === null
          ? undefined
          : catalog.items.find((model) => model.id === requestedModelID);
        const initialModel = requestedModel ?? catalog.items[0];
        if (!initialModel) {
          setError("noModels");
          setStage("loadFailure");
          return;
        }
        setModels(catalog.items);
        setModelID(initialModel.id);
        setImageQuality(initialModel.default_quality);
        setStage("editor");
      } catch {
        if (active) {
          setError("load");
          setStage("loadFailure");
        }
      }
    };

    void loadModels();
    return () => {
      active = false;
    };
  }, [catalogLoadAttempt, requestedModelID]);

  const retryModelCatalog = () => {
    setError(null);
    setStage("loading");
    setCatalogLoadAttempt((current) => current + 1);
  };

  const resetExpiredPreparation = useCallback(() => {
    setPrepareIntent(null);
    setPreparation(null);
    setError("prepare");
    setStage("editor");
  }, []);

  const selectModel = useCallback((nextModelID: string) => {
    const nextModel = models.find((model) => model.id === nextModelID);
    if (!nextModel) {
      return;
    }
    setModelID(nextModel.id);
    setImageQuality(nextModel.default_quality);
    setPrepareIntent(null);
    setError(null);
  }, [models]);

  const changeImageQuality = useCallback((nextQuality: string) => {
    setImageQuality(nextQuality);
    setPrepareIntent(null);
    setError(null);
  }, []);

  const changePrompt = useCallback((nextPrompt: string) => {
    setPrompt(nextPrompt);
    setPrepareIntent(null);
    setError(null);
  }, []);

  const prepareImage = useCallback(async () => {
    if (!canPrepare || selectedModel === null) {
      return;
    }

    const normalizedPrompt = prompt.trim();
    const intent = prepareIntentMatches(prepareIntent, normalizedPrompt, selectedModel.id, imageQuality)
      ? prepareIntent
      : {
          prompt: normalizedPrompt,
          modelID: selectedModel.id,
          imageQuality,
          idempotencyKey: crypto.randomUUID(),
        };
    setPrepareIntent(intent);
    setError(null);
    setStage("preparing");
    try {
      const response = await webBrowserMutation("/web/v1/image-jobs/prepare", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": intent.idempotencyKey,
        },
        body: JSON.stringify({
          prompt: intent.prompt,
          model_id: intent.modelID,
          image_quality: intent.imageQuality,
        }),
      });
      if (response.status === 409) {
        resetExpiredPreparation();
        return;
      }
      if (response.status !== 201) {
        throw new Error("Unable to prepare image generation.");
      }
      setPreparation(parseImageJobPreparation(await response.json()));
      setStage("confirmation");
    } catch {
      setError("prepare");
      setStage("editor");
    }
  }, [canPrepare, imageQuality, prepareIntent, prompt, resetExpiredPreparation, selectedModel]);

  const handleJobUpdate = useCallback((nextJob: ImageJob) => {
    setActiveJob(nextJob);
    onJobChange?.(nextJob);
  }, [onJobChange]);

  const activateImage = useCallback(async () => {
    if (preparation === null || stage === "activating") {
      return;
    }

    setError(null);
    setStage("activating");
    try {
      const response = await webBrowserMutation(`/web/v1/image-jobs/${preparation.job.id}/activate`, {
        method: "POST",
      });
      if (response.status === 409) {
        resetExpiredPreparation();
        return;
      }
      if (response.status !== 200 && response.status !== 402) {
        throw new Error("Unable to activate image generation.");
      }
      const activation = parseImageJobActivation(await response.json());
      if (response.status === 402) {
        setPreparation((current) => (current === null ? null : { ...current, job: activation.job }));
        setError("insufficient");
        setStage("confirmation");
        return;
      }
      handleJobUpdate(activation.job);
      setResult(null);
      setStage("tracking");
    } catch {
      setError("activation");
      setStage("confirmation");
    }
  }, [handleJobUpdate, preparation, resetExpiredPreparation, stage]);

  const showResult = useCallback((nextResult: ImageJobResult) => {
    setResult(nextResult);
    setStage("result");
  }, []);

  const createAnother = useCallback((nextPrompt: string) => {
    setPrompt(nextPrompt);
    setPrepareIntent(null);
    setPreparation(null);
    setActiveJob(null);
    setResult(null);
    setError(null);
    setStage("editor");
  }, []);

  const editorError = error === "prepare" ? ru.imageGeneration.prepareFailure : null;
  const confirmationError = error === "insufficient"
    ? ru.imageGeneration.insufficientBalance
    : error === "activation"
      ? ru.imageGeneration.activationFailure
      : null;
  const loadFailure = error === "noModels" ? ru.imageGeneration.noModels : ru.imageGeneration.loadingFailure;

  return (
    <section aria-labelledby="image-generation-title" className={styles.panel}>
      <header className={styles.header}>
        <h2 id="image-generation-title">{ru.imageGeneration.title}</h2>
        <p>{ru.imageGeneration.description}</p>
      </header>

      {stage === "loading" ? <p role="status">{ru.imageGeneration.loadingModels}</p> : null}

      {stage === "loadFailure" ? (
        <div className={styles.loadFailure}>
          <p className={styles.error} role="alert">{loadFailure}</p>
          <Button onClick={retryModelCatalog}>{ru.imageGeneration.retryModels}</Button>
        </div>
      ) : null}

      {stage === "editor" || stage === "preparing" ? (
        <ImageGenerationEditor
          canSubmit={canPrepare}
          errorMessage={editorError}
          imageQuality={imageQuality}
          isSubmitting={stage === "preparing"}
          modelID={modelID}
          models={models}
          onImageQualityChange={changeImageQuality}
          onModelChange={selectModel}
          onPromptChange={changePrompt}
          onSubmit={() => void prepareImage()}
          prompt={prompt}
        />
      ) : null}

      {((stage === "confirmation" || stage === "activating") && preparation !== null) ? (
        <ImageGenerationConfirmation
          errorMessage={confirmationError}
          isActivating={stage === "activating"}
          onConfirm={() => void activateImage()}
          preparation={preparation}
        />
      ) : null}

      {stage === "tracking" && activeJob !== null ? (
        <ImageJobTracker job={activeJob} onJobUpdate={handleJobUpdate} onResult={showResult} />
      ) : null}

      {stage === "result" && result !== null ? (
        <ImageGenerationResult onCreateAnother={createAnother} prompt={prompt} result={result} />
      ) : null}
    </section>
  );
}

function prepareIntentMatches(
  intent: PrepareIntent | null,
  prompt: string,
  modelID: string,
  imageQuality: string,
): intent is PrepareIntent {
  return intent !== null && intent.prompt === prompt && intent.modelID === modelID && intent.imageQuality === imageQuality;
}
