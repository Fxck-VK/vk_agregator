"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { type FileResultState } from "@/features/files/FileCard/FileCard";
import { FilesEmptyState } from "@/features/files/FilesEmptyState/FilesEmptyState";
import { FilesGrid } from "@/features/files/FilesGrid/FilesGrid";
import { FilesToolbar, type FileStatusFilter } from "@/features/files/FilesToolbar/FilesToolbar";
import { FileTypeTabs, type FileCategory } from "@/features/files/FileTypeTabs/FileTypeTabs";
import { useWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { recordWorkspaceDataLoad } from "@/features/workspace/WorkspaceNavigationMetrics/workspace-navigation-metrics";
import { ru } from "@/i18n/ru";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import { createImageFilePreviewQueue, fetchImageFilesPage } from "./files-data";
import styles from "./FilesWorkspace.module.css";

function appendDistinctImageJobs(currentJobs: ImageJob[], additionalJobs: ImageJob[]): ImageJob[] {
  const knownIDs = new Set(currentJobs.map((job) => job.id));
  return [...currentJobs, ...additionalJobs.filter((job) => !knownIDs.has(job.id))];
}

function matchesStatusFilter(job: ImageJob, filter: FileStatusFilter): boolean {
  if (filter === "all") {
    return true;
  }
  if (filter === "ready") {
    return job.status === "succeeded";
  }
  return job.status !== "succeeded";
}

function isImageCategory(category: FileCategory): category is "all" | "images" {
  return category === "all" || category === "images";
}

function futureCategoryDescription(category: Exclude<FileCategory, "all" | "images">): string {
  switch (category) {
    case "reports":
      return ru.files.emptyReportsDescription;
    case "presentations":
      return ru.files.emptyPresentationsDescription;
    case "video":
      return ru.files.emptyVideoDescription;
    case "uploads":
      return ru.files.emptyUploadsDescription;
  }
}

export function FilesWorkspace() {
  const cache = useWorkspaceDataCache();
  const [cachedFirstPage] = useState(() => cache.getImageFilesFirstPage());
  const [jobs, setJobs] = useState<ImageJob[]>(() => cachedFirstPage?.items ?? []);
  const [nextCursor, setNextCursor] = useState<string | null>(() => cachedFirstPage?.next_cursor ?? null);
  const [hasLoaded, setHasLoaded] = useState(() => cachedFirstPage !== undefined);
  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [fileCategory, setFileCategory] = useState<FileCategory>("all");
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<FileStatusFilter>("all");
  const [resultsByJobID, setResultsByJobID] = useState<Record<string, ImageJobResult>>({});
  const [resultStatesByJobID, setResultStatesByJobID] = useState<Record<string, FileResultState>>({});
  const listRequestInFlight = useRef(false);
  const hasRecordedCacheHit = useRef(false);
  const imagePreviewQueueRef = useRef<ReturnType<typeof createImageFilePreviewQueue> | null>(null);
  const createPreviewQueue = useCallback(() =>
    createImageFilePreviewQueue({
      onFailure: (job) => {
        setResultStatesByJobID((currentStates) => ({ ...currentStates, [job.id]: "error" }));
      },
      onStart: (job) => {
        setResultStatesByJobID((currentStates) => ({ ...currentStates, [job.id]: "loading" }));
      },
      onSuccess: (job, result) => {
        setResultsByJobID((currentResults) => ({ ...currentResults, [job.id]: result }));
        setResultStatesByJobID((currentStates) => ({ ...currentStates, [job.id]: "idle" }));
      },
    }),
  []);
  if (imagePreviewQueueRef.current === null) {
    imagePreviewQueueRef.current = createPreviewQueue();
  }

  const loadPage = useCallback(async (cursor?: string) => {
    if (listRequestInFlight.current) {
      return;
    }

    const isFirstPage = cursor === undefined;
    const requestStartedAt = performance.now();
    listRequestInFlight.current = true;
    if (isFirstPage) {
      setIsLoading(true);
    } else {
      setIsLoadingMore(true);
    }
    setLoadFailed(false);
    try {
      const page = await fetchImageFilesPage(cursor);
      if (isFirstPage) {
        cache.setImageFilesFirstPage(page);
      }
      setJobs((currentJobs) => (isFirstPage ? page.items : appendDistinctImageJobs(currentJobs, page.items)));
      setNextCursor(page.next_cursor);
      setHasLoaded(true);
    } catch {
      setLoadFailed(true);
    } finally {
      listRequestInFlight.current = false;
      if (isFirstPage) {
        setIsLoading(false);
        recordWorkspaceDataLoad({
          type: "data",
          target: "files",
          source: "network",
          durationMs: performance.now() - requestStartedAt,
        });
      } else {
        setIsLoadingMore(false);
      }
    }
  }, [cache]);

  useEffect(() => {
    if (cachedFirstPage === undefined || hasRecordedCacheHit.current) {
      return;
    }

    hasRecordedCacheHit.current = true;
    recordWorkspaceDataLoad({ type: "data", target: "files", source: "cache", durationMs: 0 });
  }, [cachedFirstPage]);

  useEffect(() => {
    const startupTimer = window.setTimeout(() => {
      void loadPage();
    }, 0);
    return () => window.clearTimeout(startupTimer);
  }, [loadPage]);

  useEffect(() => {
    const imagePreviewQueue = imagePreviewQueueRef.current ?? createPreviewQueue();
    imagePreviewQueueRef.current = imagePreviewQueue;

    return () => {
      imagePreviewQueue.dispose();
      if (imagePreviewQueueRef.current === imagePreviewQueue) {
        imagePreviewQueueRef.current = null;
      }
    };
  }, [createPreviewQueue]);

  const requestResult = useCallback((job: ImageJob) => {
    if (job.status !== "succeeded" || resultsByJobID[job.id] !== undefined) {
      return;
    }

    imagePreviewQueueRef.current?.enqueue(job);
  }, [resultsByJobID]);

  const visibleJobs = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return jobs.filter((job) => {
      if (!matchesStatusFilter(job, statusFilter)) {
        return false;
      }
      if (normalizedQuery === "") {
        return true;
      }
      return `${job.prompt} ${job.model_name}`.toLocaleLowerCase().includes(normalizedQuery);
    });
  }, [jobs, query, statusFilter]);

  const hasImageCategory = isImageCategory(fileCategory);
  const hasImageJobs = jobs.length > 0;
  const hasActiveImageFilters = query.trim() !== "" || statusFilter !== "all";
  const shouldShowToolbar = hasImageCategory && hasImageJobs;

  return (
    <section aria-labelledby="files-title" className={styles.workspace}>
      <header className={styles.header}>
        <h1 id="files-title">{ru.files.title}</h1>
      </header>

      <FileTypeTabs onValueChange={setFileCategory} value={fileCategory} />

      <section
        aria-labelledby={`files-tab-${fileCategory}`}
        className={styles.panel}
        id="files-panel"
        role="tabpanel"
      >
        {!hasImageCategory ? (
          <FilesEmptyState
            description={futureCategoryDescription(fileCategory)}
            title={ru.files.emptyLibraryTitle}
          />
        ) : null}

        {hasImageCategory && isLoading && !hasLoaded ? <p className={styles.state} role="status">{ru.files.loading}</p> : null}
        {hasImageCategory && loadFailed && !hasLoaded ? (
          <div className={styles.failure}>
            <p role="alert">{ru.files.loadFailure}</p>
            <Button disabled={isLoading} onClick={() => void loadPage()}>{ru.files.retry}</Button>
          </div>
        ) : null}

        {shouldShowToolbar ? (
          <>
            <FilesToolbar
              onQueryChange={setQuery}
              onStatusChange={setStatusFilter}
              query={query}
              status={statusFilter}
            />
            <p className={styles.scopeNotice}>{ru.files.loadedScopeNotice}</p>
          </>
        ) : null}

        {hasImageCategory && hasLoaded && !hasImageJobs ? (
          <FilesEmptyState description={ru.files.emptyAllDescription} title={ru.files.emptyLibraryTitle} />
        ) : null}
        {hasImageCategory && hasLoaded && hasImageJobs && visibleJobs.length === 0 && hasActiveImageFilters ? (
          <p className={styles.state} role="status">{ru.files.empty}</p>
        ) : null}
        {hasImageCategory && hasLoaded && visibleJobs.length > 0 ? (
          <FilesGrid
            jobs={visibleJobs}
            onRequestResult={requestResult}
            resultsByJobID={resultsByJobID}
            resultStatesByJobID={resultStatesByJobID}
          />
        ) : null}

        {hasImageCategory && loadFailed && hasLoaded ? <p className={styles.inlineFailure} role="alert">{ru.files.loadFailure}</p> : null}
        {hasImageCategory && nextCursor !== null ? (
          <Button disabled={isLoadingMore} onClick={() => void loadPage(nextCursor)}>
            {isLoadingMore ? ru.files.loadingMore : ru.files.loadMore}
          </Button>
        ) : null}
      </section>
    </section>
  );
}
