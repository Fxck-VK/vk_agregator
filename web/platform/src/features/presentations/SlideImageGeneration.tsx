"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ImageGenerationConfirmation } from "@/features/image-generation/ImageGenerationConfirmation/ImageGenerationConfirmation";
import { useImageGeneration } from "@/features/image-generation/ImageGenerationPanel/useImageGeneration";
import { ImageJobTracker } from "@/features/image-generation/ImageJobTracker/ImageJobTracker";
import { isTerminalImageJobStatus } from "@/features/image-generation/ImageJobTracker/image-job-polling";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { useDictionary } from "@/i18n/LocaleProvider";
import { slideImageGenerationCopy as copy } from "@/i18n/presentation-slide-image";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import type { ImageJob, ImageJobResult, ImageModel } from "@/lib/web-api/contracts";

import styles from "./SlideImageGeneration.module.css";

type SlideImageGenerationProps = {
  prompt: string;
  onImage: (image: Blob) => void | Promise<void>;
  onClose: () => void;
};

const imageTypes = new Set(["image/jpeg", "image/png", "image/webp"]);
const maxImageBytes = 20 * 1024 * 1024;

export function SlideImageGeneration(props: Readonly<SlideImageGenerationProps>) {
  const t = useDictionary().imageGeneration;
  const { view } = useReleaseView();
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [dismissible, setDismissible] = useState(true);
  const [selection, setSelection] = useState<{
    view: typeof view;
    attempt: number;
    model: ImageModel | null;
    failed: boolean;
  }>();

  useEffect(() => {
    let current = true;
    void loadImageModelCatalog().then(({ items }) => {
      if (current) setSelection({ view, attempt: loadAttempt, model: items[0] ?? null, failed: false });
    }, () => {
      if (current) setSelection({ view, attempt: loadAttempt, model: null, failed: true });
    });
    return () => { current = false; };
  }, [view, loadAttempt]);

  const current = selection?.view === view && selection.attempt === loadAttempt ? selection : undefined;

  return (
    <LandingDialog title={copy.title} closeLabel={copy.close} onClose={props.onClose} dismissible={!current?.model || dismissible}>
      {!current ? <StateNotice inline kind="loading">{t.loadingModels}</StateNotice> : null}
      {current?.model ? <SlideImageJob {...props} model={current.model} onDismissibleChange={setDismissible} /> : null}
      {current && !current.model ? (
        <StateNotice inline kind="error" action={{ label: t.retryModels, onClick: () => setLoadAttempt(attempt => attempt + 1) }}>
          {current.failed ? t.loadingFailure : t.noModels}
        </StateNotice>
      ) : null}
    </LandingDialog>
  );
}

