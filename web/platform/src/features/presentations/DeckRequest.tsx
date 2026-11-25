"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { clearPendingTextJob, savePendingTextJob } from "@/features/conversations/pending-text-job";
import { presentationDeckCopy } from "@/i18n/presentation-deck";
import Link from "@/i18n/Link";
import { startGenerationPolling } from "@/lib/generation-polling";
import { DeckRequestError, fetchDeckQuote, readDeckJob, recoverDeckAttempt, submitDeck, type DeckQuote } from "./deck-generation";
import { replaceDeckRun, type DeckRun } from "./deck-run";
import styles from "./Studio.module.css";

const messages = presentationDeckCopy.request;
const stages = [messages.stageQueued, messages.stageWriting, messages.stagePreparing];

type Props = {
  accountId: string;
  prompt: string;
  run?: DeckRun;
  onRun: (run: DeckRun) => void;
  onPrompt: (prompt: string) => void;
  onResult: (answer: string, prompt: string, job: string) => void;
  onBusy: (busy: boolean) => void;
  onRestart: () => void;
};

type RunState = { stage: number; message: string; outcome?: "failed" | "missing" };
const queueStatuses = new Set(["prepared", "received", "validated", "credits_reserved", "queued"]);
const resultStatuses = new Set(["provider_succeeded", "postprocessing", "result_ready", "delivering"]);

