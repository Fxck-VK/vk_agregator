"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";

import { Button } from "@/components/ui/Button/Button";
import { ImageGenerationComposer } from "@/features/image-generation/ImageGenerationComposer/ImageGenerationComposer";
import { ImageGenerationConfirmation } from "@/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation";
import { ImageGenerationResult } from "@/features/image-generation/ImageGenerationResult/ImageGenerationResult";
import { ImageJobTracker } from "@/features/image-generation/ImageJobTracker/ImageJobTracker";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { useWorkspaceModelSelection } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";
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
  aspectRatio: string;
  outputCount: number;
  idempotencyKey: string;
};

type QualitySelection = {
  modelID: string;
  value: string;
};

type OutputCountSelection = {
  modelID: string;
  value: number;
};

type ImageGenerationPanelProps = {
  onJobChange?: (job: ImageJob) => void;
};

export function ImageGenerationPanel({ onJobChange }: Readonly<ImageGenerationPanelProps>) {
  const searchParams = useSearchParams();
  const workspaceModelSelection = useWorkspaceModelSelection();
  const setWorkspaceModelId = workspaceModelSelection?.setSelectedModelId;
  const selectedWorkspaceModelID = workspaceModelSelection?.selectedModelId ?? null;
  const initialRequest = useRef({
    modelID: searchParams.get("model"),
    imageQuality: searchParams.get("quality"),
    prompt: searchParams.get("prompt") ?? "",
    workspaceModelID: selectedWorkspaceModelID,
  });
  const [stage, setStage] = useState<PanelStage>("loading");
  const [catalogLoadAttempt, setCatalogLoadAttempt] = useState(0);
  const [models, setModels] = useState<ImageModel[]>([]);
  const [fallbackModelID, setFallbackModelID] = useState("");
  const [qualitySelection, setQualitySelection] = useState<QualitySelection>({ modelID: "", value: "" });
  const [prompt, setPrompt] = useState("");
  const [aspectRatio, setAspectRatio] = useState("16:9");
  const [outputCountSelection, setOutputCountSelection] = useState<OutputCountSelection>({ modelID: "", value: 1 });
  const [prepareIntent, setPrepareIntent] = useState<PrepareIntent | null>(null);
  const [preparation, setPreparation] = useState<ImageJobPreparation | null>(null);
  const [activeJob, setActiveJob] = useState<ImageJob | null>(null);
  const [result, setResult] = useState<ImageJobResult | null>(null);
  const [error, setError] = useState<"load" | "noModels" | "prepare" | "activation" | "insufficient" | null>(null);

  const selectedModel = useMemo(() => {
    const workspaceModel = selectedWorkspaceModelID === null
      ? undefined
      : models.find((model) => model.id === selectedWorkspaceModelID);
    return workspaceModel ?? models.find((model) => model.id === fallbackModelID) ?? null;
  }, [fallbackModelID, models, selectedWorkspaceModelID]);
  const imageQuality = selectedModel === null
    ? ""
    : qualitySelection.modelID === selectedModel.id && selectedModel.quality_options.includes(qualitySelection.value)
      ? qualitySelection.value
      : selectedModel.default_quality;
  const maxOutputCount = selectedModel?.max_output_count ?? 1;
  const outputCount = selectedModel !== null && outputCountSelection.modelID === selectedModel.id
    ? Math.min(Math.max(1, outputCountSelection.value), Math.max(1, maxOutputCount))
    : 1;
  const unitPrice = selectedModel?.price_by_quality?.[imageQuality] ?? null;
  const selectedPrice = unitPrice === null ? null : unitPrice * outputCount;
  const canPrepare = stage === "editor"
    && prompt.trim() !== ""
    && selectedModel !== null
    && imageQuality !== ""
    && selectedPrice !== null;

  useEffect(() => {
    let active = true;

    const loadModels = async () => {
      try {
        const catalog = await loadImageModelCatalog();
        if (!active) {
          return;
        }
        const requestedModel = initialRequest.current.modelID === null
          ? undefined
          : catalog.items.find((model) => model.id === initialRequest.current.modelID);
        const workspaceModel = initialRequest.current.workspaceModelID === null
          ? undefined
          : catalog.items.find((model) => model.id === initialRequest.current.workspaceModelID);
        const initialModel = requestedModel ?? workspaceModel ?? catalog.items[0];
        if (!initialModel) {
          setError("noModels");
          setStage("loadFailure");
          return;
        }
        setModels(catalog.items);
        setFallbackModelID(initialModel.id);
        setWorkspaceModelId?.(initialModel.id);
        setQualitySelection({
          modelID: initialModel.id,
          value: initialRequest.current.imageQuality !== null
            && initialModel.quality_options.includes(initialRequest.current.imageQuality)
              ? initialRequest.current.imageQuality
              : initialModel.default_quality,
        });
        setPrompt(initialRequest.current.prompt);
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
  }, [catalogLoadAttempt, setWorkspaceModelId]);

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

  const changeImageQuality = useCallback((nextQuality: string) => {
    if (selectedModel === null) {
      return;
    }
    setQualitySelection({ modelID: selectedModel.id, value: nextQuality });
    setPrepareIntent(null);
    setError(null);
  }, [selectedModel]);

  const changeAspectRatio = useCallback((nextAspectRatio: string) => {
    setAspectRatio(nextAspectRatio);
    setPrepareIntent(null);
    setError(null);
  }, []);

  const changePrompt = useCallback((nextPrompt: string) => {
    setPrompt(nextPrompt);
    setPrepareIntent(null);
    setError(null);
  }, []);

  const changeOutputCount = useCallback((nextOutputCount: number) => {
    if (selectedModel === null) {
      return;
    }
    setOutputCountSelection({
      modelID: selectedModel.id,
      value: Math.min(Math.max(1, nextOutputCount), Math.max(1, maxOutputCount)),
    });
    setPrepareIntent(null);
    setError(null);
  }, [maxOutputCount, selectedModel]);

  const prepareImage = useCallback(async () => {
    if (!canPrepare || selectedModel === null) {
      return;
    }

    const normalizedPrompt = prompt.trim();
    const intent = prepareIntentMatches(prepareIntent, normalizedPrompt, selectedModel.id, imageQuality, aspectRatio, outputCount)
      ? prepareIntent
      : {
          prompt: normalizedPrompt,
          modelID: selectedModel.id,
          imageQuality,
          aspectRatio,
          outputCount,
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
          aspect_ratio: intent.aspectRatio,
          output_count: intent.outputCount,
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
  }, [aspectRatio, canPrepare, imageQuality, outputCount, prepareIntent, prompt, resetExpiredPreparation, selectedModel]);

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
    <section aria-label={ru.imageGeneration.title} className={styles.panel}>
      {stage === "loading" ? <p role="status">{ru.imageGeneration.loadingModels}</p> : null}

      {stage === "loadFailure" ? (
        <div className={styles.loadFailure}>
          <p className={styles.error} role="alert">{loadFailure}</p>
          <Button onClick={retryModelCatalog}>{ru.imageGeneration.retryModels}</Button>
        </div>
      ) : null}

      {(stage === "editor" || stage === "preparing") && selectedModel !== null ? (
        <ImageGenerationComposer
          aspectRatio={aspectRatio}
          canSubmit={canPrepare}
          errorMessage={editorError}
          imageQuality={imageQuality}
          isSubmitting={stage === "preparing"}
          maxOutputCount={maxOutputCount}
          onAspectRatioChange={changeAspectRatio}
          onImageQualityChange={changeImageQuality}
          onOutputCountChange={changeOutputCount}
          onPromptChange={changePrompt}
          onSubmit={() => void prepareImage()}
          price={selectedPrice}
          outputCount={outputCount}
          prompt={prompt}
          qualityOptions={selectedModel.quality_options}
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
  aspectRatio: string,
  outputCount: number,
): intent is PrepareIntent {
  return intent !== null
    && intent.prompt === prompt
    && intent.modelID === modelID
    && intent.imageQuality === imageQuality
    && intent.aspectRatio === aspectRatio
    && intent.outputCount === outputCount;
}
