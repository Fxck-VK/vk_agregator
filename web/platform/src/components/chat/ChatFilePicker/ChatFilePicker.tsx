"use client";

import { SkeletonGrid, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";


import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type ChangeEvent,
} from "react";

import { Button } from "@/components/ui/Button/Button";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { FileCard, type FileResultState } from "@/features/files/FileCard/FileCard";
import {
  createImageFilePreviewQueue,
  fetchImageFileResult,
  fetchImageFilesPage,
} from "@/features/files/FilesWorkspace/files-data";
import { useOptionalWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
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
  const t = useDictionary();
  const cache = useOptionalWorkspaceDataCache();
  const [source, setSource] = useState<ChatFileSource>(initialSource);
  const [jobs, setJobs] = useState<ImageJob[]>(() => cache?.getImageFilesFirstPage()?.items ?? []);
  const [resultsByJobID, setResultsByJobID] = useState<Record<string, ImageJobResult>>(() => cache?.getImageResults() ?? {});
  const [resultStatesByJobID, setResultStatesByJobID] = useState<Record<string, FileResultState>>({});
  const [isLoading, setIsLoading] = useState(jobs.length === 0);
  const [attempt, setAttempt] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const previewQueueRef = useRef<ReturnType<typeof createImageFilePreviewQueue> | null>(null);
  const titleID = useId();
  const panelID = useId();

  const requestClose = () => {
    onClose();
  };

  useEffect(() => {
    const previous = document.activeElement;
    closeButtonRef.current?.focus();
    return () => {
      if (previous instanceof HTMLElement && previous.isConnected) previous.focus({ preventScroll: true });
    };
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
  }, [cache, source, attempt]);

  const createPreviewQueue = useCallback(() => createImageFilePreviewQueue({
      fetchResult: fetchImageFileResult,
      onFailure: (job) => setResultStatesByJobID((current) => ({ ...current, [job.id]: "error" })),
      onStart: (job) => setResultStatesByJobID((current) => ({ ...current, [job.id]: "loading" })),
      onSuccess: (job, result) => {
        cache?.setImageResult(result);
        setResultsByJobID((current) => ({ ...current, [job.id]: result }));
        setResultStatesByJobID((current) => ({ ...current, [job.id]: "idle" }));
      },
    }), [cache]);
  useEffect(() => {
    return () => {
      previewQueueRef.current?.dispose();
      previewQueueRef.current = null;
    };
  }, []);

  const requestResult = useCallback((job: ImageJob) => {
    previewQueueRef.current ??= createPreviewQueue();
    previewQueueRef.current.enqueue(job);
  }, [createPreviewQueue]);
  const selectArtifact = (job: ImageJob, artifact: ImageJobResult["artifacts"][number]) => {
    onSelect({
      id: artifact.id,
      mimeType: artifact.mime_type,
      name: job.prompt,
      previewUrl: `/web/v1/image-artifacts/${artifact.id}`,
      source: "generated",
    });
  };

  const selectFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file !== undefined) {
      onSelect(attachmentFromFile(file));
    }
    event.target.value = "";
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
    <ModalBackdrop onClose={requestClose}>
      {(requestAnimatedClose) => <section
        aria-labelledby={titleID}
        aria-modal="true"
        className={styles.dialog}
        onKeyDown={(event) => {
          if (event.key !== "Tab") return;
          const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(
            'button:not(:disabled):not([tabindex="-1"]), [tabindex="0"]',
          ));
          const first = controls[0];
          const last = controls.at(-1);
          if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last?.focus();
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first?.focus();
          }
        }}
        role="dialog"
      >
        <header className={styles.header}>
          <h2 id={titleID}>{t.conversations.mediaLibraryTitle}</h2>
          <ModalCloseButton
            aria-label={t.conversations.mediaLibraryClose}
            className={styles.close}
            onClick={requestAnimatedClose}
            ref={closeButtonRef}
          />
        </header>

        <div className={styles.tabs}>
          <ModeSwitchPanel
            activeID={source}
            ariaLabel={t.conversations.mediaLibraryTabs}
            items={([
              ["all", t.conversations.mediaLibraryAll],
              ["generated", t.conversations.mediaLibraryGenerated],
              ["uploaded", t.conversations.mediaLibraryUploaded],
            ] as const).map(([id, label]) => ({
              id, label, ariaControls: panelID, elementID: `${panelID}-${id}`,
            }))}
            onChange={selectSource}
            semantics="tabs"
          />
        </div>

        <ScrollArea
          aria-labelledby={`${panelID}-${source}`}
          className={styles.panel}
          id={panelID}
          role="tabpanel"
          trackPlacement="outside"
        >
          {showGenerated && isLoading && jobs.length === 0 ? <SkeletonGrid count={3} label={t.files.loading} /> : null}
          {showGenerated && loadFailed ? <StateNotice kind="error" action={{ label: t.files.retry, onClick: () => { setLoadFailed(false); setIsLoading(true); setAttempt(value => value + 1); } }}>{t.files.loadFailure}</StateNotice> : null}
          {showGenerated && !isLoading && !loadFailed && generatedJobs.length === 0 ? (
            <StateNotice>{t.conversations.mediaLibraryEmptyGenerated}</StateNotice>
          ) : null}
          {showGenerated && generatedJobs.length > 0 ? (
            <MasonryGrid>
              {generatedJobs.map((job) => (
                <li key={job.id}>
                  <FileCard
                    isRetrying={false}
                    job={job}
                    onRequestResult={requestResult}
                    onRetryJob={() => undefined}
                    result={resultsByJobID[job.id] ?? null}
                    resultState={resultStatesByJobID[job.id] ?? "idle"}
                    selectionAction={{ label: t.conversations.mediaLibraryChoose, onSelect: selectArtifact }}
                  />
                </li>
              ))}
            </MasonryGrid>
          ) : null}
          {source === "uploaded" ? (
            <StateNotice>{t.conversations.mediaLibraryEmptyUploaded}</StateNotice>
          ) : null}
        </ScrollArea>

        <footer className={styles.footer}>
          <Button className={styles.upload} onClick={() => inputRef.current?.click()} variant="outline">
            <svg aria-hidden="true" viewBox="0 0 24 24">
              <path d="M12 15V4m0 0L8 8m4-4 4 4M5 14v4a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.7" />
            </svg>
            {t.conversations.mediaLibraryUpload}
          </Button>
          <input
            accept={acceptedMediaTypes}
            aria-label={t.conversations.mediaLibraryUpload}
            className={styles.fileInput}
            onChange={selectFile}
            ref={inputRef}
            tabIndex={-1}
            type="file"
          />
        </footer>
      </section>}
    </ModalBackdrop>
  );

  return content;
}
