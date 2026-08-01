"use client";

/* eslint-disable @next/next/no-img-element */

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import {
  parseImageJobActivation,
  parseImageJobPreparation,
  parseImageJobResult,
  parseImageModelList,
  type ImageJob,
  type ImageJobPreparation,
  type ImageJobResult,
  type ImageModel,
} from "@/lib/web-api/contracts";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

import styles from "./ImageGenerationPanel.module.css";

type PanelStage = "closed" | "loading" | "editor" | "preparing" | "confirmation" | "activating" | "tracking" | "result";

type PrepareIntent = {
  prompt: string;
  modelID: string;
  imageQuality: string;
  idempotencyKey: string;
};

const completedFailureStatuses = new Set<ImageJob["status"]>([
  "rejected",
  "failed_terminal",
  "cancelled",
  "expired",
  "refunded",
]);

export function ImageGenerationPanel() {
  const [stage, setStage] = useState<PanelStage>("closed");
  const [models, setModels] = useState<ImageModel[]>([]);
  const [modelID, setModelID] = useState("");
  const [imageQuality, setImageQuality] = useState("");
  const [prompt, setPrompt] = useState("");
  const [prepareIntent, setPrepareIntent] = useState<PrepareIntent | null>(null);
  const [preparation, setPreparation] = useState<ImageJobPreparation | null>(null);
  const [activeJob, setActiveJob] = useState<ImageJob | null>(null);
  const [result, setResult] = useState<ImageJobResult | null>(null);
  const [error, setError] = useState<"load" | "prepare" | "activation" | "insufficient" | "status" | "result" | null>(null);

  const selectedModel = useMemo(() => models.find((model) => model.id === modelID) ?? null, [modelID, models]);
  const modelQualities = selectedModel?.quality_options ?? [];
  const canPrepare = stage === "editor" && prompt.trim() !== "" && selectedModel !== null && imageQuality !== "";
  const canRefreshStatus =
    stage === "tracking" &&
    activeJob !== null &&
    (!isImageJobTerminal(activeJob) || (activeJob.status === "succeeded" && error === "result"));

  const resetExpiredPreparation = () => {
    setPrepareIntent(null);
    setPreparation(null);
    setError("prepare");
    setStage("editor");
  };

  const openGenerator = async () => {
    if (stage === "loading") {
      return;
    }

    setError(null);
    setStage("loading");
    try {
      const response = await webBrowserFetch("/web/v1/image-models");
      if (response.status !== 200) {
        throw new Error("Unable to load image models.");
      }
      const catalog = parseImageModelList(await response.json());
      const firstModel = catalog.items[0];
      if (!firstModel) {
        throw new Error("No image models available.");
      }
      setModels(catalog.items);
      setModelID(firstModel.id);
      setImageQuality(firstModel.default_quality);
      setStage("editor");
    } catch {
      setError("load");
      setStage("closed");
    }
  };

  const selectModel = (nextModelID: string) => {
    const nextModel = models.find((model) => model.id === nextModelID);
    if (!nextModel) {
      return;
    }
    setModelID(nextModel.id);
    setImageQuality(nextModel.default_quality);
    setPrepareIntent(null);
    setError(null);
  };

  const changeImageQuality = (nextQuality: string) => {
    setImageQuality(nextQuality);
    setPrepareIntent(null);
    setError(null);
  };

  const changePrompt = (nextPrompt: string) => {
    setPrompt(nextPrompt);
    setPrepareIntent(null);
    setError(null);
  };

  const prepareImage = async () => {
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
  };

  const activateImage = async () => {
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
      setActiveJob(activation.job);
      setResult(null);
      setStage("tracking");
    } catch {
      setError("activation");
      setStage("confirmation");
    }
  };

  const refreshStatus = async () => {
    if (activeJob === null || !canRefreshStatus) {
      return;
    }

    setError(null);
    let updatedJob: ImageJob;
    try {
      const response = await webBrowserFetch(`/web/v1/image-jobs/${activeJob.id}`);
      if (response.status !== 200) {
        throw new Error("Unable to load image job.");
      }
      updatedJob = parseImageJobActivation(await response.json()).job;
    } catch {
      setError("status");
      return;
    }

    setActiveJob(updatedJob);
    if (updatedJob.status !== "succeeded") {
      return;
    }

    try {
      const resultResponse = await webBrowserFetch(`/web/v1/image-jobs/${updatedJob.id}/result`);
      if (resultResponse.status !== 200) {
        setError("result");
        return;
      }
      setResult(parseImageJobResult(await resultResponse.json()));
      setStage("result");
    } catch {
      setError("result");
    }
  };

  const createAnother = () => {
    setPrompt("");
    setPrepareIntent(null);
    setPreparation(null);
    setActiveJob(null);
    setResult(null);
    setError(null);
    setStage("editor");
  };

  return (
    <section aria-labelledby="image-generation-title" className={styles.panel}>
      <header className={styles.header}>
        <h2 id="image-generation-title">{ru.imageGeneration.title}</h2>
        <p>{ru.imageGeneration.description}</p>
      </header>

      {stage === "closed" ? (
        <div className={styles.closedState}>
          <Button onClick={openGenerator}>{ru.imageGeneration.open}</Button>
          {error === "load" ? (
            <p className={styles.error} role="alert">
              {ru.imageGeneration.loadingFailure}
            </p>
          ) : null}
        </div>
      ) : null}

      {stage === "loading" ? <p role="status">{ru.imageGeneration.loadingModels}</p> : null}

      {stage === "editor" || stage === "preparing" ? (
        <form
          className={styles.editor}
          onSubmit={(event) => {
            event.preventDefault();
            void prepareImage();
          }}
        >
          <label>
            <span>{ru.imageGeneration.modelLabel}</span>
            <select disabled={stage === "preparing"} onChange={(event) => selectModel(event.target.value)} value={modelID}>
              {models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{ru.imageGeneration.qualityLabel}</span>
            <select
              disabled={stage === "preparing" || modelQualities.length === 0}
              onChange={(event) => changeImageQuality(event.target.value)}
              value={imageQuality}
            >
              {modelQualities.map((quality) => (
                <option key={quality} value={quality}>
                  {quality}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.promptField}>
            <span>{ru.imageGeneration.promptLabel}</span>
            <textarea
              disabled={stage === "preparing"}
              onChange={(event) => changePrompt(event.target.value)}
              placeholder={ru.imageGeneration.promptPlaceholder}
              required
              rows={5}
              value={prompt}
            />
          </label>
          <Button disabled={!canPrepare} type="submit">
            {stage === "preparing" ? ru.imageGeneration.preparing : ru.imageGeneration.prepare}
          </Button>
          {error === "prepare" ? (
            <p className={styles.error} role="alert">
              {ru.imageGeneration.prepareFailure}
            </p>
          ) : null}
        </form>
      ) : null}

      {stage === "confirmation" && preparation !== null ? (
        <section aria-labelledby="image-confirmation-title" className={styles.confirmation}>
          <h3 id="image-confirmation-title">{ru.imageGeneration.confirmationTitle}</h3>
          <dl>
            <div>
              <dt>{ru.imageGeneration.costLabel}</dt>
              <dd>{formatStars(preparation.job.cost_estimate)}</dd>
            </div>
            <div>
              <dt>{ru.imageGeneration.balanceLabel}</dt>
              <dd>{formatStars(preparation.balance)}</dd>
            </div>
            <div>
              <dt>{ru.imageGeneration.balanceAfterLabel}</dt>
              <dd>{formatStars(Math.max(0, preparation.balance - preparation.job.cost_estimate))}</dd>
            </div>
          </dl>
          <Button onClick={activateImage}>
            {ru.imageGeneration.confirm} · {formatStars(preparation.job.cost_estimate)}
          </Button>
          {error === "insufficient" ? (
            <p className={styles.error} role="alert">
              {ru.imageGeneration.insufficientBalance}
            </p>
          ) : null}
          {error === "activation" ? (
            <p className={styles.error} role="alert">
              {ru.imageGeneration.activationFailure}
            </p>
          ) : null}
        </section>
      ) : null}

      {stage === "activating" ? <p role="status">{ru.imageGeneration.activating}</p> : null}

      {stage === "tracking" && activeJob !== null ? (
        <JobStatus job={activeJob} error={error} onRefresh={refreshStatus} refreshable={canRefreshStatus} />
      ) : null}

      {stage === "result" && result !== null ? (
        <section aria-labelledby="image-result-title" className={styles.result}>
          <h3 id="image-result-title">{ru.imageGeneration.resultTitle}</h3>
          <div className={styles.artifacts}>
            {result.artifacts.map((artifact) => (
              <img
                alt={ru.imageGeneration.resultImageAlt}
                height={artifact.height || undefined}
                key={artifact.id}
                src={`/web/v1/image-artifacts/${artifact.id}`}
                width={artifact.width || undefined}
              />
            ))}
          </div>
          <Button onClick={createAnother}>{ru.imageGeneration.createAnother}</Button>
        </section>
      ) : null}
    </section>
  );
}

function JobStatus({
  job,
  error,
  onRefresh,
  refreshable,
}: Readonly<{
  job: ImageJob;
  error: "load" | "prepare" | "activation" | "insufficient" | "status" | "result" | null;
  onRefresh: () => Promise<void>;
  refreshable: boolean;
}>) {
  const [isRefreshing, setIsRefreshing] = useState(false);

  const refresh = async () => {
    if (isRefreshing) {
      return;
    }
    setIsRefreshing(true);
    try {
      await onRefresh();
    } finally {
      setIsRefreshing(false);
    }
  };

  return (
    <section aria-labelledby="image-job-status-title" className={styles.status}>
      <h3 id="image-job-status-title">{ru.imageGeneration.statusTitle}</h3>
      <p className={styles.statusValue}>{imageJobStatusLabel(job.status)}</p>
      {refreshable ? (
        <Button disabled={isRefreshing} onClick={refresh}>
          {isRefreshing ? ru.imageGeneration.statusRefreshing : ru.imageGeneration.statusRefresh}
        </Button>
      ) : null}
      {error === "status" ? (
        <p className={styles.error} role="alert">
          {ru.imageGeneration.statusFailure}
        </p>
      ) : null}
      {error === "result" ? (
        <p className={styles.error} role="alert">
          {ru.imageGeneration.resultFailure}
        </p>
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

function isImageJobTerminal(job: ImageJob): boolean {
  return job.status === "succeeded" || completedFailureStatuses.has(job.status);
}

function imageJobStatusLabel(status: ImageJob["status"]): string {
  const labels: Record<ImageJob["status"], string> = {
    prepared: "Ожидает подтверждения",
    received: "Получена",
    validated: "Проверяется",
    rejected: "Отклонена",
    awaiting_payment: "Ожидает пополнения",
    credits_reserved: "Готовится к запуску",
    queued: "В очереди",
    dispatching_provider: "Передаётся в обработку",
    provider_submitted: "Передана в обработку",
    provider_pending: "Ожидает обработки",
    provider_processing: "Генерируется",
    provider_succeeded: "Результат обрабатывается",
    provider_failed: "Не удалось обработать",
    postprocessing: "Подготавливаем результат",
    result_ready: "Результат готовится к выдаче",
    delivering: "Результат выдаётся",
    succeeded: "Готово",
    failed_retryable: "Временная ошибка",
    failed_terminal: "Не удалось завершить",
    cancelled: "Отменена",
    expired: "Срок задачи истёк",
    refunded: "Средства возвращены",
  };
  return labels[status];
}

function formatStars(value: number): string {
  return `${value} ★`;
}