function SlideImageJob({ prompt, onImage, onClose, model, onDismissibleChange }: Readonly<SlideImageGenerationProps & {
  model: ImageModel;
  onDismissibleChange: (dismissible: boolean) => void;
}>) {
  const t = useDictionary().imageGeneration;
  const [description, setDescription] = useState(prompt);
  const [attachmentFailed, setAttachmentFailed] = useState(false);
  const [trackerFailed, setTrackerFailed] = useState(false);
  const attachment = useRef<AbortController | null>(null);
  const writePending = useRef(false);
  const content = useRef<HTMLDivElement>(null);
  const textarea = useRef<HTMLTextAreaElement>(null);
  const job = useImageGeneration({ model, promptValue: description, onPromptChange: setDescription });
  const { activeJob, preparation, handleJobUpdate, showResult, result, stage } = job;
  const preparedJobID = preparation?.job.id;

  useEffect(() => () => attachment.current?.abort(), []);
  useEffect(() => {
    if (stage === "editor") textarea.current?.focus();
    else if (document.activeElement === document.body) {
      const next = content.current?.querySelector<HTMLButtonElement>("button:not(:disabled)");
      const dialog = content.current?.closest<HTMLElement>("[role=dialog]");
      (next ?? dialog?.querySelector<HTMLButtonElement>("button:not(:disabled)") ?? dialog)?.focus();
    }
  }, [stage]);

  async function writeOnce(operation: () => Promise<void>) {
    if (writePending.current) return;
    writePending.current = true;
    try { await operation(); }
    finally { writePending.current = false; }
  }

  const attach = useCallback(async (next: ImageJobResult) => {
    if (attachment.current) return;
    const request = new AbortController();
    attachment.current = request;
    const signal = AbortSignal.any([request.signal, browserSessionSnapshot().signal]);
    setAttachmentFailed(false);
    try {
      const [artifact] = next.artifacts;
      if (next.job_id !== preparedJobID || next.artifacts.length !== 1 || artifact.deleted ||
          !imageTypes.has(artifact.mime_type) || artifact.size_bytes > maxImageBytes) {
        throw new Error("Slide image unavailable");
      }
      const response = await webBrowserFetch(`/web/v1/image-artifacts/${artifact.id}`, { signal, cache: "no-store", redirect: "error" });
      if (response.status !== 200) throw new Error("Slide image unavailable");
      const image = await response.blob();
      signal.throwIfAborted();
      if (image.type !== artifact.mime_type || image.size === 0 || image.size > maxImageBytes) {
        throw new Error("Slide image unavailable");
      }
      await onImage(image);
      if (!signal.aborted) onClose();
    } catch {
      if (!signal.aborted) setAttachmentFailed(true);
    } finally {
      attachment.current = null;
    }
  }, [onClose, onImage, preparedJobID]);

  const receiveResult = useCallback((next: ImageJobResult) => {
    if (next.job_id !== preparedJobID) throw new Error("Slide result does not match its job");
    showResult(next);
    void attach(next);
  }, [attach, preparedJobID, showResult]);

  const updateStatus = useCallback((next: ImageJob) => {
    if (next.id !== preparedJobID) {
      setTrackerFailed(true);
      return;
    }
    setTrackerFailed(false);
    handleJobUpdate(next);
  }, [handleJobUpdate, preparedJobID]);
  const trackError = useCallback(() => setTrackerFailed(true), []);

  const editing = stage === "editor" || stage === "preparing";
  const confirming = stage === "confirmation" || stage === "activating";
  const trackingJob = stage === "tracking" ? activeJob : null;
  const hasMatchingJob = trackingJob !== null && trackingJob.id === preparedJobID;
  const completed = stage === "result" ? result : null;
  const trackingPending = hasMatchingJob && !trackerFailed && !isTerminalImageJobStatus(trackingJob.status);
  const dismissible = stage !== "activating" && !trackingPending && (completed === null || attachmentFailed);
  const price = job.composerProps.price;

  useLayoutEffect(() => onDismissibleChange(dismissible), [onDismissibleChange, dismissible]);

  return (
    <div className={styles.content} ref={content}>
      {editing ? (
        <form className={styles.form} onSubmit={event => { event.preventDefault(); void writeOnce(job.prepareImage); }}>
          <label className={styles.field}>
            <span>{t.promptLabel}</span>
            <textarea ref={textarea} tabIndex={0} rows={4} value={description} disabled={stage === "preparing"} onChange={event => job.changePrompt(event.target.value)} />
          </label>
          <div className={styles.summary}>
            <span>{t.modelLabel} · {model.name}</span>
            {price === null ? <span>{t.priceUnavailable}</span> : <CreditAmount prefix={t.priceLabel} value={price} />}
          </div>
          {job.editorError ? <StateNotice inline kind="error">{job.editorError}</StateNotice> : null}
          <Button className={styles.submit} type="submit" disabled={!job.canPrepare}>
            {stage === "preparing" ? t.preparing : t.generate}
          </Button>
        </form>
      ) : null}
      {confirming && preparation ? (
        <>
          <ImageGenerationConfirmation
            balanceKnown={job.confirmationBalanceKnown}
            errorMessage={job.confirmationError}
            isActivating={stage === "activating"}
            onConfirm={() => void writeOnce(job.activateImage)}
            preparation={preparation}
          />
          {stage === "confirmation" && !job.confirmationError ? (
            <Button className={styles.submit} variant="outline" onClick={() => job.createAnother(description)}>{copy.editPrompt}</Button>
          ) : null}
        </>
      ) : null}
      {trackingJob && !hasMatchingJob ? <StateNotice inline kind="error">{t.resultFailure}</StateNotice> : null}
      {hasMatchingJob ? (
        <>
          <ImageJobTracker job={trackingJob} onError={trackError} onJobUpdate={updateStatus} onResult={receiveResult} />
          {isTerminalImageJobStatus(trackingJob.status) && trackingJob.status !== "succeeded" ? (
            <StateNotice inline kind="error">{copy.failed}</StateNotice>
          ) : null}
        </>
      ) : null}
      {completed && attachmentFailed ? (
        <StateNotice inline kind="error" action={{ label: copy.retryAttachment, onClick: () => void attach(completed) }}>
          {copy.attachmentFailed}
        </StateNotice>
      ) : null}
      {completed && !attachmentFailed ? <StateNotice inline kind="loading">{copy.attaching}</StateNotice> : null}
    </div>
  );
}
