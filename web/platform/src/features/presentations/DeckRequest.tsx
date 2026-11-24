"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import Link from "@/i18n/Link";
import { presentationDeckCopy } from "@/i18n/presentation-deck";
import { startGenerationPolling } from "@/lib/generation-polling";
import { DeckRequestError, fetchDeckQuote, readDeckJob, recoverDeckAttempt, submitDeck, type DeckQuote } from "./deck-generation";
import styles from "./Studio.module.css";

export function DeckRequest({ accountId, prompt, onResult, onBusy }: { accountId: string; prompt: string; onResult: (value: string) => void; onBusy: (busy: boolean) => void }) {
  const [quote, setQuote] = useState<DeckQuote>();
  const [revision, setRevision] = useState(0);
  const [status, setStatus] = useState("");
  const [pending, setPending] = useState(false);
  const [submitted, setSubmitted] = useState<{ conversation: string; job: string }>();
  const [paused, setPaused] = useState(false);
  const [failed, setFailed] = useState(false);
  const retry = useRef<Awaited<ReturnType<typeof recoverDeckAttempt>> | null>(null);
  const writing = useRef<AbortController | null>(null);
  const poll = useRef<ReturnType<typeof startGenerationPolling> | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    fetchDeckQuote(controller.signal).then(result => {
      if (!controller.signal.aborted) { setQuote(result); setStatus(""); }
    }).catch(() => { if (!controller.signal.aborted) setStatus(presentationDeckCopy.request.quoteUnavailable); });
    return () => controller.abort();
  }, [revision]);

  useEffect(() => () => writing.current?.abort(), []);

  useEffect(() => {
    if (!submitted) return;
    const tracker = startGenerationPolling({
      delayMs: attemptNo => Math.min(7_500, 1_000 + attemptNo * 500),
      timeoutMs: 180_000, pauseWhenHidden: true,
      poll: async signal => {
        const result = await readDeckJob(submitted.conversation, submitted.job, prompt, signal);
        if (signal.aborted) return false;
        if (result.kind === "pending") return true;
        retry.current?.clear();
        onBusy(false);
        if (result.kind === "ready") onResult(result.text);
        else { setFailed(true); setStatus(presentationDeckCopy.request.failed); }
        return false;
      },
      onExhausted: () => { setPaused(true); setStatus(presentationDeckCopy.request.exhausted); },
    });
    poll.current = tracker;
    return () => tracker.stop();
  }, [submitted, prompt, onResult, onBusy]);

  async function generate() {
    if (!quote || writing.current || submitted) return;
    const controller = new AbortController();
    writing.current = controller;
    onBusy(true);
    setPending(true);
    setStatus(presentationDeckCopy.request.submitting);
    try {
      retry.current ??= await recoverDeckAttempt(accountId, prompt, quote);
      controller.signal.throwIfAborted();
      const job = await submitDeck(retry.current.attempt, quote, prompt, controller.signal);
      if (controller.signal.aborted) return;
      setStatus(presentationDeckCopy.request.generating);
      setSubmitted(job);
    } catch (error) {
      if (controller.signal.aborted) return;
      setStatus(error instanceof DeckRequestError ? error.message : presentationDeckCopy.request.connectionLost);
      if (error instanceof DeckRequestError && error.status === 409) { setQuote(undefined); retry.current = null; }
      onBusy(false);
    } finally {
      if (!controller.signal.aborted) setPending(false);
      writing.current = null;
    }
  }

  return <section className={styles.request} aria-label={presentationDeckCopy.request.sectionLabel}>
    {quote && <p className={styles.quote}><span>{quote.model_name}</span><CreditAmount value={quote.credits} prefix={presentationDeckCopy.request.costPrefix} /></p>}
    {!submitted && (quote ? <Button className={styles.primary} onClick={generate} disabled={pending}>{pending ? presentationDeckCopy.request.submitPending : presentationDeckCopy.request.submit}</Button> :
      <Button variant="outline" onClick={() => setRevision(n => n + 1)} disabled={!status}>{status ? presentationDeckCopy.request.refreshQuote : presentationDeckCopy.request.quotePending}</Button>)}
    <p role="status" className={styles.feedback}>{status}</p>
    {submitted && <div className={styles.actions}>
      {paused && !failed && <Button variant="outline" onClick={() => { setPaused(false); setStatus(presentationDeckCopy.request.statusPending); poll.current?.refresh(); }}>{presentationDeckCopy.request.checkStatus}</Button>}
      <Link href={`/app/chat/${submitted.conversation}`}>{presentationDeckCopy.request.openHistory}</Link>
    </div>}
  </section>;
}