export function DeckRequest({ accountId, prompt, run, onRun, onPrompt, onResult, onBusy, onRestart }: Props) {
  const [quote, setQuote] = useState<DeckQuote>();
  const [model, setModel] = useState("");
  const [revision, setRevision] = useState(0);
  const [pollRevision, setPollRevision] = useState(0);
  const [status, setStatus] = useState("");
  const [pending, setPending] = useState(false);
  const [progress, setProgress] = useState<RunState>({ stage: 0, message: messages.locating });
  const [paused, setPaused] = useState(false);
  const retry = useRef<Awaited<ReturnType<typeof recoverDeckAttempt>> | null>(null);
  const writing = useRef<AbortController | null>(null);
  const restoredPrompt = useRef("");
  const trackedJob = useRef("");
  const runConversation = run?.conversation;
  const runJob = run?.job;
  const runPending = run?.pending;

  useEffect(() => {
    if (runConversation) return;
    const controller = new AbortController();
    fetchDeckQuote(controller.signal).then(result => {
      if (!controller.signal.aborted) { setQuote(result); setStatus(""); }
    }).catch(() => { if (!controller.signal.aborted) setStatus(messages.quoteUnavailable); });
    return () => controller.abort();
  }, [revision, runConversation]);

  useEffect(() => () => writing.current?.abort(), []);

  useEffect(() => {
    if (!runConversation) return;
    const tracker = startGenerationPolling({
      delayMs: attemptNo => Math.min(7_500, 1_000 + attemptNo * 500),
      timeoutMs: 180_000,
      pauseWhenHidden: true,
      poll: async signal => {
        try {
          const result = await readDeckJob({ conversation: runConversation, job: runJob }, signal);
          if (signal.aborted) return false;
          if (result.prompt && result.prompt !== restoredPrompt.current) {
            restoredPrompt.current = result.prompt;
            onPrompt(result.prompt);
          }
          if (result.modelName) setModel(result.modelName);
          if (runPending) onRun({ conversation: runConversation, job: result.job });
          if (result.kind === "pending") {
            const stage = queueStatuses.has(result.status) ? 0 : resultStatuses.has(result.status) ? 2 : 1;
            setProgress({ stage, message: stages[stage] });
            if (trackedJob.current !== result.job) {
              trackedJob.current = result.job;
              savePendingTextJob(runConversation, result.job, 0);
            }
            return true;
          }
          retry.current?.clear();
          clearPendingTextJob(runConversation);
          onBusy(false);
          if (result.kind === "ready") {
            onResult(result.text, result.prompt, result.job);
          } else if (result.kind === "missing") {
            setProgress({ stage: 2, message: messages.answerMissing, outcome: "missing" });
          } else {
            const error = messages.failures[result.failureCode as keyof typeof messages.failures];
            setProgress({ stage: 1, message: error ?? messages.failed, outcome: "failed" });
          }
          return false;
        } catch (error) {
          if (signal.aborted) return false;
          if (error instanceof DeckRequestError && error.status === 404) {
            if (runPending) {
              setProgress({ stage: 0, message: messages.confirming });
              return true;
            }
            setProgress({ stage: 0, message: messages.runUnavailable, outcome: "missing" });
            onBusy(false);
            return false;
          }
          setProgress(current => ({ ...current, message: messages.checkUnavailable }));
          return true;
        }
      },
      onExhausted: () => { setPaused(true); setProgress(current => ({ ...current, message: messages.exhausted })); },
    });
    return () => tracker.stop();
  }, [runConversation, runJob, runPending, pollRevision, onRun, onPrompt, onResult, onBusy]);

  async function generate() {
    if (!quote || writing.current || run) return;
    const controller = new AbortController();
    writing.current = controller;
    onBusy(true);
    setPending(true);
    setStatus(messages.submitting);
    try {
      retry.current ??= await recoverDeckAttempt(accountId, prompt, quote);
      controller.signal.throwIfAborted();
      const job = await submitDeck(retry.current.attempt, quote, prompt, controller.signal, conversation => {
        replaceDeckRun({ conversation, pending: true });
      });
      if (controller.signal.aborted) return;
      trackedJob.current = job.job;
      savePendingTextJob(job.conversation, job.job, 0);
      onRun(job);
    } catch (error) {
      if (controller.signal.aborted) return;
      const conversation = retry.current?.attempt.conversation;
      if (conversation && (!(error instanceof DeckRequestError) || error.status >= 500)) {
        onRun({ conversation, pending: true });
      } else {
        if (conversation) replaceDeckRun();
        setStatus(error instanceof DeckRequestError ? error.message : messages.connectionLost);
        if (error instanceof DeckRequestError && error.status === 409) { setQuote(undefined); retry.current = null; }
        onBusy(false);
      }
    } finally {
      if (!controller.signal.aborted) setPending(false);
      writing.current = null;
    }
  }

  if (run) return <section className={styles.request} aria-label={messages.sectionLabel}>
    {model || quote ? <p>{model || quote?.model_name}</p> : null}
    <ol className={styles.runStages} aria-label={messages.progressLabel}>
      {stages.map((stage, index) => <li key={stage} aria-current={index === progress.stage ? "step" : undefined} data-done={index < progress.stage}>
        <span aria-hidden="true" />{stage}
      </li>)}
    </ol>
    <p role="status" className={progress.outcome ? styles.error : styles.feedback}>{progress.message}</p>
    <div className={styles.actions}>
      {(paused || progress.outcome === "missing") && <Button variant="outline" onClick={() => {
        setPaused(false);
        setProgress(current => ({ stage: current.stage, message: messages.statusPending }));
        setPollRevision(value => value + 1);
      }}>{messages.checkStatus}</Button>}
      {(progress.outcome || paused && runPending) && <Button variant="outline" onClick={onRestart}>{messages.backToBrief}</Button>}
      <Link href={"/app/chat/" + run.conversation}>{messages.openHistory}</Link>
    </div>
  </section>;

  return <section className={styles.request} aria-label={messages.sectionLabel}>
    {quote && <p className={styles.quote}><span>{quote.model_name}</span><CreditAmount value={quote.credits} prefix={messages.costPrefix} /></p>}
    {quote ? <Button className={styles.primary} onClick={generate} disabled={pending}>{pending ? messages.submitPending : messages.submit}</Button> :
      <Button variant="outline" onClick={() => setRevision(n => n + 1)} disabled={!status}>{status ? messages.refreshQuote : messages.quotePending}</Button>}
    <p role="status" className={styles.feedback}>{status}</p>
  </section>;
}
