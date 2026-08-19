"use client";

/* eslint-disable @next/next/no-img-element */

import {
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type MouseEvent,
} from "react";

import {
  createImageFilePreviewQueue,
  fetchImageFileResult,
  fetchImageFilesPage,
} from "@/features/files/FilesWorkspace/files-data";
import { useOptionalWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { ru } from "@/i18n/ru";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import styles from "./ChatFilePicker.module.css";

export type ChatFileSource = "all" | "generated" | "uploaded";

export type ChatMediaAttachment = {
  file?: File;
  id: string;
  mimeType: string;
  name: string;
  previewUrl?: string;
  source: Exclude<ChatFileSource, "all">;
};

type ChatFilePickerProps = {
  initialSource: Exclude<ChatFileSource, "all">;
  onClose: () => void;
  onSelect: (attachment: ChatMediaAttachment) => void;
};

const acceptedMediaTypes = "image/*,video/*,audio/*,application/pdf";

export function attachmentFromFile(file: File): ChatMediaAttachment {
  return {
    file,
    id: `local-${crypto.randomUUID()}`,
    mimeType: file.type || "application/octet-stream",
    name: file.name,
    source: "uploaded",
  };
}

export function ChatFilePicker({ initialSource, onClose, onSelect }: Readonly<ChatFilePickerProps>) {
  const cache = useOptionalWorkspaceDataCache();
  const [source, setSource] = useState<ChatFileSource>(initialSource);
  const [jobs, setJobs] = useState<ImageJob[]>(() => cache?.getImageFilesFirstPage()?.items ?? []);
  const [resultsByJobID, setResultsByJobID] = useState<Record<string, ImageJobResult>>({});
  const [isLoading, setIsLoading] = useState(jobs.length === 0);
  const [loadFailed, setLoadFailed] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const previousBodyOverflowRef = useRef("");

  const requestClose = () => {
    document.body.style.overflow = previousBodyOverflowRef.current;
    onClose();
  };

  useEffect(() => {
    previousBodyOverflowRef.current = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        requestClose();
      }
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      document.body.style.overflow = previousBodyOverflowRef.current;
    };
    // The modal owns one close callback for its mounted lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (source === "uploaded") {
      return;
    }

    let active = true;
    void fetchImageFilesPage()
      .then((page) => {
        if (!active) {
          return;
        }
        cache?.setImageFilesFirstPage(page);
        setJobs(page.items);
      })
      .catch(() => {
        if (active) {
          setLoadFailed(true);
        }
      })
      .finally(() => {
        if (active) {
          setIsLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [cache, source]);

  useEffect(() => {
    if (source === "uploaded" || jobs.length === 0) {
      return;
    }

    const queue = createImageFilePreviewQueue({
      fetchResult: fetchImageFileResult,
      onFailure: () => undefined,
      onStart: () => undefined,
      onSuccess: (job, result) => {
        setResultsByJobID((current) => ({ ...current, [job.id]: result }));
      },
    });
    jobs.filter((job) => job.status === "succeeded").forEach((job) => queue.enqueue(job));
    return () => queue.dispose();
  }, [jobs, source]);

  const selectFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file !== undefined) {
      onSelect(attachmentFromFile(file));
    }
    event.target.value = "";
  };
  const closeFromBackdrop = (event: MouseEvent<HTMLDivElement>) => {
    if (event.currentTarget === event.target) {
      requestClose();
    }
  };
  const selectSource = (nextSource: ChatFileSource) => {
    if (nextSource !== "uploaded") {
      setIsLoading(jobs.length === 0);
      setLoadFailed(false);
    }
    setSource(nextSource);
  };

  const generatedJobs = jobs.filter((job) => job.status === "succeeded");
  const showGenerated = source === "all" || source === "generated";
  const content = (
    <div className={styles.backdrop} onMouseDown={closeFromBackdrop}>
      <section aria-labelledby="chat-file-picker-title" aria-modal="true" className={styles.dialog} role="dialog">
        <header className={styles.header}>
          <h2 id="chat-file-picker-title">{ru.conversations.mediaLibraryTitle}</h2>
          <button
            aria-label={ru.conversations.mediaLibraryClose}
            className={styles.close}
            onClick={requestClose}
            ref={closeButtonRef}
            type="button"
          >
            <svg aria-hidden="true" viewBox="0 0 24 24">
              <path d="m6 6 12 12M18 6 6 18" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
            </svg>
          </button>
        </header>

        <div aria-label={ru.conversations.mediaLibraryTabs} className={styles.tabs} role="tablist">
          {(["all", "generated", "uploaded"] as const).map((tab) => (
            <button
              aria-controls="chat-file-picker-panel"
              aria-selected={source === tab}
              className={styles.tab}
              key={tab}
              onClick={() => selectSource(tab)}
              role="tab"
              type="button"
            >
              {tab === "all"
                ? ru.conversations.mediaLibraryAll
                : tab === "generated"
                  ? ru.conversations.mediaLibraryGenerated
                  : ru.conversations.mediaLibraryUploaded}
            </button>
          ))}
        </div>

        <div className={styles.panel} id="chat-file-picker-panel" role="tabpanel">
          {showGenerated && isLoading ? <p className={styles.state} role="status">{ru.files.loading}</p> : null}
          {showGenerated && loadFailed ? <p className={styles.state} role="alert">{ru.files.loadFailure}</p> : null}
          {showGenerated && !isLoading && !loadFailed && generatedJobs.length === 0 ? (
            <p className={styles.state}>{ru.conversations.mediaLibraryEmptyGenerated}</p>
          ) : null}
          {showGenerated && generatedJobs.length > 0 ? (
            <ol className={styles.grid}>
              {generatedJobs.flatMap((job) => {
                const result = resultsByJobID[job.id];
                if (result === undefined) {
                  return [
                    <li className={styles.card} key={job.id}>
                      <div aria-hidden="true" className={styles.skeleton} />
                      <span>{job.prompt}</span>
                    </li>,
                  ];
                }
                return result.artifacts.map((artifact) => {
                  const previewUrl = `/web/v1/image-artifacts/${artifact.id}`;
                  const attachment: ChatMediaAttachment = {
                    id: artifact.id,
                    mimeType: artifact.mime_type,
                    name: job.prompt,
                    previewUrl,
                    source: "generated",
                  };
                  return (
                    <li className={styles.card} key={artifact.id}>
                      <img alt={job.prompt} src={previewUrl} />
                      <div className={styles.cardMeta}>
                        <span>{job.prompt}</span>
                        <small>{job.model_name} · {job.image_quality}</small>
                      </div>
                      <button
                        aria-label={`${ru.conversations.mediaLibraryChoose} «${job.prompt}»`}
                        className={styles.choose}
                        onClick={() => onSelect(attachment)}
                        type="button"
                      >
                        {ru.conversations.mediaLibraryChoose}
                      </button>
                    </li>
                  );
                });
              })}
            </ol>
          ) : null}
          {source === "uploaded" ? (
            <p className={styles.state}>{ru.conversations.mediaLibraryEmptyUploaded}</p>
          ) : null}
        </div>

        <footer className={styles.footer}>
          <button className={styles.upload} onClick={() => inputRef.current?.click()} type="button">
            <svg aria-hidden="true" viewBox="0 0 24 24">
              <path d="M12 15V4m0 0L8 8m4-4 4 4M5 14v4a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.7" />
            </svg>
            {ru.conversations.mediaLibraryUpload}
          </button>
          <input
            accept={acceptedMediaTypes}
            aria-label={ru.conversations.mediaLibraryUpload}
            className={styles.fileInput}
            onChange={selectFile}
            ref={inputRef}
            tabIndex={-1}
            type="file"
          />
        </footer>
      </section>
    </div>
  );

  return content;
}
