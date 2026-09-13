"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";

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

export type ImageGenerationOptions = {
  initialValues?: { modelID: string | null; imageQuality: string | null; prompt: string };
  access?: "authenticated" | "guest";
  model?: ImageModel | null;
  promptValue?: string;
  onPromptChange?: (prompt: string) => void;
  onBusyChange?: (busy: boolean) => void;
  onJobChange?: (job: ImageJob) => void;
};

export function useImageGeneration({ access = "authenticated", model, promptValue, onPromptChange, onBusyChange, onJobChange, initialValues }: Readonly<ImageGenerationOptions>) {
  const router = useRouter();
  const workspaceModelSelection = useWorkspaceModelSelection();
  const setWorkspaceModelId = workspaceModelSelection?.setSelectedModelId;
  const selectedWorkspaceModelID = workspaceModelSelection?.selectedModelId ?? null;
  const initialRequest = useRef({
    modelID: initialValues?.modelID ?? null,
    imageQuality: initialValues?.imageQuality ?? null,
    prompt: initialValues?.prompt ?? "",
    workspaceModelID: selectedWorkspaceModelID,
  });
  const [stage, setStage] = useState<PanelStage>(model === undefined ? "loading" : "editor");
  const [catalogLoadAttempt, setCatalogLoadAttempt] = useState(0);
  const [models, setModels] = useState<ImageModel[]>([]);
  const [fallbackModelID, setFallbackModelID] = useState("");
  const [qualitySelection, setQualitySelection] = useState<QualitySelection>({ modelID: "", value: "" });
  const [localPrompt, setLocalPrompt] = useState("");
  const prompt = promptValue ?? localPrompt;
  const setPrompt = useCallback((value: string) => {
    setLocalPrompt(value);
    onPromptChange?.(value);
  }, [onPromptChange]);
  const [aspectRatio, setAspectRatio] = useState("16:9");
  const [outputCountSelection, setOutputCountSelection] = useState<OutputCountSelection>({ modelID: "", value: 1 });
  const [prepareIntent, setPrepareIntent] = useState<PrepareIntent | null>(null);
  const [preparation, setPreparation] = useState<ImageJobPreparation | null>(null);
  const [activeJob, setActiveJob] = useState<ImageJob | null>(null);
  const [result, setResult] = useState<ImageJobResult | null>(null);
  const [error, setError] = useState<"load" | "noModels" | "prepare" | "activation" | "insufficient" | null>(null);

  const selectedModel = useMemo(() => {
    if (model !== undefined) return model;
    const workspaceModel = selectedWorkspaceModelID === null
      ? undefined
      : models.find((model) => model.id === selectedWorkspaceModelID);
    return workspaceModel ?? models.find((model) => model.id === fallbackModelID) ?? null;
  }, [fallbackModelID, model, models, selectedWorkspaceModelID]);
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

  const busy = stage === "preparing" || stage === "confirmation" || stage === "activating" || stage === "tracking";
  useEffect(() => {
    onBusyChange?.(busy);
  }, [busy, onBusyChange]);

  useEffect(() => {
    // A parent can supply a model from the shared catalogue for an inline composer.
    // In that case route parameters and the header's selection do not override it.
    if (model !== undefined) return;
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
  }, [catalogLoadAttempt, model, setPrompt, setWorkspaceModelId]);

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
  }, [setPrompt]);

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

    if (access === "guest") {
      router.push("/login");
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
  }, [access, aspectRatio, canPrepare, imageQuality, outputCount, prepareIntent, prompt, resetExpiredPreparation, router, selectedModel]);

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
  }, [setPrompt]);

  const editorError = error === "prepare" ? ru.imageGeneration.prepareFailure : null;
  const confirmationError = error === "insufficient"
    ? ru.imageGeneration.insufficientBalance
    : error === "activation"
      ? ru.imageGeneration.activationFailure
      : null;
  const loadFailure = error === "noModels" ? ru.imageGeneration.noModels : ru.imageGeneration.loadingFailure;

  const reset = useCallback(() => {
    if (busy) return;
    setQualitySelection({ modelID: "", value: "" });
    setAspectRatio("16:9");
    setOutputCountSelection({ modelID: "", value: 1 });
    setPrepareIntent(null);
    setPreparation(null);
    setActiveJob(null);
    setResult(null);
    setError(null);
    setStage("editor");
  }, [busy]);

  return {
    stage, busy, selectedModel, canPrepare, prompt, preparation, activeJob, result,
    editorError, confirmationError, loadFailure,
    retryModelCatalog, prepareImage, activateImage, handleJobUpdate, showResult, createAnother, changePrompt, reset,
    composerProps: {
      access,
      aspectRatio,
      canSubmit: canPrepare,
      errorMessage: editorError,
      imageQuality,
      isSubmitting: stage === "preparing",
      maxOutputCount,
      onAspectRatioChange: changeAspectRatio,
      onImageQualityChange: changeImageQuality,
      onOutputCountChange: changeOutputCount,
      onPromptChange: changePrompt,
      onSubmit: () => void prepareImage(),
      price: selectedPrice,
      outputCount,
      prompt,
      qualityOptions: selectedModel?.quality_options ?? [],
    },
  };
}

export type ImageGenerationController = ReturnType<typeof useImageGeneration>;

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
