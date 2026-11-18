"use client";

import { type FormEvent, useEffect, useRef, useState, useSyncExternalStore } from "react";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { useMessages } from "@/i18n/LocaleProvider";
import type { MessageKey } from "@/i18n/messages";
import { startGenerationPolling } from "@/lib/generation-polling";

import {
  audioDownloadJobId,
  AudioDownloadRequestError,
  loadAudioDownload,
  submitAudioDownload,
  type AudioDownloadJob,
} from "./audio-download-api";
import styles from "./AudioDownloadWorkspace.module.css";

const lastJobKey = "epifera.audio-download.last-job.v1";
const terminalFailures = new Set(["failed_terminal", "cancelled", "expired", "refunded", "rejected"]);
const subscribeToLastJob = () => () => undefined;
const noLastJob = () => null;

function lastJobId(): string | null {
  try {
    const parsed = audioDownloadJobId.safeParse(sessionStorage.getItem(lastJobKey));
    return parsed.success ? parsed.data : null;
  } catch {
    return null;
  }
}

function rememberJob(id: string | null) {
  try {
    if (id === null) sessionStorage.removeItem(lastJobKey);
    else sessionStorage.setItem(lastJobKey, id);
  } catch {
    // Polling still works when browser storage is unavailable.
  }
}

function submitError(cause: unknown): MessageKey {
  if (!(cause instanceof AudioDownloadRequestError)) return "audioDownload.submitError";
  switch (cause.status) {
    case 400: return "audioDownload.invalidLink";
    case 401:
    case 403: return "audioDownload.signInError";
    case 429: return "audioDownload.tooManyRequests";
    case 503: return "audioDownload.unavailable";
    default: return "audioDownload.submitError";
  }
}

function jobErrorLabel(job: AudioDownloadJob): MessageKey | null {
  if (job.status === "succeeded" && !job.artifact_id) return "audioDownload.notFound";
  if (!terminalFailures.has(job.status)) return null;
  switch (job.error_code) {
    case "download_unavailable": return "audioDownload.unavailable";
    case "download_too_large": return "audioDownload.limitExceeded";
    case "download_timeout": return "audioDownload.conversionTimeout";
    case "content_rejected": return "audioDownload.contentRejected";
    default: return "audioDownload.downloadError";
  }
}

export function AudioDownloadWorkspace() {
  const msg = useMessages();
  const { allowsPath } = useReleaseView();
  const showAudioNavigation = allowsPath("/app/music") || allowsPath("/app/speech");
  const savedJobId = useSyncExternalStore(subscribeToLastJob, lastJobId, noLastJob);
  const [url, setURL] = useState("");
  const [job, setJob] = useState<AudioDownloadJob | null>(null);
  const [busy, setBusy] = useState(false);
  const [timedOut, setTimedOut] = useState(false);
  const [error, setError] = useState<MessageKey | null>(null);
  const pending = useRef<{ url: string; key: string } | null>(null);
  const submitting = useRef(false);
  const jobFailed = job !== null && terminalFailures.has(job.status);
  const succeeded = job?.status === "succeeded";
  const activeJobId = jobFailed || succeeded ? null : job?.id ?? savedJobId;
  const readyArtifact = succeeded ? job.artifact_id : undefined;

  useEffect(() => {
    if (!activeJobId || timedOut) return;
    const polling = startGenerationPolling({
      delayMs: 2500,
      pauseWhenHidden: true,
      timeoutMs: 180_000,
      onExhausted: () => setTimedOut(true),
      poll: async signal => {
        try {
          const latest = await loadAudioDownload(activeJobId, signal);
          if (signal.aborted) return false;
          setJob(latest);
          if (jobErrorLabel(latest)) {
            rememberJob(null);
            return false;
          }
          return latest.status !== "succeeded";
        } catch (cause) {
          if (signal.aborted) return false;
          if (cause instanceof AudioDownloadRequestError && [401, 403, 404].includes(cause.status)) {
            rememberJob(null);
            setJob({ id: activeJobId, status: "expired" });
            setError(cause.status === 404 ? "audioDownload.notFound" : "audioDownload.signInError");
            return false;
          }
          return true;
        }
      },
    });
    return polling.stop;
  }, [activeJobId, timedOut]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedURL = url.trim();
    if (!trimmedURL || submitting.current || activeJobId) return;
    submitting.current = true;
    setBusy(true);
    setError(null);
    try {
      if (pending.current?.url !== trimmedURL) pending.current = { url: trimmedURL, key: crypto.randomUUID() };
      const created = await submitAudioDownload(trimmedURL, pending.current.key);
      rememberJob(jobErrorLabel(created) ? null : created.id);
      setJob(created);
      setURL("");
      pending.current = null;
    } catch (cause) {
      if (cause instanceof AudioDownloadRequestError && cause.status === 409) pending.current = null;
      setError(submitError(cause));
    } finally {
      submitting.current = false;
      setBusy(false);
    }
  };

  const visibleError = busy ? null : error ?? (job && jobErrorLabel(job));

  return (
    <WorkspacePageFrame className={showAudioNavigation ? styles.withAudioNavigation : undefined}>
      <section className={styles.workspace} aria-labelledby="audio-download-title">
        <div className={styles.heading}>
          <h1 id="audio-download-title">{msg("audioDownload.title")}</h1>
          <p id="audio-download-description">{msg("audioDownload.description")}</p>
        </div>
        <form className={styles.form} onSubmit={submit}>
          <label className={styles.field}>
            {msg("audioDownload.linkLabel")}
            <InputSurface>
              <input type="url" autoComplete="off" spellCheck={false} maxLength={2048} required
                aria-describedby="audio-download-description" value={url} disabled={busy || Boolean(activeJobId)}
                onChange={event => { setURL(event.target.value); setError(null); }} />
            </InputSurface>
          </label>
          <Button type="submit" disabled={busy || Boolean(activeJobId) || !url.trim()}>{msg("audioDownload.prepare")}</Button>
        </form>
        {visibleError ? <StateNotice kind="error" inline>{msg(visibleError)}</StateNotice> : null}
        {(busy || activeJobId) && !timedOut ? <StateNotice kind="loading" inline>{msg("audioDownload.processing")}</StateNotice> : null}
        {activeJobId && timedOut ? (
          <div className={styles.actions}>
            <StateNotice kind="info" inline>{msg("audioDownload.statusTimeout")}</StateNotice>
            <Button variant="outline" onClick={() => setTimedOut(false)}>{msg("audioDownload.checkStatus")}</Button>
          </div>
        ) : null}
        {readyArtifact && !busy ? (
          <StateNotice kind="success" inline>
            <a className={styles.download} href={`/web/v1/audio-download-artifacts/${readyArtifact}`} download>{msg("audioDownload.download")}</a>
          </StateNotice>
        ) : null}
      </section>
    </WorkspacePageFrame>
  );
}
